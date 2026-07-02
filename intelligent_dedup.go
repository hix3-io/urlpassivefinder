package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// IntelligentDeduplicator provides advanced deduplication capabilities
type IntelligentDeduplicator struct {
	// Phase 1: URL deduplication
	urlSeen map[string]bool
	urlMu   sync.RWMutex
	
	// Phase 2: Host similarity deduplication
	hostFingerprints map[string]string // host -> content hash
	hostClusters     []HostCluster
	
	// Phase 3: False positive detection
	falsePositiveMap   map[string]bool // host:fingerprint -> is false positive
	confirmedCatchAll  map[string]bool // host:fingerprint -> confirmed catch-all
	pathCatchAll       map[string]string // host:path -> catch-all fingerprint
	baselineTestPaths  []string
	
	// Mutex for thread safety
	fpMu sync.RWMutex
	
	// HTTP client for fingerprinting
	client *http.Client
}

// HostCluster represents a group of similar hosts
type HostCluster struct {
	Representative string   // Primary host
	Members        []string // All hosts in cluster
	Fingerprint    string   // Content fingerprint
	WordSet        map[string]bool // Words for similarity
}

// NewIntelligentDeduplicator creates a new deduplicator instance
func NewIntelligentDeduplicator() *IntelligentDeduplicator {
	return &IntelligentDeduplicator{
		urlSeen:          make(map[string]bool),
		hostFingerprints: make(map[string]string),
		hostClusters:     make([]HostCluster, 0),
		falsePositiveMap: make(map[string]bool),
		confirmedCatchAll: make(map[string]bool),
		pathCatchAll:     make(map[string]string),
		baselineTestPaths: []string{
			"/nonexistent-page-xyz-" + generateRandomString(8),
			"/fakeroute-test-abc-" + generateRandomString(8),
			"/.git/config-test-" + generateRandomString(8),
			"/admin-test-panel-" + generateRandomString(8),
		},
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:    100,
				MaxIdleConnsPerHost: 10,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ========== PHASE 1: URL DEDUPLICATION ==========

// CleanURL strips unnecessary query parameters from static assets
func (d *IntelligentDeduplicator) CleanURL(rawURL string) string {
	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	
	// Normalize scheme and host
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	
	// Check if this is a static asset
	path := strings.ToLower(u.Path)
	staticExtensions := []string{
		".js", ".css", ".jpg", ".jpeg", ".png", ".gif", ".svg",
		".woff", ".woff2", ".ttf", ".eot", ".otf", ".ico",
		".mp4", ".mp3", ".webm", ".ogg", ".wav",
	}
	
	// Strip query params from static assets (cache busting)
	for _, ext := range staticExtensions {
		if strings.Contains(path, ext) {
			u.RawQuery = ""
			u.Fragment = ""
			break
		}
	}
	
	return u.String()
}

// IsURLSeen checks if a URL has been seen (after cleaning)
func (d *IntelligentDeduplicator) IsURLSeen(rawURL string) bool {
	cleanURL := d.CleanURL(rawURL)
	
	d.urlMu.RLock()
	defer d.urlMu.RUnlock()
	return d.urlSeen[cleanURL]
}

// MarkURLSeen marks a URL as seen
func (d *IntelligentDeduplicator) MarkURLSeen(rawURL string) {
	cleanURL := d.CleanURL(rawURL)
	
	d.urlMu.Lock()
	defer d.urlMu.Unlock()
	d.urlSeen[cleanURL] = true
}

// ========== PHASE 2: HOST SIMILARITY DEDUPLICATION ==========

// DeduplicateHostsBySimilarity groups similar hosts using Jaccard similarity
func (d *IntelligentDeduplicator) DeduplicateHostsBySimilarity(hosts []string, similarityThreshold float64) []string {
	if len(hosts) <= 1 {
		return hosts
	}
	
	// Fetch content and create word sets for each host
	type hostData struct {
		host     string
		wordSet  map[string]bool
		hash     string
		hasError bool
	}
	
	hostDataList := make([]hostData, 0, len(hosts))
	
	// Fingerprint each host
	for _, host := range hosts {
		// Try HTTPS first, then HTTP
		var body []byte
		var hash string
		var err error
		
		for _, scheme := range []string{"https", "http"} {
			testURL := scheme + "://" + host + "/"
			body, hash, err = d.fetchAndFingerprint(testURL)
			if err == nil {
				break
			}
		}
		
		if err != nil {
			hostDataList = append(hostDataList, hostData{
				host:     host,
				hasError: true,
			})
			continue
		}
		
		// Create word set for similarity
		wordSet := d.createWordSet(string(body))
		
		hostDataList = append(hostDataList, hostData{
			host:    host,
			wordSet: wordSet,
			hash:    hash,
		})
		
		// Store fingerprint
		d.hostFingerprints[host] = hash
	}
	
	// Cluster similar hosts
	d.hostClusters = make([]HostCluster, 0)
	exactHashMap := make(map[string]int) // hash -> cluster index
	
	for _, hd := range hostDataList {
		if hd.hasError {
			// Keep errored hosts as their own cluster
			d.hostClusters = append(d.hostClusters, HostCluster{
				Representative: hd.host,
				Members:        []string{hd.host},
			})
			continue
		}
		
		// Fast path: exact hash match
		if clusterIdx, exists := exactHashMap[hd.hash]; exists {
			d.hostClusters[clusterIdx].Members = append(d.hostClusters[clusterIdx].Members, hd.host)
			continue
		}
		
		// Find best matching cluster using Jaccard similarity
		bestClusterIdx := -1
		bestSimilarity := 0.0
		
		for i, c := range d.hostClusters {
			if len(c.WordSet) == 0 {
				continue
			}
			similarity := d.jaccardSimilarity(hd.wordSet, c.WordSet)
			if similarity > bestSimilarity {
				bestSimilarity = similarity
				bestClusterIdx = i
			}
		}
		
		if bestClusterIdx >= 0 && bestSimilarity >= similarityThreshold {
			// Add to existing cluster
			d.hostClusters[bestClusterIdx].Members = append(d.hostClusters[bestClusterIdx].Members, hd.host)
		} else {
			// Create new cluster
			newCluster := HostCluster{
				Representative: hd.host,
				Members:        []string{hd.host},
				Fingerprint:    hd.hash,
				WordSet:        hd.wordSet,
			}
			d.hostClusters = append(d.hostClusters, newCluster)
			exactHashMap[hd.hash] = len(d.hostClusters) - 1
		}
	}
	
	// Extract unique hosts (one per cluster)
	unique := make([]string, 0, len(d.hostClusters))
	for _, c := range d.hostClusters {
		unique = append(unique, c.Representative)
	}
	
	return unique
}

// fetchAndFingerprint fetches content and creates fingerprint
func (d *IntelligentDeduplicator) fetchAndFingerprint(testURL string) ([]byte, string, error) {
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	
	// Read first 100KB for fingerprinting
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 100*1024))
	
	// Create SHA256 hash
	hash := sha256.Sum256(body)
	hashStr := hex.EncodeToString(hash[:])
	
	return body, hashStr, nil
}

