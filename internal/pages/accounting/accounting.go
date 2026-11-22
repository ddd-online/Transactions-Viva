// Package accounting is the 记账 section: 记录, 分析, 标签 and 模板. The four
// sub-functions are the section's second-level navigation (the shell's
// Sidebar), so this page only declares the chosen screen. Each screen is
// shared shell for the accounting screens.
// and template*.
package accounting

import (
	"context"
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components"
	"transactions/internal/domain"
	"transactions/internal/pages/accounting/analysis"
	"transactions/internal/pages/accounting/ledger"
	"transactions/internal/pages/accounting/records"
	"transactions/internal/pages/accounting/tags"
	tpl "transactions/internal/pages/accounting/template"
	"transactions/internal/pages/accounting/templates"
	"transactions/internal/repository"
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
	analysis  *analysis.Model
	tags      *tags.Model
	templates *templates.Model
	ledgers   *ledger.Store
	cache     *tpl.Cache
	repo      repository.Repository
}

// New returns the empty accounting section over a ledger store of its own.
func New() *Model { return NewWithLedgers(ledger.NewStore()) }

// NewWithLedgers returns the section over a ledger store the shell also gives to
// the 日记 page, so one 账本 choice is shared by every page that reads a ledger.
// The accounting pages share one store, so their 账本 Selectors always agree, and
// the records dialog and the 模板 page share one template cache. desktop calls
// SetRepository once a workspace is open.
func NewWithLedgers(ledgers *ledger.Store) *Model {
	if ledgers == nil {
		ledgers = ledger.NewStore()
	}
	sharedTemplates := tpl.NewCache()
	return &Model{
		records:   records.NewWithStores(ledgers, sharedTemplates),
		analysis:  analysis.New(ledgers),
		tags:      tags.NewWithLedger(ledgers),
		templates: templates.NewWithLedger(sharedTemplates, ledgers),
		ledgers:   ledgers,
		cache:     sharedTemplates,
	}
}

// SetRepository connects the section to a workspace. An empty workspace gets a
// 「默认账本」 with the application's default categories and tags, so the
// records page is usable immediately. Switching to nil clears the pages.
func (m *Model) SetRepository(repo repository.Repository) error {
	m.CloseOverlays()
	m.repo = nil
	m.cache.Clear()
	if repo == nil {
		m.ledgers.Reset(nil)
		m.records.SetDataSource(nil, nil)
		m.analysis.SetDataSource(nil, nil)
		m.tags.SetDataSource(nil, nil)
		m.templates.SetDataSource(nil, nil)
		return nil
	}
	ctx := context.Background()
	ledgers, err := repo.Ledgers(ctx)
	if err != nil {
		return fmt.Errorf("读取账本失败：%w", err)
	}
	if len(ledgers) == 0 {
		if _, err := repo.CreateLedger(ctx, "默认账本", ""); err != nil {
			return fmt.Errorf("创建默认账本失败：%w", err)
		}
		ledgers, err = repo.Ledgers(ctx)
		if err != nil {
			return fmt.Errorf("读取账本失败：%w", err)
		}
	}
	m.ledgers.Reset(ledgers)
	m.repo = repo
	m.records.SetDataSource(repo, m.ledgerChanged)
	m.analysis.SetDataSource(repo, m.ledgerChanged)
	m.tags.SetDataSource(repo, m.ledgerChanged)
	m.templates.SetDataSource(repo, m.ledgerChanged)
	return nil
}

// LedgerStore is the shared list, for the shell to hand the 日记 page: both
// pages then show and follow the same 账本.
func (m *Model) LedgerStore() *ledger.Store { return m.ledgers }

// Ledgers are the workspace's ledgers, in the order the selectors show them.
// The settings page paints the same list, so a rename there is visible here.
func (m *Model) Ledgers() []domain.Ledger { return m.ledgers.List() }

// ActiveLedger is the id the accounting pages are reading.
func (m *Model) ActiveLedger() string { return m.ledgers.Active() }

