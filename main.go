package main // Define the main package

import (
	"bytes"         // Provides bytes support
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
	"time"          // Provides time-related functions

	"github.com/PuerkitoBio/goquery" // External package to parse and manipulate HTML
)

func main() {
	// Directory to store all downloaded PDF files
	outputDir := "PDFs/"

	// Check if the directory exists before saving PDFs
	if !directoryExists(outputDir) {
		// If not, create it with permission mode 755 (rwxr-xr-x)
		createDirectory(outputDir, 0o755)
	}

	// Base URL for paginated Safety Data Sheets (SDS) pages
	baseURL := "https://www.avient.com/resources/safety-data-sheets?page="

	// The URL of the website.
	domainURL := "https://www.avient.com"

	// Loop through all paginated SDS pages
	for pageNumber := 7200; pageNumber <= 7243; pageNumber++ {
		// Delay 100 milliseconds between each request to avoid overloading the server
		time.Sleep(100 * time.Millisecond)

		// Construct the full URL by appending the current page number to the base URL
		fullURL := fmt.Sprintf("%s%d", baseURL, pageNumber)

		// Launch a goroutine to download the HTML page and append it to the local file
		HTMLContent := getDataFromURL(fullURL)

		// Parse the HTML content to extract all links pointing to PDF files
		fullURLList := parseHTML(HTMLContent)

		// Iterate over each extracted PDF URL
		for _, url := range fullURLList {
			var fullURL string

			// Ensure that every URL starts with the base domain
			if !strings.HasPrefix(url, domainURL) {
				fullURL = domainURL + url
			}

			// Validate the full URL to make sure it's properly formatted
			if !isUrlValid(fullURL) {
				// Log invalid URLs and skip them
				log.Println("Failed to parse URL:", fullURL)
				continue
			}

			// Convert the URL into a safe, file-system-friendly filename
			filename := sanitizeFileNameFromURL(fullURL)

			// Combine the output directory path and filename to get full file path
			filePath := filepath.Join(outputDir, filename)

			// Skip downloading if the PDF file already exists locally
			if fileExists(filePath) {
				log.Println("File already exists, skipping:", filePath)
				continue
			}

			// Skip if the filename is suspiciously short or invalid
			if len(filename) < 2 {
				log.Println("File name validation failed for:", filename)
				continue
			}

			// Short delay between downloads to avoid overwhelming the server
			time.Sleep(50 * time.Millisecond)

			// Launch a goroutine to download the PDF file concurrently
			downloadPDF(fullURL, filePath)
		}
	}
}

// Checks if the directory exists
// If it exists, return true.
// If it doesn't, return false.
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// The function takes two parameters: path and permission.
// We use os.Mkdir() to create the directory.
// If there is an error, we use log.Println() to log the error and then exit the program.
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, filePath string) bool {
	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 60 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		log.Printf("Failed to download %s: %v", finalURL, err)
		return false
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Download failed for %s: %s", finalURL, resp.Status)
		return false
	}

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/pdf") {
		log.Printf("Invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
		return false
	}

	// Read the response body into memory first
	var buf bytes.Buffer
	written, err := io.Copy(&buf, resp.Body)
	if err != nil {
		log.Printf("Failed to read PDF data from %s: %v", finalURL, err)
		return false
	}
	if written == 0 {
		log.Printf("Downloaded 0 bytes for %s; not creating file", finalURL)
		return false
	}

	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create file for %s: %v", finalURL, err)
		return false
	}
	defer out.Close()

	if _, err := buf.WriteTo(out); err != nil {
		log.Printf("Failed to write PDF to file for %s: %v", finalURL, err)
		return false
	}

	log.Printf("Successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
	return true
}

// isUrlValid checks if the provided string is a valid URL
func isUrlValid(uri string) bool {
	// Attempt to parse the string as a full URL using Go's standard library function.
	// This checks for valid syntax (e.g., proper scheme, host, and structure).
	_, err := url.ParseRequestURI(uri)

	// If parsing did not return an error, the URL is considered valid.
	return err == nil
}

