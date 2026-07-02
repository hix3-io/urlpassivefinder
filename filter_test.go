package main

import (
	"testing"
)

func TestFilter_ShouldFilter(t *testing.T) {
	filter := NewFilter([]string{"jpg", "css", "js"}, false)

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "normal URL should not be filtered",
			url:      "http://example.com/api/users",
			expected: false,
		},
		{
			name:     "image extension should be filtered",
			url:      "http://example.com/image.jpg",
			expected: true,
		},
		{
			name:     "CSS file should be filtered",
			url:      "http://example.com/styles.css",
			expected: true,
		},
		{
			name:     "JavaScript file should be filtered",
			url:      "http://example.com/script.js",
			expected: true,
		},
		{
			name:     "static path should be filtered",
			url:      "http://example.com/static/image.png",
			expected: true,
		},
		{
			name:     "assets path should be filtered",
			url:      "http://example.com/assets/font.woff",
			expected: true,
		},
		{
			name:     "CDN domain should be filtered",
			url:      "http://cdn.example.com/file.png",
			expected: true,
		},
		{
			name:     "CloudFront CDN should be filtered",
			url:      "http://d123.cloudfront.net/image.jpg",
			expected: true,
		},
		{
			name:     "invalid URL should be filtered",
			url:      "not-a-valid-url",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.ShouldFilter(tt.url)
			if result != tt.expected {
				t.Errorf("ShouldFilter(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestFilter_IsInScope(t *testing.T) {
	filter := NewFilter([]string{}, false)

	tests := []struct {
		name             string
		url              string
		domain           string
		includeSubdomains bool
		expected         bool
	}{
		{
			name:             "exact domain match",
			url:              "http://example.com/path",
			domain:           "example.com",
			includeSubdomains: false,
			expected:         true,
		},
		{
			name:             "subdomain with inclusion",
			url:              "http://api.example.com/path",
			domain:           "example.com",
			includeSubdomains: true,
			expected:         true,
		},
		{
			name:             "subdomain without inclusion",
			url:              "http://api.example.com/path",
			domain:           "example.com",
			includeSubdomains: false,
			expected:         false,
		},
		{
			name:             "different domain",
			url:              "http://other.com/path",
			domain:           "example.com",
			includeSubdomains: true,
			expected:         false,
		},
		{
			name:             "URL with port",
			url:              "http://example.com:8080/path",
			domain:           "example.com",
			includeSubdomains: false,
			expected:         true,
		},
		{
			name:             "case insensitive",
			url:              "http://EXAMPLE.COM/path",
			domain:           "example.com",
			includeSubdomains: false,
			expected:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.IsInScope(tt.url, tt.domain, tt.includeSubdomains)
			if result != tt.expected {
				t.Errorf("IsInScope(%q, %q, %v) = %v, want %v", 
					tt.url, tt.domain, tt.includeSubdomains, result, tt.expected)
			}
		})
	}
}

func TestFilter_HasBlacklistedExtension(t *testing.T) {
	filter := NewFilter([]string{"jpg", "png", "css"}, false)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "JPG extension",
			path:     "/path/image.jpg",
			expected: true,
		},
		{
			name:     "uppercase extension",
			path:     "/path/IMAGE.JPG",
			expected: true,
		},
		{
			name:     "no extension",
			path:     "/path/file",
			expected: false,
		},
		{
			name:     "allowed extension",
			path:     "/path/document.pdf",
			expected: false,
		},
		{
			name:     "path with query params",
			path:     "/path/file.js?v=1",
			expected: false, // Extension checking only looks at path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.hasBlacklistedExtension(tt.path)
			if result != tt.expected {
				t.Errorf("hasBlacklistedExtension(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilter_IsStaticFile(t *testing.T) {
	filter := NewFilter([]string{}, false)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "static path",
			path:     "/static/css/style.css",
			expected: true,
		},
		{
			name:     "assets path",
			path:     "/assets/js/app.js",
			expected: true,
		},
		{
			name:     "webpack path",
			path:     "/webpack/bundle.js",
			expected: true,
		},
		{
			name:     "nuxt path",
			path:     "/_nuxt/app.js",
			expected: true,
		},
		{
			name:     "normal API path",
			path:     "/api/users",
			expected: false,
		},
		{
			name:     "root path",
			path:     "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.isStaticFile(tt.path)
			if result != tt.expected {
				t.Errorf("isStaticFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilter_IsCDNURL(t *testing.T) {
	filter := NewFilter([]string{}, false)

	tests := []struct {
		name     string
		host     string
		path     string
		expected bool
	}{
		{
			name:     "cdn subdomain",
			host:     "cdn.example.com",
			path:     "/file.js",
			expected: true,
		},
		{
			name:     "CloudFront",
			host:     "d123.cloudfront.net",
			path:     "/image.jpg",
			expected: true,
		},
		{
			name:     "AWS S3",
			host:     "bucket.amazonaws.com",
			path:     "/file.pdf",
			expected: true,
		},
		{
			name:     "jsdelivr CDN",
			host:     "cdn.jsdelivr.net",
			path:     "/npm/package/file.js",
			expected: true,
		},
		{
			name:     "CDN path pattern",
			host:     "example.com",
			path:     "/cdn-cgi/scripts/script.js",
			expected: true,
		},
		{
			name:     "regular domain",
			host:     "example.com",
			path:     "/api/data",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.isCDNURL(tt.host, tt.path)
			if result != tt.expected {
				t.Errorf("isCDNURL(%q, %q) = %v, want %v", tt.host, tt.path, result, tt.expected)
			}
		})
	}
}

func TestFilter_AddRemoveExtension(t *testing.T) {
	filter := NewFilter([]string{"jpg"}, false)

	// Test initial state
	if !filter.hasBlacklistedExtension("/image.jpg") {
		t.Error("JPG should be blacklisted initially")
	}
	if filter.hasBlacklistedExtension("/image.png") {
		t.Error("PNG should not be blacklisted initially")
	}

	// Add PNG
	filter.AddExtension("png")
	if !filter.hasBlacklistedExtension("/image.png") {
		t.Error("PNG should be blacklisted after adding")
	}

	// Remove JPG
	filter.RemoveExtension("jpg")
	if filter.hasBlacklistedExtension("/image.jpg") {
		t.Error("JPG should not be blacklisted after removing")
	}

	// Test adding without dot
	filter.AddExtension("gif")
	if !filter.hasBlacklistedExtension("/image.gif") {
		t.Error("GIF should be blacklisted after adding without dot")
	}
}

// Benchmark tests
func BenchmarkFilter_ShouldFilter(b *testing.B) {
	filter := NewFilter([]string{"jpg", "css", "js", "png", "gif"}, false)
	url := "http://example.com/api/users/profile"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.ShouldFilter(url)
	}
}

func BenchmarkFilter_IsInScope(b *testing.B) {
	filter := NewFilter([]string{}, false)
	url := "http://api.example.com/users"
	domain := "example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filter.IsInScope(url, domain, true)
	}
}