// Package stock is the 股票 section: 账户, 持仓, 记录, 心得, 统计, 天梯 and
// 设置. The seven sub-functions are the section's second-level navigation (the
// shell's Sidebar); this page only declares the chosen screen. The screens are
// migrated from the old project's src/pages/stock*.tsx, and the market data and
// fee rules from electron/services/stock-* and shared/domain.
package stock

import (
	"github.com/vivaui/viva/ui"
	"transactions/internal/components"
)

type tab uint8

const (
	tabAccount tab = iota
	tabPositions
	tabRecords
	tabLessons
	tabStatistics
	tabLadder
	tabSettings
)

var tabs = [...]tab{
	tabAccount, tabPositions, tabRecords, tabLessons, tabStatistics, tabLadder, tabSettings,
}

var tabLabels = [...]string{"账户", "持仓", "记录", "心得", "统计", "天梯", "设置"}

var notes = [...]string{
	"资金账户：本金、支取、利息归本、可用现金与总资产。",
	"持仓列表、本轮盈亏、每日复盘，以及加仓 / 减仓 / 清仓。",
	"成交流水、按轮次查看历史、编辑与回滚。",
	"交易心得的列表与编辑。",
	"已实现盈亏、胜率、盈亏比与按轮次汇总的曲线。",
	"连板天梯；需要联网，失败时明确说明而不是留空。",
	"费用参数、本轮复盘模板、交易标签与危险操作。",
}

// Model is the 股票 page's state.
type Model struct {
	tab tab
}

// New returns the default page (账户).
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
