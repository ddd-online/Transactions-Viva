// Package diary is the 日记 section: one entry per book per local date, a
// year → month → day tree of the dates that have one, and an editor for the day
// the tree has selected. The entry shape, mood values and strict YYYY-MM-DD
// dates come from internal/domain.
//
// The page owns no SQL and no files: a workspace reaches it as
// repository.Repository through SetDataSource, and importing or exporting diary
// files belongs to the shell (internal/diary), so this page has no command for
// them.
package diary

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

// saveDelay is how long the editor waits after the last change before it writes.
// The page used the same 600 ms: long enough that typing a sentence is one
// write, short enough that 已保存 is honest by the time the user looks at it.
const saveDelay = 600 * time.Millisecond

// savePhase is what the toolbar says about the draft.
type savePhase uint8

const (
	// saveSaved is the resting state: the editor holds what the workspace holds,
	// or the day has no entry and nothing was typed.
	saveSaved savePhase = iota
	// savePending is a draft that has not reached the workspace yet.
	savePending
	// saveFailed is a write the workspace refused; the draft is still here.
	saveFailed
)

// Model is the 日记 page's state.
type Model struct {
	repo repository.Repository
	// books are the workspace's diary books and bookID the one the editor shows.
	// They belong to the workspace, not to a ledger: a diary is written into a
	// book, and the ledger a workspace is reading has nothing to do with it.
	books  []domain.DiaryBook
	bookID string

	// dates is the active ledger's entry dates, newest first: the tree's source.
	dates []domain.DiaryDateItem

	// selected is the local date the editor shows, and content / mood the draft
	// it holds. wordCount follows content, in code points.
	selected  string
	content   string
	mood      string
	wordCount int

	// phase says whether the draft reached the workspace; editedAt is when it
	// last changed, on the canvas clock.
	phase    savePhase
	dirty    bool
	editedAt time.Duration
	// noticedMood is the mood the draft last reported, so a pick is found by
	// comparing rather than by a Changed flag the picker does not raise.
	noticedMood string
	// body is the text area's editor. The page reads its text at the top of the
	// paint, before the header declares the word count: a string binding is
	// synced while the field is declared, which is one step too late for the
	// count to include what was just typed.
	body ui.Field
	// hasStored is false while the selected day has no entry, so an untouched
	// empty day is never written.
	hasStored bool

	// expanded holds the year and month keys whose children show, and rows are
	// the visible tree rows rebuilt from both.
	expanded map[string]bool
	rows     []row

	tree ui.TreeState
	pick datepicker.State

	// deleting names the day the delete confirmation is open for.
	deleting   string
	deleteOpen bool

	errorMessage string
	toaster      ui.Toaster
	bounds       gfx.Rect
}

// New returns the page with today selected and the tree empty; desktop calls
// SetDataSource once a workspace is open.
func New() *Model {
	m := &Model{
		selected: domain.Today(),
		expanded: map[string]bool{},
		pick:     datepicker.NewState(),
	}
	m.tree.Key = func(index int) int { return index }
	m.tree.Node = m.node
	m.tree.Label = m.rowLabel
	m.tree.RowHeight = layout.TreeRowHeight
	return m
}

// SetDataSource connects the page to a workspace. The ledger comes from the
// shared ledger.Store, so the diary follows the 账本 the 记账 pages show.
func (m *Model) SetDataSource(repo repository.Repository) {
	m.repo = repo
	m.bookID = ""
	if repo == nil {
		m.books, m.dates, m.rows = nil, nil, nil
		m.content, m.mood, m.wordCount = "", "", 0
		m.phase, m.dirty, m.hasStored = saveSaved, false, false
		m.errorMessage = ""
		return
	}
	m.Reload()
}

