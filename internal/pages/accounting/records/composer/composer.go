// Package composer is the records page's 新增记录 / 编辑记录 dialog: the form a
// record is written in, and the draft it hands back to the page to save.
package composer

import (
	"strings"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/selectbox"
	tpl "transactions/internal/pages/accounting/template"
)

// kindLabels is the order of the 类型 choice; a record's stored kind is its value
// in kindValue.
var kindLabels = [...]string{"收入", "支出", "转账"}

// markOutlier is the 标记 value that marks a record as an outlier.
const markOutlier = "离群值"

// markLabels are the 标记 choices. 无标记 clears the outlier flag.
var markLabels = [...]string{"无标记", markOutlier}

// Draft is what the dialog edits, and what it hands to the page to save. It is
// the page's shape of a record before validation: money as typed, the date as
// text, and the type as its label.
type Draft struct {
	EditingID   string // empty for a new record
	Kind        string // one of kindLabels
	Amount      string // yuan, as typed
	Category    string
	Description string
	Tags        []string
	Outlier     bool
	EventDate   string // YYYY-MM-DD, empty when nothing is linked
	Date        string // YYYY-MM-DD
}

// Spec is what the page hands the dialog each paint.
type Spec struct {
	// Categories is the 分类 choices for a type, in the page's order. current is
	// the draft's own category, which the list keeps even when it belongs to
	// another type: the old form's autocomplete kept its text when the type
	// changed.
	Categories func(kind, current string) []string
	// TagOptions are the 标签 choices under a type and category; the dialog
	// asks again when either changes.
	TagOptions func(kind, category string) []string
	// Templates are the 模板 choices.
	Templates func() []tpl.Template
	// OnSave validates and writes the draft. A non-nil error keeps the dialog
	// open with the message under its fields.
	OnSave func(Draft) error
	// OnTemplate turns the dialog's repeating fields into a template; it returns
	// the stored one, with its id, and an error the dialog shows.
	OnTemplate func(tpl.Template) (tpl.Template, error)
}

// fields are the FormField scratch values, one per field; ui.FormField keeps its
// layout here between paints.
type fields struct {
	template    ui.FormFieldState
	date        ui.FormFieldState
	kind        ui.FormFieldState
	category    ui.FormFieldState
	tags        ui.FormFieldState
	mark        ui.FormFieldState
	eventDate   ui.FormFieldState
	description ui.FormFieldState
	amount      ui.FormFieldState
}

// tags is the Viva multi-select behind the 标签 field, with the stable identities
// its painted hit regions need.
type tags struct {
	tree    gfx.Tree
	feature gfx.Scope
	options gfx.Scope
	removes gfx.Scope
	input   gfx.ID
	trigger gfx.ID
	dismiss gfx.ID
	scope   gfx.ID
	state   viva.MultiSelectState
	values  []string // the options the state was built from
}

// State is the dialog's own state. Build it with New and keep it in the page's
// model.
type State struct {
	open        bool
	draft       Draft
	fields      fields
	tags        tags
	datePick    datepicker.State
	eventPick   datepicker.State
	mark        string
	templateID  string
	templateMsg string
	templateErr string
	err         string
}

// New builds the dialog's state and its identities.
func New() State {
	var state State
	state.tags.feature = state.tags.tree.Feature("record-tags")
	state.tags.options = state.tags.tree.Sub(state.tags.feature, state.tags.tree.Name("option"))
	state.tags.removes = state.tags.tree.Sub(state.tags.feature, state.tags.tree.Name("remove"))
	state.tags.state.AllowCreate = true
	state.tags.state.MatchCaseInsensitive = true
	state.datePick = datepicker.NewState()
	state.eventPick = datepicker.NewState()
	return state
}

// IsOpen says whether the dialog is showing.
func (state *State) IsOpen() bool { return state.open }

