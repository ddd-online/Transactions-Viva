// Package settings is the 设置 section. It is a page in the shell's content
// area, not a dialog: the application's navigation rail stays visible and the
// settings replace the functional page, as they do in the reference application.
//
// The layout follows that application's settings page: the sections are a list
// on the left, and the chosen section is a title plus grouped cards on the
// right. The commands are the shell's: this package only reports what the user
// chose, so it holds no configuration of its own.
package settings

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/config"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/pages/settings/controls"
)

type tab uint8

const (
	tabGeneral tab = iota
	tabLedgers
	tabStockAccounts
	tabStockSettings
	tabData
	tabDiary
	tabAbout
)

// The page's own measurements, taken from the reference application's settings
// The page's own measurements: the shared rail width, a 760 wide reading column
// with the page's inset on either side, and a 36 high settings row.
const (
	navWidth      = layout.RailWidth
	columnMax     = float32(760)
	columnMargin  = layout.GroupGap
	columnTop     = float32(32)
	columnBottom  = layout.GroupGap
	navEntryHigh  = float32(36)
	navEntrySpace = float32(4)
)

// setting is one row of a not-yet-migrated section. label is the row's name and
// note its explanation.
type setting struct {
	title string
	note  string
}

// section is one category of the panel, in the order the list shows.
type section struct {
	tab   tab
	title string
	rows  []setting
}

var sections = [...]section{
	{
		tab:   tabGeneral,
		title: "通用设置",
	},
	{
		tab:   tabLedgers,
		title: "账本管理",
	},
	{
		tab:   tabStockAccounts,
		title: "股票账户",
	},
	{
		tab:   tabStockSettings,
		title: "股票设置",
	},
	{
		tab:   tabData,
		title: "数据备份",
		rows: []setting{
			{"最近工作空间", "最近打开的本地工作空间，可直接切换。"},
			{"备份位置", "手动备份与恢复前的副本都放这里。"},
			{"手动备份", "数据库与图片资产一起保存。"},
			{"自动备份", "每天第一次打开时存一份只含数据库的快照，保留最近 7 份。"},
			{"从备份恢复", "用备份替换当前工作空间，完成后重启应用。"},
		},
	},
	{
		tab:   tabDiary,
		title: "日记配置",
	},
	{
		tab:   tabAbout,
		title: "关于软件",
	},
}

// Spec is what the shell hands the page each paint: the configuration's current
// values and the commands that write it. A command returns an error; the page
// shows it as a toast and leaves the value to the shell.
type Spec struct {
	Workspace     string
	Appearance    config.Appearance
	CloseBehavior config.CloseBehavior
	Features      config.FeatureFlags
	// TrayAvailable says whether the application could put an icon in the
	// system's status area. Without one, 最小化到托盘 would hide the window with
	// no way back, so the close behavior row explains that it quits instead.
	TrayAvailable bool

	// Ledgers are the open workspace's ledgers, newest last. The settings page
	// manages them; the accounting pages read the same list through their
	// shared store, so a rename here is the name they show.
	Ledgers []domain.Ledger
	// ActiveLedger is the id the accounting pages are reading, marked 当前 in
	// the list; empty when no workspace is open.
	ActiveLedger string

	OnPickWorkspace func() error
	OnAppearance    func(config.Appearance) error
	OnCloseBehavior func(config.CloseBehavior) error
	OnFeature       func(config.FeatureKey, bool) error
	// CreateLedger adds a ledger and makes it the accounting pages' active one;
	// UpdateLedger rewrites its name and description; DeleteLedger removes it
	// with its records. Each returns what the user should read on failure.
	OnCreateLedger func(name, description string) error
	OnUpdateLedger func(id, name, description string) error
	OnDeleteLedger func(id string) error

	// StockAccounts are the workspace's stock accounts; the 股票 tabs read the
	// same list through their shared store. ActiveStockAccount is marked 当前.
	StockAccounts        []domain.StockAccount
	ActiveStockAccount   string
	OnCreateStockAccount func(name, description string) error
	OnUpdateStockAccount func(id, name, description string) error
	OnDeleteStockAccount func(id string) error

	// Stock is the active account's settings, read by the shell each paint; the
	// callbacks write them and return what the section should show.
	Stock                     *StockConfig
	StockError                string
	OnSetStockAccount         func(id string) error
	OnSaveStockTags           func(tags []string) ([]string, error)
	OnSaveStockReviewTemplate func(template string, daily *string) (domain.StockReviewSetting, error)
	OnSaveStockFee            func(fee domain.RoundFee) error
	OnRollbackStockOperation  func() (domain.StockOperationRollback, error)
	OnArchiveStockAccount     func(name string) error
	OnResetStockAccount       func() error

	// OpenURL opens a place outside the window in the user's browser, and
	// CopyText puts text on the clipboard; 关于软件 uses both.
	OpenURL  func(target string) error
	CopyText func(value string) error

	// DiaryBooks are the open workspace's diary books and ActiveDiaryBook the one
	// the 日记 page shows; the import and export act on that book.
	DiaryBooks      []domain.DiaryBook
	ActiveDiaryBook string
	// OnCreateDiaryBook adds a book and makes it the active one; the update
	// rewrites name and description; the delete removes a book with its entries.
	OnCreateDiaryBook func(name, description string) error
	OnUpdateDiaryBook func(id, name, description string) error
	OnDeleteDiaryBook func(id string) error

	// ImportDiary and ExportDiary are 日记配置's two commands: the shell asks the
	// user for a directory and moves the files, so the page stays free of the
	// file system. nil means no workspace is open and the buttons are disabled.
	ImportDiary func() (DiaryImport, error)
	ExportDiary func(scope domain.DiaryExportScope, period string) (DiaryExport, error)
}