// Reload reads the active ledger's dates and the selected day's entry. The shell
// calls it when the page is shown and after an import brought new entries.
func (m *Model) Reload() {
	m.errorMessage = ""
	if m.repo == nil {
		m.books, m.dates, m.rows = nil, nil, nil
		return
	}
	m.loadBooks()
	if m.bookID == "" {
		m.dates, m.rows = nil, nil
		return
	}
	dates, err := m.repo.DiaryDates(context.Background(), m.bookID)
	if err != nil {
		m.errorMessage = "读取日记列表失败：" + err.Error()
		dates = nil
	}
	m.dates = dates
	m.loadEntry()
	m.openAround(m.selected)
	m.rebuildRows()
}

// loadBooks reads the workspace's diary books and keeps the current one when it
// still exists. An empty workspace gets one 默认日记本, the way it gets a
// 默认账本: the diary page always has a book to write into.
func (m *Model) loadBooks() {
	books, err := m.repo.DiaryBooks(context.Background())
	if err != nil {
		m.errorMessage = "读取日记本失败：" + err.Error()
		return
	}
	if len(books) == 0 {
		if _, err := m.repo.CreateDiaryBook(context.Background(), "默认日记本", ""); err != nil {
			m.errorMessage = "创建默认日记本失败：" + err.Error()
			return
		}
		if books, err = m.repo.DiaryBooks(context.Background()); err != nil {
			m.errorMessage = "读取日记本失败：" + err.Error()
			return
		}
	}
	m.books = books
	for _, book := range books {
		if book.ID == m.bookID {
			return
		}
	}
	m.bookID = books[0].ID
}

// Books are the workspace's diary books, in the order the settings page and the
// diary toolbar show them.
func (m *Model) Books() []domain.DiaryBook { return m.books }

// ActiveBook is the id of the book the diary page is showing.
func (m *Model) ActiveBook() string { return m.bookID }

// BookName is a book's name; an unknown id is shown as written.
func (m *Model) BookName(id string) string {
	for _, book := range m.books {
		if book.ID == id {
			return book.Name
		}
	}
	return id
}

// SetBook chooses the book the diary page shows and reloads onto it.
func (m *Model) SetBook(id string) {
	if id == m.bookID {
		return
	}
	m.flush()
	m.bookID = id
	m.dates, m.rows = nil, nil
	m.expanded = map[string]bool{}
	m.loadEntry()
	if m.repo != nil && m.bookID != "" {
		if dates, err := m.repo.DiaryDates(context.Background(), m.bookID); err == nil {
			m.dates = dates
		} else {
			m.errorMessage = "读取日记列表失败：" + err.Error()
		}
	}
	m.openAround(m.selected)
	m.rebuildRows()
}

// ChangeBook reloads after the settings page added, renamed or removed a book.
func (m *Model) ChangeBook() { m.Reload() }

// CreateBook adds a book and makes it the one the diary page shows, as 新增账本
// does for ledgers.
func (m *Model) CreateBook(name, description string) error {
	if m.repo == nil {
		return errNoWorkspace
	}
	created, err := m.repo.CreateDiaryBook(context.Background(), name, description)
	if err != nil {
		return err
	}
	m.bookID = created.ID
	m.Reload()
	return nil
}

// UpdateBook rewrites a book's name and description.
func (m *Model) UpdateBook(id, name, description string) error {
	if m.repo == nil {
		return errNoWorkspace
	}
	if err := m.repo.UpdateDiaryBook(context.Background(), id, name, description); err != nil {
		return err
	}
	m.Reload()
	return nil
}

// DeleteBook removes a book with its entries and falls back to the first book.
func (m *Model) DeleteBook(id string) error {
	if m.repo == nil {
		return errNoWorkspace
	}
	if err := m.repo.DeleteDiaryBook(context.Background(), id); err != nil {
		return err
	}
	if m.bookID == id {
		m.bookID = ""
		m.dates, m.rows = nil, nil
		m.content, m.mood, m.wordCount = "", "", 0
		m.body.SetText("")
		m.hasStored, m.dirty, m.phase = false, false, saveSaved
	}
	m.Reload()
	return nil
}

// CloseOverlays writes a pending draft before the shell hides the page and
// closes the date panel and the confirmation.
func (m *Model) CloseOverlays() {
	m.flush()
	m.pick.Open = false
	m.deleteOpen = false
}

