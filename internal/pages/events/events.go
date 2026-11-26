// Package events is the 事件 section: a workspace's key events, one per local
// date, with colour, body and photo assets, and the transactions whose 关联事件
// 日期 names the day.
//
// Events belong to the workspace, not to a ledger, so this page has no ledger
// picker and every workspace has one set of events. Records stay per ledger,
// and the linked panel shows the records of every ledger that name the day.
//
// The page owns no SQL and no files: a workspace reaches it as
// repository.Repository through SetDataSource, and picking, storing and deleting
// image files belongs to the shell through Commands.
package events

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/emptystate"
	"transactions/internal/components/imageviewer"
	"transactions/internal/components/layout"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

// The page's measurements. The left rail uses the shared page rail width so it
// lines up with every other section; the linked-trades rail keeps its own clamp
// because it is a secondary pane, not a section rail.
const (
	linksMin       float32 = 180
	linksMax       float32 = 280
	linksShare             = 0.19
	cardMinHeight  float32 = 80
	stageMinHeight float32 = 160
	thumbWidth     float32 = 160
	thumbHeight    float32 = 90
	swatchSize     float32 = 22
	// emptyGalleryHeight keeps the no-photo state a line of information, not a
	// pane: the body is the content of a day without pictures. bodyEmptyHeight
	// is the same for a day with pictures and no text.
	emptyGalleryHeight float32 = 120
	bodyEmptyHeight    float32 = 96
	bodyEditHeight     float32 = 160
	linkedAmountWidth  float32 = 80
)

// AddImagesResult is what one 添加图片 did: how many images were stored and the
// sentences the page shows for the picked files it could not use.
type AddImagesResult struct {
	Added    int
	Problems []string
}

// Commands is the platform work the page needs: native pickers and the files
// behind the workspace. The shell supplies them; tests replace the shell's
// pickers, not the page.
type Commands struct {
	// AddImages opens the image picker and stores the picked stills (with
	// their same-named motion videos) on the day's event.
	AddImages func(date string) (AddImagesResult, error)
	// DeleteEvent removes the day's event, its image rows and their files.
	DeleteEvent func(date string) error
	// DeleteImage removes one image row and its files.
	DeleteImage func(imageID string) error
	// SaveImage copies one stored image to where the user saves it.
	SaveImage func(relativePath string) error
	// LoadImage decodes one stored image for painting (a thumbnail, or the
	// display file when no view copy exists).
	LoadImage func(relativePath string) (gfx.Bitmap, error)
	// LoadView decodes the small stage copy of a display image, making it on
	// first use when the workspace predates view copies.
	LoadView func(relativePath string) (gfx.Bitmap, error)
	// SetLinksOpen persists the linked-transactions rail's open state.
	SetLinksOpen func(open bool) error
}

// addForm is the 新增事件 dialog.
type addForm struct {
	open  bool
	date  string
	title string
	pick  datepicker.State
}

// displayRequest names one stage copy. Serial changes when the selected event or
// picture changes, so a path the effect already loaded is requested again after
// its bitmap was released.
type displayRequest struct {
	Path   string
	Serial int
}

// confirmation is one delete dialog: the event's day and name, or the image.
type confirmation struct {
	open    bool
	date    string
	title   string
	imageID string
}

// preview is the lightbox's place in the day's list; the zoom, the pan and the
// dialog itself belong to the viewer both pages share.
type preview struct {
	index int
}

// Model is the 事件 page's state.
type Model struct {
	repo repository.Repository
	cmds Commands

	// linksOpen is whether the linked-transactions rail shows. The shell keeps
	// the same value in the configuration and persists it.
	linksOpen bool

	// year is the calendar year the list shows; date is the selected event's
	// day ("" when nothing is selected).
	year string
	date string
	// events is the year's list, newest first; event is the selected row.
	events   []domain.KeyEvent
	event    domain.KeyEvent
	hasEvent bool

	// images are the event's pictures in their stored order, selectedImageID
	// the one the stage shows ("" means the first), linked the transactions of
	// every ledger that name the day, and ledgerNames names those ledgers.
	images          []domain.KeyEventImage
	selectedImageID string
	linked          []repository.RecordEntry
	ledgerNames     map[string]string

	// editing and draft are the body editor; color is the event's current tone.
	editing bool
	draft   string
	color   string

	// bits caches the current day's decoded thumbnails; bitErrs remembers the
	// paths that could not be decoded so a paint does not retry them.
	// displayPath is the one full photo the stage and the lightbox paint, and
	// blurID/blurBitmap the one backdrop behind it. A gallery can hhundreds
	// of pictures and a year can hhundreds of days: keeping every viewed
	// full photo and every backdrop is not a cache, it is a leak.
	bits          map[string]gfx.Bitmap
	bitErrs       map[string]string
	displayPath   string
	displaySerial int
	display       *ui.EffectState[gfx.Bitmap, displayRequest]
	blurID        string
	blurBitmap    gfx.Bitmap

	add         addForm
	deleteEvent confirmation
	deleteImage confirmation
	preview     preview
	viewer      *imageviewer.Viewer
	pick        datepicker.State

	errorMessage string
	bounds       gfx.Rect

	// pageScroll is the keyboard's handle on the page's scroller: PageUp and
	// PageDown move it, so the year's list and a long description are readable
	// without the pointer.
	pageScroll layout.PageScroll
}

