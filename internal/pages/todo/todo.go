// Package todo is the 待办 section: 待办 and 历史. The two sub-functions are
// the section's second-level navigation (the shell's Sidebar); this page only
// declares the chosen screen. The screens are migrated from the old project's
// src/pages/todo*; the quadrant, six-level urgency and importance, and card
// ordering rules live in domain.
package todo

import (
	"github.com/vivaui/viva/ui"
	"transactions/internal/components"
)

type tab uint8

const (
	tabBoard tab = iota
	tabHistory
)

var tabs = [...]tab{tabBoard, tabHistory}

var tabLabels = [...]string{"待办", "历史"}

var notes = [...]string{
	"按主题分组的进行中事项、四象限与进度记录。",
	"已完成事项，按完成时间回看。",
}

// Model is the 待办 page's state.
type Model struct {
	tab tab
}

// New returns the default page (待办).
func New() *Model { return &Model{} }

// Tabs are the section's second-level navigation entries.
func (m *Model) Tabs() []string { return tabLabels[:] }

// Tab is the index of the chosen sub-function.
func (m *Model) Tab() int { return int(m.tab) }

// SelectTab chooses a sub-function by index.
func (m *Model) SelectTab(index int) {
	if index >= 0 && index < len(tabs) {
		m.tab = tabs[index]
	}
}

// View declares the page.
func (m *Model) View(c ui.C) {
	components.Pending(c, tabLabels[m.tab], notes[m.tab])
}
