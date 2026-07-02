package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// OTXProvider implements the AlienVault OTX provider
type OTXProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewOTXProvider creates a new OTX provider
func NewOTXProvider(rateLimit int) Provider {
	if rateLimit <= 0 {
		rateLimit = 10
	}
	return &OTXProvider{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (o *OTXProvider) Name() string {
	return "otx"
}

// Fetch retrieves URLs from AlienVault OTX
func (o *OTXProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting
	if err := o.limiter.Wait(ctx); err != nil {
		return err
	}

	// OTX API for passive DNS and URL list
	page := 1
	hasNext := true

	for hasNext && page <= 10 {
		apiURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?page=%d", domain, page)

		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

		resp, err := o.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("OTX API returned status %d", resp.StatusCode)
		}

		// Parse JSON response
		var otxResp struct {
			HasNext bool `json:"has_next"`
			URLList []struct {
				URL string `json:"url"`
			} `json:"url_list"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&otxResp); err != nil {
			return err
		}

		// Extract URLs
		for _, item := range otxResp.URLList {
			if item.URL != "" {
				results <- item.URL
			}
		}

		hasNext = otxResp.HasNext
		page++
	}

	return nil
}