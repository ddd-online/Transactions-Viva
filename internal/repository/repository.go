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

// RecordSortField is a column the SQL query may order by. It is an enum, not a
// column name, so the store never interpolates caller text into ORDER BY.
type RecordSortField string

const (
	RecordSortDate   RecordSortField = "date"
	RecordSortAmount RecordSortField = "amount"
)

// RecordSort is one ORDER BY key. The first key is the primary sort; the page
// keeps the click order so several columns can be active.
type RecordSort struct {
	Field RecordSortField
	Desc  bool
}

// RecordQuery selects one ledger's records within a local-date range. Both
// dates are inclusive and empty strings mean no bound on that side. Sorts are
// applied in SQL over the whole range; an empty list means newest first.
type RecordQuery struct {
	LedgerID  string
	StartDate string
	EndDate   string
	Sorts     []RecordSort
}

// RecordEntry is a record with its tag names, the shape pages paint.
type RecordEntry struct {
	Record domain.TransactionRecord
	Tags   []string
}

// ChartPoint is one time bucket of one line. Time is "YYYY-MM" for a monthly
// chart and "YYYY" for a yearly chart; missing buckets are present with 0.
type ChartPoint struct {
	Time   string
	Amount int64
}

// ChartLineData is one chart curve, already aligned with the chart's labels.
type ChartLineData struct {
	Label           string
	TransactionType string
	Data            []ChartPoint
}

// ChartStatistics is the range's income / expense / transfer totals, used by the
// analysis summary.
type ChartStatistics struct {
	Income   int64
	Expense  int64
	Transfer int64
}

// ChartQuery asks for a line chart's buckets in a local-date range.
type ChartQuery struct {
	LedgerID    string
	StartDate   string
	EndDate     string
	Granularity string // "month" or "year"
	Lines       []domain.ChartLine
}

// ChartPieQuery asks for a pie chart's groups in a local-date range.
type ChartPieQuery struct {
	LedgerID             string
	StartDate            string
	EndDate              string
	GroupField           string // "transactionType", "category" or "tag"
	GroupTransactionType string
	GroupCategory        string
}

// ChartPieSlice is one positive group of a pie chart.
type ChartPieSlice struct {
	Key    string
	Label  string
	Amount int64
}

// ChartPieResult carries both the slices and the three empty-state counts:
// rangeRecords is everything in the range, filteredRecords / filteredTotal are
// after the type/category scope, and total is the slices' sum (tags may repeat).
type ChartPieResult struct {
	Slices          []ChartPieSlice
	Total           int64
	RangeRecords    int
	FilteredRecords int
	FilteredTotal   int64
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
	// UpdateLedger renames a ledger and replaces its description. An empty
	// description clears it; a missing ledger is ErrNotFound.
	UpdateLedger(ctx context.Context, id, name, description string) error
	// DeleteLedger removes the ledger and its records, tags, categories,
	// charts and templates in one transaction.
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
	// the key-event date is not copied.
	SyncRecord(ctx context.Context, id, targetLedgerID string) error

	// EnsureChartPresets seeds a ledger's two preset charts once. A marker in
	// app_meta means a deleted preset never comes back.
	EnsureChartPresets(ctx context.Context, ledgerID string) error
	Charts(ctx context.Context, ledgerID string) ([]domain.Chart, error)
	CreateChart(ctx context.Context, chart domain.Chart) (string, error)
	// UpdateChart writes a chart's editable fields; a preset keeps its preset
	// flag and creation time.
	UpdateChart(ctx context.Context, chart domain.Chart) error
	DeleteChart(ctx context.Context, chartID string) error
	// ReorderCharts writes sort_order 0..n-1 for the given chart ids.
	ReorderCharts(ctx context.Context, ledgerID string, ids []string) error
	// ChartData aggregates each line into the range's month/year buckets.
	ChartData(ctx context.Context, query ChartQuery) ([]ChartLineData, ChartStatistics, error)
	// ChartPie aggregates one ledger range by type, category or tag.
	ChartPie(ctx context.Context, query ChartPieQuery) (ChartPieResult, error)

	// DiaryBooks lists the workspace's diary books. They belong to the
	// workspace, not to a ledger: a diary is written into a book.
	DiaryBooks(ctx context.Context) ([]domain.DiaryBook, error)
	CreateDiaryBook(ctx context.Context, name, description string) (domain.DiaryBook, error)
	// UpdateDiaryBook renames a book and replaces its description; an empty
	// description clears it.
	UpdateDiaryBook(ctx context.Context, id, name, description string) error
	// DeleteDiaryBook removes a book with its entries. The last book stays:
	// ErrInvalid, as the last ledger's deletion.
	DeleteDiaryBook(ctx context.Context, id string) error

	// DiaryDates lists one book's entry dates, newest first, with the word
	// count and mood the 日记 tree paints.
	DiaryDates(ctx context.Context, bookID string) ([]domain.DiaryDateItem, error)
	// DiaryEntry reads one day; a day without an entry is ErrNotFound.
	DiaryEntry(ctx context.Context, bookID, date string) (domain.DiaryEntry, error)
	// SaveDiary writes one day, replacing the day's entry when it exists (the row
	// keeps its id and created_at, and updated_at moves to this write) and counting
	// the text's code points.
	SaveDiary(ctx context.Context, bookID, date, content, mood string) (domain.DiaryEntry, error)
	DeleteDiary(ctx context.Context, bookID, date string) error
	// DiaryEntries is one book's whole diary, oldest first, for an export.
	DiaryEntries(ctx context.Context, bookID string) ([]domain.DiaryEntry, error)

	// Key events belong to the workspace, not to a ledger: a workspace has one
	// set of events and one event per local date. The 关联事件日期 a record
	// carries is a plain date; the event page reads the records that name a day
	// from every ledger.
	KeyEventsByYear(ctx context.Context, year string) ([]domain.KeyEvent, error)
	// KeyEvent reads one day; a day without an event is ErrNotFound.
	KeyEvent(ctx context.Context, date string) (domain.KeyEvent, error)
	// SaveKeyEvent writes one day's event and names it by the event title rule.
	SaveKeyEvent(ctx context.Context, date, title, content, color string) (domain.KeyEvent, error)
	// DeleteKeyEvent removes the day's event and its image rows in one
	// transaction and returns the images, so the caller can delete their files.
	DeleteKeyEvent(ctx context.Context, date string) ([]domain.KeyEventImage, error)
	KeyEventImages(ctx context.Context, date string) ([]domain.KeyEventImage, error)
	// CreateKeyEventImage appends one image to its day's list; the store assigns
	// its id (when empty) and sort_order.
	CreateKeyEventImage(ctx context.Context, image domain.KeyEventImage) (domain.KeyEventImage, error)
	// DeleteKeyEventImage removes one image row and returns it for file cleanup;
	// a missing id is not an error.
	DeleteKeyEventImage(ctx context.Context, imageID string) (domain.KeyEventImage, error)
	// KeyEventRecords lists the workspace's records whose 关联事件日期 is date,
	// across all ledgers, newest transaction first.
	KeyEventRecords(ctx context.Context, date string) ([]RecordEntry, error)

	Templates(ctx context.Context, ledgerID string) ([]domain.TransactionTemplate, error)
	CreateTemplate(ctx context.Context, template domain.TransactionTemplate) (string, error)
	DeleteTemplate(ctx context.Context, id string) error
	// ReorderTemplates writes sort_order 0..n-1 for the given template ids.
	ReorderTemplates(ctx context.Context, ledgerID string, ids []string) error
}
