package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
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
func NewWaybackProvider(config *Config) Provider {
	return &WaybackProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("wayback")), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name
func (w *WaybackProvider) Name() string {
	return "wayback"
}

// Fetch retrieves URLs from the Wayback Machine using TEXT API like waybackurls for maximum results
func (w *WaybackProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
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
	
	// Process both URLs (domain and subdomains)
	for urlIndex, searchURL := range urls {
		// Create request with user agent rotation
		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", userAgents[urlIndex % len(userAgents)])

		// Make request with retry logic
		retries := 3
		var resp *http.Response
		for i := 0; i < retries; i++ {
			resp, err = w.client.Do(req)
			if err != nil {
				if i < retries-1 {
					time.Sleep(2 * time.Second)
					continue
				}
				return err
			}
			
			if resp.StatusCode == 429 {
				resp.Body.Close()
				if i < retries-1 {
					time.Sleep(5 * time.Second)
					continue
				}
				return fmt.Errorf("wayback API rate limited after %d retries", retries)
			}
			
			break
		}
		
		if resp.StatusCode != 200 {
			resp.Body.Close()
			// Skip if no results for this pattern
			if resp.StatusCode == 404 {
				continue
			}
			return fmt.Errorf("wayback API returned status %d", resp.StatusCode)
		}

		// Stream process TEXT response line by line for memory efficiency
		scanner := bufio.NewScanner(resp.Body)
		// Increase buffer size for VERY large responses (wayback can return 10k+ URLs)
		buf := make([]byte, 0, 1024*1024) // 1MB initial buffer
		scanner.Buffer(buf, 100*1024*1024) // 100MB max buffer for huge responses
		
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
				return ctx.Err()
			default:
				url := strings.TrimSpace(scanner.Text())
				if url != "" && url != "original" {
					results <- url
				}
			}
		}

		resp.Body.Close()
		
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading wayback response: %v", err)
		}
	}

	return nil
}

// buildURL constructs the Wayback Machine CDX API URL with pagination like gau
func (w *WaybackProvider) buildURL(domain string, page int) string {
	// Use gau's exact method with pagination
	// Support subdomains with *.domain.com pattern
	searchDomain := domain
	if !strings.Contains(domain, "*.") && !strings.HasPrefix(domain, "*.") {
		searchDomain = "*." + domain
	}
	return fmt.Sprintf(
		"https://web.archive.org/cdx/search/cdx?url=%s/*&output=json&collapse=urlkey&fl=original&pageSize=100&page=%d",
		searchDomain, page,
	)
}

// CommonCrawlProvider implements the Common Crawl provider
type CommonCrawlProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewCommonCrawlProvider creates a new Common Crawl provider
func NewCommonCrawlProvider(config *Config) Provider {
	return &CommonCrawlProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("commoncrawl")), 1),
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// Name returns the provider name
func (c *CommonCrawlProvider) Name() string {
	return "commoncrawl"
}

// Fetch retrieves URLs from Common Crawl
func (c *CommonCrawlProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	// Get latest index
	indexURL, err := c.getLatestIndex(ctx)
	if err != nil {
		return err
	}

	// Build search URL
	searchURL := fmt.Sprintf("%s?url=*.%s/*&output=json&fl=url", indexURL, domain)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	// Make request
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("common crawl API returned status %d", resp.StatusCode)
	}

	// Stream process results
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				results <- line
			}
		}
	}

	return scanner.Err()
}

// getLatestIndex fetches the latest Common Crawl index URL
func (c *CommonCrawlProvider) getLatestIndex(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://index.commoncrawl.org/collinfo.json", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// For simplicity, we'll use a known good index
	// In production, you'd parse the JSON to get the latest
	return "http://index.commoncrawl.org/CC-MAIN-2024-10-index", nil
}