// Open shows the dialog on a draft: a new record, or the one being edited.
func (state *State) Open(draft Draft) {
	state.open = true
	state.draft = draft
	state.templateID, state.templateMsg, state.templateErr, state.err = "", "", "", ""
	state.mark = ""
	if draft.Outlier {
		state.mark = markOutlier
	}
	state.tags.state.Open = false
	state.datePick.Open, state.eventPick.Open = false, false
	state.setTags(draft.Tags)
}

// Close hides the dialog and everything it floats.
func (state *State) Close() {
	state.open = false
	state.tags.state.Open = false
	state.datePick.Open, state.eventPick.Open = false, false
}

// View declares the dialog while it is open.
func (state *State) View(c ui.C, spec Spec) {
	state.syncTagOptions(spec.TagOptions, state.draft.Kind, state.draft.Category)
	title := "新增记录"
	if state.draft.EditingID != "" {
		title = "编辑记录"
	}
	ui.ModalWith(c, &state.open, title, ui.ModalSpec{Width: 640}, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			state.templateField(c, spec)
			ui.Row(c).Gap(12).Children(func() {
				state.formField(c, &state.fields.date, "日期", func() {
					datepicker.Picker(c, &state.datePick,
						state.dateSpec("日期", "YYYY-MM-DD", &state.draft.Date, c.Bounds()))
				})
				state.formField(c, &state.fields.kind, "类型", func() {
					ui.Choice(c, &state.draft.Kind, kindLabels[:], nil).Grow(1)
				})
			})
			ui.Row(c).Gap(12).Children(func() {
				state.formField(c, &state.fields.category, "分类", func() {
					selectbox.Select(c, &state.draft.Category,
						spec.Categories(state.draft.Kind, state.draft.Category), nil).
						Placeholder("选择分类").Label("分类")
				})
				state.formField(c, &state.fields.tags, "标签", func() { state.multiSelect(c, spec) })
			})
			ui.Row(c).Gap(12).Children(func() {
				state.formField(c, &state.fields.mark, "标记", func() {
					selectbox.Select(c, &state.mark, markLabels[:], nil).Placeholder("选择标记")
				})
				state.formField(c, &state.fields.eventDate, "关联事件日期", func() {
					datepicker.Picker(c, &state.eventPick,
						state.dateSpec("关联事件日期", "请选择日期", &state.draft.EventDate, c.Bounds()))
				})
			})
			state.wideFormField(c, &state.fields.description, "描述", func() {
				ui.TextField(c, &state.draft.Description, "描述", "描述消费内容")
			})
			state.wideFormField(c, &state.fields.amount, "金额", func() {
				ui.TextField(c, &state.draft.Amount, "金额", "0.00").Mono()
			})
			if state.err != "" {
				ui.Text(c, state.err).FontSize(11).Color(c.Theme().Danger)
			}
			ui.Divider(c)
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			state.open = false
		}
		if ui.Button(c, "保存").Clicked() {
			state.save(spec)
		}
	})
}

// templateField is the 模板 row: the template picker and 保存为模板.
func (state *State) templateField(c ui.C, spec Spec) {
	state.wideFormField(c, &state.fields.template, "模板", func() {
		ui.Row(c).Gap(8).Children(func() {
			templates := spec.Templates()
			ids := make([]string, len(templates))
			for index := range templates {
				ids[index] = templates[index].ID
			}
			name := func(id string) string {
				for _, template := range templates {
					if template.ID == id {
						return template.Name
					}
				}
				return id
			}
			if selectbox.Select(c, &state.templateID, ids, name).
				Placeholder("选择模板").Grow(1).Changed() {
				state.applyTemplate(spec)
			}
			if ui.Button(c, "保存为模板").Secondary().Clicked() {
				state.saveTemplate(spec)
			}
		})
	})
	if state.templateMsg != "" {
		ui.Text(c, state.templateMsg).FontSize(11).Color(c.Theme().Success)
	}
	if state.templateErr != "" {
		ui.Text(c, state.templateErr).FontSize(11).Color(c.Theme().Danger)
	}
}