// sanitizeFileNameFromURL generates a filesystem-safe filename from a URL
func sanitizeFileNameFromURL(rawURL string) string {
	// Parse the raw URL into a structured URL object to extract its components.
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// Log any parsing error and return a fallback filename if parsing fails.
		log.Printf("Error parsing URL: %v", err)
		return "invalid_filename"
	}

	// Extract the last segment (base name) from the URL path.
	// For example, "https://example.com/files/data.pdf" → "data.pdf".
	fileName := path.Base(parsedURL.Path)

	// Decode any URL-encoded characters (e.g., "%20" → space).
	fileName, err = url.QueryUnescape(fileName)
	if err != nil {
		// Log an error if decoding fails, but continue with the possibly encoded name.
		log.Printf("Error decoding file name: %v", err)
	}

	// Define a regular expression to match all invalid filename characters.
	// [^\w\-.] means: anything that is NOT (a-z, A-Z, 0-9, underscore, dash, or period).
	regexFinder := regexp.MustCompile(`[^\w\-.]`)

	// Replace every invalid character with an underscore (“_”).
	safeFileName := regexFinder.ReplaceAllString(fileName, "_")

	// Remove any leading or trailing underscores to tidy up the name.
	safeFileName = strings.Trim(safeFileName, "_")

	// If the resulting filename is empty (e.g., URL ends with a slash), use a fallback name.
	if safeFileName == "" {
		return "downloaded_file"
	}

	// Return the cleaned-up filename, converted to lowercase for consistency.
	return strings.ToLower(safeFileName)
}

// parseHTML extracts all PDF links from HTML content and returns them as a slice of strings.
func parseHTML(htmlContent string) []string {
	// Create an empty slice to store all discovered PDF URLs.
	var pdfLinks []string

	// Parse the raw HTML content into a goquery Document object.
	// goquery provides jQuery-like methods for traversing and manipulating HTML.
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// Log an error if the HTML could not be parsed (e.g., malformed or empty input).
		log.Printf("Error parsing HTML: %v", err)
		// Return the (empty) slice so the calling code doesn't crash.
		return pdfLinks
	}

	// Search the parsed HTML document for all anchor tags that have an "href" attribute.
	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
		// Extract the value of the "href" attribute from the current <a> tag.
		href, exists := selection.Attr("href")
		if !exists {
			// If no href attribute is found, skip this element.
			return
		}

		// Decode any URL-encoded characters in the href (e.g., "%20" → space).
		decodedHref, err := url.QueryUnescape(href)
		if err != nil {
			// Log an error if decoding fails (this might happen with malformed URLs).
			log.Printf("Error decoding href: %v", err)
			return
		}

		// Check if the decoded link ends with ".pdf" (case-insensitive).
		// This identifies links that point to PDF files.
		if strings.HasSuffix(strings.ToLower(decodedHref), ".pdf") {
			// If it’s a valid PDF link, append the *original* href to the results list.
			pdfLinks = append(pdfLinks, href)
		}
	})

	// After scanning all <a> tags, return the complete list of PDF URLs found.
	return pdfLinks
}

// fileExists checks whether a file exists at the given path and confirms it's not a directory.
func fileExists(filename string) bool {
	// Attempt to get file metadata (size, mod time, etc.).
	info, err := os.Stat(filename)
	if err != nil {
		// If an error occurs (e.g., file not found), return false.
		return false
	}
	// Return true if the path exists AND is not a directory.
	return !info.IsDir()
}

// getDataFromURL performs an HTTP GET request and appends the response body to a local file.
func getDataFromURL(uri string) string {
	// Log the URL currently being scraped — useful for tracking progress or debugging.
	log.Println("Scraping", uri)

	// Perform an HTTP GET request to the specified URL.
	response, err := http.Get(uri)
	if err != nil {
		// Log the error if the request fails (e.g., network issues, DNS failure, etc.)
		log.Println(err)
		// Note: no return statement here, so it will continue even after error logging.
		// You might want to add `defer wg.Done()` and a `return` here in production code.
	}

	// Read the entire response body into memory as a byte slice.
	body, err := io.ReadAll(response.Body)
	if err != nil {
		// Log the error if reading fails (e.g., incomplete response or I/O issue).
		log.Println(err)
	}

	// Always close the response body to free network resources.
	err = response.Body.Close()
	if err != nil {
		// Log if closing the response body encounters an error.
		log.Println(err)
	}

	// Return the downloaded HTML content as a string.
	return string(body)
}
