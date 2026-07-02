package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const version = "v0.4.0"

var (
	domainFlag    = flag.String("d", "", "Single domain to query")
	listFlag      = flag.String("l", "", "File containing domains")
	outputFlag    = flag.String("o", "", "Output file (default: stdout)")
	jsonFlag      = flag.Bool("json", false, "JSON output format")
	jsonlFlag     = flag.Bool("jsonl", false, "JSONL output format")
	threadsFlag   = flag.Int("t", 2, "Number of worker threads")
	providersFlag = flag.String("p", "all", "Comma-separated providers to use")
	timeoutFlag   = flag.Duration("timeout", 10*time.Minute, "Overall timeout")
	verboseFlag   = flag.Bool("v", false, "Verbose output")
	versionFlag   = flag.Bool("version", false, "Show version")
	checkFlag     = flag.Bool("check", false, "Check URLs for reachability")
	only200Flag   = flag.Bool("200", false, "Show only 200 OK responses (requires -check)")
	sortSizeFlag  = flag.Bool("size", false, "Sort by response size (requires -check)")
	noFilterFlag  = flag.Bool("nofilter", false, "Disable smart filtering (WordPress, blogs, JS libs)")
	configFlag    = flag.String("c", "", "Config file path")
	filterFlag    = flag.String("filter", "jpg,jpeg,png,gif,css,js,ico,woff,woff2,ttf,eot,svg", "Extensions to filter")
	includeSubsFlag = flag.Bool("include-subs", true, "Include subdomains")
	silentFlag    = flag.Bool("silent", false, "Silent mode (no banner)")
)

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("URLPassiveFinder %s\n", version)
		os.Exit(0)
	}

	if !*silentFlag {
		printBanner()
	}

	// Load configuration
	config, err := LoadConfig(*configFlag)
	if err != nil && *configFlag != "" {
		log.Fatalf("Error loading config: %v", err)
	}

	// Get input domains
	domains := getInputDomains()
	if len(domains) == 0 {
		log.Fatal("No domains provided. Use -d for single domain or -l for list file")
	}

	// Setup context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	// Handle interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if *verboseFlag {
			log.Println("Received interrupt signal, shutting down...")
		}
		cancel()
	}()

	// Initialize components
	dedup := NewDeduplicator()
	filter := NewFilter(strings.Split(*filterFlag, ","), *noFilterFlag)
	output := NewOutput(*outputFlag, *jsonFlag, *jsonlFlag)
	defer output.Close()

	// Initialize providers
	providers := initializeProviders(config)
	if len(providers) == 0 {
		log.Fatal("No providers available")
	}

	// Create worker pool
	pool := NewWorkerPool(*threadsFlag)
	results := make(chan string, 10000)
	
	// Start result processor
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		processResults(results, dedup, filter, output)
	}()

	// Start workers
	pool.Start(ctx, providers)

	// Submit work
	for _, domain := range domains {
		if *verboseFlag {
			log.Printf("Processing domain: %s", domain)
		}
		for _, provider := range providers {
			work := Work{
				Domain:   domain,
				Provider: provider,
				Results:  results,
			}
			pool.Submit(work)
		}
	}

	// Wait for completion
	pool.Wait()
	close(results)
	wg.Wait()

	if !*silentFlag && *verboseFlag {
		printStats(dedup.Count())
	}
}

func printBanner() {
	fmt.Print(`
 _   _ ____  _     ____               _           _____ _           _           
| | | |  _ \| |   |  _ \ __ _ ___ ___(_)_   _____| ____(_)_ __   __| | ___ _ __ 
| | | | |_) | |   | |_) / _` + "`" + ` / __/ __| \ \ / / _ \  _| | | '_ \ / _` + "`" + ` |/ _ \ '__|
| |_| |  _ <| |___|  __/ (_| \__ \__ \ |\ V /  __/ |___| | | | | (_| |  __/ |   
 \___/|_| \_\_____|_|   \__,_|___/___/_| \_/ \___|_____|_|_| |_|\__,_|\___|_|   
                                                                    ` + version + `

`)
}

func getInputDomains() []string {
	var domains []string

	// Single domain from flag
	if *domainFlag != "" {
		domains = append(domains, *domainFlag)
	}

	// Domains from file
	if *listFlag != "" {
		file, err := os.Open(*listFlag)
		if err != nil {
			log.Fatalf("Error opening file: %v", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			domain := strings.TrimSpace(scanner.Text())
			if domain != "" && !strings.HasPrefix(domain, "#") {
				domains = append(domains, domain)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading file: %v", err)
		}
	}

	// Domains from stdin
	if *domainFlag == "" && *listFlag == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				domain := strings.TrimSpace(scanner.Text())
				if domain != "" && !strings.HasPrefix(domain, "#") {
					domains = append(domains, domain)
				}
			}
		}
	}

	return domains
}

func initializeProviders(config *Config) []Provider {
	var providers []Provider
	requestedProviders := strings.Split(*providersFlag, ",")

	// Map of available providers
	availableProviders := map[string]func(*Config) Provider{
		"wayback":        NewWaybackProvider,
		"commoncrawl":    NewCommonCrawlProvider,
		"otx":            NewOTXProvider,
		"urlscan":        NewURLScanProvider,
		"virustotal":     NewVirusTotalProvider,
		"github":         NewGitHubProvider,
		"crtsh":          NewCrtshProvider,
		"shodan":         NewShodanProvider,
		"securitytrails": NewSecurityTrailsProvider,
		"censys":         NewCensysProvider,
		"threatcrowd":    NewThreatCrowdProvider,
	}

	// Initialize requested providers
	for _, name := range requestedProviders {
		name = strings.TrimSpace(strings.ToLower(name))
		
		if name == "all" {
			// Add all available providers
			for providerName, initFunc := range availableProviders {
				if config.IsProviderEnabled(providerName) {
					providers = append(providers, initFunc(config))
					if *verboseFlag {
						log.Printf("Initialized provider: %s", providerName)
					}
				}
			}
			break
		} else if initFunc, exists := availableProviders[name]; exists {
			if config.IsProviderEnabled(name) {
				providers = append(providers, initFunc(config))
				if *verboseFlag {
					log.Printf("Initialized provider: %s", name)
				}
			}
		}
	}

	return providers
}

func processResults(results <-chan string, dedup *Deduplicator, filter *Filter, output *Output) {
	var collectedURLs []string
	for url := range results {
		// Normalize URL
		normalized := NormalizeURL(url)
		
		// Check if already seen
		if dedup.IsSeen(normalized) {
			continue
		}
		dedup.Add(normalized)

		// Apply filters
		if filter.ShouldFilter(normalized) {
			continue
		}

		// Collect URLs for potential checking
		if *checkFlag {
			collectedURLs = append(collectedURLs, url)
		} else {
			// Direct output
			output.Write(url)
		}
	}

	// URL checking phase
	if *checkFlag && len(collectedURLs) > 0 {
		grouped := CheckURLs(collectedURLs, *threadsFlag, *verboseFlag)
		PrintCheckedResults(grouped, *only200Flag, *sortSizeFlag)
		return
	}
}

func printStats(totalURLs int) {
	fmt.Fprintf(os.Stderr, "\n[+] Total unique URLs found: %d\n", totalURLs)
}