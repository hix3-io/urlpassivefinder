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

// CrtshProvider implements the crt.sh Certificate Transparency provider
type CrtshProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewCrtshProvider creates a new crt.sh provider
func NewCrtshProvider(rateLimit int) Provider {
	if rateLimit <= 0 {
		rateLimit = 5
	}
	return &CrtshProvider{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (c *CrtshProvider) Name() string {
	return "crtsh"
}

// Fetch retrieves subdomains from crt.sh Certificate Transparency logs
func (c *CrtshProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	// Query crt.sh API with JSON output
	searchURL := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("crt.sh returned status %d", resp.StatusCode)
	}

	// Parse JSON response
	var certificates []struct {
		NameValue string `json:"name_value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&certificates); err != nil {
		return err
	}

	// Extract unique domains from certificates
	seen := make(map[string]bool)
	for _, cert := range certificates {
		// Split by newline as certificates can contain multiple domains
		domains := strings.Split(cert.NameValue, "\n")
		for _, d := range domains {
			d = strings.TrimSpace(d)
			d = strings.TrimPrefix(d, "*.")
			
			if d != "" && !seen[d] {
				seen[d] = true
				// Convert subdomain to base URL for discovery
				results <- fmt.Sprintf("https://%s/", d)
				results <- fmt.Sprintf("http://%s/", d)
			}
		}
	}

	return nil
}