// OTXProvider implements the AlienVault OTX provider
type OTXProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewOTXProvider creates a new OTX provider
func NewOTXProvider(config *Config) Provider {
	return &OTXProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("otx")), 1),
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

	// Build OTX URL
	searchURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list", domain)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	// Make request
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OTX API returned status %d", resp.StatusCode)
	}

	// Parse complete JSON response
	var otxResponse struct {
		URLList []struct {
			URL string `json:"url"`
		} `json:"url_list"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&otxResponse); err != nil {
		return err
	}

	// Extract all URLs from response
	for _, urlData := range otxResponse.URLList {
		if urlData.URL != "" {
			results <- urlData.URL
		}
	}

	return nil
}

// URLScanProvider implements the URLScan.io provider
type URLScanProvider struct {
	limiter *rate.Limiter
	client  *http.Client
	apiKey  string
}

// NewURLScanProvider creates a new URLScan provider
func NewURLScanProvider(config *Config) Provider {
	return &URLScanProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("urlscan")), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: config.GetAPIKey("urlscan"),
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

	// Build URLScan search URL - get actual scan results, not API metadata
	searchURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=100", domain)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")
	if u.apiKey != "" {
		req.Header.Set("API-Key", u.apiKey)
	}

	// Make request
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("URLScan API returned status %d", resp.StatusCode)
	}

	// The issue: URLScan search API returns metadata about scans, not the URLs found in scans
	// We need to extract the 'page.url' field from search results, not scan result APIs
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := scanner.Text()
			// Extract page URLs from URLScan search results
			if pageURL := extractPageURLFromURLScan(line); pageURL != "" {
				// Filter out URLScan's own API URLs
				if !strings.Contains(pageURL, "urlscan.io/api/") && 
				   !strings.Contains(pageURL, "urlscan.io/result/") {
					results <- pageURL
				}
			}
		}
	}

	return scanner.Err()
}

// extractURLFromJSON extracts URLs from JSON lines (simplified)
func extractURLFromJSON(line string) string {
	// This is a simplified JSON parser for demo
	// In production, use proper JSON unmarshaling
	if strings.Contains(line, "http") {
		start := strings.Index(line, "http")
		if start != -1 {
			end := start
			for i := start; i < len(line); i++ {
				if line[i] == '"' || line[i] == ',' || line[i] == '}' {
					end = i
					break
				}
			}
			if end > start {
				return line[start:end]
			}
		}
	}
	return ""
}

// extractPageURLFromURLScan extracts page URLs from URLScan search results
func extractPageURLFromURLScan(line string) string {
	// Look for "page":{"url":"..." pattern in URLScan JSON
	if strings.Contains(line, `"page"`) && strings.Contains(line, `"url"`) {
		// Find the page.url field specifically
		pageIndex := strings.Index(line, `"page"`)
		if pageIndex == -1 {
			return ""
		}
		
		// Look for url field within the page object
		searchFrom := pageIndex
		urlIndex := strings.Index(line[searchFrom:], `"url":"`)
		if urlIndex == -1 {
			return ""
		}
		
		start := searchFrom + urlIndex + 7 // Skip `"url":"`
		end := start
		for i := start; i < len(line); i++ {
			if line[i] == '"' {
				end = i
				break
			}
		}
		
		if end > start {
			url := line[start:end]
			// Validate it's a proper URL
			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				return url
			}
		}
	}
	return ""
}

// VirusTotalProvider implements passive URL discovery using VirusTotal API v3
type VirusTotalProvider struct {
	client  *http.Client
	limiter *rate.Limiter
	apiKey  string
}

// NewVirusTotalProvider creates a new VirusTotal provider
func NewVirusTotalProvider(globalConfig *Config) Provider {
	config := globalConfig.Providers["virustotal"]
	client := &http.Client{Timeout: 30 * time.Second}
	limiter := rate.NewLimiter(rate.Every(time.Duration(config.RateLimit)*time.Millisecond), 1)
	return &VirusTotalProvider{
		client:  client,
		limiter: limiter,
		apiKey:  config.APIKey,
	}
}

// Name returns the provider name
func (v *VirusTotalProvider) Name() string {
	return "virustotal"
}

