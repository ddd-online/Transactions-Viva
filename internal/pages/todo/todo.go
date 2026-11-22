// Package todo is the 待办 section: 待办 and 历史. Cards are themes holding
// today's items; history is completed items. Both belong to the active ledger,
// like the old application's todo tables.
package todo

import (
	"context"
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/pages/accounting/ledger"
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

	repo    repository.Repository
	todos   repository.TodoRepository
	ledgers *ledger.Store

	cards        []domain.TodoCard
	history      []domain.TodoHistory
	loaded       bool
	loadedLedger string
	err          string

	view view

	// collapsed / expanded keep the board's local disclosure state. Card
	// themes start open; an item starts closed and shows its progress when open.
	collapsed map[string]bool
	expanded  map[string]bool

	// progressDraft is the inline "add progress" text per item.
	progressDraft map[string]string

	cardForm   cardForm
	itemForm   itemForm
	deleteCard deleteCard
	deleteItem deleteItem
	progress   progressPanel
	cardMenu   bool
	cardMenuID string

	bounds  gfx.Rect
	toaster ui.Toaster
}

// New returns the empty section over a ledger list of its own.
func New() *Model { return NewWithLedgers(ledger.NewStore()) }

// NewWithLedgers returns the section over the shell's shared ledger list, so
// the 待办 page follows the same 账本 choice as 记账 and 日记.
func NewWithLedgers(ledgers *ledger.Store) *Model {
	if ledgers == nil {
		ledgers = ledger.NewStore()
	}
	return &Model{
		ledgers:       ledgers,
		collapsed:     map[string]bool{},
		expanded:      map[string]bool{},
		progressDraft: map[string]string{},
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
	m.cards, m.history = nil, nil
	m.loaded = false
	m.loadedLedger = ""
	m.err = ""
	m.progressDraft = map[string]string{}
	m.expanded = map[string]bool{}
	m.collapsed = map[string]bool{}
	if repo == nil || todos == nil {
		m.ledgers.Reset(nil)
		return nil
	}
	ledgers, err := repo.Ledgers(context.Background())
	if err != nil {
		return err
	}
	m.ledgers.Reset(ledgers)
	return nil
}

// CloseOverlays dismisses every dialog when the workspace changes.
func (m *Model) CloseOverlays() {
	m.cardForm.open = false
	m.itemForm.open = false
	m.deleteCard.open = false
	m.deleteItem.open = false
	m.progress.close()
	m.cardMenu = false
	m.cardMenuID = ""
}

func (m *Model) sync() {
	if m.repo == nil || m.todos == nil {
		return
	}
	active := m.ledgers.Active()
	if active == m.loadedLedger && m.loaded {
		return
	}
	m.loadedLedger = active
	m.reload()
}

func (m *Model) reload() {
	if m.todos == nil || m.ledgers.Active() == "" {
		m.cards, m.history = nil, nil
		m.loaded = true
		return
	}
	ctx := context.Background()
	cards, err := m.todos.TodoCards(ctx, m.ledgers.Active())
	if err != nil {
		m.err = err.Error()
		m.cards, m.history = nil, nil
		m.loaded = true
		return
	}
	history, err := m.todos.TodoHistory(ctx, m.ledgers.Active())
	if err != nil {
		m.err = err.Error()
		m.cards, m.history = nil, nil
		m.loaded = true
		return
	}
	m.cards, m.history = cards, history
	m.loaded = true
	m.err = ""
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

func (m *Model) View(c ui.C) {
	m.bounds = c.Bounds()
	m.sync()
	ui.Column(c).Fill().Children(func() {
		m.toolbar(c)
		ui.Divider(c)
		m.body(c)
	})
	m.cardFormView(c)
	m.itemFormView(c)
	m.deleteCardView(c)
	m.deleteItemView(c)
	m.progressView(c)
	ui.Toasts(c, &m.toaster)
}

func (m *Model) toolbar(c ui.C) {
	ui.Row(c).Height(layout.ToolbarHeight).FillWidth().
		PadX(layout.Inset).Gap(layout.Gap).Items(ui.Center).Children(func() {
		if ledger.Picker(c, m.ledgers) {
			m.loaded = false
			m.sync()
		}
		ui.Divider(c).Height(24)
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
			if m.view == viewBoard && len(m.cards) > 0 {
				allCollapsed := true
				for _, card := range m.cards {
					if !m.collapsed[card.ID] {
						allCollapsed = false
						break
					}
				}
				label := "全部收起"
				if allCollapsed {
					label = "全部展开"
				}
				if ui.Button(c, label).Ghost().Small().Clicked() {
					for _, card := range m.cards {
						m.collapsed[card.ID] = !allCollapsed
					}
				}
			}
			ui.Spacer(c)
			if ui.Button(c, "新建主题").Icon(icons.Plus).Clicked() {
				m.openCardForm()
			}
		} else {
			ui.Spacer(c)
			typography.Caption(c, fmt.Sprintf("已完成 %d 项 · 按完成时间倒序", len(m.history)))
		}
	})
}

func (m *Model) body(c ui.C) {
	if m.repo == nil || m.todos == nil {
		m.emptyPage(c, "还没有打开工作空间", "打开工作空间后，这里的待办会按账本显示。")
		return
	}
	if len(m.ledgers.List()) == 0 {
		m.emptyPage(c, "还没有账本", "先在记账页创建或选择一个账本，待办会跟着账本走。")
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

func (m *Model) emptyPage(c ui.C, title, note string) {
	ui.Column(c).Fill().Center().Gap(8).Children(func() {
		typography.Title(c, title)
		typography.Body(c, note).Muted().TextCenter()
	})
}
