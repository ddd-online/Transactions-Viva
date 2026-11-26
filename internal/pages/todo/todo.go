// Package todo is the 待办 section: 待办 and 历史. Cards are themes holding
// today's items; history is completed items. Both belong to the workspace:
// one workspace has one todo board, independent of the ledger choice.
package todo

import (
	"context"
	"fmt"
	"sort"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

type tab uint8

const (
	tabBoard tab = iota
	tabHistory
)

var tabs = [...]tab{tabBoard, tabHistory}

var tabLabels = [...]string{"待办", "历史"}

type view uint8

const (
	viewBoard view = iota
	viewQuadrant
)

// Model is the 待办 page's state.
type Model struct {
	tab tab

	repo  repository.Repository
	todos repository.TodoRepository

	cards   []domain.TodoCard
	rows    []todoRow
	history []domain.TodoHistory
	loaded  bool
	err     string

	view view

	// expanded keeps the board's local disclosure state: an item starts closed
	// and shows its progress when open.
	expanded map[string]bool

	// progressDraft is the inline "add progress" text per item.
	progressDraft map[string]string

	itemForm   itemForm
	deleteCard deleteCard
	deleteItem deleteItem
	progress   progressPanel

	bounds  gfx.Rect
	toaster ui.Toaster

	// pageScroll is the keyboard's handle on the board or the history, whichever
	// is on screen: PageUp and PageDown move it.
	pageScroll layout.PageScroll

	// historyPage is the 1-based page of the completed list. The log only grows,
	// so it is paged rather than painted whole: the store keeps every entry, the
	// window shows a page of it.
	historyPage     int
	historyPageSize int
}

// New returns the empty section; the workspace is connected by SetRepository.
func New() *Model {
	return &Model{
		expanded:        map[string]bool{},
		progressDraft:   map[string]string{},
		historyPage:     1,
		historyPageSize: 20,
	}
}

// Tabs are the section's second-level navigation entries.
func (m *Model) Tabs() []string { return tabLabels[:] }

// Tab is the index of the chosen sub-function.
func (m *Model) Tab() int { return int(m.tab) }

// SelectTab chooses a sub-function by index.
func (m *Model) SelectTab(index int) {
	if index >= 0 && index < len(tabs) {
		m.tab = tabs[index]
	}
}

// SetRepository connects the section to a workspace. nil clears the page.
func (m *Model) SetRepository(repo repository.Repository, todos repository.TodoRepository) error {
	m.CloseOverlays()
	m.repo, m.todos = repo, todos
	m.cards, m.rows, m.history = nil, nil, nil
	m.loaded = false
	m.err = ""
	m.progressDraft = map[string]string{}
	m.expanded = map[string]bool{}
	return nil
}

// CloseOverlays dismisses every dialog when the workspace changes.
func (m *Model) CloseOverlays() {
	m.itemForm = itemForm{}
	m.deleteCard.open = false
	m.deleteItem.open = false
	m.progress.close()
}

func (m *Model) sync() {
	if m.repo == nil || m.todos == nil || m.loaded {
		return
	}
	m.loaded = true
	m.reload()
}

func (m *Model) reload() {
	if m.todos == nil {
		m.cards, m.rows, m.history = nil, nil, nil
		m.loaded = true
		return
	}
	ctx := context.Background()
	cards, err := m.todos.TodoCards(ctx)
	if err != nil {
		m.err = err.Error()
		m.cards, m.rows, m.history = nil, nil, nil
		m.loaded = true
		return
	}
	history, err := m.todos.TodoHistory(ctx)
	if err != nil {
		m.err = err.Error()
		m.cards, m.rows, m.history = nil, nil, nil
		m.loaded = true
		return
	}
	m.cards, m.history = cards, history
	m.rebuildRows()
	m.loaded = true
	m.err = ""
}

// rebuildRows flattens the doing items of every theme into the one table the
// board paints: newest first, the way a catch-up list reads. Rows point into
// m.cards, so they are rebuilt after every reload.
func (m *Model) rebuildRows() {
	m.rows = m.rows[:0]
	for cardIndex := range m.cards {
		card := &m.cards[cardIndex]
		for itemIndex := range card.Items {
			m.rows = append(m.rows, todoRow{
				CardID: card.ID, CardTitle: card.Title, Item: &card.Items[itemIndex],
			})
		}
	}
	sort.SliceStable(m.rows, func(i, j int) bool {
		return m.rows[i].Item.CreatedAt > m.rows[j].Item.CreatedAt
	})
}

// cardTitles is the themes the item form offers, in board order.
func (m *Model) cardTitles() []string {
	titles := make([]string, 0, len(m.cards))
	for _, card := range m.cards {
		titles = append(titles, card.Title)
	}
	return titles
}

// cardTitle is the title of the card with this id, or "" when it is gone.
func (m *Model) cardTitle(cardID string) string {
	for _, card := range m.cards {
		if card.ID == cardID {
			return card.Title
		}
	}
	return ""
}

// cardByTitle finds a theme by the name typed in the item form; trimming is the
// form's, so the match is exact.
func (m *Model) cardByTitle(title string) (domain.TodoCard, bool) {
	for _, card := range m.cards {
		if card.Title == title {
			return card, true
		}
	}
	return domain.TodoCard{}, false
}

func (m *Model) context() context.Context { return context.Background() }

func (m *Model) fail(err error) {
	if err == nil {
		return
	}
	m.err = err.Error()
	m.toaster.Show(err.Error(), "", ui.ToastError)
}

func (m *Model) succeed(text string) {
	m.err = ""
	m.toaster.Show(text, "", ui.ToastSuccess)
}

func (m *Model) activeCount() int {
	count := 0
	for _, card := range m.cards {
		count += len(card.Items)
	}
	return count
}

// Key gives the board or the history PageUp and PageDown; arrows stay with the
// row that has the focus. Escape belongs to an open date panel before it belongs
// to the form: closing the whole form would throw away what was typed.
func (m *Model) Key(event gfx.KeyEvent) gfx.EventResult {
	if event.Key == gfx.KeyEscape && m.itemForm.open &&
		(m.itemForm.startPick.Open || m.itemForm.duePick.Open) {
		m.itemForm.startPick.Open, m.itemForm.duePick.Open = false, false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return m.pageScroll.Key(event)
}

func (m *Model) View(c ui.C) {
	m.bounds = c.Bounds()
	m.sync()
	layout.Page(c, m.toolbar, func(c ui.C) {
		m.body(c)
	})
	m.itemFormView(c)
	m.deleteCardView(c)
	m.deleteItemView(c)
	m.progressView(c)
	ui.Toasts(c, &m.toaster)
}

func (m *Model) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if m.tab == tabBoard {
			current := "列表"
			if m.view == viewQuadrant {
				current = "四象限"
			}
			if ui.Choice(c, &current, []string{"列表", "四象限"}, nil).Changed() {
				if current == "四象限" {
					m.view = viewQuadrant
				} else {
					m.view = viewBoard
				}
			}
			ui.Spacer(c)
			if ui.Button(c, "新建事项").Icon(icons.Plus).Clicked() {
				m.openItemForm("", nil)
			}
		} else {
			ui.Spacer(c)
			typography.Caption(c, fmt.Sprintf("已完成 %d 项 · 按完成时间倒序", len(m.history)))
		}
	})
}

func (m *Model) body(c ui.C) {
	if m.repo == nil || m.todos == nil {
		emptystate.Page(c, &icons.Todo, "还没有打开工作空间",
			"打开工作空间后，这里的待办会保存在工作空间里。", nil)
		return
	}
	if m.err != "" {
		ui.Row(c).FillWidth().PadSides(layout.Inset, layout.Inset, 8, layout.Inset).Children(func() {
			typography.Caption(c, m.err).Color(c.Theme().Danger)
		})
	}
	switch {
	case m.tab == tabHistory:
		m.historyView(c)
	case m.view == viewQuadrant:
		m.quadrant(c)
	default:
		m.board(c)
	}
}