// Fetch retrieves URLs from VirusTotal API v3
func (v *VirusTotalProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if v.apiKey == "" {
		return fmt.Errorf("VirusTotal API key required")
	}

	// Rate limiting
	if err := v.limiter.Wait(ctx); err != nil {
		return err
	}

	// VirusTotal API v3 - Get detected URLs for domain
	searchURL := fmt.Sprintf("https://www.virustotal.com/api/v3/domains/%s/urls", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")
	req.Header.Set("x-apikey", v.apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("VirusTotal API returned status %d", resp.StatusCode)
	}

	// Parse URLs response
	var urlsResponse struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&urlsResponse); err != nil {
		return err
	}

	// Extract URLs from response
	for _, urlData := range urlsResponse.Data {
		if urlData.ID != "" {
			// Decode base64 URL ID to get actual URL
			if decoded, err := base64.StdEncoding.DecodeString(urlData.ID); err == nil {
				url := string(decoded)
				if strings.HasPrefix(url, "http") {
					results <- url
				}
			}
		}
	}

	return nil
}

// GitHubProvider implements passive URL discovery using GitHub search API  
type GitHubProvider struct {
	client  *http.Client
	limiter *rate.Limiter
	apiKey  string
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(globalConfig *Config) Provider {
	config := globalConfig.Providers["github"]
	client := &http.Client{Timeout: 30 * time.Second}
	limiter := rate.NewLimiter(rate.Every(time.Duration(config.RateLimit)*time.Millisecond), 1)
	return &GitHubProvider{
		client:  client,
		limiter: limiter,
		apiKey:  config.APIKey,
	}
}

// Name returns the provider name
func (g *GitHubProvider) Name() string {
	return "github"
}

// Fetch retrieves URLs from GitHub search API
func (g *GitHubProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	// Rate limiting
	if err := g.limiter.Wait(ctx); err != nil {
		return err
	}

	// GitHub search for domain in code
	searchURL := fmt.Sprintf("https://api.github.com/search/code?q=%s&type=Code&per_page=100", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "token "+g.apiKey)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	// Parse search results
	var searchResponse struct {
		Items []struct {
			HTMLURL string `json:"html_url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return err
	}

	// Extract URLs from search results
	for _, item := range searchResponse.Items {
		if item.HTMLURL != "" && strings.Contains(item.HTMLURL, domain) {
			results <- item.HTMLURL
		}
	}

	return nil
}

// CrtshProvider implements the crt.sh Certificate Transparency provider
type CrtshProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewCrtshProvider creates a new crt.sh provider
func NewCrtshProvider(config *Config) Provider {
	return &CrtshProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("crtsh")), 1),
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

// ShodanProvider implements the Shodan search engine provider
type ShodanProvider struct {
	limiter *rate.Limiter
	client  *http.Client
	apiKey  string
}

// NewShodanProvider creates a new Shodan provider
func NewShodanProvider(config *Config) Provider {
	return &ShodanProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("shodan")), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: config.GetAPIKey("shodan"),
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

// SecurityTrailsProvider implements the SecurityTrails DNS history provider
type SecurityTrailsProvider struct {
	limiter *rate.Limiter
	client  *http.Client
	apiKey  string
}

// NewSecurityTrailsProvider creates a new SecurityTrails provider
func NewSecurityTrailsProvider(config *Config) Provider {
	return &SecurityTrailsProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("securitytrails")), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: config.GetAPIKey("securitytrails"),
	}
}

// Name returns the provider name
func (st *SecurityTrailsProvider) Name() string {
	return "securitytrails"
}

// Fetch retrieves DNS history from SecurityTrails
func (st *SecurityTrailsProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if st.apiKey == "" {
		return fmt.Errorf("SecurityTrails API key not configured")
	}

	// Rate limiting
	if err := st.limiter.Wait(ctx); err != nil {
		return err
	}

	// Get subdomains
	searchURL := fmt.Sprintf("https://api.securitytrails.com/v1/domain/%s/subdomains", domain)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("APIKEY", st.apiKey)
	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	resp, err := st.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SecurityTrails API returned status %d", resp.StatusCode)
	}

	// Parse response
	var stResp struct {
		Subdomains []string `json:"subdomains"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&stResp); err != nil {
		return err
	}

	// Generate URLs from subdomains
	for _, subdomain := range stResp.Subdomains {
		fullDomain := fmt.Sprintf("%s.%s", subdomain, domain)
		results <- fmt.Sprintf("https://%s/", fullDomain)
		results <- fmt.Sprintf("http://%s/", fullDomain)
	}

	// Also get DNS history for more data
	historyURL := fmt.Sprintf("https://api.securitytrails.com/v1/history/%s/dns/a", domain)
	
	histReq, err := http.NewRequestWithContext(ctx, "GET", historyURL, nil)
	if err != nil {
		return nil // Don't fail if history fails
	}

	histReq.Header.Set("APIKEY", st.apiKey)
	histReq.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	histResp, err := st.client.Do(histReq)
	if err != nil {
		return nil
	}
	defer histResp.Body.Close()

	if histResp.StatusCode == http.StatusOK {
		var histData struct {
			Records []struct {
				Values []struct {
					IP string `json:"ip"`
				} `json:"values"`
				Organizations []string `json:"organizations"`
			} `json:"records"`
		}

		if err := json.NewDecoder(histResp.Body).Decode(&histData); err == nil {
			// Could extract additional info from historical DNS records
			// For now we focus on subdomains
		}
	}

	return nil
}

// CensysProvider implements the Censys Internet scan data provider
type CensysProvider struct {
	limiter    *rate.Limiter
	client     *http.Client
	apiID      string
	apiSecret  string
}

// NewCensysProvider creates a new Censys provider
func NewCensysProvider(config *Config) Provider {
	return &CensysProvider{
		limiter: rate.NewLimiter(rate.Limit(config.GetRateLimit("censys")), 1),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiID:     config.GetAPIKey("censys_id"),
		apiSecret: config.GetAPIKey("censys_secret"),
	}
}

// Name returns the provider name
func (c *CensysProvider) Name() string {
	return "censys"
}

// Fetch retrieves data from Censys search API
func (c *CensysProvider) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if c.apiID == "" || c.apiSecret == "" {
		return fmt.Errorf("Censys API credentials not configured")
	}

	// Rate limiting
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}

	// Search for certificates containing the domain
	searchURL := "https://search.censys.io/api/v2/certificates/search"
	
	query := fmt.Sprintf(`{"q":"names: %s","per_page":100}`, domain)
	
	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, strings.NewReader(query))
	if err != nil {
		return err
	}

	// Basic auth
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.apiID, c.apiSecret)))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "URLPassiveFinder/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Censys API returned status %d", resp.StatusCode)
	}

	// Parse response
	var censysResp struct {
		Result struct {
			Hits []struct {
				Names []string `json:"names"`
			} `json:"hits"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&censysResp); err != nil {
		return err
	}

	// Extract unique domains
	seen := make(map[string]bool)
	for _, hit := range censysResp.Result.Hits {
		for _, name := range hit.Names {
			name = strings.TrimPrefix(name, "*.")
			if name != "" && !seen[name] && strings.Contains(name, domain) {
				seen[name] = true
				results <- fmt.Sprintf("https://%s/", name)
				results <- fmt.Sprintf("http://%s/", name)
			}
		}
	}

	return nil
}

// ThreatCrowdProvider implements the ThreatCrowd threat intelligence provider
type ThreatCrowdProvider struct {
	limiter *rate.Limiter
	client  *http.Client
}

// NewThreatCrowdProvider creates a new ThreatCrowd provider
func NewThreatCrowdProvider(config *Config) Provider {
	return &ThreatCrowdProvider{
		limiter: rate.NewLimiter(rate.Every(10*time.Second), 1), // ThreatCrowd has strict rate limits
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