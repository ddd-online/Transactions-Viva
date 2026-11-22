// Package template holds the 记账 · 模板 shape and the in-memory cache the
// composer and the 模板 page read while they paint. The database is the source
// of truth (internal/store); this package only converts stored rows and keeps
// the current ledger's list warm so a paint never queries SQLite.
package template

import (
	"encoding/json"
	"strings"

	"transactions/internal/domain"
)

// Kind values are the transaction types a template belongs to. They are the
// stored form; KindLabel is what the page shows.
const (
	KindExpense  = "expense"
	KindIncome   = "income"
	KindTransfer = "transfer"
)

// Kinds are the template form's types, in the form's order.
var Kinds = [...]string{KindExpense, KindIncome, KindTransfer}

// KindLabel is the Chinese label of a stored kind.
func KindLabel(kind string) string {
	switch kind {
	case KindIncome:
		return "收入"
	case KindTransfer:
		return "转账"
	}
	return "支出"
}

// KindValue is the stored kind of a Chinese label; an unknown label is 支出.
func KindValue(label string) string {
	switch label {
	case "收入":
		return KindIncome
	case "转账":
		return KindTransfer
	}
	return KindExpense
}

// Template is one shortcut for a record's repeating fields. The mark is stored
// for the list, but applying a template leaves the draft's mark alone, as the
// dialog does.
type Template struct {
	ID          string
	LedgerID    string
	Name        string
	Kind        string // one of Kinds
	Category    string
	Description string
	Tags        []string
	Outlier     bool
}

// Cache holds the active ledger's templates in display order. The 记录 composer
// and the 模板 page share one Cache, so a template saved in either shows in both
// immediately; internal/store reloads it after a write.
type Cache struct {
	items map[string][]Template
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{items: map[string][]Template{}}
}

// Set replaces one ledger's templates. The slice is copied so the caller may
// reuse its own slice.
func (c *Cache) Set(ledgerID string, items []Template) {
	if c == nil {
		return
	}
	if c.items == nil {
		c.items = map[string][]Template{}
	}
	c.items[ledgerID] = append([]Template(nil), items...)
}

// List is one ledger's templates in display order.
func (c *Cache) List(ledgerID string) []Template {
	if c == nil {
		return nil
	}
	return c.items[ledgerID]
}

// Clear drops every ledger's cache, used when the workspace changes.
func (c *Cache) Clear() {
	if c == nil {
		return
	}
	c.items = map[string][]Template{}
}

// FromDomain converts one stored template to the page shape.
func FromDomain(one domain.TransactionTemplate) Template {
	return Template{
		ID:          one.TemplateID,
		LedgerID:    one.LedgerID,
		Name:        one.TemplateName,
		Kind:        one.TransactionType,
		Category:    one.Category,
		Description: one.Description,
		Tags:        ParseTags(one.Tags),
		Outlier:     OutlierOf(one.Flags),
	}
}

// ToDomain converts a page template to the stored shape. Tags become a JSON
// array and the mark becomes the flags string ("outlier" or empty).
func ToDomain(one Template) domain.TransactionTemplate {
	return domain.TransactionTemplate{
		TemplateID:      one.ID,
		LedgerID:        one.LedgerID,
		TemplateName:    one.Name,
		TransactionType: one.Kind,
		Category:        one.Category,
		Tags:            EncodeTags(one.Tags),
		Flags:           FlagsOf(one.Outlier),
		Description:     one.Description,
	}
}

// ParseTags reads the tags JSON array. A null or malformed value is empty, as
// the service did.
func ParseTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

// EncodeTags writes a JSON array, using [] for an empty set.
func EncodeTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// OutlierOf reads the template flags. Templates stored "outlier" as a bare
// string; imported data may also carry the record JSON shape.
func OutlierOf(flags string) bool {
	flags = strings.TrimSpace(flags)
	if flags == "outlier" {
		return true
	}
	if strings.HasPrefix(flags, "{") {
		var parsed struct {
			Outlier bool `json:"outlier"`
		}
		if json.Unmarshal([]byte(flags), &parsed) == nil {
			return parsed.Outlier
		}
	}
	return false
}

// FlagsOf is the stored flags string for a template's mark.
func FlagsOf(outlier bool) string {
	if outlier {
		return "outlier"
	}
	return ""
}
