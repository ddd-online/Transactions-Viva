// Package repository 的股票数据契约。股票账户属于工作空间，不属于账本；
// 页面只依赖这里的接口，SQLite 实现放在 internal/store。
package repository

import (
	"context"

	"transactions/internal/domain"
)

// StockRepository is the workspace data the 股票 section uses. Every stock row
// is scoped by an account id; the market ladder tables are workspace-level.
//
// The interface grows with each migrated screen; the account methods are the
// foundation every other stock read/write uses.
type StockRepository interface {
	StockAccounts(ctx context.Context) ([]domain.StockAccount, error)
	// CreateStockAccount appends an account; a blank or duplicate name is
	// ErrInvalid / ErrDuplicate.
	CreateStockAccount(ctx context.Context, name, description string) (domain.StockAccount, error)
	// UpdateStockAccount rewrites name and description; a missing account is
	// ErrNotFound.
	UpdateStockAccount(ctx context.Context, id, name, description string) error
	// DeleteStockAccount removes the account and every stock row it owns in one
	// transaction. The last account stays: ErrInvalid.
	DeleteStockAccount(ctx context.Context, id string) error

	// StockAccountSummary is the cash summary (no quotes): principal, available
	// cash, withdrawals, interest and position cost.
	StockAccountSummary(ctx context.Context, accountID string) (domain.StockAccountSummary, error)
	// StockFundRecords reads one page, newest record first; page size is clamped
	// to 10..100.
	StockFundRecords(ctx context.Context, accountID string, page, pageSize int) (domain.StockFundPage, error)
	// The three principal-class commands each write one fund record, recompute
	// the cash chain and log one rollbackable operation in the same transaction.
	AddStockPrincipal(ctx context.Context, accountID string, amountCents int64, date string) error
	WithdrawStock(ctx context.Context, accountID string, amountCents int64, date string) error
	AddStockInterest(ctx context.Context, accountID string, amountCents int64, date string) error

	// StockOperations lists the rollbackable operations, newest first.
	StockOperations(ctx context.Context, accountID string) ([]domain.StockOperation, error)
}
