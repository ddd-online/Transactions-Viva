// Package desktop is the application shell: the window, the custom title bar,
// the left navigation rail and the section content. Feature pages live in
// internal/pages and know nothing about the shell.
package desktop

import (
	"context"
	"os"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/appicon"
	"transactions/internal/backup"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/config"
	"transactions/internal/icons"
	"transactions/internal/pages/accounting"
	"transactions/internal/pages/accounting/ledger"
	"transactions/internal/pages/diary"
	"transactions/internal/pages/events"
	"transactions/internal/pages/notes"
	"transactions/internal/pages/settings"
	"transactions/internal/pages/stock"
	"transactions/internal/pages/todo"
	"transactions/internal/stockquote"
	storepkg "transactions/internal/store"
)

// Model is the shell's state and the owners of every page's state. A page
// model is created once and kept, so switching sections preserves its tab,
// filters and scroll position.
type Model struct {
	section Section
	dark    bool
	theme   gfx.Theme

	// config is the loaded user configuration; configStore writes it back when
	// a setting changes. A model built by New has no store and keeps its
	// configuration in memory.
	config      config.AppConfig
	configStore *config.Store

	// invalidator asks the window for a frame when something outside a click
	// changes what is painted (a system appearance change, a close prompt).
	invalidator *app.Invalidator
	// quit ends the application after an update's installer has started.
	quit func()
	// launchUpdate starts the downloaded installer; tests replace it.
	launchUpdate func(string) error
	// checkUpdateFn and downloadUpdateFn stand in for the GitHub client;
	// a run leaves them nil and they reach the real one.
	checkUpdateFn    func(context.Context) (settings.UpdateRelease, error)
	downloadUpdateFn func(context.Context, string, string, func(settings.UpdateProgress)) (string, error)

	// window and tray are the native handles Run opens; close.go and tray.go
	// use them. trayOK says the status area accepted the icon.
	window *app.Window
	tray   *app.StatusItem
	trayOK bool

	// development is true under `viva dev`'s reload host, which paints only on
	// its own requests; see requestClose.
	development bool

	// pickWorkspace chooses the workspace directory; tests replace it.
	pickWorkspace func() (string, error)

	// closeAsk is the 关闭窗口 dialog, raised by the close button when the
	// configuration asks every time. closeAllow is the pending close.
	closeAsk   bool
	closeAllow func()

	// tree is the page's identity tree. New names the page's feature in it so
	// this page's identities stay out of the development reload host's scope,
	// where its window buttons live.
	tree gfx.Tree

	// expanded is the section whose second-level entries are showing; only one
	// section is expanded at a time. A section without sub-functions closes the
	// open one.
	expanded     Section
	expandedOpen bool

	accounting *accounting.Model
	// ledgers is the list every ledger-aware page reads: 记账 and 日记.
	ledgers  *ledger.Store
	stock    *stock.Model
	todo     *todo.Model
	events   *events.Model
	diary    *diary.Model
	notes    *notes.Model
	settings *settings.Model
	backups  *backup.Service

	// workspace is the open SQLite workspace; nil means the user still has to
	// choose one. workspaceErr is the last open failure, shown by the gate.
	workspace    *storepkg.Workspace
	workspaceErr string

	// pickDirectory is the platform's directory chooser, for the workspace and
	// the diary import / export; tests replace it.
	pickDirectory func(title string) (string, error)

	// pickImages and pickSaveFile are the platform's image dialogs for the 事件
	// page. Tests queue answers through the driver; a run opens the real
	// pickers.
	pickImages   func() ([]string, error)
	pickNoteFile func() (string, error)
	pickSaveFile func(options app.SaveFileOptions) (string, error)
	pickBackup   func() (string, error)
}

// New returns the shell with the default configuration, pinned to the light
// appearance. It owns no configuration file, which keeps tests independent of
// the machine and its user settings; Run loads the file and calls
// NewWithConfig.
func New() *Model {
	cfg := config.Default()
	cfg.Appearance = config.AppearanceLight
	return NewWithConfig(cfg, nil)
}

