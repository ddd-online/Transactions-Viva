package records

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"

	"transactions/internal/pages/accounting/records/composer"
)

func TestRecordsRefilter(t *testing.T) {
	now := time.Now()
	m := &Model{
		records: []record{
			{id: "a", kind: kindExpense, description: "早餐", category: "餐饮", tags: []string{"日常"}, amount: 1200, at: now.Add(-time.Hour)},
			{id: "b", kind: kindIncome, description: "工资", category: "工资奖金", tags: []string{"月薪"}, amount: 100000, at: now.Add(-2 * time.Hour)},
			{id: "c", kind: kindExpense, description: "房租", category: "居住", tags: []string{"固定支出"}, amount: 450000, at: now.Add(-3 * time.Hour)},
		},
		rangeStart: "2000-01-01",
		rangeEnd:   "2100-01-01",
		sortField:  sortDate,
		sortDesc:   true,
	}
	m.refilter()

	if len(m.shown) != 3 {
		t.Fatalf("shown = %d, want 3", len(m.shown))
	}
	if m.shown[0].id != "a" {
		t.Fatalf("newest first: first row = %q", m.shown[0].id)
	}
	if m.stats != [3]int64{100000, 451200, 0} {
		t.Fatalf("stats = %v", m.stats)
	}

	m.sortField, m.sortDesc = sortAmount, true
	m.refilter()
	if m.shown[0].id != "c" {
		t.Fatalf("amount sort first row = %q, want c", m.shown[0].id)
	}
}

func TestRecordActions(t *testing.T) {
	m := New()
	m.rangeStart, m.rangeEnd = "2000-01-01", "2100-01-01"
	m.refilter()
	if len(m.shown) < 2 {
		t.Fatalf("shown = %d, want at least 2", len(m.shown))
	}
	id := m.shown[1].id
	rec := m.find(id)
	if rec == nil {
		t.Fatalf("the page has no row %q", id)
	}

	amount := rec.amount
	if err := m.saveDraft(composer.Draft{
		EditingID:   id,
		Kind:        kindLabel(rec.kind),
		Amount:      "99.99",
		Category:    rec.category,
		Description: "改过的描述",
		Tags:        rec.tags,
		Outlier:     rec.outlier,
		Date:        rec.at.Format(dateLayout),
	}); err != nil {
		t.Fatalf("editing failed: %v", err)
	}
	rec = m.find(id)
	if rec == nil || rec.description != "改过的描述" || rec.amount != 9999 || rec.amount == amount {
		t.Fatalf("edit saved %+v", rec)
	}
	if m.notice != "记录已更新" {
		t.Fatalf("edit notice = %q", m.notice)
	}

	m.syncTo(id, "家庭账本")
	if !strings.Contains(m.notice, "家庭账本") {
		t.Fatalf("sync notice = %q", m.notice)
	}

	before := len(m.records)
	m.deleteRecord(id)
	if m.find(id) != nil {
		t.Fatal("delete left the record")
	}
	if len(m.records) != before-1 {
		t.Fatalf("records = %d, want %d", len(m.records), before-1)
	}
	if m.notice != "记录已删除" {
		t.Fatalf("delete notice = %q", m.notice)
	}
}

func TestLedgerSelectFiltersTheTable(t *testing.T) {
	m := New()
	m.rangeStart, m.rangeEnd = "2000-01-01", "2100-01-01"
	m.refilter()
	if m.ledgerID != ledgerHome {
		t.Fatalf("the page starts on %q, want the family ledger", m.ledgerID)
	}
	home := len(m.shown)
	if home == 0 {
		t.Fatal("the family ledger has no rows")
	}
	for _, rec := range m.shown {
		if rec.ledgerID != ledgerHome {
			t.Fatalf("row %q leaked from ledger %q", rec.id, rec.ledgerID)
		}
	}

	m.ledgerID = ledgerTravel
	m.refilter()
	if len(m.shown) == 0 || len(m.shown) == home {
		t.Fatalf("the travel ledger shows %d rows, the family ledger %d", len(m.shown), home)
	}
	for _, rec := range m.shown {
		if rec.ledgerID != ledgerTravel {
			t.Fatalf("row %q belongs to ledger %q", rec.id, rec.ledgerID)
		}
	}
	if want := [3]int64{30000, 214000, 0}; m.stats != want {
		t.Fatalf("travel statistics = %v, want %v", m.stats, want)
	}

	// A record written from the ledger's page belongs to that ledger.
	m.rangeStart, m.rangeEnd = "2000-01-01", "2100-01-01"
	if err := m.saveDraft(composer.Draft{
		Kind: "支出", Amount: "12.34", Category: "餐饮美食",
		Description: "旅行早餐", Date: "2026-01-02",
	}); err != nil {
		t.Fatalf("saving into the travel ledger: %v", err)
	}
	var added *record
	for index := range m.records {
		if m.records[index].description == "旅行早餐" {
			added = &m.records[index]
		}
	}
	if added == nil || added.ledgerID != ledgerTravel {
		t.Fatalf("the new record = %+v, want the travel ledger", added)
	}

	// The family ledger still has its own rows and does not see the new one.
	m.ledgerID = ledgerHome
	m.refilter()
	if got := len(m.shown); got != home {
		t.Fatalf("the family ledger = %d rows, want its original %d", got, home)
	}
}

