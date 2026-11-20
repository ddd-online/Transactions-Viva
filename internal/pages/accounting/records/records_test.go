package records

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"

	"transactions/internal/domain"
	"transactions/internal/pages/accounting/records/composer"
	tpl "transactions/internal/pages/accounting/template"
	"transactions/internal/store/storetest"
)

func newTestModel(t *testing.T) (*Model, *storetest.Fixture) {
	t.Helper()
	fixture := storetest.Open(t)
	m := New()
	m.ledgers.Reset([]domain.Ledger{fixture.Home, fixture.Travel})
	m.SetDataSource(fixture.Workspace, nil)
	m.rangeStart, m.rangeEnd = "2000-01-01", "2100-01-01"
	m.Reload()
	return m, fixture
}

func TestRecordActions(t *testing.T) {
	m, fixture := newTestModel(t)
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

	m.syncTo(id, fixture.Travel.ID)
	if !strings.Contains(m.notice, "旅行账本") {
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
	m, fixture := newTestModel(t)
	if m.ledgerID != fixture.Home.ID {
		t.Fatalf("the page starts on %q, want the family ledger", m.ledgerID)
	}
	home := len(m.shown)
	if home == 0 {
		t.Fatal("the family ledger has no rows")
	}
	for _, rec := range m.shown {
		if rec.ledgerID != fixture.Home.ID {
			t.Fatalf("row %q leaked from ledger %q", rec.id, rec.ledgerID)
		}
	}

	m.ledgers.SetActive(fixture.Travel.ID)
	m.Reload()
	if len(m.shown) == 0 || len(m.shown) == home {
		t.Fatalf("the travel ledger shows %d rows, the family ledger %d", len(m.shown), home)
	}
	for _, rec := range m.shown {
		if rec.ledgerID != fixture.Travel.ID {
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
	if added == nil || added.ledgerID != fixture.Travel.ID {
		t.Fatalf("the new record = %+v, want the travel ledger", added)
	}

	// The family ledger still has its own rows and does not see the new one.
	m.ledgers.SetActive(fixture.Home.ID)
	m.Reload()
	if got := len(m.shown); got != home {
		t.Fatalf("the family ledger = %d rows, want its original %d", got, home)
	}
}

func TestSaveDraftValidates(t *testing.T) {
	m, _ := newTestModel(t)
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
	var last *record
	for index := range m.records {
		if m.records[index].description == "" && m.records[index].amount == 1200 {
			last = &m.records[index]
		}
	}
	if last == nil || last.category != "餐饮美食" || !last.outlier ||
		last.keyEventDate != "2026-01-01" || len(last.tags) != 1 {
		t.Fatalf("saved row = %+v", last)
	}
}

func TestSaveTemplate(t *testing.T) {
	m, _ := newTestModel(t)
	before := len(m.templates.List(m.ledgerID))
	template, err := m.saveTemplate(tpl.Template{
		Name: "支出 · 测试", Kind: tpl.KindExpense, Category: "测试", Description: "测试",
	})
	if err != nil {
		t.Fatalf("saveTemplate: %v", err)
	}
	if template.ID == "" {
		t.Fatal("the stored template has no id")
	}
	if template.LedgerID != m.ledgerID {
		t.Fatalf("template ledger = %q, want %q", template.LedgerID, m.ledgerID)
	}
	if len(m.templates.List(m.ledgerID)) != before+1 {
		t.Fatalf("templates = %d, want %d", len(m.templates.List(m.ledgerID)), before+1)
	}
}

func TestCategoryNamesFollowTheType(t *testing.T) {
	m, _ := newTestModel(t)
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

// TestRecordSortingIsInSQL drives the header sort against the real store: the
// whole range comes back ordered by the database, not by a page comparison.
func TestRecordSortingIsInSQL(t *testing.T) {
	m, _ := newTestModel(t)
	today := time.Now().Format(dateLayout)
	m.rangeStart, m.rangeEnd = today, today
	m.Reload()
	names := func() []string {
		out := make([]string, len(m.shown))
		for index, rec := range m.shown {
			out[index] = rec.description
		}
		return out
	}
	assertOrder := func(want ...string) {
		t.Helper()
		got := names()
		if len(got) != len(want) {
			t.Fatalf("rows = %v, want %v", got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("rows = %v, want %v", got, want)
			}
		}
	}

	// No sort means newest first, as the SQL default.
	assertOrder("社保代扣", "工资到账", "地铁通勤", "午餐 · 公司楼下", "早餐 · 豆浆油条")

	// Amount descending uses the signed value: income and transfer above
	// expenses, and the smaller expense (-4) above the larger one (-32).
	m.cycleSort(sortAmount)
	assertOrder("工资到账", "社保代扣", "地铁通勤", "早餐 · 豆浆油条", "午餐 · 公司楼下")

	// The second click is ascending.
	m.cycleSort(sortAmount)
	assertOrder("午餐 · 公司楼下", "早餐 · 豆浆油条", "地铁通勤", "社保代扣", "工资到账")

	// The third click removes the column and returns to the date default.
	m.cycleSort(sortAmount)
	if len(m.sorts) != 0 {
		t.Fatalf("sorts = %v, want none", m.sorts)
	}
	assertOrder("社保代扣", "工资到账", "地铁通勤", "午餐 · 公司楼下", "早餐 · 豆浆油条")

	// Clicking amount first makes it the primary key even after date joins.
	m.cycleSort(sortAmount)
	m.cycleSort(sortDate)
	assertOrder("工资到账", "社保代扣", "地铁通勤", "早餐 · 豆浆油条", "午餐 · 公司楼下")

	// Clicking date first makes date primary; amount only breaks ties.
	m.sorts = nil
	m.cycleSort(sortDate)
	m.cycleSort(sortAmount)
	assertOrder("社保代扣", "工资到账", "地铁通勤", "午餐 · 公司楼下", "早餐 · 豆浆油条")
}
