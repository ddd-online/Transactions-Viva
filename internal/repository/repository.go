// Package repository is the data contract between the accounting pages and the
// workspace store. Pages depend on this package (and domain) only; the SQLite
// implementation lives in internal/store, so no page contains SQL.
package repository

import (
	"context"
	"errors"

	"transactions/internal/domain"
)

// ErrDuplicate is returned when a unique key (a ledger name is not unique, but
// a category/tag key is) already exists.
var ErrDuplicate = errors.New("数据已存在")

// ErrNotFound is returned when an update targets a row that does not exist.
var ErrNotFound = errors.New("数据不存在")

// ErrInvalid is returned for a missing ledger id, an unknown transaction type
// or another request the store refuses.
var ErrInvalid = errors.New("无效的数据")

// RecordQuery selects one ledger's records within a local-date range. Both
// dates are inclusive and empty strings mean no bound on that side.
type RecordQuery struct {
	LedgerID  string
	StartDate string
	EndDate   string
}

// RecordEntry is a record with its tag names, the shape pages paint.
type RecordEntry struct {
	Record domain.TransactionRecord
	Tags   []string
}

// LabelSnapshot is one ledger's categories and tags with the record counts the
// 标签 page and the composer/template forms need.
type LabelSnapshot struct {
	Categories     []domain.Category
	Tags           []domain.Tag
	CategoryCounts map[string]int
	TagCounts      map[string]int
}

// Repository is the workspace data the 记账 section uses. Contexts are for
// cancellation and future background loading; the local SQLite implementation
// completes synchronously.
type Repository interface {
	Ledgers(ctx context.Context) ([]domain.Ledger, error)
	// CreateLedger inserts a ledger and seeds its 19 default categories and 57
	// tags in the same transaction.
	CreateLedger(ctx context.Context, name, description string) (domain.Ledger, error)
	// DeleteLedger removes the ledger and its records, tags, categories and
	// templates in one transaction.
	DeleteLedger(ctx context.Context, id string) error

	// Labels returns categories, tags and their counts for one ledger.
	Labels(ctx context.Context, ledgerID string) (LabelSnapshot, error)
	CreateCategory(ctx context.Context, ledgerID, name, transactionType string) error
	// DeleteCategory also deletes the category's tags.
	DeleteCategory(ctx context.Context, ledgerID, name, transactionType string) error
	// ReorderCategories writes sort_order 0..n-1 for the given names.
	ReorderCategories(ctx context.Context, ledgerID, transactionType string, names []string) error

	CreateTag(ctx context.Context, ledgerID, name, categoryTransactionType string) error
	// DeleteTag also removes the tag from transaction_record_tag rows.
	DeleteTag(ctx context.Context, ledgerID, name, categoryTransactionType string) error
	// ReorderTags writes sort_order 0..n-1 for the given names.
	ReorderTags(ctx context.Context, ledgerID, categoryTransactionType string, names []string) error

	Records(ctx context.Context, query RecordQuery) ([]RecordEntry, error)
	Record(ctx context.Context, id string) (RecordEntry, error)
	CreateRecord(ctx context.Context, record domain.TransactionRecord, tags []string) (string, error)
	// UpdateRecord keeps the record id and replaces its tag links.
	UpdateRecord(ctx context.Context, record domain.TransactionRecord, tags []string) error
	DeleteRecord(ctx context.Context, id string) error
	// SyncRecord copies a record and its tags to another ledger with a new id;
	// the key-event date is not copied, as the old row action did.
	SyncRecord(ctx context.Context, id, targetLedgerID string) error

	Templates(ctx context.Context, ledgerID string) ([]domain.TransactionTemplate, error)
	CreateTemplate(ctx context.Context, template domain.TransactionTemplate) (string, error)
	DeleteTemplate(ctx context.Context, id string) error
	// ReorderTemplates writes sort_order 0..n-1 for the given template ids.
	ReorderTemplates(ctx context.Context, ledgerID string, ids []string) error
}
