package settings

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/buildinfo"
	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/icons"
	"transactions/internal/pages/settings/controls"
)

// Where the project lives and who draws the window. One place, so the cards and
// the colophon cannot drift apart.
const (
	projectURL = "https://github.com/ddd-online/Transactions-Viva"
	issuesURL  = projectURL + "/issues"
	vivaURL    = "https://vivaui.dev"
)

// aboutMarkSize is the identity tile of the header, and aboutGlyphSize the
// application mark inside it. The mark is not square: its height follows its
// view box.
const (
	aboutMarkSize  = float32(48)
	aboutGlyphSize = float32(27)
)

// about is 关于软件: what this copy is, how to keep it up to date, where the
// project lives. The composition follows motty's about page — an identity header
// with one copy action, the build's facts as a tile grid, the update row, two
// places to go and the framework's colophon — carried by this application's type
// scale and its own links.
func (m *Model) about(c ui.C, spec Spec) {
	facts := buildinfo.Read()
	ui.Column(c).Gap(layout.GroupGap).Children(func() {
		m.aboutHeader(c, spec, facts)
		controls.Section(c, "版本信息", func() { m.aboutFacts(c, facts) })
		controls.Section(c, "更新", func() {
			controls.Group(c, "", func() {
				controls.Row(c, "检查更新", "新版本检查尚未接入。", func() {
					typography.Caption(c, "暂未支持")
				})
			})
		})
		controls.Section(c, "项目", func() { m.aboutLinks(c, spec) })
		m.aboutColophon(c, spec)
	})
}

// aboutHeader is the application's identity: the mark, the name, one line about
// what this is, and the one action the page owns — copying the build's facts for
// a bug report.
func (m *Model) aboutHeader(c ui.C, spec Spec, facts buildinfo.Facts) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Background(theme.Card).
		Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).Children(func() {
		ui.Row(c).MinHeight(96).FillWidth().PadSides(18, 18, 16, 16).Gap(16).
			Items(ui.Center).Children(func() {
			mark := ui.Row(c).Size(aboutMarkSize, aboutMarkSize).Shrink(0).Center().
				Radius(theme.Metrics.RadiusSurface).Background(theme.Primary)
			mark.Children(func() {
				glyph := icons.AppMark
				height := aboutGlyphSize * glyph.ViewHeight / glyph.ViewBox
				ui.Icon(c, glyph).Size(aboutGlyphSize, height).Color(theme.PrimaryForeground)
			})
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				typography.Title(c, "Transactions")
				typography.CaptionParagraph(c, "本地优先的个人财务工作台：账本、记录、分类标签与图表都在你自己的目录里。", 2)
			})
			if ui.Button(c, "复制版本信息").Outline().Clicked() {
				m.copyBuildInfo(spec, facts)
			}
		})
	})
}

// aboutFact is one fact of the build: its name and its value.
type aboutFact struct {
	label string
	value string
}

// aboutFacts is the build grid: two columns, two facts each, separated by the
// card's hairlines. The values are mono — they are machine facts, quoted exactly.
func (m *Model) aboutFacts(c ui.C, facts buildinfo.Facts) {
	controls.Card(c, func() {
		ui.Row(c).FillWidth().Children(func() {
			aboutFactColumn(c,
				aboutFact{"版本", facts.VersionText()},
				aboutFact{"构建", facts.BuildText()},
			)
			ui.Divider(c)
			aboutFactColumn(c,
				aboutFact{"运行平台", facts.Platform},
				aboutFact{"运行时", facts.Runtime},
			)
		})
	})
}

func aboutFactColumn(c ui.C, first, second aboutFact) {
	ui.Column(c).Grow(1).Basis(0).Children(func() {
		aboutFactTile(c, first)
		ui.Divider(c)
		aboutFactTile(c, second)
	})
}

func aboutFactTile(c ui.C, fact aboutFact) {
	ui.Column(c).FillWidth().MinHeight(64).PadSides(16, 16, 12, 12).
		Justify(ui.Center).Gap(2).Children(func() {
		typography.Caption(c, fact.label)
		typography.Body(c, fact.value).Mono().Ellipsis()
	})
}

// aboutLinks are the two places to go: the project and its issue tracker.
func (m *Model) aboutLinks(c ui.C, spec Spec) {
	ui.Row(c).FillWidth().Gap(layout.BlockGap).Children(func() {
		m.aboutLinkCard(c, spec,
			"项目主页", "源码、发布版本与使用说明。", projectURL)
		m.aboutLinkCard(c, spec,
			"问题反馈", "在 GitHub 上报告问题或提出建议。", issuesURL)
	})
}

// aboutLinkCard is one place to go: its name, what it is for, and the sign that
// it opens outside the window. The whole card is the target.
func (m *Model) aboutLinkCard(c ui.C, spec Spec, label, description, target string) {
	theme := c.Theme()
	card := ui.Column(c).Grow(1).Basis(0).Height(86).PadSides(14, 14, 12, 12).
		Gap(4).Justify(ui.Center).
		Background(theme.Card).Border(1, theme.Border).
		Radius(theme.Metrics.RadiusControl)
	card = card.HitWith(ui.ID(c), label, gfx.HitOptions{
		Role: gfx.RoleLink, Cursor: gfx.PointerCursorHand, Description: description,
	}).Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	card.Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(6).Children(func() {
			typography.Body(c, label).Grow(1).Ellipsis()
			ui.Icon(c, icons.ExternalLink).Size(14, 14).Color(theme.MutedForeground)
		})
		typography.CaptionParagraph(c, description, 2)
	})
	if card.Clicked() {
		m.openExternal(spec, target)
	}
}

// aboutColophon closes the page with the framework that draws the window. It is
// muted until the pointer rests on it or it takes the keyboard: a blue line under
// the last card would be the loudest thing on a quiet page.
func (m *Model) aboutColophon(c ui.C, spec Spec) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
		line := ui.Row(c).Items(ui.Center).Gap(4).PadSides(8, 8, 8, 8).
			Radius(theme.Metrics.RadiusItem())
		line = line.HitWith(ui.ID(c), "界面由 Viva 绘制", gfx.HitOptions{
			Role: gfx.RoleLink, Cursor: gfx.PointerCursorHand,
			Description: "在浏览器里打开 Viva 的主页",
		}).Hover(ui.Look{}.Background(theme.Hover)).
			Focus(ui.Look{}.Ring(2, theme.Ring))
		line.Children(func() {
			ui.Icon(c, icons.ExternalLink).Size(11, 11).Color(theme.MutedForeground)
			typography.Caption(c, "界面由 Viva 绘制")
		})
		if line.Clicked() {
			m.openExternal(spec, vivaURL)
		}
	})
}

// copyBuildInfo puts the build's facts on the clipboard, ready to paste into a
// bug report.
func (m *Model) copyBuildInfo(spec Spec, facts buildinfo.Facts) {
	if spec.CopyText == nil {
		return
	}
	if err := spec.CopyText(facts.CopyText()); err != nil {
		m.toaster.Show("复制版本信息失败："+err.Error(), "", ui.ToastError)
		return
	}
	m.toaster.Show("版本信息已复制", "", ui.ToastSuccess)
}

// openExternal opens a place outside the window, in the user's browser.
func (m *Model) openExternal(spec Spec, target string) {
	if spec.OpenURL == nil {
		return
	}
	if err := spec.OpenURL(target); err != nil {
		m.toaster.Show("打开链接失败："+err.Error(), "", ui.ToastError)
	}
}
