package composer

import (
	"errors"
	"testing"

	tpl "transactions/internal/pages/accounting/template"
)

// TestTemplateAppliesToTheDraft checks a chosen template fills the repeating
// fields and leaves the date and amount alone.
func TestTemplateAppliesToTheDraft(t *testing.T) {
	state := New()
	state.Open(Draft{Kind: "收入", Amount: "12.00", Date: "2026-01-02"})
	spec := Spec{Templates: func() []tpl.Template {
		return []tpl.Template{{
			ID: "t1", Name: "午餐", Kind: tpl.KindExpense, Category: "餐饮美食",
			Description: "午餐", Tags: []string{"日常"},
		}}
	}}
	state.templateID = "t1"
	state.applyTemplate(spec)
	if state.draft.Kind != "支出" || state.draft.Category != "餐饮美食" ||
		state.draft.Description != "午餐" || len(state.draft.Tags) != 1 || state.draft.Tags[0] != "日常" {
		t.Fatalf("template did not fill the draft: %+v", state.draft)
	}
	if state.draft.Amount != "12.00" || state.draft.Date != "2026-01-02" {
		t.Fatalf("template changed the amount or the date: %+v", state.draft)
	}
}

// TestSaveKeepsTheDialogOpenOnError checks the page's validation message stays
// on screen, and a taken draft closes the dialog.
func TestSaveKeepsTheDialogOpenOnError(t *testing.T) {
	state := New()
	state.Open(Draft{Kind: "支出", Date: "2026-01-02"})

	failure := errors.New("请填写分类")
	state.save(Spec{OnSave: func(Draft) error { return failure }})
	if !state.IsOpen() {
		t.Fatal("a rejected draft closed the dialog")
	}
	if state.err != failure.Error() {
		t.Fatalf("error shown = %q", state.err)
	}

	saved := false
	state.save(Spec{OnSave: func(Draft) error { saved = true; return nil }})
	if !saved || state.IsOpen() {
		t.Fatal("a taken draft kept the dialog open")
	}
}

// TestSaveTemplateReportsTheStoredTemplate checks the dialog selects what the
// page stored and says so.
func TestSaveTemplateReportsTheStoredTemplate(t *testing.T) {
	state := New()
	state.Open(Draft{Kind: "支出", Category: "餐饮美食", Description: "午餐", Date: "2026-01-02"})
	state.saveTemplate(Spec{OnTemplate: func(one tpl.Template) tpl.Template {
		one.ID = "tpl-1"
		return one
	}})
	if state.templateID != "tpl-1" || state.templateMsg != "模板已保存" {
		t.Fatalf("saving a template left %+v", state)
	}
}
