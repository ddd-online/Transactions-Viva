// Package settings is the 应用设置 section. It is a page in the shell's content
// area, not a dialog: the application's navigation rail stays visible and the
// settings replace the functional page, as they do in the reference application.
//
// The layout follows that application's settings page: the sections are a list
// on the left, and the chosen section is a title plus grouped cards on the
// right. The commands are the shell's: this package only reports what the user
// chose, so it holds no configuration of its own.
package settings

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/config"
	"transactions/internal/pages/settings/controls"
)

type tab uint8

const (
	tabGeneral tab = iota
	tabFeatures
	tabData
	tabDiary
	tabAbout
)

// The page's own measurements, taken from the reference application's settings
// page: a 208 wide category list, a 760 wide reading column with 32 of side
// room, and a 64 high settings row.
const (
	navWidth      = float32(208)
	columnMax     = float32(760)
	columnMargin  = float32(32)
	columnTop     = float32(34)
	columnBottom  = float32(28)
	sectionTitle  = float32(26)
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
	note  string
	rows  []setting
}

var sections = [...]section{
	{
		tab:   tabGeneral,
		title: "通用设置",
		note:  "工作空间、外观（跟随系统 / 浅色 / 深色）与窗口关闭行为。",
	},
	{
		tab:   tabFeatures,
		title: "功能开关",
		note:  "记账、股票、待办、事件、日记五个开关；关掉的功能从左侧导航消失，设置本身不受开关约束。",
	},
	{
		tab:   tabData,
		title: "数据备份",
		note:  "打开工作空间、手动备份、每日自动备份，以及从备份恢复（恢复后重启）。",
		rows: []setting{
			{"最近工作空间", "最近打开的本地工作空间，可直接切换。"},
			{"备份位置", "手动备份与恢复前的副本都放这里。"},
			{"手动备份", "数据库与 data/assets 一起保存。"},
			{"自动备份", "每天第一次打开时存一份只含库的快照，保留最近 7 份。"},
			{"从备份恢复", "覆盖当前工作空间，恢复完成后重启应用。"},
		},
	},
	{
		tab:   tabDiary,
		title: "日记配置",
		note:  "扫描目录、导入与导出日记文件。",
		rows: []setting{
			{"扫描目录", "日记文件的来源目录。"},
			{"导入与导出", "批量导入或导出日记文件。"},
		},
	},
	{
		tab:   tabAbout,
		title: "关于软件",
		note:  "版本、检查更新，以及下载与安装。",
		rows: []setting{
			{"版本", "当前安装的 Transactions 版本。"},
			{"检查更新", "手动检查更新，或让应用定期检查。"},
			{"GitHub", "项目主页与问题反馈。"},
		},
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

	OnPickWorkspace func() error
	OnAppearance    func(config.Appearance) error
	OnCloseBehavior func(config.CloseBehavior) error
	OnFeature       func(config.FeatureKey, bool) error
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
	ui.Toasts(c, &m.toaster)
}

// nav is the category list: the panel's name over one row per section, the
// chosen row filled and the others quiet.
func (m *Model) nav(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(navWidth).FillHeight().Children(func() {
		ui.Text(c, "设置").FontSize(19).Bold().LineHeight(28).
			MarginSides(26, 8, 10, 22)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(navEntrySpace).MarginBottom(8).Children(func() {
				for index := range sections {
					entry := &sections[index]
					active := m.tab == entry.tab
					c.KeyInt(index)
					row := ui.Row(c).Height(navEntryHigh).Shrink(0).MarginX(12).
						PadSides(0, 8, 0, 12).Items(ui.Center).
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
						ui.Text(c, entry.title).FontSize(theme.Metrics.FontSizeM).
							Weight(gfx.WeightMedium).Color(foreground).Grow(1).Ellipsis()
					})
					if row.Clicked() {
						m.tab = entry.tab
					}
				}
			})
		})
	})
}

// page is the chosen section: its title, its explanation and its controls.
func (m *Model) page(c ui.C, entry section, spec Spec) {
	theme := c.Theme()
	ui.Scroll(c).Grow(1).FillHeight().Items(ui.Center).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(columnMax).
			MarginSides(columnTop, columnMargin, columnBottom, columnMargin).
			Children(func() {
				ui.Text(c, entry.title).FontSize(sectionTitle).Bold().
					LineHeight(34).MarginBottom(8)
				ui.Paragraph(c, entry.note, 3).FontSize(theme.Metrics.FontSizeM).
					Muted().LineHeight(20).MarginBottom(24)
				switch entry.tab {
				case tabGeneral:
					m.general(c, spec)
				case tabFeatures:
					m.features(c, spec)
				default:
					m.pending(c, entry)
				}
			})
	})
}

// general is 通用设置: the workspace, the appearance, the close behavior and the
// not-yet-migrated proxy row.
func (m *Model) general(c ui.C, spec Spec) {
	theme := c.Theme()
	trayNote := ""
	if !spec.TrayAvailable {
		trayNote = "当前系统没有可用的托盘，选「最小化到托盘」时关闭会直接退出。"
	}
	controls.Group(c, "", func() {
		controls.Row(c, "工作空间", workspaceNote(spec.Workspace), func() {
			if ui.Button(c, "切换").Secondary().Clicked() {
				m.report("打开工作空间失败", spec.OnPickWorkspace)
			}
		})
		ui.Divider(c)
		controls.Row(c, "外观", "", func() {
			value := spec.Appearance
			if ui.Choice(c, &value, appearanceOptions[:], appearanceLabel).Changed() {
				m.report("保存失败", func() error { return spec.OnAppearance(value) })
			}
		})
		ui.Divider(c)
		controls.Row(c, "关闭行为", trayNote, func() {
			value := spec.CloseBehavior
			if ui.Choice(c, &value, closeBehaviorOptions[:], closeBehaviorLabel).Changed() {
				m.report("保存失败", func() error { return spec.OnCloseBehavior(value) })
			}
		})
		ui.Divider(c)
		controls.Row(c, "网络代理", "行情、涨停池与更新检查走代理；尚未迁移。", func() {
			ui.Text(c, "未迁移").FontSize(theme.Metrics.FontSizeS).Muted()
		})
	})
}

func workspaceNote(workspace string) string {
	if workspace == "" {
		return "尚未选择工作空间"
	}
	return workspace
}

// features is 功能开关: one switch per optional feature.
func (m *Model) features(c ui.C, spec Spec) {
	controls.Group(c, "", func() {
		for index, entry := range featureRows {
			if index > 0 {
				ui.Divider(c)
			}
			entry := entry
			controls.Row(c, entry.title, "", func() {
				on := spec.Features.Enabled(entry.key)
				if ui.Switch(c, &on, entry.title+"功能开关").Changed() {
					m.report("保存失败", func() error { return spec.OnFeature(entry.key, on) })
				}
			})
		}
	})
}

// pending is a section whose controls are migrated in a later stage: each row
// reports that rather than pretending to work.
func (m *Model) pending(c ui.C, entry section) {
	theme := c.Theme()
	controls.Group(c, "", func() {
		for index := range entry.rows {
			if index > 0 {
				ui.Divider(c)
			}
			row := entry.rows[index]
			c.KeyInt(index)
			controls.Row(c, row.title, row.note, func() {
				ui.Text(c, "未迁移").FontSize(theme.Metrics.FontSizeS).Muted()
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