// DiaryImport is what one batch import did, as the section's status line reads
// it. Canceled is the user closing the directory picker: nothing to report.
type DiaryImport struct {
	Canceled bool
	Imported int
	Total    int
	Failures []string
}

// DiaryExport is what one export wrote. Canceled is the same closed picker.
type DiaryExport struct {
	Canceled bool
	Written  int
	Total    int
	Failures []string
}

// appearanceOptions is the segmented 外观 control, in the old page's order.
var appearanceOptions = [...]config.Appearance{
	config.AppearanceLight, config.AppearanceDark, config.AppearanceSystem,
}

func appearanceLabel(value config.Appearance) string {
	switch value {
	case config.AppearanceLight:
		return "浅色"
	case config.AppearanceDark:
		return "深色"
	}
	return "跟随系统"
}

// closeBehaviorOptions is the segmented 关闭行为 control.
var closeBehaviorOptions = [...]config.CloseBehavior{
	config.CloseQuit, config.CloseTray, config.CloseAsk,
}

func closeBehaviorLabel(value config.CloseBehavior) string {
	switch value {
	case config.CloseQuit:
		return "直接退出"
	case config.CloseTray:
		return "最小化到托盘"
	}
	return "每次询问"
}

// featureRows are the five feature switches in the old settings page's order.
var featureRows = [...]struct {
	key   config.FeatureKey
	title string
}{
	{config.FeatureAccounting, "记账"},
	{config.FeatureStock, "股票"},
	{config.FeatureKeyEvent, "事件"},
	{config.FeatureDiary, "日记"},
	{config.FeatureTodo, "待办"},
}

// Model is the settings page's state: the chosen section, kept while the shell
// shows another page and back, and the toast a failed write reports.
type Model struct {
	tab     tab
	toaster ui.Toaster

	// ledgerForm is the 新增 / 编辑账本 dialog; id empty means create.
	ledgerForm ledgerForm
	// ledgerDelete is the confirmation for removing one ledger.
	ledgerDelete ledgerDelete

	// stockForm / stockDelete are the same shapes for a stock account.
	stockForm   ledgerForm
	stockDelete ledgerDelete
	// stock is 股票设置's drafts; its data is Spec.Stock.
	stock stockState

	// diary is 日记配置's own state: what an export covers, the period it
	// picked, and the last command's status or failures.
	diary diaryState

	// bookForm is the 新建 / 编辑日记本 dialog; id empty means create.
	bookForm bookForm
	// bookDelete is the confirmation for removing one diary book.
	bookDelete bookDelete
}

// bookForm is one diary book being created or edited, with the field states its
// two rows keep across paints.
type bookForm struct {
	open        bool
	id          string
	name        string
	description string
	nameField   ui.FormFieldState
	descField   ui.FormFieldState
	err         string
}

// bookDelete names the diary book a confirmation is open for.
type bookDelete struct {
	open bool
	id   string
	name string
}

// diaryState is the 日记配置 section's draft: the export scope, the period it
// applies to, that period's picker, and the last command's result.
type diaryState struct {
	scope   domain.DiaryExportScope
	period  string
	pick    datepicker.State
	status  string
	failure string
}

// ledgerForm is one ledger being created or edited. The two FormFieldStates are
// the id, name and error rows' layout, kept across paints like any control.
type ledgerForm struct {
	open        bool
	id          string
	name        string
	description string
	nameField   ui.FormFieldState
	descField   ui.FormFieldState
	err         string
}

// ledgerDelete names the ledger a confirmation is open for.
type ledgerDelete struct {
	open bool
	id   string
	name string
}

// New returns the settings page on 通用设置.
func New() *Model { return &Model{} }

