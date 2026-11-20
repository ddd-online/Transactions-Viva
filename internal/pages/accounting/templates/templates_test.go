package templates

import (
	"testing"

	"transactions/internal/pages/accounting/ledger"
	tpl "transactions/internal/pages/accounting/template"
)

func TestNewDefaults(t *testing.T) {
	m := New(tpl.NewStore())
	if got := len(m.items()); got != 3 {
		t.Fatalf("templates = %d, want 3", got)
	}
	if got := len(m.categories[tpl.KindExpense]); got != 9 {
		t.Fatalf("expense categories = %d, want 9", got)
	}
	if got := len(m.categories[tpl.KindIncome]); got != 8 {
		t.Fatalf("income categories = %d, want 8", got)
	}
	if got := len(m.categories[tpl.KindTransfer]); got != 2 {
		t.Fatalf("transfer categories = %d, want 2", got)
	}
	if got := len(m.tags[tagKey(tpl.KindExpense, "餐饮美食")]); got != 4 {
		t.Fatalf("餐饮美食 tags = %d, want 4", got)
	}
}

func TestFormStartsOnExpenseAndResets(t *testing.T) {
	m := New(tpl.NewStore())
	m.openForm()
	if !m.formOpen || m.form.kind != tpl.KindExpense || m.form.name != "" || m.form.category != "" {
		t.Fatalf("the new form is %+v", m.form)
	}
	m.form.name, m.form.category, m.form.err = "x", "餐饮美食", "错误"
	m.form.tags = []string{"咖啡"}
	m.closeForm()
	if m.formOpen || m.form.name != "" || m.form.category != "" || m.form.err != "" || len(m.form.tags) != 0 {
		t.Fatalf("the closed form kept %+v", m.form)
	}
}

func TestSaveRequiresNameAndCategory(t *testing.T) {
	m := New(tpl.NewStore())
	m.openForm()
	m.save()
	if m.form.err != "模板名称不能为空" {
		t.Fatalf("empty name error = %q", m.form.err)
	}
	m.form.name = " 咖啡外带 "
	m.save()
	if m.form.err != "分类不能为空" {
		t.Fatalf("empty category error = %q", m.form.err)
	}

	m.form.category = "餐饮美食"
	m.form.description = " 外带 "
	m.form.tags = []string{"咖啡"}
	m.form.outlier = true
	before := len(m.items())
	m.save()
	if m.formOpen || m.form.err != "" {
		t.Fatalf("a saved form stayed open: %+v", m.form)
	}
	if len(m.items()) != before+1 {
		t.Fatalf("templates = %d, want %d", len(m.items()), before+1)
	}
	stored := m.items()[before]
	if stored.Name != "咖啡外带" || stored.Kind != tpl.KindExpense ||
		stored.Description != "外带" || !stored.Outlier || len(stored.Tags) != 1 {
		t.Fatalf("stored = %+v", stored)
	}
	if m.form.name != "" || m.form.kind != tpl.KindExpense || m.form.outlier {
		t.Fatalf("the form did not reset: %+v", m.form)
	}
}

func TestKindAndCategoryClearTheirDependants(t *testing.T) {
	m := New(tpl.NewStore())
	m.openForm()
	m.form.category, m.form.tags, m.form.err = "餐饮美食", []string{"咖啡"}, "x"
	m.changeKind()
	if m.form.category != "" || len(m.form.tags) != 0 || m.form.err != "" {
		t.Fatalf("changeKind left %+v", m.form)
	}
	m.form.category, m.form.tags, m.form.err = "工资奖金", []string{"工资"}, "x"
	m.changeCategory()
	if len(m.form.tags) != 0 || m.form.err != "" {
		t.Fatalf("changeCategory left %+v", m.form)
	}
	if m.form.category != "工资奖金" {
		t.Fatal("changeCategory cleared the category")
	}
}

func TestTagSelectionFollowsTheCategory(t *testing.T) {
	m := New(tpl.NewStore())
	m.openForm()
	m.form.category = "餐饮美食"
	m.syncTagOptions()
	if got := len(m.tagSelect.state.Choices()); got != 4 {
		t.Fatalf("餐饮美食 choices = %d, want 4", got)
	}
	m.tagSelect.state.SetValues([]string{"咖啡", "外卖"})
	m.syncTags()
	if len(m.form.tags) != 2 || m.form.tags[0] != "咖啡" {
		t.Fatalf("tags = %v", m.form.tags)
	}
	m.resetTagSelect()
	if len(m.form.tags) != 0 || len(m.tagSelect.state.Values()) != 0 {
		t.Fatalf("reset left %v / %v", m.form.tags, m.tagSelect.state.Values())
	}

	m.tagSelect.state.SetValues([]string{"咖啡"})
	m.syncTags()
	m.changeCategory()
	if len(m.form.tags) != 0 || len(m.tagSelect.state.Values()) != 0 {
		t.Fatal("changing the category kept the old tags")
	}
}

func TestDelete(t *testing.T) {
	m := New(tpl.NewStore())
	before := len(m.items())
	one := m.items()[0]
	m.askDelete(one)
	if !m.deleteOpen || m.deleteName != one.Name {
		t.Fatalf("the confirmation did not open on %+v", one)
	}
	m.confirmDelete()
	if m.deleteOpen || len(m.items()) != before-1 {
		t.Fatalf("templates = %d, want %d", len(m.items()), before-1)
	}
	for _, entry := range m.items() {
		if entry.ID == one.ID {
			t.Fatal("the deleted template is still listed")
		}
	}
}

func TestListReorderMovesTheStore(t *testing.T) {
	m := New(tpl.NewStore())
	first := m.items()[0]
	m.list.Reorder([]int{0}, 2)
	if got := m.items()[1].ID; got != first.ID {
		t.Fatalf("after moving the first row before the third: %v", m.items())
	}
}

func TestCloseOverlays(t *testing.T) {
	m := New(tpl.NewStore())
	m.formOpen, m.deleteOpen, m.form.err = true, true, "x"
	m.CloseOverlays()
	if m.formOpen || m.deleteOpen || m.form.err != "" {
		t.Fatal("CloseOverlays left a dialog open")
	}
}

func TestLedgerSwitchShowsItsTemplates(t *testing.T) {
	m := New(tpl.NewStore())
	if got := len(m.items()); got != 3 {
		t.Fatalf("family templates = %d, want 3", got)
	}
	m.ledgers.SetActive(ledger.TravelID)
	m.syncLedger()
	items := m.items()
	if len(items) != 2 || items[0].ID != "tpl-flight" {
		t.Fatalf("travel templates = %v", items)
	}
	if m.ledgerID != ledger.TravelID {
		t.Fatalf("ledger = %q", m.ledgerID)
	}

	// A template created now belongs to the travel ledger.
	m.form.name, m.form.category = "门票", "娱乐休闲"
	m.save()
	if got := m.items(); len(got) != 3 || got[2].LedgerID != ledger.TravelID {
		t.Fatalf("after saving: %v", got)
	}
	if got := m.store.List(ledger.HomeID); len(got) != 3 {
		t.Fatalf("the family ledger = %d templates, want 3", len(got))
	}
}
