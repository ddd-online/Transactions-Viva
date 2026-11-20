// Package accounting is the 记账 section: 记录, 分析, 标签 and 模板. The four
// sub-functions are the section's second-level navigation (the shell's
// Sidebar), so this page only declares the chosen screen. Each screen is
// migrated from the old project's src/pages/records*, charts*, category-tag*
// and template*.
package accounting

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components"
	"transactions/internal/pages/accounting/records"
	"transactions/internal/pages/accounting/tags"
	"transactions/internal/pages/accounting/template"
	"transactions/internal/pages/accounting/templates"
)

type tab uint8

const (
	tabRecords tab = iota
	tabAnalysis
	tabTags
	tabTemplates
)

var tabs = [...]tab{tabRecords, tabAnalysis, tabTags, tabTemplates}

var tabLabels = [...]string{"记录", "分析", "标签", "模板"}

var notes = [...]string{
	"记录表格、筛选与收支合计，以及一笔记录的新增 / 编辑 / 删除。",
	"月度与年度趋势、饼图、曲线配置与右侧分组合计。",
	"分类与标签的管理、排序，以及它们和记录的关联。",
	"交易模板的列表、排序与编辑，用于快速记一笔。",
}

// Model is the 记账 page's state. The chosen tab is the section's second-level
// selection; the shell's Sidebar draws and changes it.
type Model struct {
	tab       tab
	records   *records.Model
	tags      *tags.Model
	templates *templates.Model
}

// New returns the default page (记录). The records dialog and the 模板 page
// share one template store, so a template created on either shows in both.
func New() *Model {
	store := template.NewStore()
	return &Model{
		records:   records.NewWithTemplates(store),
		tags:      tags.New(),
		templates: templates.New(store),
	}
}

// Tabs are the section's second-level navigation entries.
func (m *Model) Tabs() []string { return tabLabels[:] }

// Tab is the index of the chosen sub-function.
func (m *Model) Tab() int { return int(m.tab) }

// CloseOverlays closes the floating panels of the chosen screen. The shell calls
// it when the user leaves the sub-function under them, so a picker does not
// follow the user to another screen.
func (m *Model) CloseOverlays() {
	m.records.CloseOverlays()
	m.tags.CloseOverlays()
	m.templates.CloseOverlays()
}

// SelectTab chooses a sub-function by index.
func (m *Model) SelectTab(index int) {
	if index >= 0 && index < len(tabs) {
		m.tab = tabs[index]
	}
}

// View declares the page. The 记录, 标签 and 模板 frontends are migrated with
// mock data; the 分析 tab keeps the honest pending state until its stage.
func (m *Model) View(c ui.C) {
	switch m.tab {
	case tabRecords:
		m.records.View(c)
	case tabTags:
		m.tags.View(c)
	case tabTemplates:
		m.templates.View(c)
	default:
		components.Pending(c, tabLabels[m.tab], notes[m.tab])
	}
}

// Key routes the records page's canvas controls (the tags multi-select) from
// the shell's Config.OnKey.
func (m *Model) Key(event gfx.KeyEvent) gfx.EventResult {
	switch m.tab {
	case tabRecords:
		return m.records.Key(event)
	case tabTemplates:
		return m.templates.Key(event)
	}
	return gfx.EventResult{}
}

// Scroll routes the tags multi-select popup's wheel.
func (m *Model) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	switch m.tab {
	case tabRecords:
		return m.records.Scroll(event)
	case tabTemplates:
		return m.templates.Scroll(event)
	}
	return gfx.EventResult{}
}

// TagsEditor is the native editor behind the tags multi-select while it has
// focus, for Config.TextInput.
func (m *Model) TagsEditor(id gfx.ID) *text.Editor {
	switch m.tab {
	case tabRecords:
		return m.records.TagsEditor(id)
	case tabTemplates:
		return m.templates.TagsEditor(id)
	}
	return nil
}
