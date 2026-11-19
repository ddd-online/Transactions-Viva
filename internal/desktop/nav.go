package desktop

import (
	"github.com/vivaui/viva/gfx"
	"transactions/internal/icons"
)

// Section is one entry of the left navigation rail. The order here is also the
// order the rail shows, and the fallback order when a feature is switched off.
type Section uint8

const (
	SectionAccounting Section = iota
	SectionStock
	SectionTodo
	SectionEvents
	SectionDiary
	SectionSettings
)

type navEntry struct {
	section Section
	label   string
	icon    gfx.Icon
}

// navSections is the rail without 应用设置, which sits at the bottom.
var navSections = [...]navEntry{
	{SectionAccounting, "记账", icons.Wallet},
	{SectionStock, "股票", icons.Stock},
	{SectionTodo, "待办", icons.Todo},
	{SectionEvents, "事件", icons.Events},
	{SectionDiary, "日记", icons.Diary},
}

// maxSectionTabs is the widest sub-function list (股票 has seven). The arrays
// below are sized once so a paint never grows them.
const maxSectionTabs = 8

// initNavIDs names the sidebar items in the page's own identity tree. The
// component paints them inside a Draw callback, where c.Key/ui.ID cannot name
// anything, so the model makes the names once and reuses them for every paint.
func (m *Model) initNavIDs() {
	scope := m.tree.Feature("navigation")
	for index, entry := range navSections {
		m.navIDs[index] = m.tree.ID(scope, entry.label)

		labels := m.pageTabs(entry.section)
		if len(labels) == 0 {
			continue
		}
		tabScope := m.tree.Sub(scope, m.tree.Name(entry.label))
		for tabIndex, label := range labels {
			m.subIDs[entry.section][tabIndex] = m.tree.ID(tabScope, label)
		}
	}
	m.navSettingsID = m.tree.ID(scope, "settings")
}

// pageTabs returns the section's second-level entries, or nil for a section
// that has none.
func (m *Model) pageTabs(section Section) []string {
	switch section {
	case SectionAccounting:
		return m.accounting.Tabs()
	case SectionStock:
		return m.stock.Tabs()
	case SectionTodo:
		return m.todo.Tabs()
	}
	return nil
}

// pageTab is the section's chosen sub-function.
func (m *Model) pageTab(section Section) int {
	switch section {
	case SectionAccounting:
		return m.accounting.Tab()
	case SectionStock:
		return m.stock.Tab()
	case SectionTodo:
		return m.todo.Tab()
	}
	return 0
}

// selectTab chooses a section's sub-function.
func (m *Model) selectTab(section Section, index int) {
	switch section {
	case SectionAccounting:
		m.accounting.SelectTab(index)
	case SectionStock:
		m.stock.SelectTab(index)
	case SectionTodo:
		m.todo.SelectTab(index)
	}
}

// Label is the section's name in the title bar and accessible labels.
func (s Section) Label() string {
	if s == SectionSettings {
		return "应用设置"
	}
	for _, entry := range navSections {
		if entry.section == s {
			return entry.label
		}
	}
	return ""
}