// New returns the page on the current year with nothing selected.
func New() *Model {
	m := &Model{
		year:      strconv.Itoa(time.Now().Year()),
		linksOpen: true,
		pick:      datepicker.NewState(),
		add:       addForm{pick: datepicker.NewState()},
		bits:      map[string]gfx.Bitmap{},
		bitErrs:   map[string]string{},
	}
	m.viewer = imageviewer.New()
	m.display = ui.NewEffectKeyed(func(_ context.Context, request displayRequest) (gfx.Bitmap, error) {
		loader := m.cmds.LoadView
		if loader == nil {
			loader = m.cmds.LoadImage
		}
		if loader == nil {
			return gfx.Bitmap{}, fmt.Errorf("图片无法显示")
		}
		return loader(request.Path)
	}).Sequential()
	return m
}

// SetDataSource connects the page to a workspace. A nil repository empties it,
// as it does when the workspace closes.
func (m *Model) SetDataSource(repo repository.Repository) {
	m.repo = repo
	m.clearDetail()
	m.events = nil
	if repo != nil {
		m.Reload()
	}
}

// SetCommands hands the page the shell's file and picker work.
func (m *Model) SetCommands(cmds Commands) { m.cmds = cmds }

// SetLinksOpen restores the linked rail's stored state.
func (m *Model) SetLinksOpen(open bool) { m.linksOpen = open }

// Reload reads the chosen year's events and the selected day's detail. The
// shell calls it when the page is entered; the page calls it after its own
// writes.
func (m *Model) Reload() {
	m.errorMessage = ""
	m.dropBitmaps()
	if m.repo == nil {
		m.events = nil
		m.clearDetail()
		return
	}
	ctx := context.Background()
	events, err := m.repo.KeyEventsByYear(ctx, m.year)
	if err != nil {
		m.errorMessage = "读取事件失败：" + err.Error()
		events = nil
	}
	m.events = events
	if m.date != "" && !m.findEvent() {
		m.clearDetail()
	}
	if m.hasEvent {
		m.loadDetail()
		m.ensureBitmaps()
	}
}

// CloseOverlays closes the floating panels when the navigation leaves the page.
func (m *Model) CloseOverlays() {
	m.pick.Open = false
	m.add.open = false
	m.deleteEvent.open = false
	m.deleteImage.open = false
	m.viewer.Hide()
}

// View paints the toolbar and the three panes.
// Key gives the page's scroller PageUp and PageDown, and gives an open date
// panel Escape before the dialog takes it: closing the dialog would throw away
// the title being typed.
func (m *Model) Key(event gfx.KeyEvent) gfx.EventResult {
	if event.Key == gfx.KeyEscape && m.add.open && m.add.pick.Open {
		m.add.pick.Open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	if m.add.open {
		if result := m.add.pick.Key(event); result.Handled {
			return result
		}
	}
	return m.pageScroll.Key(event)
}

func (m *Model) View(c ui.C) {
	m.bounds = c.Bounds()
	layout.Page(c, m.toolbar, func(c ui.C) {
		if m.errorMessage != "" {
			layout.ErrorBanner(c, m.errorMessage)
		}
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			width := c.Bounds().Width
			m.rail(c, layout.RailWidth)
			ui.Divider(c)
			m.detail(c)
			if m.linksOpen {
				ui.Divider(c)
				m.linksPanel(c, clamp(width*linksShare, linksMin, linksMax))
			}
		})
	})
	m.addDialog(c)
	m.deleteEventDialog(c)
	m.deleteImageDialog(c)
	m.previewDialog(c)
}

