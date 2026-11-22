package todo

import (
	"fmt"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

const (
	boardUrgencyWidth   = float32(64)
	boardImportantWidth = float32(64)
	boardDueWidth       = float32(104)
	boardProgressWidth  = float32(64)
	boardActionWidth    = float32(104)
)

// board is the doing items table: one header row, then a band per theme and its
// item rows. It has no cards and no horizontal scroll; the theme band carries
// the theme actions and every column fits the window.
func (m *Model) board(c ui.C) {
	if len(m.cards) == 0 {
		theme := c.Theme()
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Icon(c, icons.Todo).Size(30, 30).Color(theme.MutedForeground)
			typography.Title(c, "把下一件事放到板上")
			typography.Body(c, "先建一个主题，再往里面加事项。主题像一组，待办事项只占一行。").Muted().TextCenter()
			if ui.Button(c, "新建主题").Icon(icons.Plus).Clicked() {
				m.openCardForm()
			}
		})
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().PadSides(layout.Inset, layout.Inset, 20, layout.Inset).Children(func() {
			m.boardHeader(c)
			for index := range m.cards {
				ui.Divider(c)
				m.group(c, index, &m.cards[index])
			}
		})
	})
}

func (m *Model) boardHeader(c ui.C) {
	ui.Row(c).Height(34).FillWidth().PadSides(14, 10, 8, 10).Gap(10).Items(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Children(func() {
			typography.Caption(c, "事项")
		})
		m.boardColumnHeader(c, "紧急", boardUrgencyWidth)
		m.boardColumnHeader(c, "重要", boardImportantWidth)
		m.boardColumnHeader(c, "截止", boardDueWidth)
		m.boardColumnHeader(c, "进度", boardProgressWidth)
		m.boardColumnHeader(c, "操作", boardActionWidth)
	})
}

func (m *Model) boardColumnHeader(c ui.C, label string, width float32) {
	ui.Row(c).Width(width).Justify(ui.Center).Children(func() {
		typography.Caption(c, label)
	})
}

// group is one theme: a compact band with its actions, then its doing items.
func (m *Model) group(c ui.C, index int, card *domain.TodoCard) {
	theme := c.Theme()
	open := !m.collapsed[card.ID]
	c.Key("todo-card-" + card.ID)
	ui.Row(c).Height(40).FillWidth().PadSides(14, 8, 8, 10).Gap(8).Items(ui.Center).
		Background(theme.Muted).Children(func() {
		title := ui.Row(c).Grow(1).Gap(6).Items(ui.Center).
			Hit(ui.ID(c), "折叠主题 "+card.Title).Children(func() {
			typography.Body(c, card.Title).Bold().Ellipsis()
			m.disclosure(c, open)
		})
		if title.Clicked() {
			m.collapsed[card.ID] = open
		}
		more := ui.Button(c, "主题操作").Ghost().Icon(icons.GripVertical).IconOnly().
			Tooltip("调整主题顺序").
			Hover(ui.Look{}.Background(theme.Accent)).
			Pressed(ui.Look{}.Background(theme.Muted))
		if more.Clicked() {
			m.cardMenu = true
			m.cardMenuID = card.ID
		}
		if m.cardMenuID == card.ID {
			ui.Menu(c, more, &m.cardMenu, func(menu *ui.MenuItems) {
				if menu.Item("前移").Disabled(index == 0).Clicked() {
					m.moveCard(index, -1)
				}
				if menu.Item("后移").Disabled(index == len(m.cards)-1).Clicked() {
					m.moveCard(index, 1)
				}
				menu.Separator()
				if menu.Item("删除主题").Destructive().Clicked() {
					m.deleteCard = deleteCard{open: true, id: card.ID, title: card.Title}
				}
			})
		}
		if ui.Button(c, "删除主题").Ghost().Icon(icons.Trash2).IconOnly().
			Tooltip("删除主题").
			Hover(ui.Look{}.Background(theme.Accent)).
			Pressed(ui.Look{}.Background(theme.Muted)).Clicked() {
			m.deleteCard = deleteCard{open: true, id: card.ID, title: card.Title}
		}
		if ui.Button(c, "添加事项").Ghost().Icon(icons.Plus).IconOnly().
			Tooltip("添加事项").
			Hover(ui.Look{}.Background(theme.Accent)).
			Pressed(ui.Look{}.Background(theme.Muted)).Clicked() {
			m.collapsed[card.ID] = false
			m.openItemForm(card.ID, nil)
		}
	})
	if !open {
		return
	}
	if len(card.Items) == 0 {
		ui.Divider(c)
		ui.Row(c).Height(44).FillWidth().PadSides(14, 10, 8, 10).Gap(10).Items(ui.Center).Children(func() {
			typography.Caption(c, "还没有事项").Muted().Grow(1)
			if ui.Button(c, "添加第一件事项").Link().Clicked() {
				m.openItemForm(card.ID, nil)
			}
		})
		return
	}
	for index := range card.Items {
		ui.Divider(c)
		m.itemRow(c, &card.Items[index])
	}
}

