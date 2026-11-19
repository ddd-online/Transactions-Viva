// Package settings is the 应用设置 section. It is a page in the shell's content
// area, not a dialog: the application's navigation rail stays visible and the
// settings replace the functional page, as they do in the reference application.
//
// The layout follows that application's settings page: the sections are a list
// on the left, and the chosen section is a title plus grouped cards on the
// right. The panel's five sections mirror src/pages/settings*.
package settings

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components"
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

// setting is one row of a section's card. The controls are migrated with the
// section's stage, so for now a row reports its status instead of pretending to
// work: label, explanation and the right-hand status.
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
		note:  "窗口关闭行为、外观（跟随系统 / 浅色 / 深色）、代理与最近工作空间。",
		rows: []setting{
			{"工作空间", "账本与记录都保存在所选的本地目录；尚未打开工作空间。"},
			{"外观", "浅色、深色或跟随系统；标题栏右侧的开关也能切换。"},
			{"关闭行为", "直接退出、最小化到托盘，或每次询问。"},
			{"网络代理", "行情、涨停池与更新检查走代理，其余功能离线可用。"},
		},
	},
	{
		tab:   tabFeatures,
		title: "功能开关",
		note:  "记账、股票、待办、事件、日记五个开关。关掉的功能从左侧导航消失，设置本身不受开关约束。",
		rows: []setting{
			{"记账", ""},
			{"股票", ""},
			{"待办", ""},
			{"事件", ""},
			{"日记", ""},
		},
	},
	{
		tab:   tabData,
		title: "数据",
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
		note:  "版本、检查更新、下载与安装，以及第三方许可。",
		rows: []setting{
			{"版本", "当前安装的 Transactions 版本。"},
			{"检查更新", "手动检查更新，或让应用定期检查。"},
			{"GitHub", "项目主页与问题反馈。"},
			{"第三方许可", "Viva、Lucide 等组件的许可与来源。"},
		},
	},
}

// Model is the settings page's state: the chosen section, kept while the shell
// shows another page and back.
type Model struct {
	tab tab
}

// New returns the settings page on 通用设置.
func New() *Model { return &Model{} }

// View declares the page: the category list, a divider and the chosen section.
func (m *Model) View(c ui.C) {
	ui.Row(c).Fill().Children(func() {
		m.nav(c)
		// A vertical divider is a Divider in a Row.
		ui.Divider(c)
		m.page(c, sections[m.tab])
	})
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

// page is the chosen section: its title, its explanation and one card of rows.
func (m *Model) page(c ui.C, entry section) {
	theme := c.Theme()
	ui.Scroll(c).Grow(1).FillHeight().Items(ui.Center).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(columnMax).
			MarginSides(columnTop, columnMargin, columnBottom, columnMargin).
			Children(func() {
				ui.Text(c, entry.title).FontSize(sectionTitle).Bold().
					LineHeight(34).MarginBottom(8)
				ui.Paragraph(c, entry.note, 3).FontSize(theme.Metrics.FontSizeM).
					Muted().LineHeight(20).MarginBottom(24)
				components.SettingsGroup(c, "", func() {
					for index := range entry.rows {
						if index > 0 {
							ui.Divider(c)
						}
						c.KeyInt(index)
						components.SettingsRow(c, entry.rows[index].title, entry.rows[index].note,
							func() {
								// The row's control arrives with the section's
								// migration stage; until then it reports that the
								// setting is not wired rather than acting on it.
								ui.Text(c, "未迁移").FontSize(theme.Metrics.FontSizeS).Muted()
							})
					}
				})
			})
	})
}
