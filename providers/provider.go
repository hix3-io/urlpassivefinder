package providers

import (
	"context"
)

// Provider interface defines the contract for URL providers
type Provider interface {
	Name() string
	Fetch(ctx context.Context, domain string, results chan<- string) error
}