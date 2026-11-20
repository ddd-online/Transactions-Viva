package analysis

import (
	"strings"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// tagSelect is the curve dialog's 标签 multi-select. The identities come from a
// tree of its own, so its options and removals do not collide with the page.
type tagSelect struct {
	tree    gfx.Tree
	feature gfx.Scope
	options gfx.Scope
	removes gfx.Scope
	input   gfx.ID
	trigger gfx.ID
	dismiss gfx.ID
	scope   gfx.ID
	state   viva.MultiSelectState
	values  []string // the choices the state was built from
}

func (m *Model) initTagSelect() {
	m.tagSelect.feature = m.tagSelect.tree.Feature("analysis-curve-tags")
	m.tagSelect.options = m.tagSelect.tree.Sub(m.tagSelect.feature, m.tagSelect.tree.Name("option"))
	m.tagSelect.removes = m.tagSelect.tree.Sub(m.tagSelect.feature, m.tagSelect.tree.Name("remove"))
	m.tagSelect.state.MatchCaseInsensitive = true
}

// tagField paints Viva's multi-select for the curve's tags. Its choices follow
// the selected transaction type and, when set, category.
func (m *Model) tagField(c ui.C) {
	theme := c.Theme()
	m.syncTagOptions()
	options := m.lineTagOptions()
	disabled := len(options) == 0 && len(m.form.lineTags) == 0
	placeholder := "选择一个或多个标签"
	if disabled {
		placeholder = "该类型下暂无标签"
	}
	// These four are focus and input identities, so they come from the page's
	// tree rather than the component's option scopes.
	m.tagSelect.input = ui.ID(c)
	m.tagSelect.trigger = ui.ID(c)
	m.tagSelect.dismiss = ui.ID(c)
	m.tagSelect.scope = ui.ID(c)
	// The dialog has its own compositing layer; 20 keeps the field and its popup
	// above it.
	ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight).Draw(func(d *ui.DrawContext) {
		viva.PaintMultiSelect(d.Canvas, *d.Theme, d.Rect, &m.tagSelect.state, viva.MultiSelectOptions{
			InputID: m.tagSelect.input, TriggerID: m.tagSelect.trigger,
			DismissID: m.tagSelect.dismiss, Scope: m.tagSelect.scope,
			OptionIDs: m.tagSelect.options, RemoveIDs: m.tagSelect.removes,
			Label: "曲线标签", Placeholder: placeholder,
			EmptyLabel: "没有匹配项", CreatePrefix: "创建：",
			Viewport: c.Bounds(), Layer: 20, MaxRows: 5, Disabled: disabled,
		})
		if disabled {
			return
		}
		changed := false
		switch {
		case d.Canvas.Clicked(m.tagSelect.input):
			m.tagSelect.state.Open, changed = true, true
		case d.Canvas.Clicked(m.tagSelect.trigger):
			m.tagSelect.state.Open, changed = !m.tagSelect.state.Open, true
		case d.Canvas.Clicked(m.tagSelect.dismiss):
			m.tagSelect.state.Open, changed = false, true
		default:
			if index, ok := d.Canvas.ClickedIn(m.tagSelect.options); ok {
				changed = m.tagSelect.state.Choose(index) || changed
			}
			if index, ok := d.Canvas.ClickedIn(m.tagSelect.removes); ok {
				changed = m.tagSelect.state.Remove(index) || changed
			}
		}
		m.syncTags()
		if changed {
			d.Canvas.RequestFrame()
		}
	})
}

// lineTagOptions are the choices for the current type and category. With no
// category, all of the type's tags are available (with names de-duplicated).
func (m *Model) lineTagOptions() []string {
	kind := m.lineKindValue()
	category := strings.TrimSpace(m.form.lineCategory)
	if category != "" {
		return m.tagKeyed[kind+"\x00"+category]
	}
	seen := map[string]bool{}
	options := make([]string, 0, len(m.tagsByKind[kind]))
	for _, tag := range m.tagsByKind[kind] {
		if seen[tag] {
			continue
		}
		seen[tag] = true
		options = append(options, tag)
	}
	return options
}

// syncTagOptions hands the multi-select its choices when the type/category
// changes.
func (m *Model) syncTagOptions() {
	options := m.lineTagOptions()
	// Keep an edited line's existing tags visible even if the category no longer
	// offers them, so they can be removed rather than silently dropped.
	for _, tag := range m.form.lineTags {
		if !contains(options, tag) {
			options = append(options, tag)
		}
	}
	same := len(options) == len(m.tagSelect.values)
	for index := range options {
		if !same || options[index] != m.tagSelect.values[index] {
			same = false
			break
		}
	}
	if same {
		return
	}
	m.tagSelect.values = append(m.tagSelect.values[:0], options...)
	choices := make([]viva.MultiSelectOption, len(options))
	for index, value := range options {
		choices[index] = viva.MultiSelectOption{Value: value, Label: value}
	}
	m.tagSelect.state.SetOptions(choices)
}

// syncTags copies the multi-select's values into the form.
func (m *Model) syncTags() {
	m.form.lineTags = append(m.form.lineTags[:0], m.tagSelect.state.Values()...)
}

// setTags replaces the selected values when the dialog opens.
func (m *Model) setTags(values []string) {
	query := &m.tagSelect.state.Query
	query.ReplaceRange(text.Range{Start: 0, End: query.RuneCount()}, "")
	m.tagSelect.state.Open = false
	m.tagSelect.state.SetValues(values)
	m.syncTags()
}

// resetTagSelect clears the query and the selected values after a type or
// category change.
func (m *Model) resetTagSelect() {
	m.tagSelect.state.Open = false
	query := &m.tagSelect.state.Query
	query.ReplaceRange(text.Range{Start: 0, End: query.RuneCount()}, "")
	m.tagSelect.state.SetValues(nil)
	m.form.lineTags = nil
}

// Key routes the tag popup's navigation keys and query while the curve dialog is
// open. Config.OnKey reaches this through the accounting page.
func (m *Model) Key(event gfx.KeyEvent) gfx.EventResult {
	// A focused chart is stepped with the arrows, Home and End.
	if !m.chartHit.IsZero() && event.Target == m.chartHit {
		return m.chartKey(event)
	}
	if !m.dialogOpen || m.modal != modalLine {
		return gfx.EventResult{}
	}
	if event.Target != m.tagSelect.input &&
		(!m.tagSelect.state.Open || event.Key != gfx.KeyEscape && event.Key != gfx.KeyTab) {
		return gfx.EventResult{}
	}
	result := m.tagSelect.state.Key(event)
	m.syncTags()
	return result
}

// Scroll moves the tag popup while it is open.
func (m *Model) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	if !m.dialogOpen || m.modal != modalLine || !m.tagSelect.state.Open || !m.tagSelect.state.Wheel(event) {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// TagsEditor is the native editor of the tag query while the field is focused,
// for Config.TextInput.
func (m *Model) TagsEditor(id gfx.ID) *text.Editor {
	if !m.dialogOpen || m.modal != modalLine || id != m.tagSelect.input {
		return nil
	}
	return &m.tagSelect.state.Query
}

func splitTagCategory(value string) (category, kind string, ok bool) {
	index := strings.LastIndex(value, ":")
	if index <= 0 || index == len(value)-1 {
		return "", "", false
	}
	return value[:index], value[index+1:], true
}
