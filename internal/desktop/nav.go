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

// navSections is the rail without 设置, which sits at the bottom.
var navSections = [...]navEntry{
	{SectionAccounting, "记账", icons.Wallet},
	{SectionStock, "股票", icons.Stock},
	{SectionTodo, "待办", icons.Todo},
	{SectionEvents, "事件", icons.Events},
	{SectionDiary, "日记", icons.Diary},
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

// closeOverlays closes the floating panels of the section the user is leaving. A
// page that floats a picker or a popup over itself implements CloseOverlays.
func (m *Model) closeOverlays(section Section) {
	var page any
	switch section {
	case SectionAccounting:
		page = m.accounting
	case SectionStock:
		page = m.stock
	case SectionTodo:
		page = m.todo
	}
	if closer, ok := page.(interface{ CloseOverlays() }); ok {
		closer.CloseOverlays()
	}
}

// openSection shows a section and, when it has sub-functions, expands its
// second-level entries and closes the list of the section that was open.
func (m *Model) openSection(section Section) {
	previous, tab := m.section, m.pageTab(m.section)
	m.section = section
	if len(m.pageTabs(section)) > 0 {
		// A section with sub-functions opens them and closes any other
		// section's list; its current sub-function stays selected.
		m.expanded, m.expandedOpen = section, true
	} else {
		m.expandedOpen = false
	}
	m.closeLeft(previous, tab)
}

// openTab shows one of the expanded section's sub-functions.
func (m *Model) openTab(section Section, index int) {
	previous, tab := m.section, m.pageTab(m.section)
	m.section = section
	m.selectTab(section, index)
	m.closeLeft(previous, tab)
}

// closeLeft closes the floating panels of the screen the navigation leaves, so
// a picker or popup does not follow the user to another section or sub-function.
func (m *Model) closeLeft(previous Section, previousTab int) {
	if m.section != previous || m.pageTab(m.section) != previousTab {
		m.closeOverlays(previous)
	}
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
		return "设置"
	}
	for _, entry := range navSections {
		if entry.section == s {
			return entry.label
		}
	}
	return ""
}
