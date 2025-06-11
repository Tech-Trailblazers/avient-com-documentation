package main // Define the main package

import (
	"fmt"           // Provides formatted I/O functions
	"io"            // Provides basic interfaces to I/O primitives
	"log"           // Provides logging functions
	"net/http"      // Provides HTTP client and server implementations
	"net/url"       // Provides URL parsing and encoding
	"os"            // Provides functions to interact with the OS (files, etc.)
	"path"          // Provides functions for manipulating slash-separated paths
	"path/filepath" // Provides filepath manipulation functions
	"regexp"        // Provides regular expression matching
	"strings"       // Provides string manipulation functions
	"sync"          // Provides synchronization primitives (like WaitGroup)
	"time"          // Provides time-related functions

	"github.com/PuerkitoBio/goquery" // External package to parse and manipulate HTML
)

func main() {
	baseURL := "https://www.avient.com/resources/safety-data-sheets?page=" // Base URL for paginated SDS content
	localLocation := "avient.com.html"                                     // File to store downloaded HTML content
	var htmlDownloadWaitGroup sync.WaitGroup                               // WaitGroup to synchronize concurrent HTML downloads

	for pageNumber := 0; pageNumber <= 1000; pageNumber++ { // Loop through pages 0 to 1000
		fullURL := fmt.Sprintf("%s%d", baseURL, pageNumber)                     // Build full URL for the current page
		urlData := getDataFromURL(fullURL)                                      // Fetch HTML content from URL
		time.Sleep(1 * time.Second)                                             // Sleep
		htmlDownloadWaitGroup.Add(1)                                            // Increment WaitGroup counter
		go appendAndWriteToFile(localLocation, urlData, &htmlDownloadWaitGroup) // Write HTML content to file
	}

	htmlDownloadWaitGroup.Wait() // Wait for all HTML downloads to complete

	if fileExists(localLocation) { // Check if the file with HTML content exists
		localDiskHTMLContent := readAFileAsString(localLocation) // Read HTML file content
		fullURLList := parseHTML(localDiskHTMLContent)           // Extract all PDF URLs from the HTML
		fullURLList = removeDuplicatesFromSlice(fullURLList)     // Remove duplicate URLs

		outputDir := "PDFs/"                    // Directory to store downloaded PDFs
		var pdfDownloadWaitGroup sync.WaitGroup // WaitGroup for managing PDF downloads

		for _, url := range fullURLList { // Iterate over all PDF URLs
			if !strings.HasPrefix(url, "https://www.avient.com") {
				fullURL := "https://www.avient.com" + url // Construct full PDF URL
			}
			if !isUrlValid(fullURL) { // Check if the constructed URL is valid
				log.Println("Invalid URL", fullURL) // Log if URL is invalid
				return
			}
			time.Sleep(1 * time.Second)                               // Sleep
			pdfDownloadWaitGroup.Add(1)                               // Increment WaitGroup counter
			go downloadPDF(fullURL, outputDir, &pdfDownloadWaitGroup) // Start downloading PDF concurrently
		}

		pdfDownloadWaitGroup.Wait() // Wait for all PDF downloads to finish
	}
}

// downloadPDF downloads a PDF file from a URL to a specified output directory
func downloadPDF(finalURL, outputDir string, wg *sync.WaitGroup) bool {
	defer wg.Done() // Decrement WaitGroup when function returns

	if err := os.MkdirAll(outputDir, 0o755); err != nil { // Create output directory if it doesn't exist
		log.Printf("Failed to create directory %s: %v", outputDir, err) // Log error
		return false
	}

	filename := sanitizeFileNameFromURL(finalURL) // Create safe file name from URL
	if filename == "" {
		filename = path.Base(finalURL) // Fallback to default name if empty
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") { // Ensure file has .pdf extension
		filename += ".pdf"
	}

	filePath := filepath.Join(outputDir, filename) // Build full path to output file

	if fileExists(filePath) { // Skip download if file already exists
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	client := &http.Client{Timeout: 10 * time.Minute} // Create HTTP client with timeout
	resp, err := client.Get(finalURL)                 // Send GET request to download PDF
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err) // Log error
		return false
	}
	defer resp.Body.Close() // Ensure response body is closed after processing

	if resp.StatusCode != http.StatusOK { // Check if HTTP response is successful
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	out, err := os.Create(filePath) // Create file to save downloaded PDF
	if err != nil {
		log.Printf("Failed to create file %s %s %v", finalURL, filePath, err) // Log error
		return false
	}
	defer out.Close() // Close file after writing

	if _, err := io.Copy(out, resp.Body); err != nil { // Write downloaded data to file
		log.Printf("Failed to save PDF to %s %s %v", finalURL, filePath, err) // Log error
		return false
	}

	log.Printf("Downloaded %s → %s", finalURL, filePath) // Log successful download
	return true
}