// View paints the toolbar, the date tree and the editor.
func (m *Model) View(c ui.C) {
	m.bounds = c.Bounds()
	if m.repo != nil && len(m.books) == 0 {
		// No book loaded yet (the page was opened before a workspace, or the
		// settings page removed the one that was showing).
		m.Reload()
	}
	m.noticeEdits(c.Canvas().Now())
	m.settleDraft(c)
	layout.Page(c, m.toolbar, func(c ui.C) {
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			m.rail(c)
			ui.Divider(c)
			m.editor(c)
		})
	})
	m.deleteDialog(c)
	ui.Toasts(c, &m.toaster)
}

// toolbar is the band above the two columns: the diary book, 今天, the date field,
// and the draft's state at the right. Deleting a day belongs to that day's row in
// the tree, not to the band: the row is where the entry lives.
func (m *Model) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		m.bookPicker(c)
		ui.Divider(c).Height(24)
		if ui.Button(c, "今天").Secondary().Clicked() {
			m.selectDate(domain.Today())
		}
		datepicker.Picker(c, &m.pick, datepicker.Spec{
			Label: "日记日期", Start: m.selected, Grain: datepicker.ByDay,
			Placeholder: "选择日期", CloseOnPick: true, Viewport: m.bounds,
			OnRange: func(start, _ string) { m.selectDate(start) },
		})
		ui.Spacer(c)
		m.saveState(c)
	})
}

// saveState is the quiet line at the toolbar's trailing edge: a normal state stays
// in the muted tone, only a failure takes color.
func (m *Model) saveState(c ui.C) {
	theme := c.Theme()
	label, color := "已保存", theme.MutedForeground
	switch m.phase {
	case savePending:
		label = "未保存"
	case saveFailed:
		label, color = "保存失败", theme.Danger
	}
	typography.Caption(c, label).Color(color)
}

// expandAllButton is the one command of the tree header: it opens every year and
// month, or closes them back to the years. It is the notes tree's key with the same
// two labels and the same two marks, so the two rails are read the same way.
func (m *Model) expandAllButton(c ui.C) {
	theme := c.Theme()
	label, icon := "展开全部", icons.ListTree
	if m.allExpanded() {
		label, icon = "收起全部", icons.ListX
	}
	button := ui.ButtonBase(c, label).Size(28, 28).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Pressed(ui.Look{}.Background(state.Pressed(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip(label)
	button.Children(func() {
		ui.Icon(c, icon).Size(15, 15)
	})
	if button.Clicked() {
		m.toggleExpandAll()
	}
}

// rail is the date tree column: its own header with the count, the command that
// opens or closes every branch, then the tree.
func (m *Model) rail(c ui.C) {
	layout.Rail(c, layout.RailWidth, func(c ui.C) {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, "日记")
			layout.SectionCount(c, strconv.Itoa(len(m.dates)))
			ui.Spacer(c)
			// A diary with a year of entries is a long way down when every month has
			// to be opened by hand, so the tree carries the notes tree's one command.
			// A tree with nothing in it has nothing to open, so it keeps no key.
			if len(m.dates) > 0 {
				m.expandAllButton(c)
			}
		})
		ui.Divider(c)
		switch {
		case m.bookID == "":
			emptystate.Notice(c, "请先在设置里创建日记本")
		case len(m.dates) == 0:
			emptystate.Notice(c, "暂无日记")
		default:
			// The tree keeps the page's inset on both sides: its rows are a
			// column inside the panel, not bands welded to the rail's edges.
			ui.Column(c).Grow(1).FillWidth().PadX(layout.Inset).Children(func() {
				ui.Tree(c, &m.tree, len(m.rows), m.tree.Label, func(i int) {
					m.rowContent(c, i)
				}).Label("日记列表").Grow(1)
			})
			m.treeDone()
		}
	})
}

