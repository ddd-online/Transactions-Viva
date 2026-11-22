package todo

import (
	"fmt"
	"time"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// quadrant is the four-box view of doing items by urgency and importance.
func (m *Model) quadrant(c ui.C) {
	groups := [4][]quadrantItem{}
	for _, card := range m.cards {
		for _, item := range card.Items {
			index := 0
			if item.Urgency >= 1 && item.Importance >= 1 {
				index = 0
			} else if item.Urgency < 1 && item.Importance >= 1 {
				index = 1
			} else if item.Urgency >= 1 {
				index = 2
			} else {
				index = 3
			}
			groups[index] = append(groups[index], quadrantItem{Card: card.Title, Item: item})
		}
	}
	titles := [4]string{"重要且紧急", "重要不紧急", "紧急不重要", "不紧急不重要"}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().PadSides(layout.Inset, layout.Inset, 20, layout.Inset).
			Gap(12).Children(func() {
			ui.Row(c).FillWidth().Items(ui.Start).Gap(12).Children(func() {
				m.quadrantBox(c, titles[0], groups[0])
				m.quadrantBox(c, titles[1], groups[1])
			})
			ui.Row(c).FillWidth().Items(ui.Start).Gap(12).Children(func() {
				m.quadrantBox(c, titles[2], groups[2])
				m.quadrantBox(c, titles[3], groups[3])
			})
		})
	})
}

type quadrantItem struct {
	Card string
	Item domain.TodoItem
}

func (m *Model) quadrantBox(c ui.C, title string, items []quadrantItem) {
	theme := c.Theme()
	ui.Column(c).Grow(1).MinHeight(230).Background(theme.Card).
		Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).Children(func() {
		ui.Row(c).Height(42).FillWidth().PadX(14).Items(ui.Center).Gap(8).Children(func() {
			typography.Body(c, title).Bold()
			typography.Caption(c, fmt.Sprintf("%d", len(items))).Mono()
			ui.Spacer(c)
		})
		ui.Divider(c)
		if len(items) == 0 {
			ui.Column(c).Fill().Center().Children(func() {
				typography.Caption(c, "暂时空着").Muted()
			})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Pad(6).Gap(2).Children(func() {
				for index := range items {
					row := items[index]
					action := ui.Column(c).FillWidth().PadSides(8, 8, 6, 8).Gap(2).
						Radius(theme.Metrics.RadiusItem()).Hover(ui.Look{}.Background(theme.Accent)).
						Hit(ui.ID(c), "编辑 "+row.Item.Title).Children(func() {
						typography.Body(c, row.Item.Title).MaxLines(1).Ellipsis()
						typography.Caption(c, row.Card)
					})
					if action.Clicked() {
						m.openItemForm(row.Item.CardID, &row.Item)
					}
				}
			})
		})
	})
}

// historyView is the completed items list, one card with hairlines so the
// completion date and card title line up across rows.
func (m *Model) historyView(c ui.C) {
	theme := c.Theme()
	if len(m.history) == 0 {
		ui.Column(c).Fill().Center().Gap(8).Children(func() {
			ui.Icon(c, icons.CircleCheck).Size(28, 28).Color(theme.MutedForeground)
			typography.Body(c, "还没有完成的事项").Muted()
			typography.Caption(c, "在待办里勾选完成，它会出现在这里。").Muted()
		})
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().PadSides(layout.Inset, layout.Inset, 20, layout.Inset).Children(func() {
			ui.Column(c).FillWidth().Background(theme.Card).
				Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).Children(func() {
				ui.Row(c).Height(40).FillWidth().PadX(14).Items(ui.Center).Children(func() {
					typography.Caption(c, fmt.Sprintf("已完成 %d 项", len(m.history)))
				})
				for index := range m.history {
					entry := &m.history[index]
					ui.Divider(c)
					m.historyRow(c, entry)
				}
			})
		})
	})
}

func (m *Model) historyRow(c ui.C, entry *domain.TodoHistory) {
	theme := c.Theme()
	item := entry.Item
	c.Key("todo-history-" + item.ID)
	ui.Row(c).FillWidth().PadSides(14, 10, 10, 14).Gap(10).Items(ui.Center).Children(func() {
		ui.Icon(c, icons.CircleCheck).Size(14, 14).Shrink(0).Color(theme.Success)
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			typography.Body(c, item.Title).MaxLines(1).Ellipsis()
			ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
				typography.Caption(c, entry.CardTitle)
				typography.Caption(c, "完成于 "+time.Unix(item.CompletedAt, 0).Format("2006-01-02 15:04")).Mono()
			})
		})
		if len(item.Progress) > 0 {
			label := fmt.Sprintf("进度 %d/%d", domain.TodoDoneCount(item.Progress), len(item.Progress))
			if ui.Button(c, label).Ghost().Clicked() {
				m.openProgress(item)
			}
		}
		if ui.Button(c, "恢复").Outline().Small().Clicked() {
			m.setStatus(item.ID, domain.TodoStatusDoing)
		}
		if ui.Button(c, "删除已完成事项").Ghost().Icon(icons.Trash2).IconOnly().
			Tooltip("删除").Clicked() {
			m.deleteItem = deleteItem{open: true, id: item.ID, title: item.Title}
		}
	})
}

// historyRow is one completed item.
