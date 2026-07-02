package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ToolConfig represents configuration for each tool
type ToolConfig struct {
	Name       string
	BinaryPath string
	Args       []string
	Timeout    time.Duration
	Enabled    bool
}

// Orchestrator manages the execution of all URL discovery tools
type Orchestrator struct {
	tools       []ToolConfig
	workers     int
	outputFile  string
	verbose     bool
	dedup       map[string]bool
	dedupMutex  sync.RWMutex
	resultsChan chan string
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(workers int, outputFile string, verbose bool) *Orchestrator {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	
	return &Orchestrator{
		tools:       []ToolConfig{},
		workers:     workers,
		outputFile:  outputFile,
		verbose:     verbose,
		dedup:       make(map[string]bool),
		resultsChan: make(chan string, 10000),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// InitializeTools sets up all available tools
func (o *Orchestrator) InitializeTools(domain string) {
	binDir := "bin"
	
	// Define all tools with their configurations
	tools := []ToolConfig{
		{
			Name:       "wayback-urls",
			BinaryPath: filepath.Join(binDir, "wayback-urls"),
			Args:       []string{"-d", domain},
			Timeout:    5 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "crtsh-urls",
			BinaryPath: filepath.Join(binDir, "crtsh-urls"),
			Args:       []string{"-d", domain, "-urls"},
			Timeout:    2 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "commoncrawl-urls",
			BinaryPath: filepath.Join(binDir, "commoncrawl-urls"),
			Args:       []string{"-d", domain, "-max", "1000"},
			Timeout:    3 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "github-search-urls",
			BinaryPath: filepath.Join(binDir, "github-search-urls"),
			Args:       []string{"-d", domain},
			Timeout:    2 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "rapiddns-urls",
			BinaryPath: filepath.Join(binDir, "rapiddns-urls"),
			Args:       []string{"-d", domain, "-urls"},
			Timeout:    2 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "sitemap-urls",
			BinaryPath: filepath.Join(binDir, "sitemap-urls"),
			Args:       []string{"-d", domain, "-robots"},
			Timeout:    1 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "publicwww-urls",
			BinaryPath: filepath.Join(binDir, "publicwww-urls"),
			Args:       []string{"-d", domain},
			Timeout:    2 * time.Minute,
			Enabled:    true,
		},
		{
			Name:       "jsextract-urls",
			BinaryPath: filepath.Join(binDir, "jsextract-urls"),
			Args:       []string{"-d", domain},
			Timeout:    3 * time.Minute,
			Enabled:    true,
		},
	}
	
	// Check which tools exist
	for _, tool := range tools {
		if _, err := os.Stat(tool.BinaryPath); err == nil {
			o.tools = append(o.tools, tool)
			if o.verbose {
				log.Printf("✓ Tool available: %s", tool.Name)
			}
		} else if o.verbose {
			log.Printf("✗ Tool not found: %s", tool.Name)
		}
	}
}

// ExecuteTool runs a single tool and captures its output
func (o *Orchestrator) ExecuteTool(tool ToolConfig) {
	defer o.wg.Done()
	
	if o.verbose {
		log.Printf("🔍 Running %s...", tool.Name)
	}
	
	// Create context with timeout for this specific tool
	ctx, cancel := context.WithTimeout(o.ctx, tool.Timeout)
	defer cancel()
	
	// Execute the tool
	cmd := exec.CommandContext(ctx, tool.BinaryPath, tool.Args...)
	
	// Capture output
	output, err := cmd.StdoutPipe()
	if err != nil {
		if o.verbose {
			log.Printf("❌ Error creating pipe for %s: %v", tool.Name, err)
		}
		return
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		if o.verbose {
			log.Printf("❌ Error starting %s: %v", tool.Name, err)
		}
		return
	}
	
	// Read output line by line
	scanner := bufio.NewScanner(output)
	urlCount := 0
	
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
			// Check deduplication
			o.dedupMutex.RLock()
			seen := o.dedup[url]
			o.dedupMutex.RUnlock()
			
			if !seen {
				o.dedupMutex.Lock()
				o.dedup[url] = true
				o.dedupMutex.Unlock()
				
				// Send to results channel
				select {
				case o.resultsChan <- url:
					urlCount++
				case <-ctx.Done():
					break
				}
			}
		}
	}
	
	// Wait for command to finish
	cmd.Wait()
	
	if o.verbose {
		log.Printf("✓ %s found %d unique URLs", tool.Name, urlCount)
	}
}

// Run starts the orchestrator
func (o *Orchestrator) Run(domain string) error {
	// Initialize tools
	o.InitializeTools(domain)
	
	if len(o.tools) == 0 {
		return fmt.Errorf("no tools available")
	}
	
	// Start output writer
	go o.writeResults()
	
	// Create worker pool
	workerChan := make(chan ToolConfig, len(o.tools))
	
	// Start workers
	for i := 0; i < o.workers; i++ {
		go func() {
			for tool := range workerChan {
				o.ExecuteTool(tool)
			}
		}()
	}
	
	// Submit all tools to worker pool
	for _, tool := range o.tools {
		o.wg.Add(1)
		workerChan <- tool
	}
	
	// Wait for all tools to complete
	o.wg.Wait()
	close(workerChan)
	
	// Close results channel
	close(o.resultsChan)
	
	// Wait a bit for writer to finish
	time.Sleep(1 * time.Second)
	
	return nil
}

// writeResults writes deduplicated results to file or stdout
func (o *Orchestrator) writeResults() {
	var writer *os.File
	var err error
	
	if o.outputFile != "" {
		writer, err = os.Create(o.outputFile)
		if err != nil {
			log.Fatalf("Error creating output file: %v", err)
		}
		defer writer.Close()
	} else {
		writer = os.Stdout
	}
	
	bufferedWriter := bufio.NewWriter(writer)
	defer bufferedWriter.Flush()
	
	for url := range o.resultsChan {
		fmt.Fprintln(bufferedWriter, url)
	}
}

// GetStats returns statistics about the discovery
func (o *Orchestrator) GetStats() (int, int) {
	o.dedupMutex.RLock()
	defer o.dedupMutex.RUnlock()
	return len(o.dedup), len(o.tools)
}

// Main function for the orchestrator
func runOrchestrator() {
	var (
		domain     = flag.String("d", "", "Target domain")
		outputFile = flag.String("o", "", "Output file (default: stdout)")
		workers    = flag.Int("w", 3, "Number of parallel workers")
		verbose    = flag.Bool("v", false, "Verbose output")
		listTools  = flag.Bool("list", false, "List available tools")
	)
	
	flag.Parse()
	
	if *listTools {
		fmt.Println("Available tools:")
		orchestrator := NewOrchestrator(1, "", true)
		orchestrator.InitializeTools("dummy")
		return
	}
	
	if *domain == "" {
		log.Fatal("Domain is required. Use -d flag")
	}
	
	// Print banner
	fmt.Print(`
╔══════════════════════════════════════════════╗
║     URL Discovery Tools Orchestrator         ║
║         Unified Passive Discovery            ║
╚══════════════════════════════════════════════╝
`)
	
	startTime := time.Now()
	
	// Create and run orchestrator
	orchestrator := NewOrchestrator(*workers, *outputFile, *verbose)
	
	fmt.Printf("🎯 Target domain: %s\n", *domain)
	fmt.Printf("⚙️  Workers: %d\n", *workers)
	
	if err := orchestrator.Run(*domain); err != nil {
		log.Fatalf("Error running orchestrator: %v", err)
	}
	
	// Get and display statistics
	totalURLs, toolsUsed := orchestrator.GetStats()
	duration := time.Since(startTime)
	
	fmt.Printf("\n")
	fmt.Printf("╔══════════════════════════════════════════════╗\n")
	fmt.Printf("║                  Summary                     ║\n")
	fmt.Printf("╚══════════════════════════════════════════════╝\n")
	fmt.Printf("✓ Total unique URLs found: %d\n", totalURLs)
	fmt.Printf("✓ Tools used: %d\n", toolsUsed)
	fmt.Printf("✓ Time taken: %s\n", duration.Round(time.Second))
	
	if *outputFile != "" {
		fmt.Printf("✓ Results saved to: %s\n", *outputFile)
	}
}