package providers

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// WaybackProvider implements the Wayback Machine provider
type WaybackProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewWaybackProvider creates a new Wayback Machine provider
func NewWaybackProvider(rateLimit int) Provider {
	if rateLimit <= 0 {
		rateLimit = 10
	}
	return &WaybackProvider{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (w *WaybackProvider) Name() string {
	return "wayback"
}

// Fetch retrieves URLs from the Wayback Machine using TEXT API
func (w *WaybackProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
	}
	
	// Rate limiting
	if err := w.limiter.Wait(ctx); err != nil {
		return err
	}

	// Like waybackurls, make TWO requests: one for domain and one for subdomains
	urls := []string{
		fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=%s/*&output=text&fl=original&collapse=urlkey", domain),
		fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=text&fl=original&collapse=urlkey", domain),
	}

	for i, searchURL := range urls {
		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			continue
		}

		// Rotate user agents
		req.Header.Set("User-Agent", userAgents[i%len(userAgents)])

		resp, err := w.client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		// Stream results line by line
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url != "" {
				results <- url
			}
		}
	}

	return nil
}