// createWordSet creates a set of words for Jaccard similarity
func (d *IntelligentDeduplicator) createWordSet(content string) map[string]bool {
	wordSet := make(map[string]bool)
	content = strings.ToLower(content)
	
	// Extract words (3+ chars)
	var word strings.Builder
	for _, r := range content {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			word.WriteRune(r)
		} else {
			if word.Len() >= 3 {
				wordSet[word.String()] = true
			}
			word.Reset()
		}
	}
	if word.Len() >= 3 {
		wordSet[word.String()] = true
	}
	
	return wordSet
}

// jaccardSimilarity calculates Jaccard similarity between two word sets
func (d *IntelligentDeduplicator) jaccardSimilarity(set1, set2 map[string]bool) float64 {
	if len(set1) == 0 || len(set2) == 0 {
		return 0.0
	}
	
	// Count intersection
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}
	
	// Union = |A| + |B| - |A ∩ B|
	union := len(set1) + len(set2) - intersection
	
	if union == 0 {
		return 0.0
	}
	
	return float64(intersection) / float64(union)
}

// ========== PHASE 3: FALSE POSITIVE DETECTION ==========

// DetectFalsePositives probes hosts to detect catch-all responses
func (d *IntelligentDeduplicator) DetectFalsePositives(host string) {
	// Try both HTTP and HTTPS
	for _, scheme := range []string{"https", "http"} {
		d.detectFalsePositivesForScheme(host, scheme)
	}
}

