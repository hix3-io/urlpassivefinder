# Best Techniques for Passive URL Discovery - Technical Deep Dive

## Core Techniques Analysis

After analyzing gau, gauplus, waybackurls, waymore, urlfinder, and sigurlfind3r, here are the best techniques extracted from all tools:

## 1. API Query Optimization Techniques

### 1.1 Wayback Machine CDX API
```go
// Best technique from waybackurls & urlfinder
searchURL := fmt.Sprintf(
    "http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=txt&fl=original&collapse=urlkey",
    domain
)
```
**Key optimizations:**
- Use `collapse=urlkey` to deduplicate at source
- `fl=original` to get only URLs (not timestamps)
- `*.domain/*` pattern for subdomain inclusion
- Text output for streaming processing

### 1.2 Common Crawl Index Selection
```python
# Best technique from waymore
def getLatestCommonCrawlIndex():
    resp = requests.get("http://index.commoncrawl.org/collinfo.json")
    indexes = resp.json()
    return indexes[0]["cdx-api"]  # Always use latest index
```
**Key optimizations:**
- Dynamically fetch latest index
- Cache index URL for session
- Use CDX API for pagination support

### 1.3 URLScan Search After
```go
// Best technique from gau
func formatURL(domain string, searchAfter string) string {
    if searchAfter != "" {
        return fmt.Sprintf("%s?q=domain:%s&size=10000&search_after=%s", 
                          baseURL, domain, searchAfter)
    }
    return fmt.Sprintf("%s?q=domain:%s&size=10000", baseURL, domain)
}
```
**Key optimizations:**
- Use `search_after` for deep pagination
- Maximum page size (10000) to reduce requests
- Domain-specific queries

## 2. Concurrent Fetching Patterns

### 2.1 Worker Pool Pattern (Best from gau)
```go
type Runner struct {
    sync.WaitGroup
    Providers []Provider
    threads   uint
}

func (r *Runner) Start(ctx context.Context, workChan chan Work, results chan string) {
    for i := uint(0); i < r.threads; i++ {
        r.Add(1)
        go func() {
            defer r.Done()
            r.worker(ctx, workChan, results)
        }()
    }
}
```
**Key benefits:**
- Controlled concurrency
- Context-based cancellation
- Provider abstraction

### 2.2 Async/Await Pattern (Best from waymore)
```python
async def fetch_all_sources(domain):
    tasks = [
        fetch_wayback(domain),
        fetch_commoncrawl(domain),
        fetch_otx(domain),
        fetch_urlscan(domain)
    ]
    results = await asyncio.gather(*tasks)
    return merge_results(results)
```
**Key benefits:**
- Non-blocking I/O
- Simultaneous source queries
- Easy error aggregation

## 3. Pagination Strategies

### 3.1 Adaptive Pagination (Best from Common Crawl)
```go
// Get total pages first
func getPagination(domain string) (pages int, err error) {
    url := fmt.Sprintf("%s?url=%s/*&showNumPages=true", apiURL, domain)
    resp := makeRequest(url)
    return resp["pages"], nil
}

// Then fetch in parallel
func fetchAllPages(domain string, totalPages int) {
    pageChans := make([]chan []string, totalPages)
    for page := 0; page < totalPages; page++ {
        go fetchPage(domain, page, pageChans[page])
    }
}
```

### 3.2 Resume Token Pattern (Best from URLScan)
```json
// URLScan response includes continuation token
{
    "results": [...],
    "has_more": true,
    "search_after": ["1234567890", "uuid-here"]
}
```

## 4. Rate Limiting & Retry Strategies

### 4.1 Exponential Backoff (Best from waymore)
```python
def retry_with_backoff(func, max_retries=5):
    for attempt in range(max_retries):
        try:
            return func()
        except RateLimitError:
            wait_time = 2 ** attempt + random.uniform(0, 1)
            time.sleep(wait_time)
    raise MaxRetriesExceeded
```

### 4.2 Per-Source Rate Limiting (Best from urlfinder)
```go
type RateLimiter struct {
    limiters map[string]*rate.Limiter
}

func NewRateLimiter() *RateLimiter {
    return &RateLimiter{
        limiters: map[string]*rate.Limiter{
            "wayback":     rate.NewLimiter(10, 1),  // 10 req/s
            "commoncrawl": rate.NewLimiter(5, 1),   // 5 req/s
            "urlscan":     rate.NewLimiter(2, 1),   // 2 req/s
        },
    }
}
```

## 5. Deduplication Techniques

### 5.1 URL Normalization (Best combined approach)
```go
func normalizeURL(rawURL string) string {
    // 1. Decode URL encoding
    decoded, _ := url.QueryUnescape(rawURL)
    
    // 2. Parse URL
    u, _ := url.Parse(decoded)
    
    // 3. Normalize components
    u.Scheme = strings.ToLower(u.Scheme)
    u.Host = strings.ToLower(u.Host)
    
    // 4. Sort query parameters
    u.RawQuery = sortQueryParams(u.Query())
    
    // 5. Remove default ports
    u.Host = removeDefaultPort(u.Host, u.Scheme)
    
    // 6. Remove trailing slash from path
    u.Path = strings.TrimRight(u.Path, "/")
    
    return u.String()
}
```

### 5.2 Efficient Set Operations (Best from gau)
```go
// Using mapset for O(1) lookups
seen := mapset.NewSet[string]()
for url := range results {
    normalized := normalizeURL(url)
    if !seen.Contains(normalized) {
        seen.Add(normalized)
        output <- url
    }
}
```