// View declares the page: the category list, a divider and the chosen section.
func (m *Model) View(c ui.C, spec Spec) {
	ui.Row(c).Fill().Children(func() {
		m.nav(c)
		// A vertical divider is a Divider in a Row.
		ui.Divider(c)
		m.page(c, sections[m.tab], spec)
	})
	m.ledgerDialog(c, spec)
	m.ledgerDeleteDialog(c, spec)
	m.stockAccountDialog(c, spec)
	m.stockAccountDeleteDialog(c, spec)
	m.stockTemplateDialog(c, spec)
	m.stockFeeDialog(c, spec)
	m.stockOperationsDialog(c, spec)
	m.stockArchiveDialog(c, spec)
	m.stockResetDialog(c, spec)
	m.bookDialog(c, spec)
	m.bookDeleteDialog(c, spec)
	ui.Toasts(c, &m.toaster)
}

// nav is the category list: the panel's name over one row per section, the
// chosen row filled and the others quiet.
func (m *Model) nav(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(navWidth).FillHeight().Children(func() {
		typography.Title(c, "设置").
			MarginSides(layout.BlockGap+layout.Gap, layout.Gap, layout.BlockGap, layout.GroupGap)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(navEntrySpace).MarginBottom(8).Children(func() {
				for index := range sections {
					entry := &sections[index]
					active := m.tab == entry.tab
					c.KeyInt(index)
					row := ui.Row(c).Height(navEntryHigh).Shrink(0).MarginX(layout.BlockGap).
						PadSides(0, layout.Gap, 0, layout.BlockGap).Items(ui.Center).
						Radius(theme.Metrics.RadiusControl)
					if active {
						row = row.Background(theme.Accent)
					}
					row = row.HitWith(ui.ID(c), entry.title,
						gfx.HitOptions{Role: gfx.RoleMenuItem, Selected: active}).
						Hover(ui.Look{}.Background(theme.Accent)).
						Focus(ui.Look{}.Ring(2, theme.Ring))
					foreground := theme.MutedForeground
					if active {
						foreground = theme.AccentForeground
					}
					row.Children(func() {
						typography.Body(c, entry.title).
							Color(foreground).Grow(1).Ellipsis()
					})
					if row.Clicked() {
						m.tab = entry.tab
					}
				}
			})
		})
	})
}

// page is the chosen section: its title and its controls.
func (m *Model) page(c ui.C, entry section, spec Spec) {
	ui.Scroll(c).Grow(1).FillHeight().Items(ui.Center).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(columnMax).
			MarginSides(columnTop, columnMargin, columnBottom, columnMargin).
			Children(func() {
				typography.Title(c, entry.title).
					MarginBottom(layout.GroupGap)
				switch entry.tab {
				case tabGeneral:
					m.general(c, spec)
				case tabLedgers:
					m.ledgers(c, spec)
				case tabStockAccounts:
					m.stockAccounts(c, spec)
				case tabStockSettings:
					m.stockSettingsSection(c, spec)
				case tabDiary:
					m.diaryConfig(c, spec)
				case tabAbout:
					m.about(c, spec)
				default:
					m.pending(c, entry)
				}
			})
	})
}

// general is 通用设置: the application's own preferences, the proxy placeholder
// and the optional features' switches, each its own named group — the same
// rhythm as 关于软件 (版本信息 / 更新 / 项目) and 日记配置 (日记本 / 导入导出). The
// switches used to be a section of their own; they act on the navigation rather
// than on a page of it, so they belong beside the other things the application
// itself sets.
func (m *Model) general(c ui.C, spec Spec) {
	ui.Column(c).FillWidth().Gap(layout.GroupGap).Children(func() {
		m.generalPreferences(c, spec)
		m.proxy(c)
		m.featureSwitches(c, spec)
	})
}

// generalPreferences is 常规: the workspace, the appearance and the close
// behavior.
func (m *Model) generalPreferences(c ui.C, spec Spec) {
	trayNote := ""
	if !spec.TrayAvailable {
		trayNote = "当前系统没有可用的托盘，选「最小化到托盘」时关闭会直接退出。"
	}
	controls.Group(c, "常规", func() {
		controls.Row(c, "工作空间", workspaceNote(spec.Workspace), func() {
			if ui.Button(c, "切换").Secondary().Clicked() {
				m.report("打开工作空间失败", spec.OnPickWorkspace)
			}
		})
		ui.Divider(c)
		controls.Row(c, "外观", "", func() {
			value := spec.Appearance
			if ui.Choice(c, &value, appearanceOptions[:], appearanceLabel).Changed() {
				m.report("保存外观失败", func() error { return spec.OnAppearance(value) })
			}
		})
		ui.Divider(c)
		controls.Row(c, "关闭行为", trayNote, func() {
			value := spec.CloseBehavior
			if ui.Choice(c, &value, closeBehaviorOptions[:], closeBehaviorLabel).Changed() {
				m.report("保存关闭行为失败", func() error { return spec.OnCloseBehavior(value) })
			}
		})
	})
}