// toolbar is the band above the panes: the year, the linked rail's switch and
// 新增事件. The page read the active ledger from the title bar's menu; this
// page has no ledger, so the band holds nothing about one.
func (m *Model) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		datepicker.Picker(c, &m.pick, datepicker.Spec{
			Label:   "事件年份",
			Start:   m.year + "-01-01",
			End:     m.year + "-12-31",
			Grain:   datepicker.ByYear,
			Periods: true,
			// A year cell names both ends of the range at once, so the panel
			// closes on the pick as a form's date field does.
			CloseOnPick: true,
			Width:       148,
			Viewport:    m.bounds,
			OnRange: func(start, _ string) {
				m.setYear(start)
			},
		})
		ui.Spacer(c)
		label, icon := "收起关联交易", icons.ChevronRight
		if !m.linksOpen {
			label, icon = "展开关联交易", icons.ChevronLeft
		}
		if ui.Button(c, label).Ghost().Icon(icon).IconOnly().Tooltip(label).Clicked() {
			m.toggleLinks()
		}
		if ui.Button(c, "新增事件").Icon(icons.Plus).Clicked() {
			m.openAdd()
		}
	})
}

// rail is the year's events: a header with the count, then one card per day.
func (m *Model) rail(c ui.C, width float32) {
	layout.Rail(c, width, func(c ui.C) {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, "事件")
			layout.SectionCount(c, strconv.Itoa(len(m.events)))
			ui.Spacer(c)
		})
		ui.Divider(c)
		if len(m.events) == 0 {
			emptystate.Notice(c, "暂无事件")
			return
		}
		m.pageScroll.Area(ui.Scroll(c).Grow(1)).Children(func() {
			for index := range m.events {
				m.card(c, &m.events[index])
				ui.Divider(c)
			}
		})
	})
}

// card is one event: the 3 px identity bar, a title, the date with its dot and
// a one-line summary, with the delete command at the top-right.
func (m *Model) card(c ui.C, event *domain.KeyEvent) {
	theme := c.Theme()
	selected := event.Date == m.date
	tone := toneColor(event.Color, *theme)
	title := displayTitle(*event)
	background := gfx.Color{}
	if selected {
		background = state.Selected(*theme)
	}
	c.Key(event.Date)
	cardID := ui.ID(c)
	row := ui.Row(c).MinHeight(cardMinHeight).FillWidth().
		Background(background).
		HitWith(cardID, title, gfx.HitOptions{Role: gfx.RoleButton, Selected: selected}).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	row.Children(func() {
		ui.Row(c).Width(3).FillHeight().Background(tone)
		ui.Column(c).Grow(1).MinWidth(0).PadSides(12, 36, 12, 12).Gap(2).Children(func() {
			name := typography.BodyStrong(c, title).Ellipsis()
			if selected {
				name.Color(theme.Primary)
			}
			ui.Row(c).Gap(4).Items(ui.Center).Children(func() {
				ui.Row(c).Size(6, 6).Radius(3).Background(tone)
				typography.Caption(c, shortDate(event.Date))
			})
			typography.Caption(c, summary(event.Content)).Ellipsis()
		})
		m.cardDelete(c, event, title, cardID, selected)
	})
	if row.Clicked() {
		m.chooseEvent(event.Date)
	}
}

// cardDelete is the card's own delete command, quiet until the card is lit — the
// pointer on the card, the keyboard on it, or the event the page shows — the rule
// every rail row follows here. The key names its identity before it is declared,
// so moving the pointer onto the key keeps the card lit: without that the card
// would un-hover, the key would vanish under the pointer, and the click would
// never land.
func (m *Model) cardDelete(c ui.C, event *domain.KeyEvent, title string, cardID gfx.ID, selected bool) {
	theme := c.Theme()
	c.Key("event-delete-" + event.Date)
	keyID := ui.ID(c)
	lit := selected || c.Canvas().Hovered() == cardID || c.Canvas().Hovered() == keyID ||
		c.Canvas().FocusVisible() == cardID
	if !lit {
		return
	}
	remove := ui.ButtonBase(c, "删除事件「"+title+"」").Size(24, 24).Radius(theme.Metrics.RadiusControl).
		HitWith(keyID, "删除事件「"+title+"」", gfx.HitOptions{
			Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
		}).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip("删除事件").
		Absolute().Top(6).Right(6)
	remove.Children(func() {
		ui.Icon(c, icons.Trash2).Size(13, 13)
	})
	if remove.Clicked() {
		m.askDeleteEvent(event.Date, title)
	}
}