## 6. Memory Management

### 6.1 Streaming Processing (Best from waybackurls)
```go
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    line := scanner.Text()
    // Process immediately, don't store
    processURL(line)
}
```

### 6.2 Memory Threshold Monitoring (Best from waymore)
```python
def check_memory():
    process = psutil.Process()
    mem_percent = process.memory_percent()
    if mem_percent > MEMORY_THRESHOLD:
        flush_to_disk()
        gc.collect()
```

## 7. URL Extraction & Parsing

### 7.1 Regex Pattern Matching (Combined best)
```go
var urlRegex = regexp.MustCompile(
    `(?i)(?:(?:https?|ftp):\/\/)?` +  // Protocol
    `[\w/\-?=%.]+\.` +                  // Subdomain/path
    `[\w/\-&?=%.]+`                     // Domain/path
)
```

### 7.2 HTML/JS Parsing (Best from response analysis)
```python
# Extract from JavaScript
js_urls = re.findall(r'["\']((https?:)?//[^"\']+)["\']', js_content)

# Extract from HTML attributes
soup = BeautifulSoup(html, 'html.parser')
urls = [tag.get('href') for tag in soup.find_all(['a', 'link'])]
urls += [tag.get('src') for tag in soup.find_all(['script', 'img'])]
```

## 8. Filtering Strategies

### 8.1 Extension Blacklisting (Best from gau)
```go
var defaultBlacklist = []string{
    ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg",
    ".css", ".ico", ".woff", ".woff2", ".ttf", ".eot",
    ".mp4", ".mp3", ".wav", ".avi", ".mov",
}

func shouldSkipURL(url string) bool {
    lower := strings.ToLower(url)
    for _, ext := range blacklist {
        if strings.HasSuffix(lower, ext) {
            return true
        }
    }
    return false
}
```

### 8.2 Smart Scope Control (Best from urlfinder)
```go
type ScopeConfig struct {
    IncludeSubdomains bool
    RootDomainOnly    bool
    PathPrefix        string
}

func isInScope(url string, config ScopeConfig) bool {
    u, _ := url.Parse(url)
    
    if config.RootDomainOnly {
        return getRootDomain(u.Host) == getRootDomain(config.Domain)
    }
    
    if !config.IncludeSubdomains {
        return u.Host == config.Domain
    }
    
    if config.PathPrefix != "" {
        return strings.HasPrefix(u.Path, config.PathPrefix)
    }
    
    return true
}
```

## 9. Error Handling & Resilience

### 9.1 Graceful Degradation (Best from sigurlfind3r)
```go
func fetchFromAllSources(domain string) []string {
    var results []string
    var mu sync.Mutex
    var wg sync.WaitGroup
    
    sources := []Source{wayback, commoncrawl, otx, urlscan}
    
    for _, source := range sources {
        wg.Add(1)
        go func(s Source) {
            defer wg.Done()
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Source %s failed: %v", s.Name(), r)
                }
            }()
            
            urls := s.Fetch(domain)
            mu.Lock()
            results = append(results, urls...)
            mu.Unlock()
        }(source)
    }
    
    wg.Wait()
    return results
}
```

## 10. Output Optimization

### 10.1 JSONL Streaming (Best from urlfinder)
```go
type Result struct {
    URL    string `json:"url"`
    Source string `json:"source"`
    Time   int64  `json:"timestamp,omitempty"`
}

encoder := json.NewEncoder(output)
for result := range results {
    encoder.Encode(result)  // One JSON object per line
}
```

## Optimal Combined Approach

```go
// Pseudo-code for the ultimate URL discovery tool
func DiscoverURLs(domain string) {
    // 1. Setup
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()
    
    results := make(chan string, 10000)
    seen := mapset.NewSet[string]()
    
    // 2. Rate limiters per source
    limiters := setupRateLimiters()
    
    // 3. Worker pool
    pool := NewWorkerPool(10)
    
    // 4. Fetch from all sources concurrently
    sources := []Source{
        NewWaybackSource(limiters["wayback"]),
        NewCommonCrawlSource(limiters["commoncrawl"]),
        NewOTXSource(limiters["otx"]),
        NewURLScanSource(limiters["urlscan"]),
    }
    
    // 5. Stream processing with deduplication
    go func() {
        for url := range results {
            normalized := normalizeURL(url)
            if !seen.Contains(normalized) && !shouldSkipURL(url) {
                seen.Add(normalized)
                fmt.Println(url)
            }
        }
    }()
    
    // 6. Execute with timeout and error handling
    var wg sync.WaitGroup
    for _, source := range sources {
        wg.Add(1)
        pool.Submit(func() {
            defer wg.Done()
            source.FetchURLs(ctx, domain, results)
        })
    }
    
    wg.Wait()
    close(results)
}
```

## Key Takeaways - The Best Techniques

1. **Use CDX API with collapse** for Wayback Machine
2. **Implement worker pools** for concurrent fetching
3. **Stream process results** to manage memory
4. **Normalize URLs before deduplication**
5. **Use per-source rate limiting**
6. **Implement exponential backoff** for retries
7. **Filter at multiple levels** (extension, scope, MIME)
8. **Use JSONL for large outputs**
9. **Handle errors gracefully** per source
10. **Cache API endpoints** and pagination tokens

These techniques, when combined, create the most efficient and comprehensive passive URL discovery system possible.