// CreateLedger adds a ledger to the workspace, makes it the active one and
// reloads the pages onto it.
func (m *Model) CreateLedger(name, description string) error {
	if m.repo == nil {
		return errNoWorkspace
	}
	created, err := m.repo.CreateLedger(context.Background(), name, description)
	if err != nil {
		return err
	}
	if err := m.reloadLedgers(); err != nil {
		return err
	}
	m.ledgers.SetActive(created.ID)
	m.ledgerChanged()
	return nil
}

// UpdateLedger renames a ledger and rewrites its description. The list is
// re-read so every 账本 Selector and the settings rows agree; the records
// themselves do not change.
func (m *Model) UpdateLedger(id, name, description string) error {
	if m.repo == nil {
		return errNoWorkspace
	}
	if err := m.repo.UpdateLedger(context.Background(), id, name, description); err != nil {
		return err
	}
	return m.reloadLedgers()
}

// DeleteLedger removes a ledger and everything it owns. The last ledger cannot
// be deleted: accounting needs one, and the workspace would otherwise reopen on
// a fresh 默认账本 as if the deletion had failed.
func (m *Model) DeleteLedger(id string) error {
	if m.repo == nil {
		return errNoWorkspace
	}
	if len(m.ledgers.List()) <= 1 {
		return fmt.Errorf("%w：至少保留一个账本", repository.ErrInvalid)
	}
	if err := m.repo.DeleteLedger(context.Background(), id); err != nil {
		return err
	}
	if err := m.reloadLedgers(); err != nil {
		return err
	}
	m.ledgerChanged()
	return nil
}

// reloadLedgers re-reads the workspace's ledgers into the shared store. Reset
// keeps the active id when it survived and falls back to the first otherwise.
func (m *Model) reloadLedgers() error {
	if m.repo == nil {
		return errNoWorkspace
	}
	ledgers, err := m.repo.Ledgers(context.Background())
	if err != nil {
		return fmt.Errorf("读取账本失败：%w", err)
	}
	m.ledgers.Reset(ledgers)
	return nil
}

// errNoWorkspace is what a ledger command reports when no workspace is open.
var errNoWorkspace = fmt.Errorf("%w：工作空间尚未打开", repository.ErrInvalid)

// ledgerChanged reloads every accounting page after the shared 账本 Selector
// chose another ledger.
func (m *Model) ledgerChanged() {
	if m.repo == nil {
		return
	}
	m.records.Reload()
	m.analysis.Reload()
	m.tags.Reload()
	m.templates.Reload()
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
	m.analysis.CloseOverlays()
	m.tags.CloseOverlays()
	m.templates.CloseOverlays()
}

// SelectTab chooses a sub-function by index. Switching reloads the target page:
// a tag deleted on 标签 must reach the rows on 记录, and a template saved on
// 模板 must reach the 记一笔 dialog.
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
	case tabRecords:
		m.records.Reload()
	case tabAnalysis:
		m.analysis.Reload()
	case tabTags:
		m.tags.Reload()
	case tabTemplates:
		m.templates.Reload()
	}
}

// View declares the page. The four accounting pages read the workspace.
func (m *Model) View(c ui.C) {
	switch m.tab {
	case tabRecords:
		m.records.View(c)
	case tabAnalysis:
		m.analysis.View(c)
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
	case tabAnalysis:
		return m.analysis.Key(event)
	case tabTemplates:
		return m.templates.Key(event)
	}
	return gfx.EventResult{}
}

// Pointer routes the analysis chart's pointer (its hovered point) and leaves
// every other page's pointer to the shell.
func (m *Model) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if m.tab == tabAnalysis {
		return m.analysis.Pointer(event)
	}
	return gfx.EventResult{}
}

// Scroll routes the tags multi-select popup's wheel.
func (m *Model) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	switch m.tab {
	case tabRecords:
		return m.records.Scroll(event)
	case tabAnalysis:
		return m.analysis.Scroll(event)
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
	case tabAnalysis:
		return m.analysis.TagsEditor(id)
	case tabTemplates:
		return m.templates.TagsEditor(id)
	}
	return nil
}
