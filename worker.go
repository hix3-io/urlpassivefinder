package main

import (
	"context"
	"log"
	"sync"
)

// Provider interface that all sources must implement
type Provider interface {
	Name() string
	Fetch(ctx context.Context, domain string, results chan<- string) error
}

// Work represents a unit of work for the pool
type Work struct {
	Domain   string
	Provider Provider
	Results  chan<- string
}

// WorkerPool manages concurrent workers
type WorkerPool struct {
	workers  int
	workChan chan Work
	wg       sync.WaitGroup
	ctx      context.Context
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers:  workers,
		workChan: make(chan Work, workers*10),
	}
}

// Start initializes and starts the worker pool
func (p *WorkerPool) Start(ctx context.Context, providers []Provider) {
	p.ctx = ctx
	
	// Start workers
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// worker processes work items
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	
	for {
		select {
		case <-p.ctx.Done():
			if *verboseFlag {
				log.Printf("Worker %d: shutting down due to context cancellation", id)
			}
			return
		case work, ok := <-p.workChan:
			if !ok {
				if *verboseFlag {
					log.Printf("Worker %d: work channel closed", id)
				}
				return
			}
			
			// Process the work
			if err := work.Provider.Fetch(p.ctx, work.Domain, work.Results); err != nil {
				if *verboseFlag {
					log.Printf("Worker %d: error fetching from %s for %s: %v", 
						id, work.Provider.Name(), work.Domain, err)
				}
			} else if *verboseFlag {
				log.Printf("Worker %d: completed %s for %s", 
					id, work.Provider.Name(), work.Domain)
			}
		}
	}
}

// Submit adds work to the pool
func (p *WorkerPool) Submit(work Work) {
	select {
	case p.workChan <- work:
	case <-p.ctx.Done():
		// Context cancelled, don't submit more work
	}
}

// Wait waits for all workers to complete
func (p *WorkerPool) Wait() {
	close(p.workChan)
	p.wg.Wait()
}