// proxy is 网络代理, a group of its own: it is a placeholder until the proxy
// settings are migrated, and a row that only says 暂未支持 reads as a group the
// user can skip rather than as part of 常规.
func (m *Model) proxy(c ui.C) {
	controls.Group(c, "网络代理", func() {
		controls.Row(c, "代理", "行情、涨停池与更新检查走代理。", func() {
			typography.Caption(c, "暂未支持")
		})
	})
}

// featureSwitches is 功能开关: one switch per optional feature. Turning one off
// takes its section out of the navigation at once, which is why the group says
// so rather than leaving the reader to discover it.
func (m *Model) featureSwitches(c ui.C, spec Spec) {
	controls.Group(c, "功能开关", func() {
		for index, entry := range featureRows {
			if index > 0 {
				ui.Divider(c)
			}
			entry := entry
			controls.Row(c, entry.title, "", func() {
				on := spec.Features.Enabled(entry.key)
				// The row's title already says what this switch is; the label
				// stays for assistive technology and for the tests.
				if ui.Switch(c, &on, "").Label(entry.title + "功能开关").Changed() {
					m.report("保存功能开关失败", func() error { return spec.OnFeature(entry.key, on) })
				}
			})
		}
	})
}

// ledgers is 账本管理, its own section of the panel: one card, one row per
// ledger with its description, an edit and a delete command, and 新增账本 as the
// card's last row. A ledger the accounting pages are reading is marked 当前; the
// last one cannot be deleted, because accounting always needs one to read.
func (m *Model) ledgers(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	controls.Card(c, func() {
		if len(spec.Ledgers) == 0 {
			controls.Row(c, "还没有账本", "打开工作空间后可以在这里新增、重命名或删除账本。", nil)
			return
		}
		for index := range spec.Ledgers {
			if index > 0 {
				ui.Divider(c)
			}
			one := spec.Ledgers[index]
			controls.Row(c, one.Name, describe(one.Description), func() {
				if spec.ActiveLedger == one.ID {
					typography.Caption(c, "当前")
				}
				if ui.Button(c, "编辑账本 "+one.Name).Ghost().Icon(icons.Pencil).
					IconOnly().Tooltip("编辑名称与描述").Clicked() {
					m.openLedgerForm(spec, one.ID)
				}
				// The last ledger stays: a workspace without one would reopen on
				// a fresh 默认账本, as if the deletion had failed.
				last := len(spec.Ledgers) <= 1
				remove := ui.Button(c, "删除账本 "+one.Name).Ghost().Icon(icons.Trash2).
					IconOnly().Disabled(last)
				if last {
					remove = remove.Tooltip("至少保留一个账本")
				} else {
					remove = remove.Tooltip("删除账本")
				}
				if remove.Clicked() {
					m.ledgerDelete = ledgerDelete{open: true, id: one.ID, name: one.Name}
				}
			})
		}
		ui.Divider(c)
		// The action row is a 32 px button with a gap around it: shorter than a
		// settings row, because it acts on the list rather than setting it.
		ui.Row(c).Height(metrics.ControlHeight + 2*metrics.GapItem).FillWidth().
			PadX(metrics.PadSurface).Items(ui.Center).Children(func() {
			if ui.Button(c, "新增账本").Ghost().Icon(icons.Plus).Clicked() {
				m.openLedgerForm(spec, "")
			}
		})
	})
}

// openLedgerForm raises the create dialog, or the edit dialog prefilled with the
// chosen ledger.
func (m *Model) openLedgerForm(spec Spec, id string) {
	form := ledgerForm{open: true, id: id}
	for _, one := range spec.Ledgers {
		if one.ID == id {
			form.name, form.description = one.Name, one.Description
		}
	}
	m.ledgerForm = form
}