func TestSaveDraftValidates(t *testing.T) {
	m := New()
	before := len(m.records)
	cases := []struct {
		name  string
		draft composer.Draft
		want  string
	}{
		{"no category", composer.Draft{Amount: "1", Date: "2026-01-02"}, "请填写分类"},
		{"bad amount", composer.Draft{Category: "餐饮美食", Amount: "x", Date: "2026-01-02"}, "金额格式不正确"},
		{"zero amount", composer.Draft{Category: "餐饮美食", Amount: "0", Date: "2026-01-02"}, "金额需要大于 0"},
		{"bad date", composer.Draft{Category: "餐饮美食", Amount: "1", Date: "2026-13-40"}, "日期格式应为 YYYY-MM-DD"},
		{"bad event date", composer.Draft{Category: "餐饮美食", Amount: "1", Date: "2026-01-02", EventDate: "x"}, "关联事件日期格式应为 YYYY-MM-DD"},
	}
	for _, test := range cases {
		err := m.saveDraft(test.draft)
		if err == nil || err.Error() != test.want {
			t.Errorf("%s: error = %v, want %q", test.name, err, test.want)
		}
	}
	if len(m.records) != before {
		t.Fatalf("a rejected draft wrote a row")
	}

	if err := m.saveDraft(composer.Draft{
		Kind: "支出", Amount: "12.00", Category: "餐饮美食",
		Date: "2026-01-02", EventDate: "2026-01-01", Outlier: true, Tags: []string{"日常"},
	}); err != nil {
		t.Fatalf("a valid draft failed: %v", err)
	}
	last := m.records[len(m.records)-1]
	if last.amount != 1200 || last.category != "餐饮美食" || !last.outlier ||
		last.keyEventDate != "2026-01-01" || len(last.tags) != 1 {
		t.Fatalf("saved row = %+v", last)
	}
}

func TestSaveTemplate(t *testing.T) {
	m := New()
	before := len(m.templates)
	template := m.saveTemplate(composer.Template{
		Name: "支出 · 测试", Kind: "支出", Category: "测试", Description: "测试",
	})
	if template.ID == "" {
		t.Fatal("the stored template has no id")
	}
	if len(m.templates) != before+1 {
		t.Fatalf("templates = %d, want %d", len(m.templates), before+1)
	}
}

func TestCategoryNamesFollowTheType(t *testing.T) {
	m := New()
	expense := m.categoryNames("支出", "")
	if len(expense) == 0 || expense[0] != "餐饮美食" {
		t.Fatalf("支出 categories = %v", expense)
	}
	for _, name := range expense {
		if name == "工资奖金" {
			t.Fatal("an income category leaked into 支出")
		}
	}
	kept := m.categoryNames("收入", "餐饮美食")
	if len(kept) == 0 || kept[len(kept)-1] != "餐饮美食" {
		t.Fatalf("the kept category is missing: %v", kept)
	}
}

func TestTypeColorsMatchTheOldProject(t *testing.T) {
	light, dark := gfx.LightTheme(), gfx.DarkTheme()
	cases := []struct {
		name  string
		theme gfx.Theme
		kind  string
		want  gfx.Color
	}{
		{"light income", light, kindIncome, gfx.RGB24(0x16a34a)},
		{"light expense", light, kindExpense, gfx.RGB24(0xdc2626)},
		{"light transfer", light, kindTransfer, gfx.RGB24(0x3b82f6)},
		{"dark income", dark, kindIncome, gfx.RGB24(0x4ade80)},
		{"dark expense", dark, kindExpense, gfx.RGB24(0xf87171)},
		{"dark transfer", dark, kindTransfer, gfx.RGB24(0x60a5fa)},
	}
	for _, test := range cases {
		if got := kindColor(test.theme, test.kind); got != test.want {
			t.Errorf("%s = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestOutlierColorMatchesTheOldProject(t *testing.T) {
	if got := outlierColor(gfx.LightTheme()); got != gfx.RGB24(0xd97706) {
		t.Errorf("light outlier color = %v", got)
	}
	if got := outlierColor(gfx.DarkTheme()); got != gfx.RGB24(0xfbbf24) {
		t.Errorf("dark outlier color = %v", got)
	}
}
