// Package stock is the 股票 section. It owns the workspace's stock accounts and
// the seven screens; the old project's per-ledger stock data becomes one account
// per old ledger when imported. Stock accounts belong to the workspace, not to a
// ledger.
package stock

import (
	"context"
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components"
	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/stockquote"
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

// panel is one screen of the section. Every panel shares the account store.
type panel interface {
	View(c ui.C)
	Reload()
	CloseOverlays()
}

// Model is the 股票 page's state.
type Model struct {
	tab      tab
	repo     repository.StockRepository
	accounts *accountStore
	quotes   stockquote.Fetcher

	accountPanel *accountPanel

	// pending panels are filled in by the later migration milestones.
	positions  *pendingPanel
	records    *pendingPanel
	lessons    *pendingPanel
	statistics *pendingPanel
	ladder     *pendingPanel
	settings   *pendingPanel

	toaster ui.Toaster
}

// New returns the empty section. desktop calls SetRepository once a workspace
// is open; quotes may be nil, in which case market values fall back to cost.
func New(quotes stockquote.Fetcher) *Model {
	accounts := newAccountStore()
	m := &Model{
		accounts:     accounts,
		quotes:       quotes,
		accountPanel: newAccountPanel(accounts, quotes),
		positions:    newPendingPanel("持仓", "持仓列表、本轮盈亏与成交记录。"),
		records:      newPendingPanel("记录", "已清仓股票的轮次与复盘。"),
		lessons:      newPendingPanel("心得", "交易心得的列表与编辑。"),
		statistics:   newPendingPanel("统计", "已实现盈亏、胜率与回撤。"),
		ladder:       newPendingPanel("天梯", "按连板数排列的涨停梯队。"),
		settings:     newPendingPanel("设置", "费用、标签、复盘模板与数据管理。"),
	}
	m.accountPanel.onChange = m.dataChanged
	m.accountPanel.toaster = &m.toaster
	return m
}

// SetRepository connects the section to a workspace. An empty workspace gets a
// default stock account, the way it gets a default ledger and diary book.
func (m *Model) SetRepository(repo repository.StockRepository) error {
	m.repo = repo
	if repo == nil {
		m.accounts.reset(nil)
		m.accountPanel.setRepository(nil)
		return nil
	}
	if err := m.loadAccounts(true); err != nil {
		return err
	}
	m.accountPanel.setRepository(repo)
	m.dataChanged()
	return nil
}

// loadAccounts rereads the workspace's accounts and keeps the active one when
// it survived. When ensure is true an empty workspace gets 默认股票账户.
func (m *Model) loadAccounts(ensure bool) error {
	if m.repo == nil {
		return nil
	}
	items, err := m.repo.StockAccounts(context.Background())
	if err != nil {
		return fmt.Errorf("读取股票账户失败：%w", err)
	}
	if len(items) == 0 && ensure {
		if _, err := m.repo.CreateStockAccount(context.Background(), "默认股票账户", ""); err != nil {
			return fmt.Errorf("创建默认股票账户失败：%w", err)
		}
		if items, err = m.repo.StockAccounts(context.Background()); err != nil {
			return fmt.Errorf("读取股票账户失败：%w", err)
		}
	}
	m.accounts.reset(items)
	return nil
}

// Reload rereads the accounts and every panel. The shell calls it when the
// stock section is shown.
func (m *Model) Reload() {
	if m.repo == nil {
		m.SetRepository(nil)
		return
	}
	if err := m.loadAccounts(true); err != nil {
		m.accountPanel.setError(err.Error())
		return
	}
	m.dataChanged()
}

// Accounts are the workspace's stock accounts, in display order.
func (m *Model) Accounts() []domain.StockAccount { return m.accounts.list() }

// ActiveAccount is the id the stock tabs are reading.
func (m *Model) ActiveAccount() string { return m.accounts.activeID() }

// CreateAccount adds an account and makes it the active one.
func (m *Model) CreateAccount(name, description string) error {
	if m.repo == nil {
		return fmt.Errorf("还没有打开工作空间")
	}
	created, err := m.repo.CreateStockAccount(context.Background(), name, description)
	if err != nil {
		return err
	}
	if err := m.loadAccounts(false); err != nil {
		return err
	}
	m.accounts.setActive(created.ID)
	m.dataChanged()
	return nil
}

// UpdateAccount rewrites an account's name and description.
func (m *Model) UpdateAccount(id, name, description string) error {
	if m.repo == nil {
		return fmt.Errorf("还没有打开工作空间")
	}
	if err := m.repo.UpdateStockAccount(context.Background(), id, name, description); err != nil {
		return err
	}
	return m.loadAccounts(false)
}

// DeleteAccount removes an account and its stock data. The last account stays.
func (m *Model) DeleteAccount(id string) error {
	if m.repo == nil {
		return fmt.Errorf("还没有打开工作空间")
	}
	if err := m.repo.DeleteStockAccount(context.Background(), id); err != nil {
		return err
	}
	if err := m.loadAccounts(true); err != nil {
		return err
	}
	m.dataChanged()
	return nil
}

// dataChanged reloads every screen after an account switch or a write that a
// different tab can see.
func (m *Model) dataChanged() {
	m.accountPanel.Reload()
	m.positions.Reload()
	m.records.Reload()
	m.lessons.Reload()
	m.statistics.Reload()
	m.settings.Reload()
}

// Tabs are the section's second-level navigation entries.
func (m *Model) Tabs() []string { return tabLabels[:] }

// Tab is the index of the chosen sub-function.
func (m *Model) Tab() int { return int(m.tab) }

// SelectTab chooses a sub-function by index and reloads it.
func (m *Model) SelectTab(index int) {
	if index < 0 || index >= len(tabs) {
		return
	}
	changed := m.tab != tabs[index]
	m.tab = tabs[index]
	if !changed || m.repo == nil {
		return
	}
	switch m.tab {
	case tabAccount:
		m.accountPanel.Reload()
	case tabPositions:
		m.positions.Reload()
	case tabRecords:
		m.records.Reload()
	case tabLessons:
		m.lessons.Reload()
	case tabStatistics:
		m.statistics.Reload()
	case tabLadder:
		m.ladder.Reload()
	case tabSettings:
		m.settings.Reload()
	}
}

// CloseOverlays closes the floating panels of the chosen screen.
func (m *Model) CloseOverlays() {
	m.accountPanel.CloseOverlays()
	m.positions.CloseOverlays()
	m.records.CloseOverlays()
	m.lessons.CloseOverlays()
	m.statistics.CloseOverlays()
	m.ladder.CloseOverlays()
	m.settings.CloseOverlays()
}

// Pointer is the statistics chart's hover. It is a no-op until that screen is
// migrated.
func (m *Model) Pointer(event gfx.PointerEvent) gfx.EventResult { return gfx.EventResult{} }

// View declares the page.
func (m *Model) View(c ui.C) {
	switch m.tab {
	case tabAccount:
		m.accountPanel.View(c)
	case tabPositions:
		m.positions.View(c)
	case tabRecords:
		m.records.View(c)
	case tabLessons:
		m.lessons.View(c)
	case tabStatistics:
		m.statistics.View(c)
	case tabLadder:
		m.ladder.View(c)
	case tabSettings:
		m.settings.View(c)
	}
	ui.Toasts(c, &m.toaster)
}

// pendingPanel is a screen that has not been migrated yet.
type pendingPanel struct {
	title string
	note  string
}

func newPendingPanel(title, note string) *pendingPanel {
	return &pendingPanel{title: title, note: note}
}

func (p *pendingPanel) View(c ui.C)    { components.Pending(c, p.title, p.note) }
func (p *pendingPanel) Reload()        {}
func (p *pendingPanel) CloseOverlays() {}