// treeDone applies what the tree reported this paint: a branch opened or closed,
// or a day chosen. The rows it paints next follow from the state set here, which
// the click's own repaint shows.
func (m *Model) treeDone() {
	if index, ok := m.tree.Toggled(); ok {
		m.toggleRow(index)
	}
	if index, ok := m.tree.Chosen(); ok {
		m.chooseRow(index)
	}
}

// editor is the right column: the day's heading with its mood and word count,
// then the writing area.
func (m *Model) editor(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, domain.DiaryDateCn(m.selected))
			typography.Caption(c, domain.DiaryWeekdayCn(m.selected))
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 字", m.wordCount)).FontSize(typography.SizeCaption).
				Color(theme.MutedForeground).Mono()
			m.moodPicker(c)
		})
		ui.Divider(c)
		ui.Column(c).Grow(1).FillWidth().Pad(layout.Inset).
			Gap(layout.BlockGap).Children(func() {
			if m.errorMessage != "" {
				typography.Caption(c, m.errorMessage).Color(theme.Danger)
			}
			// The placeholder names the day the editor is showing: a past entry
			// is not "today's" diary.
			placeholder := "写下今天的日记……"
			if m.selected != domain.Today() {
				placeholder = "写下这天的日记……"
			}
			ui.TextArea(c, &m.body, "日记正文", placeholder).
				FillWidth().Grow(1)
		})
	})
}

// moodRow is one choice of the picker: what it stores, the emoji it paints and
// the Chinese name that names it for assistive technology and explains the face
// in its tooltip.
type moodRow struct {
	value string
	emoji string
	name  string
}

// moodRows are the picker's seven choices in the picker's order: 清除 first,
// then the six moods, angriest first. The names are the accessible labels and the
// tooltips — ui.Choice would name each segment by its emoji alone, and both a
// screen reader and a pointer would get a face with no word for it.
var moodRows = [...]moodRow{
	{value: "", emoji: "—", name: "清除心情"},
	{value: "😠", emoji: "😠", name: "生气"},
	{value: "😤", emoji: "😤", name: "烦躁"},
	{value: "😰", emoji: "😰", name: "焦虑"},
	{value: "😢", emoji: "😢", name: "难过"},
	{value: "😐", emoji: "😐", name: "平静"},
	{value: "😊", emoji: "😊", name: "开心"},
}

// moodPicker is the seven-choice mood row: the app's segmented look, built from
// ButtonBase because the segmented control cannot name an option differently
// from what it paints. One tab stop: the arrows move between the seven.
//
// The chrome follows the app's segmented control exactly (Viva's
// newSegmentPalette): the track is Muted inside a hairline, a resting segment
// shows the track, the chosen one is a raised pill — Card on light, the track
// lifted toward the foreground on dark, where Card would sink into it — a
// hovered one takes Hover, and a hairline separates neighbours unless one of
// them is the chosen pill.
func (m *Model) moodPicker(c ui.C) {
	theme := c.Theme()
	metrics := theme.Metrics
	trackFilled := theme.Muted
	pill := theme.Card
	if luminance(pill) < luminance(trackFilled)+0.02 {
		pill = trackFilled.Mix(theme.Foreground, 0.14)
	}
	selectedValue := domain.NormalizeDiaryMood(m.mood)
	ui.Row(c).FocusGroup(ui.FocusHorizontal).Items(ui.Center).
		HitWith(ui.ID(c), "心情", gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true}).
		Pad(2).Radius(metrics.RadiusControl).
		Background(trackFilled).Border(1, theme.Border).Children(func() {
		for index, option := range moodRows {
			if index > 0 {
				// A hairline between neighbours, unless the chosen pill sits on
				// one side of it: the pill keeps its capsule.
				neighbour := moodRows[index-1].value
				if neighbour != selectedValue && option.value != selectedValue {
					ui.Divider(c).Width(1).Height(11).Background(theme.Border)
				}
			}
			chosen := option.value == selectedValue
			c.KeyInt(index)
			button := ui.ButtonBase(c, option.name).Size(30, 26).
				Radius(metrics.RadiusItem()).
				Hover(ui.Look{}.Background(theme.Hover)).
				Pressed(ui.Look{}.Background(state.Pressed(*theme))).
				Focus(ui.Look{}.Ring(2, theme.Ring)).
				Tooltip(option.name)
			if chosen {
				button = button.Background(pill)
			}
			button.Children(func() {
				ui.Text(c, option.emoji).FontSize(typography.SizeBody)
			})
			if button.Clicked() {
				m.mood, m.errorMessage = option.value, ""
				m.edited(c.Canvas().Now())
				c.Canvas().RequestFrameIn(saveDelay)
			}
		}
	})
}

