package main

import (
	"fmt"
	"strings"
)

// Helper functions and utilities

// extractSimpleURL extracts URLs from text using simple pattern matching
func extractSimpleURL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	
	// Look for URL patterns
	if strings.Contains(text, "http") {
		start := strings.Index(text, "http")
		if start == -1 {
			return ""
		}
		
		// Find the end of the URL
		end := len(text)
		for i := start; i < len(text); i++ {
			ch := text[i]
			if ch == ' ' || ch == '\t' || ch == '\n' {
				end = i
				break
			}
		}
		
		if end > start {
			url := text[start:end]
			// Basic validation
			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				return url
			}
		}
	}
	
	return ""
}

// formatDuration formats a duration in a human-readable way
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	
	if minutes < 60 {
		if remainingSeconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm%ds", minutes, remainingSeconds)
	}
	
	hours := minutes / 60
	remainingMinutes := minutes % 60
	
	if remainingMinutes == 0 && remainingSeconds == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if remainingSeconds == 0 {
		return fmt.Sprintf("%dh%dm", hours, remainingMinutes)
	}
	
	return fmt.Sprintf("%dh%dm%ds", hours, remainingMinutes, remainingSeconds)
}

// isValidDomain performs basic domain validation
func isValidDomain(domain string) bool {
	if domain == "" {
		return false
	}
	
	// Remove protocol if present
	if strings.HasPrefix(domain, "http://") {
		domain = domain[7:]
	} else if strings.HasPrefix(domain, "https://") {
		domain = domain[8:]
	}
	
	// Remove path if present
	if slashIndex := strings.Index(domain, "/"); slashIndex != -1 {
		domain = domain[:slashIndex]
	}
	
	// Remove port if present
	if colonIndex := strings.LastIndex(domain, ":"); colonIndex != -1 {
		domain = domain[:colonIndex]
	}
	
	// Basic checks
	if len(domain) > 253 {
		return false
	}
	
	if strings.Contains(domain, "..") {
		return false
	}
	
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	
	// Must contain at least one dot
	if !strings.Contains(domain, ".") {
		return false
	}
	
	return true
}