// formField declares one half of a two-column form row with Viva's FormField,
// which owns the title and the control rhythm. Basis(0) makes both halves share
// the row equally instead of letting the wider control take more than its share.
func (state *State) formField(c ui.C, field *ui.FormFieldState, label string, body func()) {
	ui.FormField(c, field, label).Grow(1).Basis(0).Height(formFieldHeight).Children(body)
}

// wideFormField is a FormField that takes the dialog's whole width.
func (state *State) wideFormField(c ui.C, field *ui.FormFieldState, label string, body func()) {
	ui.FormField(c, field, label).FillWidth().Height(formFieldHeight).Children(body)
}

// formFieldHeight is the compact form row: the FormField title and its 32 px
// control. FormField reserves description and error rows by default, which this
// dialog does not use, so the fixed row keeps the old modal's density.
const formFieldHeight float32 = 64

// dateSpec is the dialog's single-day picker: the toolbar's DatePicker fixed to
// days, without the grain choice and the period arrows; it writes the day it is
// given back into value and closes.
func (state *State) dateSpec(label, placeholder string, value *string, viewport gfx.Rect) datepicker.Spec {
	return datepicker.Spec{
		Label: label, Placeholder: placeholder, Start: *value,
		Grain: datepicker.ByDay, Fill: true, CloseOnPick: true,
		Viewport: viewport,
		OnRange:  func(start, _ string) { *value = start },
	}
}

// multiSelect paints Viva's multi-select in the 标签 field and routes its pointer
// choices back into the state and the draft.
func (state *State) multiSelect(c ui.C, spec Spec) {
	// These four are focus and input identities, so they come from the page's
	// tree (ui.ID) rather than the component's option scopes.
	state.tags.input = ui.ID(c)
	state.tags.trigger = ui.ID(c)
	state.tags.dismiss = ui.ID(c)
	state.tags.scope = ui.ID(c)
	// The dialog has its own compositing layer; 20 keeps both the field and its
	// popup above it (6 painted them under the dialog, so the field vanished).
	ui.Slot(c).FillWidth().Height(c.Theme().Metrics.ControlHeight).Draw(func(d *ui.DrawContext) {
		viva.PaintMultiSelect(d.Canvas, *d.Theme, d.Rect, &state.tags.state, viva.MultiSelectOptions{
			InputID: state.tags.input, TriggerID: state.tags.trigger,
			DismissID: state.tags.dismiss, Scope: state.tags.scope,
			OptionIDs: state.tags.options, RemoveIDs: state.tags.removes,
			Label: "标签", Placeholder: "选择一个或多个标签",
			EmptyLabel: "没有匹配项", CreatePrefix: "创建：",
			Viewport: c.Bounds(), Layer: 20, MaxRows: 7,
		})
		changed := false
		switch {
		case d.Canvas.Clicked(state.tags.input):
			state.tags.state.Open, changed = true, true
		case d.Canvas.Clicked(state.tags.trigger):
			state.tags.state.Open, changed = !state.tags.state.Open, true
		case d.Canvas.Clicked(state.tags.dismiss):
			state.tags.state.Open, changed = false, true
		default:
			if index, ok := d.Canvas.ClickedIn(state.tags.options); ok {
				changed = state.tags.state.Choose(index) || changed
			}
			if index, ok := d.Canvas.ClickedIn(state.tags.removes); ok {
				changed = state.tags.state.Remove(index) || changed
			}
		}
		state.syncTags()
		if changed {
			d.Canvas.RequestFrame()
		}
	})
}

