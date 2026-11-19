package accounting

import (
	"testing"
	"time"
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

	m.query = "房"
	m.refilter()
	if len(m.shown) != 1 || m.shown[0].id != "c" {
		t.Fatalf("search left %v", m.shown)
	}

	m.query = ""
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
