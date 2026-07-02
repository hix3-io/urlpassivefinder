package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic URL",
			input:    "http://example.com/path",
			expected: "http://example.com/path",
		},
		{
			name:     "HTTPS with default port",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "HTTP with default port",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path",
		},
		{
			name:     "uppercase scheme and host",
			input:    "HTTP://EXAMPLE.COM/path",
			expected: "http://example.com/path",
		},
		{
			name:     "trailing slash removal",
			input:    "http://example.com/path/",
			expected: "http://example.com/path",
		},
		{
			name:     "query parameter sorting",
			input:    "http://example.com/path?c=3&a=1&b=2",
			expected: "http://example.com/path?a=1&b=2&c=3",
		},
		{
			name:     "fragment removal",
			input:    "http://example.com/path#fragment",
			expected: "http://example.com/path",
		},
		{
			name:     "URL encoding",
			input:    "http://example.com/path%20with%20spaces",
			expected: "http://example.com/path%20with%20spaces",
		},
		{
			name:     "empty path normalization",
			input:    "http://example.com",
			expected: "http://example.com/",
		},
		{
			name:     "no scheme",
			input:    "example.com/path",
			expected: "http://example.com/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDeduplicator(t *testing.T) {
	dedup := NewDeduplicator()

	// Test empty deduplicator
	if dedup.Count() != 0 {
		t.Errorf("New deduplicator should have count 0, got %d", dedup.Count())
	}

	// Test adding URLs
	url1 := "http://example.com/path1"
	url2 := "http://example.com/path2"
	url1Dup := "http://example.com/path1"

	// Add first URL
	if dedup.IsSeen(url1) {
		t.Error("URL should not be seen before adding")
	}
	dedup.Add(url1)
	if !dedup.IsSeen(url1) {
		t.Error("URL should be seen after adding")
	}
	if dedup.Count() != 1 {
		t.Errorf("Count should be 1 after adding one URL, got %d", dedup.Count())
	}

	// Add second URL
	dedup.Add(url2)
	if dedup.Count() != 2 {
		t.Errorf("Count should be 2 after adding two URLs, got %d", dedup.Count())
	}

	// Add duplicate
	dedup.Add(url1Dup)
	if dedup.Count() != 2 {
		t.Errorf("Count should still be 2 after adding duplicate, got %d", dedup.Count())
	}
}

func TestRemoveDefaultPort(t *testing.T) {
	tests := []struct {
		host     string
		scheme   string
		expected string
	}{
		{"example.com:80", "http", "example.com"},
		{"example.com:443", "https", "example.com"},
		{"example.com:21", "ftp", "example.com"},
		{"example.com:8080", "http", "example.com:8080"},
		{"example.com", "http", "example.com"},
	}

	for _, tt := range tests {
		result := removeDefaultPort(tt.host, tt.scheme)
		if result != tt.expected {
			t.Errorf("removeDefaultPort(%q, %q) = %q, want %q", 
				tt.host, tt.scheme, result, tt.expected)
		}
	}
}

func TestSortQueryParams(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"c=3&a=1&b=2", "a=1&b=2&c=3"},
		{"single=value", "single=value"},
		{"", ""},
		{"z=26&a=1", "a=1&z=26"},
	}

	for _, tt := range tests {
		// Parse query string to url.Values
		values, err := parseQueryString(tt.input)
		if err != nil {
			t.Fatalf("Failed to parse query string %q: %v", tt.input, err)
		}
		
		result := sortQueryParams(values)
		if result != tt.expected {
			t.Errorf("sortQueryParams(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// Helper function to parse query string for testing
func parseQueryString(s string) (map[string][]string, error) {
	if s == "" {
		return make(map[string][]string), nil
	}
	
	values := make(map[string][]string)
	pairs := strings.Split(s, "&")
	for _, pair := range pairs {
		parts := strings.Split(pair, "=")
		if len(parts) == 2 {
			key, value := parts[0], parts[1]
			values[key] = append(values[key], value)
		}
	}
	return values, nil
}

// Benchmark tests
func BenchmarkNormalizeURL(b *testing.B) {
	url := "HTTP://EXAMPLE.COM:80/path?c=3&a=1&b=2#fragment"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NormalizeURL(url)
	}
}

func BenchmarkDeduplicatorAdd(b *testing.B) {
	dedup := NewDeduplicator()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dedup.Add(fmt.Sprintf("http://example.com/path%d", i))
	}
}

func BenchmarkDeduplicatorIsSeen(b *testing.B) {
	dedup := NewDeduplicator()
	
	// Pre-populate with some URLs
	for i := 0; i < 1000; i++ {
		dedup.Add(fmt.Sprintf("http://example.com/path%d", i))
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dedup.IsSeen(fmt.Sprintf("http://example.com/path%d", i%1000))
	}
}