// ledgerDialog is the 新增 / 编辑账本 form: the ledger's name and its
// description, both trimmed before they are written.
func (m *Model) ledgerDialog(c ui.C, spec Spec) {
	if !m.ledgerForm.open {
		return
	}
	title := "新增账本"
	submit := "创建"
	if m.ledgerForm.id != "" {
		title, submit = "编辑账本", "保存"
	}
	// FormField reserves a description row and an error row this form does not
	// use; pinning each field to the control's own two-row height keeps the
	// dialog compact, as the record composer does.
	fieldHeight := c.Theme().Metrics.ControlHeight * 2
	ui.ModalWith(c, &m.ledgerForm.open, title, ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).Children(func() {
			ui.FormField(c, &m.ledgerForm.nameField, "名称").Height(fieldHeight).Children(func() {
				ui.TextField(c, &m.ledgerForm.name, "名称", "例如：家庭账本").AutoFocus()
			})
			ui.FormField(c, &m.ledgerForm.descField, "描述").Height(fieldHeight).Children(func() {
				ui.TextField(c, &m.ledgerForm.description, "描述", "例如：日常开销与储蓄")
			})
			if m.ledgerForm.err != "" {
				typography.Caption(c, m.ledgerForm.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.ledgerForm.open = false
		}
		if ui.Button(c, submit).Clicked() {
			m.submitLedger(spec)
		}
	})
}

// submitLedger writes the form through the shell and keeps the dialog open with
// the reason when the workspace refuses it.
func (m *Model) submitLedger(spec Spec) {
	name := strings.TrimSpace(m.ledgerForm.name)
	if name == "" {
		m.ledgerForm.err = "账本名称不能为空"
		return
	}
	description := strings.TrimSpace(m.ledgerForm.description)
	var err error
	if m.ledgerForm.id == "" {
		if spec.OnCreateLedger == nil {
			return
		}
		err = spec.OnCreateLedger(name, description)
		if err == nil {
			m.toaster.Show("账本已创建", "", ui.ToastSuccess)
		}
	} else {
		if spec.OnUpdateLedger == nil {
			return
		}
		err = spec.OnUpdateLedger(m.ledgerForm.id, name, description)
		if err == nil {
			m.toaster.Show("账本已保存", "", ui.ToastSuccess)
		}
	}
	if err != nil {
		m.ledgerForm.err = err.Error()
		return
	}
	m.ledgerForm.open = false
}

// ledgerDeleteDialog confirms the cascade before deleting a ledger.
func (m *Model) ledgerDeleteDialog(c ui.C, spec Spec) {
	if !m.ledgerDelete.open {
		return
	}
	ui.ModalWith(c, &m.ledgerDelete.open, "删除账本「"+m.ledgerDelete.name+"」？", ui.ModalSpec{
		Width:       420,
		Description: "记录、分类标签、模板与图表会一起删除，无法恢复。",
	}, nil, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.ledgerDelete.open = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			id := m.ledgerDelete.id
			if spec.OnDeleteLedger == nil {
				return
			}
			if err := spec.OnDeleteLedger(id); err != nil {
				m.toaster.Show("删除账本失败："+err.Error(), "", ui.ToastError)
				return
			}
			m.ledgerDelete.open = false
			m.toaster.Show("账本已删除", "", ui.ToastSuccess)
		}
	})
}

// stockAccounts is 设置 · 股票账户: a workspace-level list, managed exactly like
// the ledger list. The stock toolbar's 账户 selector reads the same list.
func (m *Model) stockAccounts(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	controls.Card(c, func() {
		if len(spec.StockAccounts) == 0 {
			controls.Row(c, "还没有股票账户", "打开股票页时会自动创建一个默认股票账户。", nil)
			return
		}
		for index := range spec.StockAccounts {
			if index > 0 {
				ui.Divider(c)
			}
			one := spec.StockAccounts[index]
			controls.Row(c, one.Name, describe(one.Description), func() {
				if spec.ActiveStockAccount == one.ID {
					typography.Caption(c, "当前")
				}
				if ui.Button(c, "编辑股票账户 "+one.Name).Ghost().Icon(icons.Pencil).
					IconOnly().Tooltip("编辑名称与描述").Clicked() {
					m.openStockAccountForm(spec, one.ID)
				}
				last := len(spec.StockAccounts) <= 1
				remove := ui.Button(c, "删除股票账户 "+one.Name).Ghost().Icon(icons.Trash2).
					IconOnly().Disabled(last)
				if last {
					remove = remove.Tooltip("至少保留一个股票账户")
				} else {
					remove = remove.Tooltip("删除股票账户")
				}
				if remove.Clicked() {
					m.stockDelete = ledgerDelete{open: true, id: one.ID, name: one.Name}
				}
			})
		}
		ui.Divider(c)
		ui.Row(c).Height(metrics.ControlHeight + 2*metrics.GapItem).FillWidth().
			PadX(metrics.PadSurface).Items(ui.Center).Children(func() {
			if ui.Button(c, "新增股票账户").Ghost().Icon(icons.Plus).Clicked() {
				m.openStockAccountForm(spec, "")
			}
		})
	})
}

func (m *Model) openStockAccountForm(spec Spec, id string) {
	form := ledgerForm{open: true, id: id}
	for _, one := range spec.StockAccounts {
		if one.ID == id {
			form.name, form.description = one.Name, one.Description
		}
	}
	m.stockForm = form
}

