package main

import (
	"net/url"
	"path"
	"strings"
)

// Filter handles URL filtering based on various criteria
type Filter struct {
	extensionBlacklist map[string]bool
	mimeBlacklist      map[string]bool
	disableAdvanced    bool
}

// NewFilter creates a new filter with default blacklists
func NewFilter(extensions []string, disableAdvanced bool) *Filter {
	f := &Filter{
		extensionBlacklist: make(map[string]bool),
		mimeBlacklist:      make(map[string]bool),
		disableAdvanced:    disableAdvanced,
	}

	// Add provided extensions to blacklist
	for _, ext := range extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		f.extensionBlacklist[strings.ToLower(ext)] = true
	}

	// Default MIME type blacklist
	defaultMimes := []string{
		"image/",
		"video/",
		"audio/",
		"font/",
		"application/octet-stream",
		"application/pdf",
		"application/zip",
		"text/css",
	}

	for _, mime := range defaultMimes {
		f.mimeBlacklist[strings.ToLower(mime)] = true
	}

	return f
}

// ShouldFilter determines if a URL should be filtered out
func (f *Filter) ShouldFilter(rawURL string) bool {
	// Basic URL validation
	if !strings.Contains(rawURL, "://") {
		return true // Filter invalid URLs
	}
	
	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return true // Filter invalid URLs
	}

	// Check extension
	if f.hasBlacklistedExtension(u.Path) {
		return true
	}

	// Check for known static file patterns
	if f.isStaticFile(u.Path) {
		return true
	}

	// Check for CDN/cache patterns
	if f.isCDNURL(u.Host, u.Path) {
		return true
	}

	// Advanced filters from urlfinder-custom (can be disabled)
	if !f.disableAdvanced {
		if f.isWordPressJunk(rawURL) {
			return true
		}

		if f.isBlogURL(rawURL) {
			return true
		}

		if f.isGenericJSLibrary(u.Path) {
			return true
		}
	}

	return false
}

// hasBlacklistedExtension checks if the URL path has a blacklisted extension
func (f *Filter) hasBlacklistedExtension(urlPath string) bool {
	ext := strings.ToLower(path.Ext(urlPath))
	return f.extensionBlacklist[ext]
}

// isStaticFile checks for patterns that indicate static files
func (f *Filter) isStaticFile(urlPath string) bool {
	staticPatterns := []string{
		"/static/",
		"/assets/",
		"/css/",
		"/js/",
		"/images/",
		"/img/",
		"/fonts/",
		"/media/",
		"/uploads/",
		"/_nuxt/",
		"/webpack/",
	}

	lowerPath := strings.ToLower(urlPath)
	for _, pattern := range staticPatterns {
		if strings.Contains(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// isCDNURL checks if the URL appears to be from a CDN
func (f *Filter) isCDNURL(host, urlPath string) bool {
	host = strings.ToLower(host)
	
	// Known CDN domains
	cdnDomains := []string{
		"cdn.",
		"assets.",
		"static.",
		"media.",
		"img.",
		"images.",
		"css.",
		"js.",
		".amazonaws.com",
		".cloudfront.net",
		".azureedge.net",
		".fastly.com",
		".jsdelivr.net",
		".unpkg.com",
		".cdnjs.cloudflare.com",
	}

	for _, pattern := range cdnDomains {
		if strings.Contains(host, pattern) {
			return true
		}
	}

	// CDN path patterns
	cdnPaths := []string{
		"/cdn-cgi/",
		"/assets/",
		"/static/",
		"/__webpack_hmr",
		"/_next/static/",
	}

	lowerPath := strings.ToLower(urlPath)
	for _, pattern := range cdnPaths {
		if strings.HasPrefix(lowerPath, pattern) {
			return true
		}
	}

	return false
}

// AddExtension adds an extension to the blacklist
func (f *Filter) AddExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	f.extensionBlacklist[strings.ToLower(ext)] = true
}

// RemoveExtension removes an extension from the blacklist
func (f *Filter) RemoveExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	delete(f.extensionBlacklist, strings.ToLower(ext))
}

// IsInScope checks if URL is within the specified scope
func (f *Filter) IsInScope(rawURL, domain string, includeSubdomains bool) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Host)
	domain = strings.ToLower(domain)

	// Remove port if present
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Exact match
	if host == domain {
		return true
	}

	// Subdomain match
	if includeSubdomains {
		return strings.HasSuffix(host, "."+domain)
	}

	return false
}

// isWordPressJunk filters WordPress-related URLs that generate noise
func (f *Filter) isWordPressJunk(rawURL string) bool {
	lowerURL := strings.ToLower(rawURL)
	wpPatterns := []string{
		"wp-content", "wp-includes", "wp-admin", "wp-json",
		"xmlrpc.php", "wp-login.php", "wp-config", "wp-cron",
		"wp-mail", "wp-signup", "wp-activate", "wp-trackback",
		"/feed/", "wp-sitemap",
	}
	
	for _, pattern := range wpPatterns {
		if strings.Contains(lowerURL, pattern) {
			return true
		}
	}
	return false
}

// isBlogURL filters blog-related URLs
func (f *Filter) isBlogURL(rawURL string) bool {
	lowerURL := strings.ToLower(rawURL)
	
	// Blog subdomain
	if strings.Contains(lowerURL, "://blog.") {
		return true
	}
	
	// Blog paths
	return strings.Contains(lowerURL, "/blog/")
}

// isGenericJSLibrary filters common JavaScript libraries
func (f *Filter) isGenericJSLibrary(urlPath string) bool {
	lowerPath := strings.ToLower(urlPath)
	
	// Library names
	jsLibraries := []string{
		"jquery", "bootstrap", "angular", "react", "vue",
		"lodash", "moment", "d3", "chart", "datatables",
		"select2", "popper", "swiper", "lightbox", "owl.carousel",
		"modernizr", "polyfill", "babel", "axios",
	}
	
	// Third-party directories
	thirdPartyDirs := []string{
		"/vendor/", "/bower/", "/node_modules/", "/bower_components/",
		"/jquery/", "/third-party/", "/external/",
	}
	
	// Check library names in JS files
	if strings.HasSuffix(lowerPath, ".js") {
		for _, lib := range jsLibraries {
			if strings.Contains(lowerPath, lib) {
				return true
			}
		}
	}
	
	// Check third-party directories
	for _, dir := range thirdPartyDirs {
		if strings.Contains(lowerPath, dir) && strings.HasSuffix(lowerPath, ".js") {
			return true
		}
	}
	
	return false
}