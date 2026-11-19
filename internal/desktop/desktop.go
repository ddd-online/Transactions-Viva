// Package desktop is the application shell: the window, the custom title bar,
// the left navigation rail, the footer and the section content. Feature pages
// live in internal/pages and know nothing about the shell.
package desktop

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/icons"
	"transactions/internal/pages/accounting"
	"transactions/internal/pages/diary"
	"transactions/internal/pages/events"
	"transactions/internal/pages/settings"
	"transactions/internal/pages/stock"
	"transactions/internal/pages/todo"
)

// Version is the application version shown in the footer. A release build can
// replace it with -ldflags "-X transactions/internal/desktop.Version=...".
var Version = "0.1.0"

// Model is the shell's state and the owners of every page's state. A page
// model is created once and kept, so switching sections preserves its tab,
// filters and scroll position.
type Model struct {
	section Section
	dark    bool
	theme   gfx.Theme

	// tree is the page's identity tree. The sidebar items are named in it once,
	// because Viva's Sidebar paints them inside a Draw callback.
	tree          gfx.Tree
	navIDs        [len(navSections)]gfx.ID
	subIDs        [int(SectionSettings) + 1][maxSectionTabs]gfx.ID
	navSettingsID gfx.ID

	// expanded is the section whose second-level entries are showing; only one
	// section is expanded at a time. A section without sub-functions closes the
	// open one.
	expanded     Section
	expandedOpen bool
	subItems     []components.SidebarSubItem

	// The sidebar is a ui.Control painted by Viva's Sidebar component. Its
	// handlers and draw function are made once, with the model.
	navHandlers ui.Handlers
	drawNav     func(*ui.DrawContext)

	accounting *accounting.Model
	stock      *stock.Model
	todo       *todo.Model
	events     *events.Model
	diary      *diary.Model
	settings   *settings.Model
}

// New returns the shell on the 记账 section in the light theme. The saved
// appearance (system / light / dark) takes over when config persistence lands.
func New() *Model {
	m := &Model{
		section:    SectionAccounting,
		theme:      newTheme(false),
		accounting: accounting.New(),
		stock:      stock.New(),
		todo:       todo.New(),
		events:     events.New(),
		diary:      diary.New(),
		settings:   settings.New(),
	}
	m.expanded, m.expandedOpen = SectionAccounting, true
	m.subItems = make([]components.SidebarSubItem, 0, maxSectionTabs)
	// The development reload host names its window buttons in a tree of its
	// own; its first feature is the buttons, at scope 2. Keep this page's
	// identities out of that scope so a host button cannot share an identity
	// with a sidebar control. A name-only feature shifts the page's tree by one.
	m.tree.Feature("transactions")
	m.initNavIDs()
	m.navHandlers.Role = gfx.RoleGroup
	m.drawNav = m.drawSidebar
	return m
}

// Run opens the application window and blocks until it closes.
func Run() error {
	model := New()
	page := ui.NewPage(ui.PageConfig{Tree: &model.tree, View: model.page, Theme: &model.theme})
	return app.RunCanvas(page.Config(newConfig(model)), new(gfx.Canvas))
}

// newConfig is the one place the native window is described. Tests build the
// same window without opening one.
func newConfig(m *Model) app.Config {
	// The window's edge is the shell around the workspace card: the title bar,
	// the footer and the navigation rail. Windows 11 composites its border around
	// a custom-titlebar window against the desktop, so the border has to name the
	// surface it meets or the desktop shows through as a white seam.
	edge := m.theme.Muted
	return app.Config{
		Title:  "Transactions",
		Width:  1500,
		Height: 1000,
		// The old application draws its own 36px title bar; Viva draws the
		// window buttons on Windows and Linux, and keeps the macOS traffic lights.
		WindowMode:     app.WindowCustomTitlebar,
		TitlebarHeight: 36,
		// The buttons Viva draws follow the theme, and are named in Chinese for
		// assistive technology. A development reload host does not receive this
		// callback, so it falls back to ClearColor to pick the button ink; a
		// clear color from the light theme keeps those buttons visible there.
		WindowControls: m.windowControls,
		ClearColor:     windowColor(edge),
		// A release window reads this on every paint, so the border follows the
		// appearance toggle; a development reload host uses ClearColor here.
		WindowBorderColor: m.windowBorderColor,
		OnKey:             m.key,
		OnScroll:          m.scroll,
		TextInput:         m.textInput,
	}
}

// key gives the records page's canvas controls (the tags multi-select) the
// keys the page itself did not take.
func (m *Model) key(event gfx.KeyEvent) gfx.EventResult {
	if m.section == SectionAccounting {
		return m.accounting.Key(event)
	}
	return gfx.EventResult{}
}

