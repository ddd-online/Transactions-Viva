package events

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/semantic"
	"transactions/internal/components/typography"
	"transactions/internal/repository"
)

// linksPanel is the right pane: the workspace's records whose 关联事件日期 names
// the selected day. Events are workspace-wide, so the pane shows every ledger's
// records and names each row's ledger.
func (m *Model) linksPanel(c ui.C, width float32) {
	theme := c.Theme()
	ui.Column(c).Width(width).FillHeight().Children(func() {
		ui.Row(c).Height(layout.SectionHeaderHeight).FillWidth().
			PadX(layout.Inset).Gap(layout.Gap).Items(ui.Center).Children(func() {
			typography.Section(c, "关联交易")
			if m.date != "" {
				typography.Caption(c, strconv.Itoa(len(m.linked))).
					Background(theme.Muted).Radius(theme.Metrics.RadiusItem()).PadX(6).PadY(1)
			}
			ui.Spacer(c)
		})
		ui.Divider(c)
		switch {
		case m.date == "":
			emptystate.Notice(c, "选择事件查看关联交易")
		case len(m.linked) == 0:
			emptystate.Notice(c, "暂无关联交易")
		default:
			ui.Scroll(c).Grow(1).Children(func() {
				for index := range m.linked {
					m.linkedRow(c, &m.linked[index])
					ui.Divider(c)
				}
			})
		}
	})
}

// linkedRow is one transaction: description (or category), signed amount, tags
// and the ledger and day it belongs to. It is read-only, like the old rail. The
// amount owns a fixed column and the description ellipsizes: a long title must
// not push the number out of the rail.
func (m *Model) linkedRow(c ui.C, entry *repository.RecordEntry) {
	theme := c.Theme()
	record := entry.Record
	ui.Column(c).FillWidth().Pad(layout.BlockGap).Gap(4).Children(func() {
		ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
			typography.Body(c, linkedTitle(record)).Weight(gfx.WeightMedium).
				Grow(1).Shrink(1).MinWidth(0).Ellipsis()
			typography.Body(c, linkedAmount(record)).Mono().Weight(gfx.WeightMedium).
				Color(semantic.Kind(*theme, record.TransactionType)).TextEnd().
				Width(linkedAmountWidth)
		})
		if len(entry.Tags) > 0 {
			ui.Row(c).FillWidth().Gap(4).Clip().Children(func() {
				for _, name := range entry.Tags {
					typography.Caption(c, name).Color(theme.Primary).
						Background(theme.Muted).Radius(theme.Metrics.RadiusItem()).
						PadSides(1, 4, 1, 4)
				}
			})
		}
		ui.Row(c).FillWidth().Gap(4).Items(ui.Center).Children(func() {
			typography.Caption(c, m.ledgerName(record.LedgerID)).Grow(1).Ellipsis()
			typography.Caption(c, linkedDate(record))
		})
	})
}
