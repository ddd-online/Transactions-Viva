package ledger

import (
	"testing"

	"transactions/internal/domain"
)

func TestStoreResetPicksTheFirstLedger(t *testing.T) {
	store := NewStore()
	if got := len(store.List()); got != 0 || store.Active() != "" {
		t.Fatalf("new store = %v, active %q", store.List(), store.Active())
	}
	store.Reset([]domain.Ledger{{ID: "home", Name: "家庭账本"}, {ID: "travel", Name: "旅行账本"}})
	if got := store.Active(); got != "home" {
		t.Fatalf("active = %q", got)
	}
	if got := store.IDs(); len(got) != 2 || got[0] != "home" || got[1] != "travel" {
		t.Fatalf("ids = %v", got)
	}
	if got := store.Name("travel"); got != "旅行账本" {
		t.Fatalf("name = %q", got)
	}
	if got := store.Name("unknown"); got != "unknown" {
		t.Fatalf("unknown name = %q", got)
	}
}

func TestSetActiveReportsChanges(t *testing.T) {
	store := NewStore()
	store.Reset([]domain.Ledger{{ID: "home", Name: "家庭账本"}, {ID: "travel", Name: "旅行账本"}})
	if store.SetActive("home") {
		t.Fatal("choosing the active ledger reported a change")
	}
	if !store.SetActive("travel") || store.Active() != "travel" {
		t.Fatalf("active = %q", store.Active())
	}
	if store.SetActive("unknown") || store.Active() != "travel" {
		t.Fatal("an unknown ledger replaced the active one")
	}
}

func TestResetKeepsTheActiveLedgerWhenItStillExists(t *testing.T) {
	store := NewStore()
	store.Reset([]domain.Ledger{{ID: "home", Name: "家庭账本"}, {ID: "travel", Name: "旅行账本"}})
	store.SetActive("travel")
	store.Reset([]domain.Ledger{{ID: "home", Name: "家庭账本"}, {ID: "travel", Name: "旅行账本"}, {ID: "new", Name: "新账本"}})
	if store.Active() != "travel" {
		t.Fatalf("active = %q", store.Active())
	}
	store.Reset([]domain.Ledger{{ID: "home", Name: "家庭账本"}})
	if store.Active() != "home" {
		t.Fatalf("active after removing travel = %q", store.Active())
	}
}
