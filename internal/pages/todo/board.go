package todo

import (
	"fmt"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// The board's columns: the theme an item belongs to, the item itself (with its
// completion circle), its two levels, its due date and its progress count. The
// item's commands live on the row's context menu, so there is no action column.
const (
	boardThemeWidth     = float32(132)
	boardCheckWidth     = float32(16)
	boardUrgencyWidth   = float32(64)
	boardImportantWidth = float32(64)
	boardDueWidth       = float32(104)
	boardProgressWidth  = float32(64)
)

// todoRow is one doing item and the theme it belongs to. The board is one flat
// table of these, newest first, not a stack of theme cards.
type todoRow struct {
	CardID    string
	CardTitle string
	Item      *domain.TodoItem
}

// board is the doing items table: one header row, then every doing item of
// every theme, newest first. There are no bands: a theme is the first column,
// not a group the page folds.
func (m *Model) board(c ui.C) {
	if len(m.rows) == 0 {
		emptystate.Page(c, &icons.Todo, "把下一件事放到板上",
			"新建事项时选一个主题；输入一个还没有的主题名，就会把它建出来。", func() {
				if ui.Button(c, "新建事项").Icon(icons.Plus).Clicked() {
					m.openItemForm("", nil)
				}
			})
		return
	}
	ui.Column(c).Fill().Children(func() {
		m.boardHeader(c)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				// The rows this frame started with: completing an item reloads the
				// list, and a reload in the middle of this loop would leave the next
				// index past the end of the shorter one. The change lands after the
				// paint (see completeLater), and the next frame shows the new list.
				rows := m.rows
				for index := range rows {
					m.itemRow(c, &rows[index])
				}
			})
		})
	})
}

func (m *Model) boardHeader(c ui.C) {
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.Inset, Gap: layout.ColumnGap}, func(c ui.C) {
		m.headerCell(c, "主题", boardThemeWidth, false)
		// The item column starts above the completion circle, so its label is
		// pushed past the circle to sit over the titles it names.
		ui.Row(c).Grow(1).Gap(layout.ColumnGap).Children(func() {
			ui.Row(c).Width(boardCheckWidth)
			typography.Caption(c, "事项")
		})
		m.headerCell(c, "紧急", boardUrgencyWidth, true)
		m.headerCell(c, "重要", boardImportantWidth, true)
		m.headerCell(c, "截止", boardDueWidth, true)
		m.headerCell(c, "进度", boardProgressWidth, true)
	})
}

func (m *Model) headerCell(c ui.C, label string, width float32, center bool) {
	row := ui.Row(c).Width(width)
	if center {
		row = row.Justify(ui.Center)
	}
	row.Children(func() {
		typography.Caption(c, label)
	})
}

// itemRow is one line of the table. It is one control: a click toggles its
// progress, and a right click opens its commands — edit, complete, delete the
// item or delete the whole theme. The nested completion circle and progress
// button keep their own hits, so those presses do not toggle the row.
func (m *Model) itemRow(c ui.C, row *todoRow) {
	theme := c.Theme()
	item := row.Item
	expanded := m.expanded[item.ID]
	c.Key("todo-item-" + item.ID)
	ui.Column(c).FillWidth().Children(func() {
		label := "展开进度 " + item.Title
		if expanded {
			label = "收起进度 " + item.Title
		}
		line := layout.TableRow(c, ui.Row(c)).FillWidth().
			Hit(ui.ID(c), label)
		ui.ContextMenu(c, line, func(menu *ui.MenuItems) {
			if menu.Item("编辑事项").Clicked() {
				m.openItemForm(item.CardID, item)
			}
			if menu.Item("标记完成").Clicked() {
				m.completeLater(c, item.ID)
			}
			if menu.Item("删除事项").Destructive().Clicked() {
				m.deleteItem = deleteItem{open: true, id: item.ID, title: item.Title}
			}
			menu.Separator()
			if menu.Item("删除主题「" + row.CardTitle + "」").Destructive().Clicked() {
				m.deleteCard = deleteCard{open: true, id: row.CardID, title: row.CardTitle}
			}
		})
		line.Children(func() {
			ui.Row(c).Width(boardThemeWidth).Children(func() {
				typography.Body(c, row.CardTitle).MaxLines(1).Ellipsis()
			})
			check := ui.Row(c).Size(boardCheckWidth, boardCheckWidth).Shrink(0).Radius(8).
				Border(1, theme.Border).Center().
				HitWith(ui.ID(c), "标记完成："+item.Title, gfx.HitOptions{
					Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
				}).Hover(ui.Look{}.Background(state.Hover(*theme))).Focus(ui.Look{}.Ring(2, theme.Ring))
			if check.Clicked() {
				m.completeLater(c, item.ID)
			}
			ui.Row(c).Grow(1).Gap(6).Items(ui.Center).Children(func() {
				typography.Body(c, item.Title).MaxLines(1).Ellipsis().Shrink(1).MinWidth(0)
				m.disclosure(c, expanded)
				ui.Spacer(c)
			})
			m.levelCell(c, boardUrgencyWidth, item.Urgency)
			m.levelCell(c, boardImportantWidth, item.Importance)
			m.dueCell(c, item.DueDate)
			m.progressCell(c, item)
		})
		if line.Clicked() {
			m.expanded[item.ID] = !expanded
		}
		if expanded {
			m.progressSection(c, item)
		}
	})
}

// completeLater marks an item done after the paint that asked for it. The board
// paints its rows in one pass, and completing an item rebuilds the list under
// that pass; doing it here keeps the loop over the list it started with.
func (m *Model) completeLater(c ui.C, itemID string) {
	c.After(func() { m.setStatus(itemID, domain.TodoStatusDone) })
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
			}).Hover(ui.Look{}.Background(state.Hover(*theme)))
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
		// The line wraps instead of clipping: a progress record is a sentence, not a
		// cell. The time and the delete key keep their place at its right.
		text := typography.CaptionParagraph(c, progress.Content, 0).Grow(1).MinWidth(0)
		if progress.Done {
			text.Muted()
		}
		typography.Caption(c, time.Unix(progress.CreatedAt, 0).Format("01-02 15:04")).Mono()
		if ui.Button(c, "删除进度").Destructive().Icon(icons.Trash2).IconOnly().
			Tooltip("删除进度").Clicked() {
			if err := m.todos.DeleteTodoProgress(m.context(), progress.ID); err != nil {
				m.fail(err)
			} else {
				m.reload()
			}
		}
	})
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