func (m *Model) stockAccountDialog(c ui.C, spec Spec) {
	if !m.stockForm.open {
		return
	}
	title, submit := "新增股票账户", "创建"
	if m.stockForm.id != "" {
		title, submit = "编辑股票账户", "保存"
	}
	fieldHeight := c.Theme().Metrics.ControlHeight * 2
	ui.ModalWith(c, &m.stockForm.open, title, ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).Children(func() {
			ui.FormField(c, &m.stockForm.nameField, "名称").Height(fieldHeight).Children(func() {
				ui.TextField(c, &m.stockForm.name, "名称", "例如：主账户").AutoFocus()
			})
			ui.FormField(c, &m.stockForm.descField, "描述").Height(fieldHeight).Children(func() {
				ui.TextField(c, &m.stockForm.description, "描述", "例如：长期持有")
			})
			if m.stockForm.err != "" {
				typography.Caption(c, m.stockForm.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.stockForm.open = false
		}
		if ui.Button(c, submit).Clicked() {
			m.submitStockAccount(spec)
		}
	})
}

func (m *Model) submitStockAccount(spec Spec) {
	name := strings.TrimSpace(m.stockForm.name)
	if name == "" {
		m.stockForm.err = "股票账户名称不能为空"
		return
	}
	description := strings.TrimSpace(m.stockForm.description)
	var err error
	if m.stockForm.id == "" {
		if spec.OnCreateStockAccount == nil {
			return
		}
		err = spec.OnCreateStockAccount(name, description)
	} else {
		if spec.OnUpdateStockAccount == nil {
			return
		}
		err = spec.OnUpdateStockAccount(m.stockForm.id, name, description)
	}
	if err != nil {
		m.stockForm.err = err.Error()
		return
	}
	m.stockForm.open = false
	if m.stockForm.id == "" {
		m.toaster.Show("股票账户已创建", "", ui.ToastSuccess)
	} else {
		m.toaster.Show("股票账户已保存", "", ui.ToastSuccess)
	}
}

func (m *Model) stockAccountDeleteDialog(c ui.C, spec Spec) {
	if !m.stockDelete.open {
		return
	}
	ui.ModalWith(c, &m.stockDelete.open, "删除股票账户「"+m.stockDelete.name+"」？", ui.ModalSpec{
		Width:       420,
		Description: "账户里的本金、资金记录、持仓、成交、轮次、心得与设置会一起删除，无法恢复。",
	}, nil, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.stockDelete.open = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			id := m.stockDelete.id
			if spec.OnDeleteStockAccount == nil {
				return
			}
			if err := spec.OnDeleteStockAccount(id); err != nil {
				m.toaster.Show("删除股票账户失败："+err.Error(), "", ui.ToastError)
				return
			}
			m.stockDelete.open = false
			m.toaster.Show("股票账户已删除", "", ui.ToastSuccess)
		}
	})
}

func workspaceNote(workspace string) string {
	if workspace == "" {
		return "尚未选择工作空间"
	}
	return workspace
}

// diaryConfig is 日记配置: the batch import and the scoped export. Both ask the
// shell for a directory; the section only reports what came of it, so the same
// card explains a failed file as it explains a finished batch.
//
// Each command sits on the row that names it, the way 通用设置 puts 切换 beside
// 工作空间: the icon-free buttons are quiet, and a finished batch reports itself
// on the line under the card.
func (m *Model) diaryConfig(c ui.C, spec Spec) {
	// Two groups, the way 关于软件 stacks 版本信息 / 更新 / 项目: a name over its
	// block, and the blocks separated by the group gap.
	ui.Column(c).FillWidth().Gap(layout.GroupGap).Children(func() {
		m.diaryBooks(c, spec)
		m.diaryTransfer(c, spec)
	})
}

// diaryBooks is 日记本: the workspace's notebooks, one row each with its name,
// its description and its commands, then 新建日记本. It follows 账本管理 exactly,
// including the last-book rule: the diary page always needs one to show.
func (m *Model) diaryBooks(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	controls.Group(c, "日记本", func() {
		if len(spec.DiaryBooks) == 0 {
			controls.Row(c, "还没有日记本", "打开工作空间后可以在这里新建、重命名或删除日记本。", nil)
			return
		}
		for index := range spec.DiaryBooks {
			if index > 0 {
				ui.Divider(c)
			}
			one := spec.DiaryBooks[index]
			controls.Row(c, one.Name, describe(one.Description), func() {
				if spec.ActiveDiaryBook == one.ID {
					typography.Caption(c, "当前")
				}
				if ui.Button(c, "编辑日记本 "+one.Name).Ghost().Icon(icons.Pencil).
					IconOnly().Tooltip("编辑名称与描述").Clicked() {
					m.openBookForm(spec, one.ID)
				}
				last := len(spec.DiaryBooks) <= 1
				remove := ui.Button(c, "删除日记本 "+one.Name).Ghost().Icon(icons.Trash2).
					IconOnly().Disabled(last)
				if last {
					remove = remove.Tooltip("至少保留一本日记本")
				} else {
					remove = remove.Tooltip("删除日记本")
				}
				if remove.Clicked() {
					m.bookDelete = bookDelete{open: true, id: one.ID, name: one.Name}
				}
			})
		}
		ui.Divider(c)
		ui.Row(c).Height(metrics.ControlHeight + 2*metrics.GapItem).FillWidth().
			PadX(metrics.PadSurface).Items(ui.Center).Children(func() {
			if ui.Button(c, "新建日记本").Ghost().Icon(icons.Plus).Clicked() {
				m.openBookForm(spec, "")
			}
		})
	})
}

