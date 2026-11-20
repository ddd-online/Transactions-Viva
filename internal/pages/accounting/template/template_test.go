package template

import (
	"testing"

	"transactions/internal/domain"
)

func TestCacheKeepsOneListPerLedger(t *testing.T) {
	cache := NewCache()
	cache.Set("home", []Template{{ID: "a", LedgerID: "home"}, {ID: "b", LedgerID: "home"}})
	cache.Set("travel", []Template{{ID: "c", LedgerID: "travel"}})
	if got := len(cache.List("home")); got != 2 {
		t.Fatalf("home = %d", got)
	}
	if got := len(cache.List("travel")); got != 1 {
		t.Fatalf("travel = %d", got)
	}
	if got := cache.List("missing"); len(got) != 0 {
		t.Fatalf("missing = %v", got)
	}
	cache.Clear()
	if len(cache.List("home")) != 0 || len(cache.List("travel")) != 0 {
		t.Fatal("Clear left a ledger's templates")
	}
}

func TestDomainConversion(t *testing.T) {
	stored := domain.TransactionTemplate{
		TemplateID: "id", LedgerID: "ledger", TemplateName: "午餐", TransactionType: KindExpense,
		Category: "餐饮美食", Tags: `["三餐","外卖"]`, Flags: "outlier", Description: "午餐",
	}
	page := FromDomain(stored)
	if page.ID != "id" || page.Name != "午餐" || page.Kind != KindExpense || page.Category != "餐饮美食" ||
		len(page.Tags) != 2 || !page.Outlier || page.Description != "午餐" {
		t.Fatalf("page = %+v", page)
	}
	back := ToDomain(page)
	if back.TemplateID != "id" || back.LedgerID != "ledger" || back.Tags != `["三餐","外卖"]` || back.Flags != "outlier" {
		t.Fatalf("back = %+v", back)
	}

	empty := ToDomain(Template{Name: "x"})
	if empty.Tags != "[]" || empty.Flags != "" {
		t.Fatalf("empty = %+v", empty)
	}
	if tags := ParseTags("null"); tags != nil {
		t.Fatalf("null tags = %v", tags)
	}
	if tags := ParseTags("[not json"); tags != nil {
		t.Fatalf("broken tags = %v", tags)
	}
	if !OutlierOf("outlier") || !OutlierOf(`{"outlier":true}`) || OutlierOf("") || OutlierOf(`{"outlier":false}`) {
		t.Fatal("OutlierOf did not read the stored flags")
	}
}

func TestKindLabels(t *testing.T) {
	for _, kind := range Kinds {
		if got := KindValue(KindLabel(kind)); got != kind {
			t.Fatalf("%q round-trips to %q", kind, got)
		}
	}
	if KindLabel(KindTransfer) != "转账" || KindLabel(KindIncome) != "收入" {
		t.Fatal("kind labels are wrong")
	}
}