// NewWithConfig returns the shell over cfg and, when store is not nil, saves
// every later change through it. The shell opens on the first enabled feature.
func NewWithConfig(cfg config.AppConfig, store *config.Store) *Model {
	// The 记账 pages and the 日记 page read one ledger list, so switching 账本 on
	// either one is the same choice.
	ledgers := ledger.NewStore()
	quotes := &stockquote.TencentFetcher{Client: stockHTTPClient(cfg.Proxy)}
	m := &Model{
		config:      cfg,
		configStore: store,
		accounting:  accounting.NewWithLedgers(ledgers),
		ledgers:     ledgers,
		stock:       stock.New(quotes),
		todo:        todo.New(),
		events:      events.New(),
		diary:       diary.New(),
		notes:       notes.New(),
		settings:    settings.New(),
		backups:     backup.New(),
	}
	// pickDirectory is the platform's directory chooser. Tests replace it, so a
	// run never opens a real dialog; the diary import and export use it too.
	m.pickDirectory = func(title string) (string, error) {
		return app.OpenDirectory(app.OpenDirectoryOptions{Title: title})
	}
	m.pickWorkspace = func() (string, error) { return m.pickDirectory("选择工作空间") }
	m.pickImages = func() ([]string, error) {
		return app.OpenFiles(app.OpenFileOptions{
			Title:         "添加事件图片",
			AllowMultiple: true,
			Filters: []app.FileFilter{{
				Name:    "图片与实况视频",
				Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.heic;*.heif;*.mov;*.mp4;*.m4v;*.webm",
			}},
		})
	}
	m.pickNoteFile = func() (string, error) {
		files, err := app.OpenFiles(app.OpenFileOptions{
			Title: "插入图片",
			Filters: []app.FileFilter{{
				Name:    "图片",
				Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.webp;*.heic;*.heif",
			}},
		})
		if err != nil || len(files) == 0 {
			return "", err
		}
		return files[0], nil
	}
	m.pickSaveFile = func(options app.SaveFileOptions) (string, error) {
		return app.SaveFile(options)
	}
	m.pickBackup = func() (string, error) {
		files, err := app.OpenFiles(app.OpenFileOptions{
			Title: "选择备份文件",
			Filters: []app.FileFilter{{
				Name:    "SQLite 数据库",
				Pattern: "*.db",
			}},
		})
		if err != nil || len(files) == 0 {
			return "", err
		}
		return files[0], nil
	}
	m.events.SetLinksOpen(cfg.KeyEventLinkedOpen)
	m.events.SetCommands(m.eventCommands())
	m.notes.SetCommands(m.notesCommands())
	m.refreshTheme()
	m.section = m.firstEnabledSection()
	if m.section != SectionSettings {
		m.expanded = m.section
		m.expandedOpen = len(m.pageTabs(m.section)) > 0
	}
	// The development reload host names its window buttons in a tree of its
	// own; its first feature is the buttons, at scope 2. Keep this page's
	// identities out of that scope so a host button cannot share an identity
	// with a sidebar control. A name-only feature shifts the page's tree by one.
	m.tree.Feature("transactions")
	return m
}

