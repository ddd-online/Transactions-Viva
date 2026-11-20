package ledger

import "testing"

func TestStoreStartsOnTheFamilyLedger(t *testing.T) {
	store := NewStore()
	if got := len(store.List()); got != 2 {
		t.Fatalf("ledgers = %d, want 2", got)
	}
	if got := store.Active(); got != HomeID {
		t.Fatalf("active = %q, want %q", got, HomeID)
	}
	if got := store.IDs(); len(got) != 2 || got[0] != HomeID || got[1] != TravelID {
		t.Fatalf("ids = %v", got)
	}
	if got := store.Name(TravelID); got != "旅行账本" {
		t.Fatalf("name = %q", got)
	}
	if got := store.Name("unknown"); got != "unknown" {
		t.Fatalf("unknown name = %q", got)
	}
}

func TestSetActiveReportsChanges(t *testing.T) {
	store := NewStore()
	if store.SetActive(HomeID) {
		t.Fatal("choosing the active ledger reported a change")
	}
	if !store.SetActive(TravelID) || store.Active() != TravelID {
		t.Fatalf("active = %q", store.Active())
	}
	if store.SetActive("unknown") || store.Active() != TravelID {
		t.Fatal("an unknown ledger replaced the active one")
	}
}
