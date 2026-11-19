package accounting

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestRecordsRefilter(t *testing.T) {
	now := time.Now()
	m := &recordsModel{
		records: []record{
			{id: "a", kind: kindExpense, description: "早餐", category: "餐饮", tags: []string{"日常"}, amount: 1200, at: now.Add(-time.Hour)},
			{id: "b", kind: kindIncome, description: "工资", category: "工资奖金", tags: []string{"月薪"}, amount: 100000, at: now.Add(-2 * time.Hour)},
			{id: "c", kind: kindExpense, description: "房租", category: "居住", tags: []string{"固定支出"}, amount: 450000, at: now.Add(-3 * time.Hour)},
		},
		rangeMode: rangeAll,
		sortField: sortDate,
		sortDesc:  true,
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
	m := newRecordsModel()
	m.rangeMode = rangeAll
	m.refilter()
	if len(m.shown) < 2 {
		t.Fatalf("shown = %d, want at least 2", len(m.shown))
	}
	id := m.shown[1].id

	m.openEdit(id)
	if m.draft.editingID != id {
		t.Fatalf("editing ID = %q, want %q", m.draft.editingID, id)
	}
	if m.draft.amount == "" || m.draft.category == "" {
		t.Fatalf("the edit dialog was not filled: %+v", m.draft)
	}
	m.draft.amount = "99.99"
	m.draft.description = "改过的描述"
	m.saveDraft()
	if rec := m.find(id); rec == nil || rec.description != "改过的描述" || rec.amount != 9999 {
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

func TestSplitTags(t *testing.T) {
	tags := splitTags(" 日常, 同事，周末、家庭 ")
	want := []string{"日常", "同事", "周末", "家庭"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %v", tags)
	}
	for index, tag := range want {
		if tags[index] != tag {
			t.Fatalf("tags = %v, want %v", tags, want)
		}
	}
}