// chooseEvent selects one day's event and reads its images and transactions.
func (m *Model) chooseEvent(date string) {
	if date == m.date && m.hasEvent {
		return
	}
	m.date = date
	if !m.findEvent() {
		m.clearDetail()
		return
	}
	m.dropBitmaps()
	m.loadDetail()
	m.ensureBitmaps()
}

// findEvent fills the selected event from the list and reports whether the day
// is still in it.
func (m *Model) findEvent() bool {
	for index := range m.events {
		if m.events[index].Date == m.date {
			m.event = m.events[index]
			m.hasEvent = true
			m.color = m.event.Color
			return true
		}
	}
	m.hasEvent = false
	return false
}

// clearDetail drops the selection and everything read for it.
func (m *Model) clearDetail() {
	m.dropBitmaps()
	m.date = ""
	m.event = domain.KeyEvent{}
	m.hasEvent = false
	m.images = nil
	m.selectedImageID = ""
	m.linked = nil
	m.ledgerNames = map[string]string{}
	m.editing = false
	m.draft = ""
	m.color = ""
}

// loadDetail reads the selected day's images and linked transactions, and the
// ledger names the linked rows are labelled with.
func (m *Model) loadDetail() {
	if m.repo == nil || !m.hasEvent {
		return
	}
	ctx := context.Background()
	images, err := m.repo.KeyEventImages(ctx, m.date)
	if err != nil {
		m.errorMessage = "读取事件图片失败：" + err.Error()
		images = nil
	}
	m.images = images
	m.keepImageSelection()

	linked, err := m.repo.KeyEventRecords(ctx, m.date)
	if err != nil {
		m.errorMessage = "读取关联交易失败：" + err.Error()
		linked = nil
	}
	m.linked = linked

	names := map[string]string{}
	if ledgers, err := m.repo.Ledgers(ctx); err == nil {
		for _, ledger := range ledgers {
			names[ledger.ID] = ledger.Name
		}
	}
	m.ledgerNames = names
	m.editing = false
	m.draft = m.event.Content
}

// reloadList refreshes the year's rows after a write and keeps the selection
// when the day is still there.
func (m *Model) reloadList() {
	if m.repo == nil {
		return
	}
	events, err := m.repo.KeyEventsByYear(context.Background(), m.year)
	if err != nil {
		m.errorMessage = "读取事件失败：" + err.Error()
		return
	}
	m.events = events
	if m.date != "" && !m.findEvent() {
		m.clearDetail()
	}
}

// setYear switches the list to another calendar year and clears the selection.
func (m *Model) setYear(start string) {
	if len(start) < 4 {
		return
	}
	year := start[:4]
	if year == m.year {
		return
	}
	m.year = year
	m.clearDetail()
	m.Reload()
}

// openAdd opens 新增事件 with a sensible day: the selected one, today when it is
// in the shown year, otherwise the year's first day.
func (m *Model) openAdd() {
	m.add.open = true
	m.add.title = ""
	m.add.date = m.addDefaultDate()
}

func (m *Model) addDefaultDate() string {
	if m.date != "" {
		return m.date
	}
	today := time.Now().Format(dateLayout)
	if strings.HasPrefix(today, m.year+"-") {
		return today
	}
	return m.year + "-01-01"
}

// submitAdd creates the day's event (an empty body; the user writes it next) and
// selects it, switching the year when the picked day is in another one.
func (m *Model) submitAdd() {
	if m.repo == nil || m.add.date == "" {
		return
	}
	m.errorMessage = ""
	if _, err := m.repo.SaveKeyEvent(context.Background(), m.add.date, m.add.title, "", ""); err != nil {
		m.errorMessage = "新增事件失败：" + err.Error()
		return
	}
	m.add.open = false
	m.add.title = ""
	if len(m.add.date) >= 4 {
		m.year = m.add.date[:4]
	}
	m.date = m.add.date
	m.Reload()
}

// startEdit opens the body editor on the stored text.
func (m *Model) startEdit() {
	if !m.hasEvent {
		return
	}
	m.draft = m.event.Content
	m.editing = true
}

func (m *Model) cancelEdit() {
	m.editing = false
	m.draft = m.event.Content
}