// detectFalsePositivesForScheme detects false positives for a specific scheme
func (d *IntelligentDeduplicator) detectFalsePositivesForScheme(host, scheme string) {
	fingerprints := make(map[string]int)
	successfulProbes := 0
	
	// Test with fake paths
	for _, path := range d.baselineTestPaths {
		testURL := scheme + "://" + host + path
		
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		
		resp, err := d.client.Do(req)
		if err != nil {
			continue
		}
		
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		
		// Only 2xx/3xx responses are catch-all candidates
		// 404 is correct behavior - NOT a catch-all!
		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			continue
		}
		
		successfulProbes++
		fingerprint := d.fingerprintResponse(body)
		fingerprints[fingerprint]++
		
		// Mark as potential false positive
		hostFingerprint := fmt.Sprintf("%s:%s", host, fingerprint)
		d.fpMu.Lock()
		d.falsePositiveMap[hostFingerprint] = true
		d.fpMu.Unlock()
	}
	
	// If same fingerprint for all successful probes, confirm as catch-all
	if successfulProbes >= 2 {
		for fingerprint, count := range fingerprints {
			if count == successfulProbes {
				hostFingerprint := fmt.Sprintf("%s:%s", host, fingerprint)
				d.fpMu.Lock()
				d.confirmedCatchAll[hostFingerprint] = true
				d.fpMu.Unlock()
			}
		}
	}
}

// fingerprintResponse creates a fingerprint from response body
func (d *IntelligentDeduplicator) fingerprintResponse(body []byte) string {
	// Create fingerprint: size_rounded:line_count
	size := len(body)
	sizeRounded := (size / 100) * 100 // Round to nearest 100
	
	lineCount := 1
	for _, b := range body {
		if b == '\n' {
			lineCount++
		}
	}
	
	return fmt.Sprintf("%d:%d", sizeRounded, lineCount)
}

// IsFalsePositive checks if a response is a false positive
func (d *IntelligentDeduplicator) IsFalsePositive(host string, body []byte) bool {
	fingerprint := d.fingerprintResponse(body)
	hostFingerprint := fmt.Sprintf("%s:%s", host, fingerprint)
	
	d.fpMu.RLock()
	defer d.fpMu.RUnlock()
	
	// Check if confirmed catch-all
	if d.confirmedCatchAll[hostFingerprint] {
		// But allow legitimate SPA responses
		if d.isLegitSPA(body) {
			return false
		}
		return true
	}
	
	// Check if potential false positive
	return d.falsePositiveMap[hostFingerprint]
}

// isLegitSPA checks if response is a legitimate SPA application
func (d *IntelligentDeduplicator) isLegitSPA(body []byte) bool {
	bodyStr := strings.ToLower(string(body))
	
	// Must be HTML
	if !strings.Contains(bodyStr, "<html") && !strings.Contains(bodyStr, "<!doctype html") {
		return false
	}
	
	// Check for SPA framework markers
	spaMarkers := []string{
		`<div id="root">`,    // React
		`<div id="app">`,     // Vue
		`<app-root>`,         // Angular
		`__next_data__`,      // Next.js
		`__nuxt__`,           // Nuxt.js
		`chunk-`,             // Webpack chunks
		`/static/js/`,        // Create React App
		`/assets/index-`,     // Vite
		`ng-version`,         // Angular
		`data-reactroot`,     // React
	}
	
	for _, marker := range spaMarkers {
		if strings.Contains(bodyStr, strings.ToLower(marker)) {
			return true
		}
	}
	
	// Large HTML with scripts = likely real SPA
	return len(body) > 10000 && strings.Contains(bodyStr, "<script")
}

