package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	// Test loading default config when no file exists
	config, err := LoadConfig("nonexistent.yaml")
	if err != nil {
		t.Fatalf("LoadConfig should not error for nonexistent file: %v", err)
	}

	// Check default providers
	if !config.IsProviderEnabled("wayback") {
		t.Error("Wayback should be enabled by default")
	}
	if !config.IsProviderEnabled("commoncrawl") {
		t.Error("CommonCrawl should be enabled by default")
	}
	if config.IsProviderEnabled("virustotal") {
		t.Error("VirusTotal should be disabled by default")
	}

	// Check default rate limits
	if config.GetRateLimit("wayback") != 10 {
		t.Errorf("Expected wayback rate limit 10, got %d", config.GetRateLimit("wayback"))
	}
}

func TestLoadConfig_YAML(t *testing.T) {
	// Create temporary config file
	configContent := `
providers:
  wayback:
    enabled: false
    rate_limit: 5
  urlscan:
    enabled: true
    api_key: "test-key"
    rate_limit: 3

filters:
  extensions: [".test", ".example"]
  
output:
  format: "jsonl"
  compress: true

performance:
  workers: 20
  timeout: "5m"
`

	tmpDir, err := os.MkdirTemp("", "urlpassivefinder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test provider settings
	if config.IsProviderEnabled("wayback") {
		t.Error("Wayback should be disabled in test config")
	}
	if !config.IsProviderEnabled("urlscan") {
		t.Error("URLScan should be enabled in test config")
	}
	if config.GetRateLimit("wayback") != 5 {
		t.Errorf("Expected wayback rate limit 5, got %d", config.GetRateLimit("wayback"))
	}
	if config.GetAPIKey("urlscan") != "test-key" {
		t.Errorf("Expected URLScan API key 'test-key', got '%s'", config.GetAPIKey("urlscan"))
	}

	// Test filters
	expectedExts := []string{".test", ".example"}
	if len(config.Filters.Extensions) != len(expectedExts) {
		t.Errorf("Expected %d extensions, got %d", len(expectedExts), len(config.Filters.Extensions))
	}

	// Test output config
	if config.Output.Format != "jsonl" {
		t.Errorf("Expected format 'jsonl', got '%s'", config.Output.Format)
	}
	if !config.Output.Compress {
		t.Error("Expected compress to be true")
	}

	// Test performance config
	if config.Performance.Workers != 20 {
		t.Errorf("Expected 20 workers, got %d", config.Performance.Workers)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Set test environment variables
	testVars := map[string]string{
		"URLSCAN_API_KEY": "env-urlscan-key",
		"VT_API_KEY":      "env-vt-key", 
		"GITHUB_TOKEN":    "env-github-token",
		"OTX_API_KEY":     "env-otx-key",
	}

	// Backup existing env vars
	oldVars := make(map[string]string)
	for key := range testVars {
		oldVars[key] = os.Getenv(key)
	}

	// Set test env vars
	for key, value := range testVars {
		os.Setenv(key, value)
	}

	// Restore env vars after test
	defer func() {
		for key, oldValue := range oldVars {
			if oldValue == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, oldValue)
			}
		}
	}()

	// Load config
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test API keys from environment
	if config.GetAPIKey("urlscan") != "env-urlscan-key" {
		t.Errorf("Expected URLScan API key from env, got '%s'", config.GetAPIKey("urlscan"))
	}
	if config.GetAPIKey("virustotal") != "env-vt-key" {
		t.Errorf("Expected VirusTotal API key from env, got '%s'", config.GetAPIKey("virustotal"))
	}
	if config.GetAPIKey("github") != "env-github-token" {
		t.Errorf("Expected GitHub token from env, got '%s'", config.GetAPIKey("github"))
	}
	if config.GetAPIKey("otx") != "env-otx-key" {
		t.Errorf("Expected OTX API key from env, got '%s'", config.GetAPIKey("otx"))
	}
}

func TestConfig_IsProviderEnabled(t *testing.T) {
	config := getDefaultConfig()

	// Test existing providers
	if !config.IsProviderEnabled("wayback") {
		t.Error("Wayback should be enabled by default")
	}
	if config.IsProviderEnabled("virustotal") {
		t.Error("VirusTotal should be disabled by default")
	}

	// Test non-existent provider
	if config.IsProviderEnabled("nonexistent") {
		t.Error("Non-existent provider should return false")
	}
}

func TestConfig_GetAPIKey(t *testing.T) {
	config := getDefaultConfig()

	// Test empty API key
	if config.GetAPIKey("urlscan") != "" {
		t.Error("API key should be empty by default")
	}

	// Test non-existent provider
	if config.GetAPIKey("nonexistent") != "" {
		t.Error("Non-existent provider should return empty string")
	}

	// Set API key
	if provider, exists := config.Providers["urlscan"]; exists {
		provider.APIKey = "test-key"
		config.Providers["urlscan"] = provider
	}

	if config.GetAPIKey("urlscan") != "test-key" {
		t.Errorf("Expected 'test-key', got '%s'", config.GetAPIKey("urlscan"))
	}
}

func TestConfig_GetRateLimit(t *testing.T) {
	config := getDefaultConfig()

	// Test existing provider with rate limit
	if config.GetRateLimit("wayback") != 10 {
		t.Errorf("Expected rate limit 10, got %d", config.GetRateLimit("wayback"))
	}

	// Test non-existent provider (should return default)
	if config.GetRateLimit("nonexistent") != 10 {
		t.Errorf("Expected default rate limit 10, got %d", config.GetRateLimit("nonexistent"))
	}

	// Test provider with zero rate limit (should return default)
	if provider, exists := config.Providers["wayback"]; exists {
		provider.RateLimit = 0
		config.Providers["wayback"] = provider
	}

	if config.GetRateLimit("wayback") != 10 {
		t.Errorf("Expected default rate limit 10 for zero value, got %d", config.GetRateLimit("wayback"))
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Create temporary invalid config file
	invalidContent := `
providers:
  wayback:
    enabled: not_a_boolean
    rate_limit: "not_a_number"
  invalid yaml syntax [
`

	tmpDir, err := os.MkdirTemp("", "urlpassivefinder-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Load config should return error
	_, err = LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig should return error for invalid YAML")
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	// Test that all expected providers exist
	expectedProviders := []string{"wayback", "commoncrawl", "otx", "urlscan", "virustotal", "github"}
	for _, provider := range expectedProviders {
		if _, exists := config.Providers[provider]; !exists {
			t.Errorf("Expected provider '%s' not found in default config", provider)
		}
	}

	// Test default extensions
	if len(config.Filters.Extensions) == 0 {
		t.Error("Default config should have extension filters")
	}

	// Check for common extensions
	extMap := make(map[string]bool)
	for _, ext := range config.Filters.Extensions {
		extMap[ext] = true
	}

	expectedExts := []string{".jpg", ".css", ".js", ".png"}
	for _, ext := range expectedExts {
		if !extMap[ext] {
			t.Errorf("Expected extension '%s' not found in default filters", ext)
		}
	}

	// Test default performance settings
	if config.Performance.Workers <= 0 {
		t.Error("Default workers should be greater than 0")
	}
	if config.Performance.Timeout == "" {
		t.Error("Default timeout should not be empty")
	}
}