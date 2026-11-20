package template

import "testing"

func TestStoreAddsMovesAndDeletes(t *testing.T) {
	store := NewStore()
	before := len(store.List())
	added := store.Add(Template{Name: "咖啡 · 餐饮美食", Kind: KindExpense, Category: "餐饮美食"})
	if added.ID == "" {
		t.Fatal("the stored template has no id")
	}
	if len(store.List()) != before+1 || store.List()[before].Name != added.Name {
		t.Fatalf("the template was not appended: %v", store.List())
	}

	store.Move(before, 0)
	if got := store.List()[0].Name; got != added.Name {
		t.Fatalf("after the move: %v", store.List())
	}
	store.Move(0, len(store.List()))
	if got := store.List()[len(store.List())-1].Name; got != added.Name {
		t.Fatalf("after moving to the end: %v", store.List())
	}

	store.Delete(added.ID)
	if len(store.List()) != before {
		t.Fatalf("templates = %d, want %d", len(store.List()), before)
	}
	for _, one := range store.List() {
		if one.ID == added.ID {
			t.Fatal("the deleted template is still listed")
		}
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
