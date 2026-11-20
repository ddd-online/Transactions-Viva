// Package template is the in-memory list of 记账 · 模板 entries: the 记录
// composer applies them and the 模板 page manages them, so the accounting model
// gives both the same Store for the run. Templates belong to a ledger, as they
// do in the old application; the store keeps one ordered list per ledger. The
// mock defaults and the storage are replaced by internal/store later.
package template

import (
	"strconv"

	"transactions/internal/pages/accounting/ledger"
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
// old dialog does.
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

// Store holds every ledger's templates, each in display order. It is shared by
// the 记录 composer and the 模板 page while the store is mock.
type Store struct {
	items  map[string][]Template
	nextID int
}

// NewStore returns the mock default templates: three for the family ledger and
// two for the travel ledger.
func NewStore() *Store {
	return &Store{items: map[string][]Template{
		ledger.HomeID: {
			{
				ID: "tpl-lunch", LedgerID: ledger.HomeID, Name: "午餐 · 餐饮美食", Kind: KindExpense,
				Category: "餐饮美食", Description: "午餐", Tags: []string{"日常"},
			},
			{
				ID: "tpl-commute", LedgerID: ledger.HomeID, Name: "地铁通勤 · 交通出行", Kind: KindExpense,
				Category: "交通出行", Description: "地铁通勤", Tags: []string{"通勤"},
			},
			{
				ID: "tpl-salary", LedgerID: ledger.HomeID, Name: "工资到账 · 工资奖金", Kind: KindIncome,
				Category: "工资奖金", Description: "工资到账", Tags: []string{"月薪"}, Outlier: true,
			},
		},
		ledger.TravelID: {
			{
				ID: "tpl-flight", LedgerID: ledger.TravelID, Name: "机票 · 交通出行", Kind: KindExpense,
				Category: "交通出行", Description: "机票",
			},
			{
				ID: "tpl-stay", LedgerID: ledger.TravelID, Name: "民宿 · 娱乐休闲", Kind: KindExpense,
				Category: "娱乐休闲", Description: "民宿两晚",
			},
		},
	}}
}

// List is one ledger's templates in display order.
func (s *Store) List(ledgerID string) []Template { return s.items[ledgerID] }

// Add appends a template to its ledger, gives it an ID when it has none, and
// returns what it stored. The tags are copied, so the caller may reuse its
// slice.
func (s *Store) Add(one Template) Template {
	if one.LedgerID == "" {
		one.LedgerID = ledger.HomeID
	}
	s.nextID++
	if one.ID == "" {
		one.ID = "tpl-new-" + strconv.Itoa(s.nextID)
	}
	one.Tags = append([]string(nil), one.Tags...)
	s.items[one.LedgerID] = append(s.items[one.LedgerID], one)
	return one
}

// Delete removes the template with the id from whichever ledger holds it.
func (s *Store) Delete(id string) {
	for ledgerID, items := range s.items {
		for index := range items {
			if items[index].ID != id {
				continue
			}
			s.items[ledgerID] = append(items[:index], items[index+1:]...)
			return
		}
	}
}

// Move takes the ledger's template at from and puts it before to, where to is an
// index in the ledger's list as it was before the move.
func (s *Store) Move(ledgerID string, from, to int) {
	items := s.items[ledgerID]
	if from < 0 || from >= len(items) || to < 0 || to > len(items) {
		return
	}
	if to > from {
		to--
	}
	if to == from {
		return
	}
	one := items[from]
	if from < to {
		copy(items[from:to], items[from+1:to+1])
	} else {
		copy(items[to+1:from+1], items[to:from])
	}
	items[to] = one
	s.items[ledgerID] = items
}
