// Package repository 的股票数据契约。股票账户属于工作空间，不属于账本；
// 页面只依赖这里的接口，SQLite 实现放在 internal/store。
package repository

import (
	"context"

	"transactions/internal/domain"
)

// StockFill 是委托里的一笔成交。价格单位厘，手数是 A 股手数（1 手 = 100 股）。
type StockFill struct {
	PriceMilli int64
	Lots       int
}

// StockOrderRequest 是一次下单。RoundFee 只在建仓时接受；其余轮次沿用建仓快照。
type StockOrderRequest struct {
	AccountID string
	StockCode string
	StockName string
	TradeType string
	TradeTime int64
	Remark    string
	Tag       string
	Fills     []StockFill
	RoundFee  *domain.RoundFee
}

// StockTradeUpdate edits one fill. TradeTime <= 0 keeps the stored time.
type StockTradeUpdate struct {
	AccountID  string
	TradeID    string
	PriceMilli int64
	Lots       int
	TradeTime  int64
}

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

	// CreateStockOrder records one order (one or more fills) and rebuilds the
	// position, rounds, fund record and operation log in one transaction.
	CreateStockOrder(ctx context.Context, request StockOrderRequest) ([]domain.StockTrade, error)
	// UpdateStockTrade edits one fill's price / lots / time, recomputes the
	// whole order's fees and replays the account.
	UpdateStockTrade(ctx context.Context, request StockTradeUpdate) error
	// DeleteStockOrder removes every fill of one order and replays the account.
	DeleteStockOrder(ctx context.Context, accountID, orderID string) error
	// RollbackStockOperation undoes the newest operation; a target that no
	// longer exists reports skipped instead of failing.
	RollbackStockOperation(ctx context.Context, accountID string) (domain.StockOperationRollback, error)

	// Raw reads used by the account / positions / history / lessons screens.
	StockPositions(ctx context.Context, accountID string) ([]domain.StockPosition, error)
	// StockTrades returns every trade of the account, time ascending, including
	// the hidden 0-share close markers.
	StockTrades(ctx context.Context, accountID string) ([]domain.StockTrade, error)
	StockRounds(ctx context.Context, accountID string) ([]domain.StockTradeRound, error)
	StockHistories(ctx context.Context, accountID string) ([]domain.StockTradeHistory, error)
	StockDayReviews(ctx context.Context, accountID string) ([]domain.StockDayReview, error)
	StockLessons(ctx context.Context, accountID string) ([]domain.StockLesson, error)
	StockLedgerEntries(ctx context.Context, accountID string) ([]domain.StockLedgerEntry, error)

	UpdateStockPositionReview(ctx context.Context, accountID, stockCode, review string) error
	UpdateStockPositionTag(ctx context.Context, accountID, stockCode, tag string) error
	// UpdateStockDayReview writes (or clears, when review is empty) one current
	// round's day review and returns the stock's current day reviews.
	UpdateStockDayReview(ctx context.Context, accountID, stockCode, date, review string) ([]domain.StockDayReview, error)
	UpdateStockRoundReview(ctx context.Context, accountID, roundID, review string) error
	UpdateStockRoundTag(ctx context.Context, accountID, roundID, tag string) error

	StockRoundOptions(ctx context.Context, accountID string) ([]domain.StockRoundOption, error)
	CreateStockLesson(ctx context.Context, accountID, text, roundID string) (domain.StockLesson, error)
	UpdateStockLesson(ctx context.Context, accountID, lessonID, text, roundID string) (domain.StockLesson, error)
	DeleteStockLesson(ctx context.Context, accountID, lessonID string) error

	StockLedger(ctx context.Context, accountID, stockCode, roundID string) (*domain.StockLedgerEntry, error)
	SaveStockLedger(ctx context.Context, entry domain.StockLedgerEntry) (domain.StockLedgerEntry, error)

	// RefreshStockName rewrites the displayed name copies after a quote lookup.
	// It does not touch updated_at of the history collection or any text
	// snapshot in fund records / operation logs.
	RefreshStockName(ctx context.Context, accountID, stockCode, stockName string) error

	StockFeeSetting(ctx context.Context, accountID string) (domain.StockFeeSetting, error)
	SaveStockFeeSetting(ctx context.Context, accountID string, fee domain.RoundFee) (domain.StockFeeSetting, error)
	StockTradeTags(ctx context.Context, accountID string) ([]string, error)
	SaveStockTradeTags(ctx context.Context, accountID string, tags []string) ([]string, error)
	StockReviewTemplates(ctx context.Context, accountID string) (domain.StockReviewSetting, error)
	// SaveStockReviewTemplate keeps the other template when daily is nil.
	SaveStockReviewTemplate(ctx context.Context, accountID, template string, daily *string) (domain.StockReviewSetting, error)

	StockStatistics(ctx context.Context, query domain.StockStatisticsQuery) (domain.StockStatistics, error)
	// ResetStockAccount clears the account's holdings / trades / settings and
	// sets its principal to zero; the named account itself stays.
	ResetStockAccount(ctx context.Context, accountID string) error
	// ArchiveStockAccount moves the account's data to a new account; the old
	// account keeps its name and settings but loses the data and principal.
	ArchiveStockAccount(ctx context.Context, accountID, name string) (domain.StockAccount, error)

	// Market ladder cache. These rows are workspace-level, not account-scoped.
	StockLimitDay(ctx context.Context, date string) (*domain.StockLimitDay, error)
	StockLimitEntries(ctx context.Context, date string) ([]domain.StockLimitEntry, error)
	SaveStockLimitDay(ctx context.Context, date, status string, count int, fetchedAt int64) error
	SaveStockLimitEntries(ctx context.Context, date string, entries []domain.StockLimitEntry, fetchedAt int64) error
	StockLimitQuotes(ctx context.Context, date string) (map[string]domain.StockLimitQuote, error)
	SaveStockLimitQuotes(ctx context.Context, date string, quotes map[string]domain.StockLimitQuote, fetchedAt int64) error
}