// scroll gives the records page's tags popup the wheel.
func (m *Model) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if m.section == SectionAccounting {
		return m.accounting.Scroll(event)
	}
	return gfx.EventResult{}
}

// textInput supplies the native editor of the tags multi-select; the page
// answers for its ordinary TextFields before asking this provider.
func (m *Model) textInput(ctx app.TextInputContext) *app.TextInput {
	if m.section != SectionAccounting {
		return nil
	}
	editor := m.accounting.TagsEditor(ctx.ID)
	if editor == nil {
		return nil
	}
	return &app.TextInput{Editor: editor}
}

// windowBorderColor is the color of the system-drawn border around the window:
// the shell surface the window's own edge is painted with, so the border does
// not read as a white ring (ClearColor's default) in either appearance.
func (m *Model) windowBorderColor() app.Color {
	return windowColor(m.theme.Muted)
}

// windowColor converts a theme color to the window config's color type.
func windowColor(color gfx.Color) app.Color {
	return app.Color{
		Red: float64(color.Red), Green: float64(color.Green),
		Blue: float64(color.Blue), Alpha: float64(color.Alpha),
	}
}

// windowControls is read on every paint, so the window buttons follow the
// appearance toggle as the rest of the window does.
func (m *Model) windowControls() app.WindowControlsStyle {
	theme := m.theme
	return app.WindowControlsStyle{
		Foreground:      theme.Foreground,
		Hover:           theme.Hover,
		Pressed:         theme.Secondary,
		Close:           theme.Danger,
		CloseForeground: theme.DangerForeground,
		Minimize:        "最小化",
		Maximize:        "最大化",
		Restore:         "还原",
		CloseName:       "关闭",
	}
}

// contentCardMargin is the room the workspace card keeps from the window's
// right and bottom edges, after the reference application: the card sits on the
// shell surface instead of filling the window.
const contentCardMargin = float32(7)

// page declares the whole window: title bar, navigation, content and footer.
// The window's own surface is the shell the card floats on.
func (m *Model) page(c ui.C) {
	leading, trailing := app.TitlebarInsets()
	theme := c.Theme()
	ui.Column(c).Fill().Background(theme.Muted).Children(func() {
		m.titlebar(c, leading, trailing)
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			m.sidebar(c)
			m.content(c)
		})
		m.footer(c)
	})
}

// titlebar is the 36px application band: the product name at the left and the
// appearance toggle at the right, clear of the window buttons.
func (m *Model) titlebar(c ui.C, leading, trailing float32) {
	theme := c.Theme()
	ui.Row(c).Height(36).FillWidth().Background(theme.Muted).
		PadLeft(leading + 12).PadRight(trailing + 8).Gap(8).Items(ui.Center).
		MovesWindow().Children(func() {
		ui.Text(c, "Transactions").FontSize(12).Bold()
		ui.Spacer(c)
		icon, tip := icons.Moon, "切换到深色模式"
		if m.dark {
			icon, tip = icons.Sun, "切换到浅色模式"
		}
		if ui.Button(c, tip).Icon(icon).IconOnly().Ghost().Tooltip(tip).Clicked() {
			m.toggleTheme()
		}
	})
}

// content is the active section, in the rounded workspace card the reference
// application floats on its shell: flush under the title bar and against the
// navigation rail, with room on the right and above the footer.
func (m *Model) content(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).
		MarginSides(0, contentCardMargin, contentCardMargin, 0).
		Radius(theme.Metrics.RadiusSurface).
		Border(1, theme.Border).
		Background(theme.Background).
		Clip().
		Children(func() {
			switch m.section {
			case SectionAccounting:
				m.accounting.View(c)
			case SectionStock:
				m.stock.View(c)
			case SectionTodo:
				m.todo.View(c)
			case SectionEvents:
				m.events.View(c)
			case SectionDiary:
				m.diary.View(c)
			case SectionSettings:
				m.settings.View(c)
			}
		})
}

// footer is the 36px band that runs under the whole window.
func (m *Model) footer(c ui.C) {
	theme := c.Theme()
	ui.Row(c).Height(36).FillWidth().Background(theme.Muted).PadX(12).Gap(8).
		Items(ui.Center).Children(func() {
		ui.Text(c, "本地优先 · 数据保存在本地工作空间").FontSize(11).Muted()
		ui.Spacer(c)
		ui.Text(c, "尚未打开工作空间").FontSize(11).Muted()
		ui.Text(c, "v"+Version).FontSize(11).Muted().Mono()
	})
}

// toggleTheme switches between the light and dark palettes. The page holds
// &m.theme, so the next paint uses the new one.
func (m *Model) toggleTheme() {
	m.dark = !m.dark
	m.theme = newTheme(m.dark)
}
