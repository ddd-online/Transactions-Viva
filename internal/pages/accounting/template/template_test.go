package template

import (
	"testing"

	"transactions/internal/pages/accounting/ledger"
)

func TestStoreKeepsOneListPerLedger(t *testing.T) {
	store := NewStore()
	home := store.List(ledger.HomeID)
	travel := store.List(ledger.TravelID)
	if len(home) != 3 || len(travel) != 2 {
		t.Fatalf("home = %d, travel = %d templates", len(home), len(travel))
	}
	if home[0].LedgerID != ledger.HomeID || travel[0].LedgerID != ledger.TravelID {
		t.Fatal("a template is in the wrong ledger's list")
	}

	added := store.Add(Template{LedgerID: ledger.TravelID, Name: "门票 · 娱乐休闲", Kind: KindExpense, Category: "娱乐休闲"})
	if added.ID == "" {
		t.Fatal("the stored template has no id")
	}
	if got := len(store.List(ledger.TravelID)); got != len(travel)+1 {
		t.Fatalf("travel templates = %d, want %d", got, len(travel)+1)
	}
	if got := len(store.List(ledger.HomeID)); got != len(home) {
		t.Fatalf("adding a travel template changed home: %d", got)
	}

	store.Move(ledger.TravelID, len(store.List(ledger.TravelID))-1, 0)
	if got := store.List(ledger.TravelID)[0].ID; got != added.ID {
		t.Fatalf("after the move: %v", store.List(ledger.TravelID))
	}
	if got := store.List(ledger.HomeID)[0].ID; got != "tpl-lunch" {
		t.Fatal("moving a travel template changed the home order")
	}

	store.Delete(added.ID)
	if got := len(store.List(ledger.TravelID)); got != len(travel) {
		t.Fatalf("travel templates = %d, want %d", got, len(travel))
	}
	for _, one := range store.List(ledger.TravelID) {
		if one.ID == added.ID {
			t.Fatal("the deleted template is still listed")
		}
	}

	// A template added without a ledger belongs to the family ledger.
	fallback := store.Add(Template{Name: "测试"})
	if fallback.LedgerID != ledger.HomeID {
		t.Fatalf("fallback ledger = %q", fallback.LedgerID)
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
