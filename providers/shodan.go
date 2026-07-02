package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// ShodanProvider implements the Shodan search engine provider
type ShodanProvider struct {
	limiter *rate.Limiter
	client  *http.Client
	apiKey  string
}

// NewShodanProvider creates a new Shodan provider
func NewShodanProvider(rateLimit int, apiKey string) Provider {
	if rateLimit <= 0 {
		rateLimit = 1
	}
	return &ShodanProvider{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// Name returns the provider name
func (s *ShodanProvider) Name() string {
	return "shodan"
}

// Fetch retrieves data from Shodan API
func (s *ShodanProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if s.apiKey == "" {
		return fmt.Errorf("Shodan API key not configured")
	}

	// Rate limiting
	if err := s.limiter.Wait(ctx); err != nil {
		return err
	}

	// Search for domain in Shodan
	searchURL := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=hostname:%s", 
		s.apiKey, domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Shodan API returned status %d", resp.StatusCode)
	}

	// Parse response
	var shodanResp struct {
		Matches []struct {
			Hostnames []string `json:"hostnames"`
			Domains   []string `json:"domains"`
			HTTP      struct {
				Host string `json:"host"`
			} `json:"http"`
			Port int `json:"port"`
			SSL  struct {
				Cert struct {
					Subject struct {
						CN string `json:"CN"`
					} `json:"subject"`
					AltNames []string `json:"altnames"`
				} `json:"cert"`
			} `json:"ssl"`
		} `json:"matches"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&shodanResp); err != nil {
		return err
	}

	// Extract URLs from matches
	seen := make(map[string]bool)
	for _, match := range shodanResp.Matches {
		// From hostnames
		for _, hostname := range match.Hostnames {
			if hostname != "" && !seen[hostname] {
				seen[hostname] = true
				protocol := "http"
				if match.Port == 443 {
					protocol = "https"
				}
				results <- fmt.Sprintf("%s://%s:%d/", protocol, hostname, match.Port)
			}
		}
		
		// From SSL cert alt names
		for _, altName := range match.SSL.Cert.AltNames {
			altName = strings.TrimPrefix(altName, "*.")
			if altName != "" && !seen[altName] {
				seen[altName] = true
				results <- fmt.Sprintf("https://%s/", altName)
			}
		}
	}

	return nil
}