// DetectPathCatchAlls detects paths that accept any suffix
func (d *IntelligentDeduplicator) DetectPathCatchAlls(host string, basePaths []string) {
	randomSuffixes := []string{
		"/nonexistent-xyz-test",
		"/fakepath-abc-check",
	}
	
	for _, basePath := range basePaths {
		// Only test executable extensions
		if !d.hasExecutableExtension(basePath) {
			continue
		}
		
		fingerprints := make(map[string]int)
		successfulProbes := 0
		
		for _, suffix := range randomSuffixes {
			for _, scheme := range []string{"https", "http"} {
				testURL := scheme + "://" + host + basePath + suffix
				
				req, err := http.NewRequest("GET", testURL, nil)
				if err != nil {
					continue
				}
				req.Header.Set("User-Agent", "Mozilla/5.0")
				
				resp, err := d.client.Do(req)
				if err != nil {
					continue
				}
				
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					continue
				}
				
				// Only 2xx/3xx are catch-all candidates
				if resp.StatusCode < 200 || resp.StatusCode >= 400 {
					continue
				}
				
				successfulProbes++
				fingerprint := d.fingerprintResponse(body)
				fingerprints[fingerprint]++
			}
		}
		
		// If same fingerprint for all successful probes, mark as path catch-all
		if successfulProbes >= 2 {
			for fingerprint, count := range fingerprints {
				if count == successfulProbes {
					pathKey := fmt.Sprintf("%s:%s", host, basePath)
					d.fpMu.Lock()
					d.pathCatchAll[pathKey] = fingerprint
					d.fpMu.Unlock()
					break
				}
			}
		}
	}
}

// hasExecutableExtension checks if a path has a server-side executable extension
func (d *IntelligentDeduplicator) hasExecutableExtension(path string) bool {
	execExtensions := []string{".php", ".asp", ".aspx", ".jsp", ".jspx", ".cfm", ".cgi", ".pl"}
	lowPath := strings.ToLower(strings.TrimSuffix(path, "/"))
	for _, ext := range execExtensions {
		if strings.HasSuffix(lowPath, ext) {
			return true
		}
	}
	return false
}

// IsPathCatchAll checks if a path is a catch-all for a host
func (d *IntelligentDeduplicator) IsPathCatchAll(host, basePath string) bool {
	pathKey := fmt.Sprintf("%s:%s", host, basePath)
	d.fpMu.RLock()
	_, isCatchAll := d.pathCatchAll[pathKey]
	d.fpMu.RUnlock()
	return isCatchAll
}

// ========== UTILITY FUNCTIONS ==========

// GetClusterReport returns a report of host clusters
func (d *IntelligentDeduplicator) GetClusterReport() []string {
	report := make([]string, 0)
	for _, cluster := range d.hostClusters {
		if len(cluster.Members) > 1 {
			report = append(report, fmt.Sprintf("Cluster: %s + %d similar hosts",
				cluster.Representative, len(cluster.Members)-1))
		}
	}
	return report
}

// GetStats returns deduplication statistics
func (d *IntelligentDeduplicator) GetStats() (urlsSeen int, clusters int, falsePositives int) {
	d.urlMu.RLock()
	urlsSeen = len(d.urlSeen)
	d.urlMu.RUnlock()
	
	clusters = len(d.hostClusters)
	
	d.fpMu.RLock()
	falsePositives = len(d.confirmedCatchAll)
	d.fpMu.RUnlock()
	
	return
}

// generateRandomString generates a random string for testing
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