// Run opens the application window and blocks until it closes. It loads the
// configuration, follows the system appearance while the configuration asks
// for it, and keeps one status-area icon so closing to the tray has a way back.
// dev selects the development config for a native window, as `go run . -dev`
// does; the reload host (`viva dev`) always counts as development, and so does a
// headless test build, which additionally refuses the production workspace.
func Run(dev bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	reloadHost := devMode()
	development := usesDevelopmentConfig(dev, reloadHost)
	store := config.Load(config.Path(home, development))
	cfg := store.Config()
	if headlessConfig {
		// A test build never opens the workspace the production config names,
		// even when the development config was pointed at it.
		cfg = guardWorkspace(cfg, home)
	}
	model := NewWithConfig(cfg, store)
	// Only the reload host needs its own close behavior: it paints on the
	// host's schedule, so a native prompt could wait for an unrelated frame.
	model.development = reloadHost
	model.openConfiguredWorkspace()
	defer model.closeWorkspace()
	page := ui.NewPage(ui.PageConfig{Tree: &model.tree, View: model.page, Theme: &model.theme})
	if reloadHost {
		// `viva dev` drives the page through the reload worker inside
		// RunCanvas. RunApplication has no such worker: it would open a native
		// window and the reload host would time out waiting for the connection.
		// The development host draws its own window, so the tray and the native
		// close prompt stay a release-build feature; closing in dev uses the
		// same configured behavior, with "tray" falling back to quit.
		return app.RunCanvas(ui.Configure(page, newConfig(model)), new(gfx.Canvas))
	}
	invalidator := &app.Invalidator{}
	model.invalidator = invalidator
	return app.RunApplication(func(application *app.Application) error {
		windowConfig := newConfig(model)
		windowConfig.Invalidator = invalidator
		windowConfig.OnClosed = application.Quit
		window, err := application.OpenWindow(ui.Configure(page, windowConfig), new(gfx.Canvas))
		if err != nil {
			return err
		}
		model.window = window
		model.quit = application.Quit
		model.trayOK = model.addTray(application)
		app.WatchSystemAppearance(func() {
			model.refreshTheme()
			invalidator.Invalidate()
		})
		return nil
	})
}

// devMode reports whether the development reload host is running this build:
// it sets VIVA_RELOAD_ADDRESS for the worker it launches.
func devMode() bool { return os.Getenv("VIVA_RELOAD_ADDRESS") != "" }

// newConfig is the one place the native window is described. Tests build the
// same window without opening one.
func newConfig(m *Model) app.Config {
	// The window's edge is the shell around the workspace card: the title bar
	// and the navigation rail. Windows 11 composites its border around a
	// custom-titlebar window against the desktop, so the border has to name the
	// surface it meets or the desktop shows through as a white seam.
	edge := m.theme.Muted
	width, height := m.config.Width, m.config.Height
	if width <= 0 {
		width = 1500
	}
	if height <= 0 {
		height = 1000
	}
	width = max(width, MinimumWindowWidth)
	height = max(height, MinimumWindowHeight)
	icon, _ := appicon.Load()
	config := app.Config{
		Title:     "Transactions",
		Icon:      icon,
		Width:     width,
		Height:    height,
		MinWidth:  MinimumWindowWidth,
		MinHeight: MinimumWindowHeight,
		// The application draws its own 36px title bar; Viva draws the
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
		OnPointer:         m.pointer,
		OnScroll:          m.scroll,
		TextInput:         m.textInput,
		// The close button saves the frame and applies the configured behavior.
		// Tests call this callback directly to exercise the prompt.
		OnCloseRequest: func(allow func()) { m.requestClose(allow) },
	}
	if m.config.X != nil && m.config.Y != nil {
		config.Origin = &gfx.Point{X: float32(*m.config.X), Y: float32(*m.config.Y)}
	}
	return config
}

// key gives the records page's canvas controls (the tags multi-select) the
// keys the page itself did not take.
func (m *Model) key(event gfx.KeyEvent) gfx.EventResult {
	if m.section == SectionAccounting {
		return m.accounting.Key(event)
	}
	return gfx.EventResult{}
}

// pointer gives the analysis chart the pointer over its plot: the chart is
// painted by a component that registers a hit region, and the page turns the
// pointer over it into the hovered point.
func (m *Model) pointer(event gfx.PointerEvent) gfx.EventResult {
	if m.section == SectionAccounting {
		return m.accounting.Pointer(event)
	}
	if m.section == SectionStock {
		return m.stock.Pointer(event)
	}
	return gfx.EventResult{}
}