// openBookForm raises the create dialog, or the edit dialog prefilled with the
// chosen book.
func (m *Model) openBookForm(spec Spec, id string) {
	form := bookForm{open: true, id: id}
	for _, one := range spec.DiaryBooks {
		if one.ID == id {
			form.name, form.description = one.Name, one.Description
		}
	}
	m.bookForm = form
}

// bookDialog is the 新建 / 编辑日记本 form: a name and a description, both
// trimmed before they are written.
func (m *Model) bookDialog(c ui.C, spec Spec) {
	if !m.bookForm.open {
		return
	}
	title, submit := "新建日记本", "创建"
	if m.bookForm.id != "" {
		title, submit = "编辑日记本", "保存"
	}
	fieldHeight := c.Theme().Metrics.ControlHeight * 2
	ui.ModalWith(c, &m.bookForm.open, title, ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).Children(func() {
			ui.FormField(c, &m.bookForm.nameField, "名称").Height(fieldHeight).Children(func() {
				ui.TextField(c, &m.bookForm.name, "名称", "例如：旅行日记").AutoFocus()
			})
			ui.FormField(c, &m.bookForm.descField, "描述").Height(fieldHeight).Children(func() {
				ui.TextField(c, &m.bookForm.description, "描述", "例如：出差与旅行")
			})
			if m.bookForm.err != "" {
				typography.Caption(c, m.bookForm.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.bookForm.open = false
		}
		if ui.Button(c, submit).Clicked() {
			m.submitBook(spec)
		}
	})
}

// submitBook writes the form through the shell and keeps the dialog open with
// the reason when the workspace refuses it.
func (m *Model) submitBook(spec Spec) {
	name := strings.TrimSpace(m.bookForm.name)
	if name == "" {
		m.bookForm.err = "日记本名称不能为空"
		return
	}
	description := strings.TrimSpace(m.bookForm.description)
	var err error
	if m.bookForm.id == "" {
		if spec.OnCreateDiaryBook == nil {
			return
		}
		err = spec.OnCreateDiaryBook(name, description)
		if err == nil {
			m.toaster.Show("日记本已创建", "", ui.ToastSuccess)
		}
	} else {
		if spec.OnUpdateDiaryBook == nil {
			return
		}
		err = spec.OnUpdateDiaryBook(m.bookForm.id, name, description)
		if err == nil {
			m.toaster.Show("日记本已保存", "", ui.ToastSuccess)
		}
	}
	if err != nil {
		m.bookForm.err = err.Error()
		return
	}
	m.bookForm.open = false
}

// bookDeleteDialog confirms removing a book and every entry in it.
func (m *Model) bookDeleteDialog(c ui.C, spec Spec) {
	if !m.bookDelete.open {
		return
	}
	ui.ModalWith(c, &m.bookDelete.open, "删除日记本「"+m.bookDelete.name+"」？", ui.ModalSpec{
		Width:       420,
		Description: "这本日记本里的所有日记会一起删除，无法恢复。",
	}, nil, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.bookDelete.open = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			id := m.bookDelete.id
			if spec.OnDeleteDiaryBook == nil {
				return
			}
			if err := spec.OnDeleteDiaryBook(id); err != nil {
				m.toaster.Show("删除日记本失败："+err.Error(), "", ui.ToastError)
				return
			}
			m.bookDelete.open = false
			m.toaster.Show("日记本已删除", "", ui.ToastSuccess)
		}
	})
}

// diaryTransfer is 导入导出: the batch import and the scoped export of the book
// the 日记 page is showing.
func (m *Model) diaryTransfer(c ui.C, spec Spec) {
	ready := spec.ImportDiary != nil && spec.ExportDiary != nil
	controls.Group(c, "导入导出", func() {
		controls.Row(c, "导入日记", importNote(spec), func() {
			button := ui.Button(c, "批量导入").Secondary().Disabled(!ready)
			if button.Clicked() {
				m.importDiary(spec)
			}
		})
		ui.Divider(c)
		controls.Row(c, "导出日记", exportNote(spec), func() {
			button := ui.Button(c, "批量导出").Secondary().Disabled(!ready)
			if button.Clicked() {
				m.exportDiary(spec)
			}
		})
		ui.Divider(c)
		controls.Row(c, "导出范围", "", func() {
			if ui.Choice(c, &m.diary.scope, domain.DiaryExportScopeNames[:],
				domain.DiaryExportScope.Label).Changed() {
				m.diary.status, m.diary.failure = "", ""
				m.ensurePeriod()
			}
			if m.diary.scope == domain.DiaryScopeAll {
				return
			}
			grain := datepicker.ByYear
			if m.diary.scope == domain.DiaryScopeMonth {
				grain = datepicker.ByMonth
			}
			datepicker.Picker(c, &m.diary.pick, datepicker.Spec{
				Label: "导出期间", Start: m.diary.period, Grain: grain,
				CloseOnPick: true, Viewport: c.Bounds(),
				OnRange: func(start, _ string) {
					m.diary.period, m.diary.status, m.diary.failure = start, "", ""
				},
			})
		})
		if m.diary.status != "" || m.diary.failure != "" || !ready {
			ui.Divider(c)
			m.diaryResultRow(c, ready)
		}
	})
}

