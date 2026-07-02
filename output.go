package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Output handles writing results in various formats
type Output struct {
	mu       sync.Mutex
	writer   *bufio.Writer
	file     *os.File
	format   OutputFormat
	encoder  *json.Encoder
}

// OutputFormat represents the output format type
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
	FormatJSONL
)

// Result represents a URL discovery result
type Result struct {
	URL       string    `json:"url"`
	Source    string    `json:"source,omitempty"`
	Domain    string    `json:"domain,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// NewOutput creates a new output handler
func NewOutput(filename string, isJSON, isJSONL bool) *Output {
	var format OutputFormat
	if isJSONL {
		format = FormatJSONL
	} else if isJSON {
		format = FormatJSON
	} else {
		format = FormatText
	}

	output := &Output{
		format: format,
	}

	// Setup output destination
	if filename == "" || filename == "-" {
		output.writer = bufio.NewWriter(os.Stdout)
	} else {
		file, err := os.Create(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		output.file = file
		output.writer = bufio.NewWriter(file)
	}

	// Setup JSON encoder for JSON formats
	if format == FormatJSON || format == FormatJSONL {
		output.encoder = json.NewEncoder(output.writer)
		output.encoder.SetIndent("", "  ")
	}

	// Write JSON array start for JSON format
	if format == FormatJSON {
		output.writer.WriteString("[\n")
		output.writer.Flush()
	}

	return output
}

// Write outputs a URL in the configured format
func (o *Output) Write(url string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	switch o.format {
	case FormatText:
		o.writeText(url)
	case FormatJSON:
		o.writeJSON(url)
	case FormatJSONL:
		o.writeJSONL(url)
	}
}

// writeText writes URL in plain text format
func (o *Output) writeText(url string) {
	o.writer.WriteString(url)
	o.writer.WriteString("\n")
	o.writer.Flush()
}

// writeJSON writes URL in JSON format (part of array)
func (o *Output) writeJSON(url string) {
	result := Result{
		URL:       url,
		Timestamp: time.Now(),
	}
	
	// Add comma if not first entry (simplified approach)
	o.writer.WriteString("  ")
	o.encoder.Encode(result)
	o.writer.WriteString(",\n")
	o.writer.Flush()
}

// writeJSONL writes URL in JSON Lines format
func (o *Output) writeJSONL(url string) {
	result := Result{
		URL:       url,
		Timestamp: time.Now(),
	}
	
	o.encoder.Encode(result)
	o.writer.Flush()
}

// WriteWithSource outputs a URL with source information
func (o *Output) WriteWithSource(url, source, domain string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	switch o.format {
	case FormatText:
		o.writeText(url)
	case FormatJSON:
		o.writeJSONWithSource(url, source, domain)
	case FormatJSONL:
		o.writeJSONLWithSource(url, source, domain)
	}
}

// writeJSONWithSource writes URL with source in JSON format
func (o *Output) writeJSONWithSource(url, source, domain string) {
	result := Result{
		URL:       url,
		Source:    source,
		Domain:    domain,
		Timestamp: time.Now(),
	}
	
	o.writer.WriteString("  ")
	o.encoder.Encode(result)
	o.writer.WriteString(",\n")
	o.writer.Flush()
}

// writeJSONLWithSource writes URL with source in JSON Lines format
func (o *Output) writeJSONLWithSource(url, source, domain string) {
	result := Result{
		URL:       url,
		Source:    source,
		Domain:    domain,
		Timestamp: time.Now(),
	}
	
	o.encoder.Encode(result)
	o.writer.Flush()
}

// Close properly closes the output and finalizes formats
func (o *Output) Close() {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Finalize JSON format
	if o.format == FormatJSON {
		// Remove last comma and close array (simplified)
		o.writer.WriteString("]\n")
	}

	// Flush and close
	if o.writer != nil {
		o.writer.Flush()
	}

	if o.file != nil {
		o.file.Close()
	}
}

// Stats represents output statistics
type Stats struct {
	TotalURLs    int           `json:"total_urls"`
	UniqueURLs   int           `json:"unique_urls"`
	FilteredURLs int           `json:"filtered_urls"`
	Sources      map[string]int `json:"sources"`
	Duration     time.Duration `json:"duration"`
}

// WriteStats outputs statistics in the configured format
func (o *Output) WriteStats(stats Stats) {
	if o.format == FormatText {
		fmt.Fprintf(os.Stderr, "\n[+] Statistics:\n")
		fmt.Fprintf(os.Stderr, "    Total URLs processed: %d\n", stats.TotalURLs)
		fmt.Fprintf(os.Stderr, "    Unique URLs found: %d\n", stats.UniqueURLs)
		fmt.Fprintf(os.Stderr, "    URLs filtered: %d\n", stats.FilteredURLs)
		fmt.Fprintf(os.Stderr, "    Duration: %v\n", stats.Duration)
		
		if len(stats.Sources) > 0 {
			fmt.Fprintf(os.Stderr, "    Sources breakdown:\n")
			for source, count := range stats.Sources {
				fmt.Fprintf(os.Stderr, "      %s: %d\n", source, count)
			}
		}
	} else {
		// For JSON formats, write stats as separate object to stderr
		encoder := json.NewEncoder(os.Stderr)
		encoder.SetIndent("", "  ")
		encoder.Encode(stats)
	}
}