func (m *Model) itemRow(c ui.C, item *domain.TodoItem) {
	theme := c.Theme()
	expanded := m.expanded[item.ID]
	c.Key("todo-item-" + item.ID)
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).MinHeight(48).FillWidth().PadSides(14, 10, 6, 14).Gap(10).
			Items(ui.Center).Children(func() {
			ui.Row(c).Grow(1).Gap(8).Items(ui.Center).Children(func() {
				check := ui.Row(c).Size(16, 16).Shrink(0).Radius(8).
					Border(1, theme.Border).Center().
					HitWith(ui.ID(c), "标记完成："+item.Title, gfx.HitOptions{
						Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
					}).Hover(ui.Look{}.Background(theme.Accent)).Focus(ui.Look{}.Ring(2, theme.Ring))
				if check.Clicked() {
					m.setStatus(item.ID, domain.TodoStatusDone)
				}
				label := "展开进度 " + item.Title
				if expanded {
					label = "收起进度 " + item.Title
				}
				// The disclosure glyph sits on the title's line; the row keeps
				// its 48 px rhythm with a single line whether open or closed.
				main := ui.Row(c).Grow(1).Gap(6).Items(ui.Center).
					Hit(ui.ID(c), label).Children(func() {
					typography.Body(c, item.Title).MaxLines(1).Ellipsis().Shrink(1).MinWidth(0)
					m.disclosure(c, expanded)
					ui.Spacer(c)
				})
				if main.Clicked() {
					m.expanded[item.ID] = !expanded
				}
			})
			m.levelCell(c, boardUrgencyWidth, item.Urgency)
			m.levelCell(c, boardImportantWidth, item.Importance)
			m.dueCell(c, item.DueDate)
			m.progressCell(c, item)
			ui.Row(c).Width(boardActionWidth).Justify(ui.Center).Gap(2).Items(ui.Center).Children(func() {
				if ui.Button(c, "编辑事项").Ghost().Icon(icons.Pencil).IconOnly().
					Tooltip("编辑事项").Clicked() {
					m.openItemForm(item.CardID, item)
				}
				if ui.Button(c, "删除事项").Ghost().Icon(icons.Trash2).IconOnly().
					Tooltip("删除事项").Clicked() {
					m.deleteItem = deleteItem{open: true, id: item.ID, title: item.Title}
				}
			})
		})
		if expanded {
			m.progressSection(c, item)
		}
	})
}

func (m *Model) levelCell(c ui.C, width float32, level int) {
	ui.Row(c).Width(width).Justify(ui.Center).Children(func() {
		typography.Caption(c, domain.TodoLevelLabel(level))
	})
}

func (m *Model) dueCell(c ui.C, dueDate string) {
	theme := c.Theme()
	ui.Row(c).Width(boardDueWidth).Justify(ui.Center).Children(func() {
		if dueDate == "" {
			typography.Caption(c, "—").Muted()
			return
		}
		today := time.Now().Format(domain.TodoDateLayout)
		label := dueDate
		color := theme.MutedForeground
		switch domain.TodoDueState(dueDate, today) {
		case "overdue":
			label, color = "逾期 · "+label, theme.Danger
		case "today":
			label, color = "今天", theme.Primary
		}
		typography.Caption(c, label).Color(color).MaxLines(1).Ellipsis()
	})
}