// diaryResultRow is the group's last row: what the last command did, or why the
// two commands are unavailable. It belongs to the 导入导出 card rather than
// floating under it, so a result reads as part of the group that produced it.
func (m *Model) diaryResultRow(c ui.C, ready bool) {
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Row(c).Shrink(0).PadX(metrics.PadSurface).PadY(metrics.GapItem * 2).
		Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				switch {
				case !ready:
					typography.Caption(c, "打开工作空间后才能导入 / 导出日记。").
						Color(theme.MutedForeground)
				case m.diary.status != "":
					typography.Caption(c, m.diary.status).Color(theme.MutedForeground)
				}
				if m.diary.failure != "" {
					typography.CaptionParagraph(c, m.diary.failure, 4).Color(theme.Danger)
				}
			})
		})
}

// describe is the second line of a list row (a ledger, a diary book): its own
// description, or a placeholder when it has none. A row without that line reads
// as if the thing had no description to give, which is not the same as "not
// written yet".
func describe(text string) string {
	if strings.TrimSpace(text) == "" {
		return "还没有描述"
	}
	return text
}

// importNote names the book an import writes into.
func importNote(spec Spec) string {
	if name := activeBookName(spec); name != "" {
		return "导入到「" + name + "」，文件名需为 YYYY-MM-DD.txt。"
	}
	return "按目录批量导入，文件名需为 YYYY-MM-DD.txt。"
}

// exportNote names the book an export reads.
func exportNote(spec Spec) string {
	if name := activeBookName(spec); name != "" {
		return "把「" + name + "」存成 YYYY-MM-DD.txt，之后可以再导入。"
	}
	return "存成 YYYY-MM-DD.txt，之后可以再导入。"
}

// activeBookName is the name of the book the diary page shows, "" when none is
// open.
func activeBookName(spec Spec) string {
	for _, book := range spec.DiaryBooks {
		if book.ID == spec.ActiveDiaryBook {
			return book.Name
		}
	}
	return ""
}

// ensurePeriod gives the export a period to start from, at the scope's grain:
// the current year or the current month.
func (m *Model) ensurePeriod() {
	if m.diary.period != "" {
		return
	}
	today, ok := domain.ParseDiaryDate(domain.DiaryToday())
	if !ok {
		return
	}
	m.diary.period = today.Format("2006-01-02")
}

// importDiary runs the shell's import and reports what it did.
func (m *Model) importDiary(spec Spec) {
	if spec.ImportDiary == nil {
		return
	}
	m.diary.status, m.diary.failure = "", ""
	result, err := spec.ImportDiary()
	if err != nil {
		m.diary.failure = err.Error()
		return
	}
	if result.Canceled {
		return
	}
	m.diary.status = fmt.Sprintf("导入 %d/%d 篇", result.Imported, result.Total)
	m.diary.failure = strings.Join(result.Failures, "\n")
}

// exportDiary runs the shell's export for the chosen scope and period.
func (m *Model) exportDiary(spec Spec) {
	if spec.ExportDiary == nil {
		return
	}
	m.diary.status, m.diary.failure = "", ""
	m.ensurePeriod()
	result, err := spec.ExportDiary(m.diary.scope, m.diary.period)
	if err != nil {
		m.diary.failure = err.Error()
		return
	}
	if result.Canceled {
		return
	}
	m.diary.status = fmt.Sprintf("导出 %d/%d 篇", result.Written, result.Total)
	m.diary.failure = strings.Join(result.Failures, "\n")
}

// pending is a section whose controls are migrated in a later stage: each row
// reports that rather than pretending to work.
func (m *Model) pending(c ui.C, entry section) {
	controls.Group(c, "", func() {
		for index := range entry.rows {
			if index > 0 {
				ui.Divider(c)
			}
			row := entry.rows[index]
			c.KeyInt(index)
			controls.Row(c, row.title, row.note, func() {
				typography.Caption(c, "暂未支持")
			})
		}
	})
}

// report runs one of the shell's commands and shows its failure as a toast.
func (m *Model) report(prefix string, action func() error) {
	if action == nil {
		return
	}
	if err := action(); err != nil {
		m.toaster.Show(prefix+"："+err.Error(), "", ui.ToastError)
	}
}
