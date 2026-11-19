// Package accounting is the 记账 section: 记录, 分析, 标签 and 模板. The four
// sub-functions are the section's second-level navigation (the shell's
// Sidebar), so this page only declares the chosen screen. Each screen is
// migrated from the old project's src/pages/records*, charts*, category-tag*
// and template*.
package accounting

import (
	"github.com/vivaui/viva/ui"
	"transactions/internal/components"
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
	tab tab
}

// New returns the default page (记录).
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
