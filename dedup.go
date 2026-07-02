package main

import (
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Deduplicator handles URL deduplication
type Deduplicator struct {
	mu   sync.RWMutex
	seen map[string]struct{}
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		seen: make(map[string]struct{}),
	}
}

// IsSeen checks if a URL has been seen before
func (d *Deduplicator) IsSeen(url string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.seen[url]
	return exists
}

// Add marks a URL as seen
func (d *Deduplicator) Add(url string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[url] = struct{}{}
}

// Count returns the number of unique URLs seen
func (d *Deduplicator) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.seen)
}

// NormalizeURL normalizes a URL for consistent deduplication
func NormalizeURL(rawURL string) string {
	// Handle empty or invalid URLs
	if rawURL == "" {
		return ""
	}

	// Decode URL encoding
	decoded, err := url.QueryUnescape(rawURL)
	if err != nil {
		decoded = rawURL
	}

	// Parse the URL
	u, err := url.Parse(decoded)
	if err != nil {
		// If parsing fails, try adding scheme
		if !strings.Contains(decoded, "://") {
			decoded = "http://" + decoded
			u, err = url.Parse(decoded)
			if err != nil {
				return rawURL // Return original if all parsing fails
			}
		} else {
			return rawURL
		}
	}

	// Normalize scheme
	if u.Scheme == "" {
		u.Scheme = "http"
	} else {
		u.Scheme = strings.ToLower(u.Scheme)
	}

	// Normalize host
	u.Host = strings.ToLower(u.Host)
	
	// Remove default ports
	u.Host = removeDefaultPort(u.Host, u.Scheme)

	// Normalize path
	if u.Path == "" {
		u.Path = "/"
	} else {
		// Remove trailing slash except for root
		if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
			u.Path = strings.TrimRight(u.Path, "/")
		}
	}

	// Sort query parameters
	if u.RawQuery != "" {
		u.RawQuery = sortQueryParams(u.Query())
	}

	// Remove fragment
	u.Fragment = ""

	return u.String()
}

// removeDefaultPort removes default ports from the host
func removeDefaultPort(host, scheme string) string {
	switch scheme {
	case "http":
		return strings.TrimSuffix(host, ":80")
	case "https":
		return strings.TrimSuffix(host, ":443")
	case "ftp":
		return strings.TrimSuffix(host, ":21")
	}
	return host
}

// sortQueryParams sorts query parameters alphabetically
func sortQueryParams(params url.Values) string {
	if len(params) == 0 {
		return ""
	}

	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		for _, v := range params[k] {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(parts, "&")
}