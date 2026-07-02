package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

// URLResult holds the result of checking a URL
type URLResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Size       int64  `json:"size"`
	Error      error  `json:"-"`
}

// HostStatus holds the status of a host
type HostStatus struct {
	Host   string `json:"host"`
	Status string `json:"status"` // "alive" or "dead"
}

// DNS cache to avoid repeated lookups
var (
	dnsCache      = make(map[string]bool)
	dnsCacheMutex sync.RWMutex
)

// CheckURLs checks a list of URLs and returns results grouped by status code
func CheckURLs(urls []string, threads int, verbose bool) map[int][]URLResult {
	if verbose {
		fmt.Printf("Checking %d URLs for reachability...\n", len(urls))
	}

	// Step 1: DNS pre-check
	hostStatus := checkDNSBulk(urls, threads, verbose)

	// Filter URLs by DNS status
	checkableURLs := make([]string, 0)
	dnsFailedURLs := make([]string, 0)

	for _, urlStr := range urls {
		host := extractHost(urlStr)
		if hostStatus[host] {
			checkableURLs = append(checkableURLs, urlStr)
		} else {
			dnsFailedURLs = append(dnsFailedURLs, urlStr)
		}
	}

	if verbose {
		resolved := len(checkableURLs)
		failed := len(dnsFailedURLs)
		fmt.Printf("DNS check: %d URLs with resolved hosts, %d with failed DNS\n", resolved, failed)
	}

	// Step 2: HTTP check
	results := processURLBatch(checkableURLs, dnsFailedURLs, threads)

	// Group by status code
	grouped := make(map[int][]URLResult)
	for _, result := range results {
		grouped[result.StatusCode] = append(grouped[result.StatusCode], result)
	}

	return grouped
}

// extractHost extracts hostname from URL
func extractHost(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}

// checkDNSBulk checks DNS for all unique hosts
func checkDNSBulk(urls []string, threads int, verbose bool) map[string]bool {
	// Extract unique hosts
	hostSet := make(map[string]bool)
	for _, urlStr := range urls {
		host := extractHost(urlStr)
		if host != "" {
			hostSet[host] = false
		}
	}

	hostStatus := make(map[string]bool)
	hostMutex := &sync.Mutex{}

	sem := make(chan struct{}, threads)
	wg := &sync.WaitGroup{}

	for host := range hostSet {
		wg.Add(1)
		go func(h string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			resolvable := checkDNS(h)

			hostMutex.Lock()
			hostStatus[h] = resolvable
			hostMutex.Unlock()
		}(host)
	}

	wg.Wait()
	return hostStatus
}

// checkDNS checks if a host has DNS records
func checkDNS(host string) bool {
	// Strip port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Check cache
	dnsCacheMutex.RLock()
	if cached, exists := dnsCache[host]; exists {
		dnsCacheMutex.RUnlock()
		return cached
	}
	dnsCacheMutex.RUnlock()

	// DNS lookup
	_, err := net.LookupHost(host)
	resolvable := err == nil

	// Cache result
	dnsCacheMutex.Lock()
	dnsCache[host] = resolvable
	dnsCacheMutex.Unlock()

	return resolvable
}

// processURLBatch processes URLs with HTTP checking
func processURLBatch(checkableURLs, dnsFailedURLs []string, threads int) []URLResult {
	results := make([]URLResult, 0)
	resultsMutex := &sync.Mutex{}

	// Add DNS failed URLs
	for _, u := range dnsFailedURLs {
		results = append(results, URLResult{
			URL:        u,
			StatusCode: 0, // 0 = DNS Failed
			Size:       0,
		})
	}

	// HTTP client optimized for speed
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}

	sem := make(chan struct{}, threads)
	wg := &sync.WaitGroup{}

	// Check each URL
	for _, urlStr := range checkableURLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := checkURL(client, u)

			resultsMutex.Lock()
			results = append(results, result)
			resultsMutex.Unlock()
		}(urlStr)
	}

	wg.Wait()
	return results
}

// checkURL checks a single URL using HEAD request
func checkURL(client *http.Client, urlStr string) URLResult {
	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		return URLResult{URL: urlStr, Error: err}
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.1 (compatible; checker)")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return URLResult{URL: urlStr, Error: err}
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	if size < 0 {
		size = 0
	}

	return URLResult{
		URL:        urlStr,
		StatusCode: resp.StatusCode,
		Size:       size,
	}
}

// PrintCheckedResults prints results grouped by status code with colors
func PrintCheckedResults(grouped map[int][]URLResult, only200 bool, sortBySize bool) {
	// Status codes order: 200 first, others ascending, 0 last
	statusCodes := make([]int, 0, len(grouped))
	for code := range grouped {
		if only200 && code != 200 {
			continue
		}
		statusCodes = append(statusCodes, code)
	}

	sort.Slice(statusCodes, func(i, j int) bool {
		if statusCodes[i] == 200 { return true }
		if statusCodes[j] == 200 { return false }
		if statusCodes[i] == 0 { return false }
		if statusCodes[j] == 0 { return true }
		return statusCodes[i] < statusCodes[j]
	})

	// Sort each group
	for code := range grouped {
		sort.Slice(grouped[code], func(i, j int) bool {
			if sortBySize {
				return grouped[code][i].Size > grouped[code][j].Size
			}
			return grouped[code][i].URL < grouped[code][j].URL
		})
	}

	totalURLs := 0
	for _, code := range statusCodes {
		results := grouped[code]
		totalURLs += len(results)

		// Color codes
		var color string
		switch {
		case code == 0: color = "\x1b[90m" // Gray
		case code >= 200 && code < 300: color = "\x1b[32m" // Green  
		case code >= 300 && code < 400: color = "\x1b[33m" // Yellow
		case code >= 400 && code < 500: color = "\x1b[31m" // Red
		default: color = "\x1b[35m" // Magenta
		}

		// Print header
		if code == 0 {
			fmt.Printf("\n[%sDNS FAILED\x1b[0m] - %d URLs\n", color, len(results))
		} else {
			fmt.Printf("\n[%s%d %s\x1b[0m] - %d URLs\n", color, code, http.StatusText(code), len(results))
		}

		// Print URLs
		for _, result := range results {
			sizeStr := "-"
			if code != 0 && result.Size > 0 {
				sizeStr = formatSize(result.Size)
			}
			fmt.Printf("%s (%s)\n", result.URL, sizeStr)
		}
	}

	fmt.Printf("\nTotal checked: %d URLs\n", totalURLs)
}

// formatSize formats bytes into human-readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}