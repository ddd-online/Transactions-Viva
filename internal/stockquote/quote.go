// Package stockquote fetches live quotes and stock names. The page depends on
// Fetcher; the concrete Tencent client lives in this package and is wired by the
// shell. A nil fetcher is valid: quotes are simply absent.
package stockquote

import (
	"context"

	"transactions/internal/domain"
)

// Fetcher is the quote source. Quotes returns only the codes it could resolve;
// a network failure is an empty map, not a page error. Name returns an empty
// string when the name is unknown or the request failed.
type Fetcher interface {
	Quotes(ctx context.Context, codes []string) (map[string]domain.StockQuote, error)
	Name(ctx context.Context, code string) (string, error)
}
