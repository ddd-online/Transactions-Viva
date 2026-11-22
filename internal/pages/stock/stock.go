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

	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/stocklimit"
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

// panel is one screen of the section. Every account-aware panel shares the
// account store.
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
	positions    *positionsPanel
	records      *recordsPanel
	lessons      *lessonsPanel
	statistics   *statisticsPanel
	ladder       *ladderPanel
	settings     *settingsPanel

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
		positions:    newPositionsPanel(accounts, quotes),
		records:      newRecordsPanel(accounts),
		lessons:      newLessonsPanel(accounts),
		statistics:   newStatisticsPanel(accounts),
		ladder:       newLadderPanel(nil),
		settings:     newSettingsPanel(accounts),
	}
	m.records.setQuotes(quotes)
	m.accountPanel.onChange = m.dataChanged
	m.accountPanel.toaster = &m.toaster
	m.positions.onChange = m.dataChanged
	m.positions.toaster = &m.toaster
	m.records.onChange = m.dataChanged
	m.records.toaster = &m.toaster
	m.records.openRound = m.openLessonForRound
	m.lessons.onChange = m.dataChanged
	m.lessons.toaster = &m.toaster
	m.lessons.onOpenRound = m.openRound
	m.statistics.onChange = m.dataChanged
	m.statistics.toaster = &m.toaster
	m.statistics.onOpenRound = m.openRound
	m.settings.toaster = &m.toaster
	m.settings.onChange = m.dataChanged
	m.settings.onAccountsChanged = func() { _ = m.loadAccounts(false) }
	return m
}

// SetRepository connects the section to a workspace. An empty workspace gets a
// default stock account, the way it gets a default ledger and diary book.
func (m *Model) SetRepository(repo repository.StockRepository) error {
	m.repo = repo
	if repo == nil {
		m.accounts.reset(nil)
		m.accountPanel.setRepository(nil)
		m.positions.setRepository(nil)
		m.records.setRepository(nil)
		m.lessons.setRepository(nil)
		m.statistics.setRepository(nil)
		m.settings.setRepository(nil)
		return nil
	}
	if err := m.loadAccounts(true); err != nil {
		return err
	}
	m.accountPanel.setRepository(repo)
	m.positions.setRepository(repo)
	m.records.setRepository(repo)
	m.lessons.setRepository(repo)
	m.statistics.setRepository(repo)
	m.settings.setRepository(repo)
	m.dataChanged()
	return nil
}

// SetLimitService wires the market ladder source. It is called after the
// workspace opens because the ladder cache lives in that workspace.
func (m *Model) SetLimitService(service *stocklimit.Service) {
	m.ladder.limits = service
	m.ladder.have = false
	m.ladder.serial++
	m.ladder.Reload()
}

// QuoteFetcher is the section's quote source, for the shell to build the ladder
// service with the same client.
func (m *Model) QuoteFetcher() stockquote.Fetcher { return m.quotes }

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
		_ = m.SetRepository(nil)
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

// openRound selects a round on the records screen.
func (m *Model) openRound(code, roundID string) {
	m.tab = tabRecords
	m.records.FocusRound(code, roundID)
}

// openLessonForRound opens the lessons composer for a round and switches to it.
func (m *Model) openLessonForRound(code, roundID string) {
	m.tab = tabLessons
	m.lessons.OpenForRound(code, roundID)
}

// SetAccount selects an account by id; the settings page and tests use it.
func (m *Model) SetAccount(id string) {
	if m.accounts.setActive(id) {
		m.dataChanged()
	}
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

// Pointer is the statistics chart's hover.
func (m *Model) Pointer(event gfx.PointerEvent) gfx.EventResult {
	return m.statistics.Pointer(event)
}

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