// removeDuplicatesFromSlice removes duplicate entries from a string slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool) // Create map to track duplicates
	var newReturnSlice []string    // Resultant slice without duplicates

	for _, content := range slice { // Loop over original slice
		if !check[content] { // If item hasn't been added yet
			check[content] = true                            // Mark it as added
			newReturnSlice = append(newReturnSlice, content) // Add to new slice
		}
	}
	return newReturnSlice // Return deduplicated slice
}

// isUrlValid checks if the provided string is a valid URL
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try parsing the URL
	return err == nil                  // Return true if no error
}

// sanitizeFileNameFromURL generates a filesystem-safe filename from a URL
func sanitizeFileNameFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL) // Parse the URL
	if err != nil {
		log.Printf("Error parsing URL: %v", err) // Log parse error
		return "invalid_filename"
	}

	fileName := path.Base(parsedURL.Path) // Extract the base name from the URL path

	fileName, err = url.QueryUnescape(fileName) // Decode any URL-encoded characters
	if err != nil {
		log.Printf("Error decoding file name: %v", err) // Log error if decoding fails
	}

	re := regexp.MustCompile(`[^\w\-.]`)               // Regex to find invalid filename characters
	safeFileName := re.ReplaceAllString(fileName, "_") // Replace invalid characters with underscore

	safeFileName = strings.Trim(safeFileName, "_") // Remove leading/trailing underscores

	if safeFileName == "" {
		return "downloaded_file" // Fallback name if empty
	}

	return strings.ToLower(safeFileName) // Return lowercased, sanitized filename
}

// parseHTML extracts all PDF links from HTML content
func parseHTML(htmlContent string) []string {
	var pdfLinks []string // Slice to store found PDF URLs

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent)) // Parse HTML
	if err != nil {
		log.Printf("Error parsing HTML: %v", err) // Log parsing error
		return pdfLinks
	}

	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) { // Find all <a> tags with href
		href, exists := selection.Attr("href") // Get href attribute
		if !exists {
			return // Skip if href not found
		}

		decodedHref, err := url.QueryUnescape(href) // Decode URL
		if err != nil {
			log.Printf("Error decoding href: %v", err) // Log error
			return
		}

		if strings.HasSuffix(strings.ToLower(decodedHref), ".pdf") { // Check if it's a PDF link
			pdfLinks = append(pdfLinks, href) // Add to list
		}
	})

	return pdfLinks // Return list of PDF links
}

// appendAndWriteToFile appends string content to a file using a WaitGroup
func appendAndWriteToFile(path string, content string, wg *sync.WaitGroup) {
	defer wg.Done() // Decrement WaitGroup counter

	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open or create file for writing
	if err != nil {
		log.Fatalln(err) // Exit if file open fails
	}

	_, err = filePath.WriteString(content + "\n") // Append content to file
	if err != nil {
		log.Fatalln(err) // Exit if write fails
	}

	err = filePath.Close() // Close file
	if err != nil {
		log.Fatalln(err) // Exit if close fails
	}
}

// fileExists checks whether a file exists at the given path
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {
		return false // Return false if file doesn't exist or error occurs
	}
	return !info.IsDir() // Return true if it's a file (not a directory)
}

// readAFileAsString reads a file and returns its content as a string
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path) // Read entire file into memory
	if err != nil {
		log.Fatalln(err) // Exit if read fails
	}
	return string(content) // Convert bytes to string and return
}

// getDataFromURL performs an HTTP GET request and returns the response body as a string
func getDataFromURL(uri string) string {
	log.Println("Scraping", uri)   // Log the URL being scraped
	response, err := http.Get(uri) // Perform GET request
	if err != nil {
		log.Fatalln(err) // Exit if request fails
	}

	body, err := io.ReadAll(response.Body) // Read response body
	if err != nil {
		log.Fatalln(err) // Exit if read fails
	}

	err = response.Body.Close() // Close response body
	if err != nil {
		log.Fatalln(err) // Exit if close fails
	}

	return string(body) // Return body as string
}