func (m *Model) progressCell(c ui.C, item *domain.TodoItem) {
	ui.Row(c).Width(boardProgressWidth).Justify(ui.Center).Children(func() {
		if len(item.Progress) == 0 {
			typography.Caption(c, "—").Muted()
			return
		}
		label := fmt.Sprintf("%d/%d", domain.TodoDoneCount(item.Progress), len(item.Progress))
		if ui.Button(c, label).Ghost().Small().Clicked() {
			m.openProgress(*item)
		}
	})
}

func (m *Model) disclosure(c ui.C, open bool) {
	glyph := icons.ChevronRight
	if open {
		glyph = icons.ChevronDown
	}
	ui.Icon(c, glyph).Size(12, 12).Color(c.Theme().MutedForeground)
}

func (m *Model) progressSection(c ui.C, item *domain.TodoItem) {
	ui.Column(c).FillWidth().PadTop(2).PadRight(14).PadBottom(10).PadLeft(38).Gap(6).Children(func() {
		ui.Divider(c)
		for index := range item.Progress {
			m.progressRow(c, &item.Progress[index])
		}
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			draft := m.progressDraft[item.ID]
			field := ui.TextField(c, &draft, "新增进度", "记一句进展…").Grow(1)
			if field.Changed() {
				m.progressDraft[item.ID] = draft
			}
			if field.Submitted() {
				m.progressDraft[item.ID] = draft
				m.addProgress(item.ID)
			}
			if ui.Button(c, "添加").Small().Outline().Clicked() {
				m.addProgress(item.ID)
			}
			if len(item.Progress) > 0 && ui.Button(c, "编辑记录").Small().Ghost().Clicked() {
				m.openProgress(*item)
			}
		})
	})
}

func (m *Model) progressRow(c ui.C, progress *domain.TodoProgress) {
	theme := c.Theme()
	c.Key("todo-progress-" + progress.ID)
	ui.Row(c).FillWidth().Gap(6).Items(ui.Center).Children(func() {
		check := ui.Row(c).Size(14, 14).Shrink(0).Radius(7).Border(1, theme.Border).Center().
			HitWith(ui.ID(c), "勾选进度", gfx.HitOptions{
				Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
			}).Hover(ui.Look{}.Background(theme.Accent))
		check.Children(func() {
			if progress.Done {
				ui.Icon(c, icons.CircleCheck).Size(12, 12).Color(theme.Success)
			}
		})
		if check.Clicked() {
			if err := m.todos.SetTodoProgressDone(m.context(), progress.ID, !progress.Done); err != nil {
				m.fail(err)
			} else {
				m.reload()
			}
		}
		text := typography.Caption(c, progress.Content).Grow(1)
		if progress.Done {
			text.Muted()
		}
		typography.Caption(c, time.Unix(progress.CreatedAt, 0).Format("01-02 15:04")).Mono()
		if ui.Button(c, "删除进度").Ghost().Icon(icons.Trash2).IconOnly().
			Tooltip("删除进度").Clicked() {
			if err := m.todos.DeleteTodoProgress(m.context(), progress.ID); err != nil {
				m.fail(err)
			} else {
				m.reload()
			}
		}
	})
}

func (m *Model) moveCard(index, delta int) {
	target := index + delta
	if index < 0 || index >= len(m.cards) || target < 0 || target >= len(m.cards) {
		return
	}
	cards := append([]domain.TodoCard(nil), m.cards...)
	cards[index], cards[target] = cards[target], cards[index]
	ids := make([]string, len(cards))
	for i := range cards {
		ids[i] = cards[i].ID
	}
	if err := m.todos.ReorderTodoCards(m.context(), ids); err != nil {
		m.fail(err)
		return
	}
	m.reload()
}

func (m *Model) addProgress(itemID string) {
	text := m.progressDraft[itemID]
	if text == "" {
		return
	}
	if _, err := m.todos.AddTodoProgress(m.context(), itemID, text); err != nil {
		m.fail(err)
		return
	}
	m.progressDraft[itemID] = ""
	m.reload()
}