// luminance is a colour's relative brightness, the figure the segmented look
// compares to decide whether its pill would sink into its track.
func luminance(color gfx.Color) float32 {
	return 0.2126*color.Red + 0.7152*color.Green + 0.0722*color.Blue
}

// errNoWorkspace is what the book commands report while no workspace is open;
// the settings section disables them then, so this only guards a stale click.
var errNoWorkspace = errors.New("还没有打开工作空间")

// bookPicker is the diary book field at the toolbar's leading edge: the same
// Select the accounting pages use for 账本, over the workspace's books.
func (m *Model) bookPicker(c ui.C) {
	if len(m.books) == 0 {
		return
	}
	ids := make([]string, len(m.books))
	for index, book := range m.books {
		ids[index] = book.ID
	}
	active := m.bookID
	if !selectbox.Select(c, &active, ids, m.BookName).Label("日记本").Changed() {
		return
	}
	if active != m.bookID {
		m.SetBook(active)
	}
}

// selectDate switches the editor to another day, writing the draft first.
func (m *Model) selectDate(date string) {
	if !domain.IsDiaryDate(date) || date == m.selected {
		return
	}
	m.flush()
	m.selected = date
	m.errorMessage = ""
	m.loadEntry()
	m.openAround(date)
	m.rebuildRows()
}

// askDeleteDate opens the confirmation for one day, the row it was asked from.
func (m *Model) askDeleteDate(date string) {
	if m.bookID == "" || date == "" {
		return
	}
	m.deleting, m.deleteOpen = date, true
}

// deleteDialog confirms removing one day's entry, the question the page
// asked in a popover beside the button.
func (m *Model) deleteDialog(c ui.C) {
	if !m.deleteOpen {
		return
	}
	ui.ModalWith(c, &m.deleteOpen, "删除日记「"+domain.DiaryDateCn(m.deleting)+"」？", ui.ModalSpec{
		Width: 400, Description: "这一天的正文会一起删除，无法恢复。",
	}, nil, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteOpen = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			m.deleteDate(m.deleting)
			m.deleteOpen = false
		}
	})
}

// deleteDate removes one day from the workspace and from the page.
func (m *Model) deleteDate(date string) {
	if m.repo == nil || m.bookID == "" {
		return
	}
	if err := m.repo.DeleteDiary(context.Background(), m.bookID, date); err != nil {
		m.errorMessage = "删除日记失败：" + err.Error()
		m.toaster.Show(m.errorMessage, "", ui.ToastError)
		return
	}
	m.dates = withoutDate(m.dates, date)
	if date == m.selected {
		// The field holds a copy of the text too: clearing only the model would
		// put the deleted day back on the next paint.
		m.content, m.mood, m.wordCount = "", "", 0
		m.body.SetText("")
		m.hasStored, m.dirty, m.phase = false, false, saveSaved
		m.noticeNow()
	}
	m.rebuildRows()
	m.toaster.Show("日记已删除", "", ui.ToastSuccess)
}

// withoutDate drops one date from the newest-first list.
func withoutDate(items []domain.DiaryDateItem, date string) []domain.DiaryDateItem {
	kept := items[:0]
	for _, item := range items {
		if item.Date != date {
			kept = append(kept, item)
		}
	}
	return kept
}

