package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// ThreatCrowdProvider implements the ThreatCrowd threat intelligence provider
type ThreatCrowdProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewThreatCrowdProvider creates a new ThreatCrowd provider
func NewThreatCrowdProvider(rateLimit int) Provider {
	// ThreatCrowd has very strict rate limits
	return &ThreatCrowdProvider{
		limiter: rate.NewLimiter(rate.Every(10*time.Second), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (tc *ThreatCrowdProvider) Name() string {
	return "threatcrowd"
}

// Fetch retrieves data from ThreatCrowd API
func (tc *ThreatCrowdProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting (very strict for ThreatCrowd)
	if err := tc.limiter.Wait(ctx); err != nil {
		return err
	}

	// Query ThreatCrowd API
	searchURL := fmt.Sprintf("https://www.threatcrowd.org/searchApi/v2/domain/report/?domain=%s", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	resp, err := tc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ThreatCrowd API returned status %d", resp.StatusCode)
	}

	// Parse response
	var tcResp struct {
		Subdomains []string `json:"subdomains"`
		Resolutions []struct {
			Domain string `json:"domain"`
		} `json:"resolutions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tcResp); err != nil {
		return err
	}

	// Extract subdomains
	seen := make(map[string]bool)
	for _, subdomain := range tcResp.Subdomains {
		if subdomain != "" && !seen[subdomain] {
			seen[subdomain] = true
			results <- fmt.Sprintf("https://%s/", subdomain)
			results <- fmt.Sprintf("http://%s/", subdomain)
		}
	}

	// Extract from resolutions
	for _, res := range tcResp.Resolutions {
		if res.Domain != "" && !seen[res.Domain] {
			seen[res.Domain] = true
			results <- fmt.Sprintf("https://%s/", res.Domain)
			results <- fmt.Sprintf("http://%s/", res.Domain)
		}
	}

	return nil
}