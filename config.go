package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Providers   map[string]ProviderConfig `yaml:"providers"`
	Filters     FilterConfig              `yaml:"filters"`
	Output      OutputConfig              `yaml:"output"`
	Performance PerformanceConfig         `yaml:"performance"`
}

// ProviderConfig holds provider-specific configuration
type ProviderConfig struct {
	Enabled   bool   `yaml:"enabled"`
	APIKey    string `yaml:"api_key"`
	RateLimit int    `yaml:"rate_limit"`
}

// FilterConfig holds filtering configuration
type FilterConfig struct {
	Extensions []string `yaml:"extensions"`
	MimeTypes  []string `yaml:"mime_types"`
}

// OutputConfig holds output configuration
type OutputConfig struct {
	Format   string `yaml:"format"`
	Compress bool   `yaml:"compress"`
}

// PerformanceConfig holds performance tuning configuration
type PerformanceConfig struct {
	Workers     int    `yaml:"workers"`
	Timeout     string `yaml:"timeout"`
	MemoryLimit string `yaml:"memory_limit"`
}

// LoadConfig loads configuration from file or returns default config
func LoadConfig(path string) (*Config, error) {
	config := getDefaultConfig()

	// If no path specified, try default locations
	if path == "" {
		homeDir, _ := os.UserHomeDir()
		defaultPaths := []string{
			filepath.Join(homeDir, ".urlpassivefinder", "config.yaml"),
			filepath.Join(homeDir, ".config", "urlpassivefinder", "config.yaml"),
			"config.yaml",
		}

		for _, p := range defaultPaths {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	// If still no path or file doesn't exist, return default config
	if path == "" {
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// Load environment variables for API keys
	config.loadEnvVars()

	return config, nil
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *Config {
	return &Config{
		Providers: map[string]ProviderConfig{
			"wayback": {
				Enabled:   true,
				RateLimit: 10,
			},
			"commoncrawl": {
				Enabled:   true,
				RateLimit: 5,
			},
			"otx": {
				Enabled:   true,
				RateLimit: 10,
			},
			"urlscan": {
				Enabled:   true,
				RateLimit: 2,
			},
			"crtsh": {
				Enabled:   true,
				RateLimit: 5,
			},
			"threatcrowd": {
				Enabled:   true,
				RateLimit: 1, // Very strict rate limit
			},
			"virustotal": {
				Enabled:   false, // Requires API key
				RateLimit: 4,
			},
			"github": {
				Enabled:   false, // Requires API key
				RateLimit: 30,
			},
			"shodan": {
				Enabled:   false, // Requires API key
				RateLimit: 1,
			},
			"securitytrails": {
				Enabled:   false, // Requires API key
				RateLimit: 1,
			},
			"censys": {
				Enabled:   false, // Requires API credentials
				RateLimit: 1,
			},
		},
		Filters: FilterConfig{
			Extensions: []string{
				".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg",
				".css", ".js", ".ico", ".woff", ".woff2", ".ttf", ".eot",
				".mp4", ".mp3", ".wav", ".avi", ".mov", ".webm",
				".pdf", ".doc", ".docx", ".xls", ".xlsx",
			},
		},
		Output: OutputConfig{
			Format:   "text",
			Compress: false,
		},
		Performance: PerformanceConfig{
			Workers:     10,
			Timeout:     "10m",
			MemoryLimit: "2GB",
		},
	}
}

// loadEnvVars loads API keys from environment variables
func (c *Config) loadEnvVars() {
	envMappings := map[string]string{
		"urlscan":    "URLSCAN_API_KEY",
		"virustotal": "VT_API_KEY",
		"github":     "GITHUB_TOKEN",
		"otx":        "OTX_API_KEY",
	}

	for provider, envVar := range envMappings {
		if val := os.Getenv(envVar); val != "" {
			if p, exists := c.Providers[provider]; exists {
				p.APIKey = val
				c.Providers[provider] = p
			} else {
				// Create provider config if it doesn't exist
				c.Providers[provider] = ProviderConfig{
					Enabled:   true,
					APIKey:    val,
					RateLimit: 10,
				}
			}
		}
	}
}

// IsProviderEnabled checks if a provider is enabled
func (c *Config) IsProviderEnabled(name string) bool {
	if provider, exists := c.Providers[name]; exists {
		return provider.Enabled
	}
	return false
}

// GetAPIKey returns the API key for a provider
func (c *Config) GetAPIKey(provider string) string {
	if p, exists := c.Providers[provider]; exists {
		return p.APIKey
	}
	return ""
}

// GetRateLimit returns the rate limit for a provider
func (c *Config) GetRateLimit(provider string) int {
	if p, exists := c.Providers[provider]; exists {
		if p.RateLimit > 0 {
			return p.RateLimit
		}
	}
	return 10 // default rate limit
}