// saveContent writes the body. The title follows the domain rule: the stored
// title, or the body's first non-empty line when it is blank.
func (m *Model) saveContent() {
	if m.repo == nil || !m.hasEvent {
		return
	}
	m.errorMessage = ""
	event, err := m.repo.SaveKeyEvent(context.Background(), m.date, m.event.Title, m.draft, m.event.Color)
	if err != nil {
		m.errorMessage = "保存失败：" + err.Error()
		return
	}
	m.event = event
	m.editing = false
	m.draft = event.Content
	m.reloadList()
}

// changeColor stores a tone at once, as the swatches did: colour is a switch,
// not part of the body draft. A failed write puts the previous tone back.
func (m *Model) changeColor(value string) {
	if m.repo == nil || !m.hasEvent || value == m.color {
		return
	}
	previous := m.color
	m.color = value
	m.errorMessage = ""
	if _, err := m.repo.SaveKeyEvent(context.Background(), m.date, m.event.Title, m.event.Content, value); err != nil {
		m.color = previous
		m.errorMessage = "保存颜色失败：" + err.Error()
		return
	}
	m.event.Color = value
	m.reloadList()
}

// addImages asks the shell for files and stores them on the selected event.
func (m *Model) addImages() {
	if !m.hasEvent {
		return
	}
	if m.cmds.AddImages == nil {
		m.errorMessage = "还不能添加图片"
		return
	}
	m.errorMessage = ""
	result, err := m.cmds.AddImages(m.date)
	if err != nil {
		m.errorMessage = "添加图片失败：" + err.Error()
		return
	}
	if len(result.Problems) > 0 {
		m.errorMessage = strings.Join(result.Problems, "；")
	}
	if result.Added > 0 {
		m.reloadImages()
		m.ensureBitmaps()
	}
}

// reloadImages refreshes the day's image list after an add or a delete.
func (m *Model) reloadImages() {
	if m.repo == nil || !m.hasEvent {
		return
	}
	images, err := m.repo.KeyEventImages(context.Background(), m.date)
	if err != nil {
		m.errorMessage = "读取事件图片失败：" + err.Error()
		return
	}
	m.images = images
	m.keepImageSelection()
	m.dropBitmaps()
}

// dropBitmaps releases every decoded picture of the day that was showing. The
// next selection decodes what it paints; keeping the previous day's thumbnails,
// full photos and backdrops would grow with every event the user opens.
func (m *Model) dropBitmaps() {
	m.bits = map[string]gfx.Bitmap{}
	m.bitErrs = map[string]string{}
	m.displayPath = ""
	m.displaySerial++
	m.blurID = ""
	m.blurBitmap = gfx.Bitmap{}
}

// cacheDisplay keeps the stage's view copy, releasing the one it replaces.
func (m *Model) cacheDisplay(path string, bitmap gfx.Bitmap) {
	if m.displayPath != "" && m.displayPath != path {
		delete(m.bits, m.displayPath)
	}
	m.bits[path] = bitmap
	m.displayPath = path
}

// keepImageSelection makes sure the stage names an image in the list.
func (m *Model) keepImageSelection() {
	for index := range m.images {
		if m.images[index].ID == m.selectedImageID {
			return
		}
	}
	if len(m.images) == 0 {
		m.selectedImageID = ""
		return
	}
	m.selectedImageID = m.images[0].ID
}

// saveImage asks the shell to save the stage's image elsewhere.
func (m *Model) saveImage() {
	image, ok := m.mainImage()
	if !ok {
		return
	}
	if m.cmds.SaveImage == nil {
		m.errorMessage = "还不能保存图片"
		return
	}
	m.errorMessage = ""
	if err := m.cmds.SaveImage(image.FilePath); err != nil {
		m.errorMessage = "保存图片失败：" + err.Error()
	}
}

// askDeleteEvent opens the confirmation for one day's event.
func (m *Model) askDeleteEvent(date, title string) {
	m.deleteEvent = confirmation{open: true, date: date, title: title}
}

// confirmDeleteEvent removes the event with its images and files.
func (m *Model) confirmDeleteEvent() {
	if m.cmds.DeleteEvent == nil {
		m.errorMessage = "还不能删除事件"
		return
	}
	m.errorMessage = ""
	if err := m.cmds.DeleteEvent(m.deleteEvent.date); err != nil {
		m.errorMessage = "删除事件失败：" + err.Error()
		return
	}
	m.deleteEvent.open = false
	m.clearDetail()
	m.Reload()
}

