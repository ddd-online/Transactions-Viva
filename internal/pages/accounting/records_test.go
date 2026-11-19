package accounting

import (
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
	if m.counts != [4]int{3, 2, 1, 0} {
		t.Fatalf("counts = %v, want [3 2 1 0]", m.counts)
	}
	if m.stats != [3]int64{100000, 451200, 0} {
		t.Fatalf("stats = %v", m.stats)
	}
	if m.countText != "共 3 条" {
		t.Fatalf("count text = %q", m.countText)
	}

	m.kind = kindIncome
	m.refilter()
	if len(m.shown) != 1 || m.shown[0].id != "b" {
		t.Fatalf("type filter left %v", m.shown)
	}

	m.kind = ""
	m.sortField, m.sortDesc = sortAmount, true
	m.refilter()
	if m.shown[0].id != "c" {
		t.Fatalf("amount sort first row = %q, want c", m.shown[0].id)
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