// scroll gives the records page's tags popup the wheel, and the notes page the
// wheel of its body: the note keeps that area's offset itself.
func (m *Model) scroll(event gfx.ScrollEvent) gfx.EventResult {
	switch m.section {
	case SectionAccounting:
		return m.accounting.Scroll(event)
	case SectionNotes:
		return m.notes.Scroll(event)
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
	// The buttons sit on the title bar, so their feedback has to read on the
	// chrome's tone: the same ink washes the page rows use, one step stronger
	// than a row's hover because these targets are small.
	return app.WindowControlsStyle{
		Foreground:      theme.Foreground,
		Hover:           state.Selected(theme),
		Pressed:         state.Pressed(theme),
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

// MinimumWindowWidth and MinimumWindowHeight match motty's main window: the
// navigation, titlebar controls and accounting content stay usable at this
// size. Viva reduces a minimum only when the display's work area is smaller.
const (
	MinimumWindowWidth  = 960
	MinimumWindowHeight = 620
)

// page declares the whole window: title bar, navigation and content. The
// window's own surface is the shell the card floats on.
func (m *Model) page(c ui.C) {
	// The update keeps running while another section is shown: an effect the
	// paint stops declaring is cancelled, so the shell declares it every paint.
	m.settings.KeepUpdate(c)
	leading, trailing := app.TitlebarInsets()
	theme := c.Theme()
	ui.Column(c).Fill().Background(theme.Muted).Children(func() {
		m.titlebar(c, leading, trailing)
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			m.sidebar(c)
			m.content(c)
		})
	})
}

// titlebar is the 36px application band: the product name at the left and the
// appearance toggle at the right, clear of the window buttons.
func (m *Model) titlebar(c ui.C, leading, trailing float32) {
	theme := c.Theme()
	ui.Row(c).Height(36).FillWidth().Background(theme.Muted).
		PadLeft(leading + 12).PadRight(trailing + 8).Gap(8).Items(ui.Center).
		MovesWindow().Children(func() {
		typography.Caption(c, "Transactions").Bold()
		ui.Spacer(c)
		icon, tip := icons.Moon, "切换到深色模式"
		if m.dark {
			icon, tip = icons.Sun, "切换到浅色模式"
		}
		button := ui.ButtonBase(c, tip).Size(32, 32).
			Radius(theme.Metrics.RadiusControl).
			Hover(ui.Look{}.Background(state.Hover(*theme))).
			Pressed(ui.Look{}.Background(state.Pressed(*theme))).
			Focus(ui.Look{}.Ring(2, theme.Ring)).
			Tooltip(tip)
		button.Children(func() {
			ui.Icon(c, icon).Size(16, 16)
		})
		if button.Clicked() {
			m.toggleTheme()
		}
	})
}

// content is the active section, in the rounded workspace card the reference
// application floats on its shell: flush under the title bar and against the
// navigation rail, with room on the right and at the bottom.
func (m *Model) content(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).
		MarginSides(0, contentCardMargin, contentCardMargin, 0).
		Radius(theme.Metrics.RadiusSurface).
		Border(1, theme.Border).
		Background(theme.Background).
		Clip().
		Children(func() {
			if m.workspace == nil && m.section != SectionSettings {
				m.workspaceGate(c)
				return
			}
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
			case SectionNotes:
				m.notes.View(c)
			case SectionSettings:
				m.settings.View(c, m.settingsSpec())
			}
		})
	m.closeModal(c)
}

// toggleTheme switches between the light and dark palettes, as an explicit
// appearance choice; from 跟随系统 the first click pins the opposite of what is
// on screen, as the titlebar toggle did.
func (m *Model) toggleTheme() {
	next := config.AppearanceDark
	if m.dark {
		next = config.AppearanceLight
	}
	_ = m.setAppearance(next)
}

// effectiveDark resolves the configured appearance against the system.
func (m *Model) effectiveDark() bool {
	switch m.config.Appearance {
	case config.AppearanceDark:
		return true
	case config.AppearanceLight:
		return false
	}
	return app.SystemAppearance() == app.AppearanceDark
}

// refreshTheme recomputes the palette. The page holds &m.theme, so the next
// paint uses the new one.
func (m *Model) refreshTheme() {
	m.dark = m.effectiveDark()
	m.theme = newTheme(m.dark)
}