// loadEntry reads the selected day from the workspace. A day without an entry
// leaves the editor empty and unmarked, so nothing is written until the user
// types.
func (m *Model) loadEntry() {
	m.hasStored = false
	m.content, m.mood, m.wordCount = "", "", 0
	m.dirty, m.phase = false, saveSaved
	m.body.SetText("")
	m.noticeNow()
	if m.repo == nil || m.bookID == "" {
		return
	}
	entry, err := m.repo.DiaryEntry(context.Background(), m.bookID, m.selected)
	if err != nil {
		// ErrNotFound is the normal empty day; anything else is worth saying.
		if !errors.Is(err, repository.ErrNotFound) {
			m.errorMessage = "读取日记失败：" + err.Error()
		}
		return
	}
	m.hasStored = true
	m.content = entry.Content
	m.mood = domain.NormalizeDiaryMood(entry.Mood)
	m.wordCount = entry.WordCount
	m.body.SetText(entry.Content)
	m.noticeNow()
}

// edited records a change to the draft: the word count follows the text and the
// write waits saveDelay from now.
func (m *Model) edited(now time.Duration) {
	m.wordCount = domain.DiaryWordCount(m.content)
	m.editedAt = now
	m.dirty = true
	if m.phase != saveFailed {
		m.phase = savePending
	}
	m.errorMessage = ""
}

// noticeEdits starts the pause when the draft differs from what the last paint
// saw, and asks for the frame that writes it. Comparing is what makes the pause
// start on the keystroke's own paint: the text area's Changed reports the frame
// after, which would delay the write by one paint every time.
func (m *Model) noticeEdits(now time.Duration) {
	typed := m.body.Text()
	if typed != m.content {
		m.content = typed
		m.edited(now)
		return
	}
	if m.mood != m.noticedMood {
		m.noticedMood = m.mood
		m.edited(now)
	}
}

// noticeNow records the draft as the field holds it, without marking it changed.
func (m *Model) noticeNow() {
	m.content = m.body.Text()
	m.noticedMood = m.mood
}

// settleDraft writes the draft once the pause after the last change has passed
// and asks for the frame that does it meanwhile. It runs at the top of the page
// function, before anything is declared, so the paint that writes a draft also
// shows what the write left behind; a goroutine per keystroke would buy nothing
// for one row.
func (m *Model) settleDraft(c ui.C) {
	if !m.dirty || m.repo == nil {
		return
	}
	if remaining := saveDelay - (c.Canvas().Now() - m.editedAt); remaining > 0 {
		c.Canvas().RequestFrameIn(remaining)
		return
	}
	m.flush()
}

// flush writes the draft now. A day without an entry whose draft is empty is not
// written: opening a date must not create an empty diary for it.
func (m *Model) flush() {
	if !m.dirty || m.repo == nil || m.bookID == "" {
		return
	}
	if !m.hasStored && strings.TrimSpace(m.content) == "" && m.mood == "" {
		m.dirty, m.phase = false, saveSaved
		return
	}
	entry, err := m.repo.SaveDiary(context.Background(), m.bookID, m.selected, m.content, m.mood)
	if err != nil {
		m.phase = saveFailed
		m.errorMessage = "保存日记失败：" + err.Error()
		return
	}
	m.hasStored = true
	m.wordCount = entry.WordCount
	m.dirty, m.phase = false, saveSaved
	m.noticeNow()
	m.dates = mergeDate(m.dates, domain.DiaryDateItem{
		Date: entry.Date, WordCount: entry.WordCount, Mood: entry.Mood,
	})
	m.rebuildRows()
}

// mergeDate puts one date's row into the newest-first list, replacing the row the
// day already had.
func mergeDate(items []domain.DiaryDateItem, item domain.DiaryDateItem) []domain.DiaryDateItem {
	kept := items[:0]
	for _, existing := range items {
		if existing.Date != item.Date {
			kept = append(kept, existing)
		}
	}
	for index := 0; index < len(kept); index++ {
		if item.Date > kept[index].Date {
			kept = append(kept, domain.DiaryDateItem{})
			copy(kept[index+1:], kept[index:])
			kept[index] = item
			return kept
		}
	}
	return append(kept, item)
}