// askDeleteImage opens the confirmation for one picture.
func (m *Model) askDeleteImage(imageID string) {
	m.deleteImage = confirmation{open: true, imageID: imageID}
}

// imageAfterDelete is the picture to show once imageID is gone: the next one,
// or the previous one when the last picture was deleted. An empty result means
// the day has no pictures left.
func (m *Model) imageAfterDelete(imageID string) string {
	for index := range m.images {
		if m.images[index].ID != imageID {
			continue
		}
		if index+1 < len(m.images) {
			return m.images[index+1].ID
		}
		if index > 0 {
			return m.images[index-1].ID
		}
		return ""
	}
	return ""
}

func (m *Model) confirmDeleteImage() {
	if m.cmds.DeleteImage == nil {
		m.errorMessage = "还不能删除图片"
		return
	}
	next := m.imageAfterDelete(m.deleteImage.imageID)
	m.errorMessage = ""
	if err := m.cmds.DeleteImage(m.deleteImage.imageID); err != nil {
		m.errorMessage = "删除图片失败：" + err.Error()
		return
	}
	m.deleteImage.open = false
	m.reloadImages()
	if next != "" {
		m.selectedImageID = next
	}
	m.keepImageSelection()
	m.ensureBitmaps()
}

// toggleLinks hides or shows the linked-transactions rail and stores the choice.
func (m *Model) toggleLinks() {
	next := !m.linksOpen
	if m.cmds.SetLinksOpen != nil {
		if err := m.cmds.SetLinksOpen(next); err != nil {
			m.errorMessage = "记住展开状态失败：" + err.Error()
			return
		}
	}
	m.linksOpen = next
}

// displayTitle is what a card and a dialog call an event: its title, or the day
// when the body did not give one yet.
func displayTitle(event domain.KeyEvent) string {
	if title := strings.TrimSpace(event.Title); title != "" {
		return title
	}
	if event.Date != "" {
		return event.Date
	}
	return "事件"
}

// summary is the one line under a card's date: the body's whitespace collapsed,
// at most 30 code points.
func summary(content string) string {
	return domain.TruncateRunes(strings.Join(strings.Fields(content), " "), 30)
}

// shortDate is the month and day a card shows.
func shortDate(date string) string {
	if len(date) >= 10 {
		return date[5:]
	}
	return date
}

// dateLayout is the local YYYY-MM-DD form the page stores days in.
const dateLayout = "2006-01-02"

// clamp keeps a responsive width inside its bounds.
func clamp(value, low, high float32) float32 {
	return max(low, min(value, high))
}

// ledgerName is a linked record's ledger; an unknown id is shown as written.
func (m *Model) ledgerName(id string) string {
	if name := m.ledgerNames[id]; name != "" {
		return name
	}
	return id
}

// mainImage is the picture the stage shows: the selected one, or the first when
// nothing is selected.
func (m *Model) mainImage() (*domain.KeyEventImage, bool) {
	if len(m.images) == 0 {
		return nil, false
	}
	for index := range m.images {
		if m.images[index].ID == m.selectedImageID {
			return &m.images[index], true
		}
	}
	return &m.images[0], true
}

// imageIndex is an image's position in the list, or 0 when it is gone.
func (m *Model) imageIndex(id string) int {
	for index := range m.images {
		if m.images[index].ID == id {
			return index
		}
	}
	return 0
}

// selectImage chooses the stage's picture.
func (m *Model) selectImage(id string) {
	if id == m.selectedImageID {
		return
	}
	m.selectedImageID = id
	m.displaySerial++
	m.ensureBitmaps()
}

// amount is the signed money a linked row shows, in the page's single format.
func linkedAmount(record domain.TransactionRecord) string {
	switch record.TransactionType {
	case "expense":
		return "-¥" + domain.CentsToYuan(record.AmountCents)
	case "income":
		return "+¥" + domain.CentsToYuan(record.AmountCents)
	}
	return "¥" + domain.CentsToYuan(record.AmountCents)
}

// linkedDate is the transaction's local day as the rail shows it.
func linkedDate(record domain.TransactionRecord) string {
	if record.TransactionAt == 0 {
		return ""
	}
	return time.Unix(record.TransactionAt, 0).Format("2006/01/02")
}

// linkedTitle is the row's leading text: description, category, or a word.
func linkedTitle(record domain.TransactionRecord) string {
	if record.Description != "" {
		return record.Description
	}
	if record.Category != "" {
		return record.Category
	}
	return "交易"
}