// syncTagOptions hands the multi-select its choices when they change.
func (state *State) syncTagOptions(options func(kind, category string) []string, kind, category string) {
	if options == nil {
		return
	}
	values := options(kind, category)
	same := len(values) == len(state.tags.values)
	for index := range values {
		if !same || values[index] != state.tags.values[index] {
			same = false
			break
		}
	}
	if same {
		return
	}
	state.tags.values = append(state.tags.values[:0], values...)
	choices := make([]viva.MultiSelectOption, len(values))
	for index, value := range values {
		choices[index] = viva.MultiSelectOption{Value: value, Label: value}
	}
	state.tags.state.SetOptions(choices)
}

// syncTags copies the multi-select's values into the draft.
func (state *State) syncTags() {
	state.draft.Tags = append(state.draft.Tags[:0], state.tags.state.Values()...)
}

// setTags replaces the multi-select's values when the dialog opens or a template
// is applied.
func (state *State) setTags(values []string) {
	query := &state.tags.state.Query
	query.ReplaceRange(text.Range{Start: 0, End: query.RuneCount()}, "")
	state.tags.state.Open = false
	state.tags.state.SetValues(values)
	state.syncTags()
}

// Editor is the native editor of the tags query while the field is focused, for
// Config.TextInput.
func (state *State) Editor(id gfx.ID) *text.Editor {
	if !state.open || id != state.tags.input {
		return nil
	}
	return &state.tags.state.Query
}

// Key routes the tags popup's navigation keys and returns the words typed into
// it; Config.OnKey reaches this through the page.
func (state *State) Key(event gfx.KeyEvent) gfx.EventResult {
	if !state.open {
		return gfx.EventResult{}
	}
	if event.Key == gfx.KeyEscape && (state.datePick.Open || state.eventPick.Open) {
		state.datePick.Open, state.eventPick.Open = false, false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if event.Target != state.tags.input &&
		(!state.tags.state.Open || event.Key != gfx.KeyEscape && event.Key != gfx.KeyTab) {
		return gfx.EventResult{}
	}
	result := state.tags.state.Key(event)
	state.syncTags()
	return result
}

// Scroll moves the tags popup while it is open.
func (state *State) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	if !state.open || !state.tags.state.Open || !state.tags.state.Wheel(event) {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// applyTemplate copies the type, category, description and tags of the chosen
// template into the draft; the date, amount and marks stay as the user set them.
func (state *State) applyTemplate(spec Spec) {
	if spec.Templates == nil {
		return
	}
	for _, entry := range spec.Templates() {
		if entry.ID != state.templateID {
			continue
		}
		state.draft.Kind = tpl.KindLabel(entry.Kind)
		state.draft.Category = entry.Category
		state.draft.Description = entry.Description
		state.setTags(entry.Tags)
		return
	}
}

// saveTemplate hands the dialog's repeating fields to the page as a template and
// selects what it stored.
func (state *State) saveTemplate(spec Spec) {
	if spec.OnTemplate == nil {
		return
	}
	name := state.draft.Kind + " · " + strings.TrimSpace(state.draft.Category)
	if strings.TrimSpace(state.draft.Category) == "" {
		name = state.draft.Kind + " · 模板"
	}
	stored, err := spec.OnTemplate(tpl.Template{
		Name:        name,
		Kind:        tpl.KindValue(state.draft.Kind),
		Category:    strings.TrimSpace(state.draft.Category),
		Description: strings.TrimSpace(state.draft.Description),
		Tags:        append([]string(nil), state.tags.state.Values()...),
		Outlier:     state.mark == markOutlier,
	})
	if err != nil {
		state.templateMsg, state.templateErr = "", "模板保存失败："+err.Error()
		return
	}
	state.templateID = stored.ID
	state.templateMsg, state.templateErr = "模板已保存", ""
}

// save validates the draft through the page and closes when it took it.
func (state *State) save(spec Spec) {
	if spec.OnSave == nil {
		return
	}
	draft := state.draft
	draft.Outlier = state.mark == markOutlier
	draft.Tags = append([]string(nil), state.tags.state.Values()...)
	if err := spec.OnSave(draft); err != nil {
		state.err = err.Error()
		return
	}
	state.err = ""
	state.open = false
}
