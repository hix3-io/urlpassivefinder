package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// URLScanProvider implements the URLScan.io provider
type URLScanProvider struct {
	limiter *rate.Limiter
	client  *http.Client
	apiKey  string
}

// NewURLScanProvider creates a new URLScan provider
func NewURLScanProvider(rateLimit int, apiKey string) Provider {
	if rateLimit <= 0 {
		rateLimit = 2
	}
	return &URLScanProvider{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// Name returns the provider name
func (u *URLScanProvider) Name() string {
	return "urlscan"
}

// Fetch retrieves URLs from URLScan.io
func (u *URLScanProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting
	if err := u.limiter.Wait(ctx); err != nil {
		return err
	}

	// URLScan.io search API
	searchURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=100", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")
	if u.apiKey != "" {
		req.Header.Set("API-Key", u.apiKey)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("URLScan API returned status %d", resp.StatusCode)
	}

	// Parse response
	var scanResp struct {
		Results []struct {
			Task struct {
				URL string `json:"url"`
			} `json:"task"`
			Page struct {
				URL string `json:"url"`
			} `json:"page"`
		} `json:"results"`
		Has_More bool   `json:"has_more"`
		Search   string `json:"search_after,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&scanResp); err != nil {
		return err
	}

	// Extract URLs
	for _, result := range scanResp.Results {
		if result.Task.URL != "" {
			results <- result.Task.URL
		}
		if result.Page.URL != "" && result.Page.URL != result.Task.URL {
			results <- result.Page.URL
		}
	}

	// Handle pagination with search_after
	if scanResp.Has_More && scanResp.Search != "" {
		// Could implement pagination here
		// For now, we get first 100 results
	}

	return nil
}