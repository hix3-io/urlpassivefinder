package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"
)

// CommonCrawlProvider implements the Common Crawl provider
type CommonCrawlProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewCommonCrawlProvider creates a new Common Crawl provider
func NewCommonCrawlProvider(rateLimit int) Provider {
	if rateLimit <= 0 {
		rateLimit = 5
	}
	return &CommonCrawlProvider{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (cc *CommonCrawlProvider) Name() string {
	return "commoncrawl"
}

// Fetch retrieves URLs from Common Crawl index
func (cc *CommonCrawlProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting
	if err := cc.limiter.Wait(ctx); err != nil {
		return err
	}

	// Get latest index
	indexURL, err := cc.getLatestIndex(ctx)
	if err != nil {
		return err
	}

	// Query for both domain and subdomains
	searchDomains := []string{domain, "*." + domain}
	
	for _, searchDomain := range searchDomains {
		// CommonCrawl index API with pagination
		page := 0
		for {
			searchURL := cc.buildSearchURL(indexURL, searchDomain, page)
			
			req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
			if err != nil {
				break
			}

			req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

			resp, err := cc.client.Do(req)
			if err != nil {
				break
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				break
			}

			// Parse JSON lines response
			var hasResults bool
			decoder := json.NewDecoder(resp.Body)
			for decoder.More() {
				var result struct {
					URL string `json:"url"`
				}
				if err := decoder.Decode(&result); err != nil {
					break
				}
				if result.URL != "" {
					results <- result.URL
					hasResults = true
				}
			}
			resp.Body.Close()

			// Stop if no more results
			if !hasResults {
				break
			}

			page++
			if page > 10 { // Limit pages to avoid excessive requests
				break
			}
		}
	}

	return nil
}

// getLatestIndex retrieves the latest Common Crawl index URL
func (cc *CommonCrawlProvider) getLatestIndex(ctx context.Context) (string, error) {
	// In a real implementation, fetch from:
	// https://index.commoncrawl.org/collinfo.json
	// For now, return a recent index
	return "http://index.commoncrawl.org/CC-MAIN-2024-10-index", nil
}

// buildSearchURL constructs the Common Crawl search URL
func (cc *CommonCrawlProvider) buildSearchURL(indexURL, searchDomain string, page int) string {
	params := url.Values{}
	params.Set("url", searchDomain)
	params.Set("output", "json")
	params.Set("fl", "url")
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	
	return fmt.Sprintf("%s?%s", indexURL, params.Encode())
}