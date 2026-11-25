package todo

import (
	"fmt"
	"strconv"
	"time"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/state"
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
	ui.Column(c).Fill().PadSides(layout.Inset, layout.Inset, 20, layout.Inset).Children(func() {
		for row := 0; row < 2; row++ {
			ui.Row(c).Grow(1).Basis(0).FillWidth().Children(func() {
				m.quadrantPane(c, titles[row*2], groups[row*2])
				ui.Divider(c)
				m.quadrantPane(c, titles[row*2+1], groups[row*2+1])
			})
			if row == 0 {
				ui.Divider(c)
			}
		}
	})
}

type quadrantItem struct {
	Card string
	Item domain.TodoItem
}

// quadrantPane is one quarter of the matrix: a heading, a hairline, then the
// items. It has no card surface; the two crossing dividers are the only
// structure, so the four quarters stay exactly equal.
func (m *Model) quadrantPane(c ui.C, title string, items []quadrantItem) {
	theme := c.Theme()
	ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, title)
			layout.SectionCount(c, strconv.Itoa(len(items)))
			ui.Spacer(c)
		})
		if len(items) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
				typography.Caption(c, "暂时空着").Muted()
			})
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().PadSides(8, 8, 8, 8).Gap(2).Children(func() {
				for index := range items {
					row := items[index]
					action := ui.Column(c).FillWidth().PadSides(10, 10, 8, 10).Gap(2).
						Radius(theme.Metrics.RadiusItem()).Hover(ui.Look{}.Background(state.Hover(*theme))).
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

// historyView is the completed items list. It is a flat table: one header row
// and hairlines between rows, so it reads as a history, not as a card stack.
func (m *Model) historyView(c ui.C) {
	if len(m.history) == 0 {
		emptystate.Page(c, &icons.CircleCheck, "暂无已完成的事项",
			"在待办里勾选完成，它会出现在这里。", nil)
		return
	}
	area := ui.Scroll(c).Grow(1).Sideways().Scrollbars()
	area.Children(func() {
		ui.Column(c).FillWidth().MinWidth(historyTableMinWidth).
			PadBottom(layout.Inset).Children(func() {
			m.historyHeader(c)
			// The rows this frame started with: restoring an item reloads the
			// list under this loop (see historyRow).
			entries := m.history
			for index := range entries {
				m.historyRow(c, &entries[index])
			}
		})
	})
}

const (
	historyTimeWidth     = float32(112)
	historyCardWidth     = float32(120)
	historyItemMinWidth  = float32(180)
	historyProgressWidth = float32(72)
	historyActionWidth   = float32(116)
	historyCheckWidth    = float32(13)
	historyTableMinWidth = historyTimeWidth + historyCardWidth + historyItemMinWidth +
		historyProgressWidth + historyActionWidth + 4*layout.ColumnGap + 2*layout.Inset
)

func (m *Model) historyHeader(c ui.C) {
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.Inset, Gap: layout.ColumnGap}, func(c ui.C) {
		// The row leads with its completion check, so the label is pushed past
		// the icon to sit over the timestamps it names.
		ui.Row(c).Width(historyTimeWidth).Gap(6).Children(func() {
			ui.Row(c).Width(historyCheckWidth)
			typography.Caption(c, "完成时间")
		})
		ui.Row(c).Width(historyCardWidth).Children(func() {
			typography.Caption(c, "主题")
		})
		ui.Row(c).Grow(1).Basis(0).MinWidth(historyItemMinWidth).Children(func() {
			typography.Caption(c, "事项").Grow(1).MinWidth(0)
		})
		ui.Row(c).Width(historyProgressWidth).Justify(ui.Center).Children(func() {
			typography.Caption(c, "进度")
		})
		ui.Row(c).Width(historyActionWidth).Justify(ui.Center).Children(func() {
			typography.Caption(c, "操作")
		})
	})
}

func (m *Model) historyRow(c ui.C, entry *domain.TodoHistory) {
	theme := c.Theme()
	item := entry.Item
	c.Key("todo-history-" + item.ID)
	// The row takes the table's geometry but no pointer wash: nothing happens when
	// it is clicked — its two buttons are the whole interaction.
	ui.Row(c).Height(layout.RowHeight).FillWidth().PadX(layout.Inset).
		Gap(layout.ColumnGap).Items(ui.Center).Children(func() {
		ui.Row(c).Width(historyTimeWidth).Gap(6).Items(ui.Center).Children(func() {
			ui.Icon(c, icons.CircleCheck).Size(historyCheckWidth, historyCheckWidth).
				Shrink(0).Color(theme.Success)
			typography.Caption(c, time.Unix(item.CompletedAt, 0).Format("01-02 15:04")).Mono()
		})
		ui.Row(c).Width(historyCardWidth).Children(func() {
			typography.Caption(c, entry.CardTitle).Grow(1).MinWidth(0).MaxLines(1).Ellipsis()
		})
		ui.Row(c).Grow(1).Basis(0).MinWidth(historyItemMinWidth).Children(func() {
			typography.Body(c, item.Title).Grow(1).MinWidth(0).MaxLines(1).Ellipsis()
		})
		ui.Row(c).Width(historyProgressWidth).Justify(ui.Center).Children(func() {
			if len(item.Progress) > 0 {
				label := fmt.Sprintf("%d/%d", domain.TodoDoneCount(item.Progress), len(item.Progress))
				if ui.Button(c, label).Ghost().Small().Clicked() {
					m.openProgress(item)
				}
			} else {
				typography.Caption(c, "—").Muted()
			}
		})
		ui.Row(c).Width(historyActionWidth).Justify(ui.Center).Gap(4).Items(ui.Center).Children(func() {
			if ui.Button(c, "恢复").Ghost().Small().Clicked() {
				// Restoring rebuilds this list, so it lands after the paint that
				// asked for it instead of under the loop above.
				c.After(func() { m.setStatus(item.ID, domain.TodoStatusDoing) })
			}
			if ui.Button(c, "删除").Destructive().Small().Clicked() {
				m.deleteItem = deleteItem{open: true, id: item.ID, title: item.Title}
			}
		})
	})
}
