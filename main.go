package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func main() {
	// Define the base URL for Safety Data Sheets
	baseURL := "https://www.avient.com/resources/safety-data-sheets?page="
	// The location to save the data.
	localLocation := "avient.com.html"
	// Loop from page 0 to page give number
	for pageNumber := 0; pageNumber <= 1000; pageNumber++ {
		// Construct the full URL with the current page number
		fullURL := fmt.Sprintf("%s%d", baseURL, pageNumber)
		// Send a GET request to the URL
		urlData := getDataFromURL(fullURL)
		appendAndWriteToFile(localLocation, urlData)
	}
	// Check if the file exists.
	if fileExists(localLocation) {
		// Read the local data
		localDiskHTMLContent := readAFileAsString(localLocation)
		// Extract the .pdf urls only.
		fullURLList := parseHTML(localDiskHTMLContent)
		// Remove duplicates from slice.
		fullURLList = removeDuplicatesFromSlice(fullURLList)
		// Directory to save PDFs
		outputDir := "PDFs/"
		// WaitGroup to manage concurrent downloads
		var downloadWaitGroup sync.WaitGroup
		// Loop over the list.
		for _, url := range fullURLList {
			fullURL := "https://www.avient.com" + url
			if !isUrlValid(fullURL) {
				log.Println("Invalid URL", fullURL)
				return
			}
			downloadWaitGroup.Add(1) // Increment WaitGroup counter
			go downloadPDF(fullURL, outputDir, &downloadWaitGroup)
		}
		downloadWaitGroup.Wait() // Wait for all downloads to finish
	}
}

// Downloads a PDF from a given URL into the specified directory; returns true if a new file was saved
func downloadPDF(finalURL, outputDir string, wg *sync.WaitGroup) bool {
	defer wg.Done() // Decrement WaitGroup counter when function exits

	if err := os.MkdirAll(outputDir, 0o755); err != nil { // Ensure output directory exists
		log.Printf("Failed to create directory %s: %v", outputDir, err) // Log directory creation error
		return false                                                    // Abort if directory cannot be created
	}

	filename := sanitizeFileNameFromURL(finalURL) // Generate file name from URL
	if filename == "" {
		filename = path.Base(finalURL) // Fallback to URL path base name
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") { // Ensure .pdf extension
		filename += ".pdf"
	}
	filePath := filepath.Join(outputDir, filename) // Create full output path

	if fileExists(filePath) { // Skip if file already exists
		log.Printf("File already exists, skipping: %s", filePath)
		return false
	}

	client := &http.Client{Timeout: 10 * time.Minute} // HTTP client with timeout

	resp, err := client.Get(finalURL) // Make GET request
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err)
		return false
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK { // Check for 200 OK
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	out, err := os.Create(filePath) // Create file to save content
	if err != nil {
		log.Printf("Failed to create file %s %s %v", finalURL, filePath, err)
		return false
	}
	defer out.Close() // Ensure file is closed after writing

	if _, err := io.Copy(out, resp.Body); err != nil { // Write response to file
		log.Printf("Failed to save PDF to %s %s %v", finalURL, filePath, err)
		return false
	}

	log.Printf("Downloaded %s → %s", finalURL, filePath) // Log success
	return true
}

// Removes duplicate strings from a slice and returns a new slice with unique values
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Map to track already seen strings
	var newReturnSlice []string     // Result slice for unique values
	for _, content := range slice { // Iterate through input slice
		if !check[content] { // If string not already seen
			check[content] = true                            // Mark string as seen
			newReturnSlice = append(newReturnSlice, content) // Add to result
		}
	}
	return newReturnSlice // Return deduplicated slice
}

// Check if the given url is valid.
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri)
	return err == nil
}

// sanitizeFileNameFromURL converts a URL into a safe file name
func sanitizeFileNameFromURL(rawURL string) string {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("Error parsing URL: %v", err)
		return "invalid_filename"
	}

	// Extract just the file name part of the path
	fileName := path.Base(parsedURL.Path)

	// Decode URL-encoded characters (e.g., %20 => space)
	fileName, err = url.QueryUnescape(fileName)
	if err != nil {
		log.Printf("Error decoding file name: %v", err)
	}

	// Replace all unsafe characters with underscores
	re := regexp.MustCompile(`[^\w\-.]`)
	safeFileName := re.ReplaceAllString(fileName, "_")

	// Trim any leading or trailing underscores
	safeFileName = strings.Trim(safeFileName, "_")

	if safeFileName == "" {
		return "downloaded_file"
	}

	return strings.ToLower(safeFileName)
}

// parseHTML parses the given HTML and returns a slice of .pdf link strings.
func parseHTML(htmlContent string) []string {
	var pdfLinks []string

	// Parse the HTML using goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		log.Printf("Error parsing HTML: %v", err)
		return pdfLinks
	}

	// Loop through all <a> tags with href attributes
	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
		href, exists := selection.Attr("href")
		if !exists {
			return
		}

		// Decode URL-encoded characters (e.g., %20)
		decodedHref, err := url.QueryUnescape(href)
		if err != nil {
			log.Printf("Error decoding href: %v", err)
			return
		}

		// Check if the decoded href ends with .pdf (case-insensitive)
		if strings.HasSuffix(strings.ToLower(decodedHref), ".pdf") {
			pdfLinks = append(pdfLinks, href) // Add original (encoded) link
		}
	})

	return pdfLinks
}

// Append and write to file
func appendAndWriteToFile(path string, content string) {
	filePath, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = filePath.WriteString(content + "\n")
	if err != nil {
		log.Fatalln(err)
	}
	err = filePath.Close()
	if err != nil {
		log.Fatalln(err)
	}
}

/*
It checks if the file exists
If the file exists, it returns true
If the file does not exist, it returns false
*/
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}
	return string(content)
}

// Send a http get request to a given url and return the data from that url.
func getDataFromURL(uri string) string {
	log.Println("Scraping", uri)
	response, err := http.Get(uri)
	if err != nil {
		log.Fatalln(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatalln(err)
	}
	err = response.Body.Close()
	if err != nil {
		log.Fatalln(err)
	}
	return string(body)
}
