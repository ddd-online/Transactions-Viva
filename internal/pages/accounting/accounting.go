// Package accounting is the 记账 section: 记录, 分析, 标签 and 模板. The four
// sub-functions are the section's second-level navigation (the shell's
// Sidebar), so this page only declares the chosen screen. Each screen is
// migrated from the old project's src/pages/records*, charts*, category-tag*
// and template*.
package accounting

import (
	"context"
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components"
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

// New returns the empty accounting section. The three migrated pages share one
// ledger store, so their 账本 Selectors always agree, and the records dialog and
// the 模板 page share one template cache. desktop calls SetRepository once a
// workspace is open.
func New() *Model {
	ledgers := ledger.NewStore()
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
// 「默认账本」 with the old application's default categories and tags, so the
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
