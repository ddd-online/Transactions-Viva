package analysis

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/semantic"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

// The table's columns, shared by the header row and the rows so the two always
// agree: a name, the type it sums, the condition it matches, and its total.
const (
	curveNameWidth    float32 = 168
	curveTypeWidth    float32 = 88
	curveOutlierWidth float32 = 96
	curveConditionMin float32 = 200
	curveAmountWidth  float32 = 128
	curveActionWidth  float32 = 64

	// curveTableMinWidth is the narrowest the columns go before the table
	// scrolls sideways instead of clipping a column.
	curveTableMinWidth float32 = curveNameWidth + curveTypeWidth + curveOutlierWidth +
		curveConditionMin + curveAmountWidth + curveActionWidth + 5*layout.ColumnGap
	// A preset has no 操作 column, so it is one action column narrower.
	curveTableMinWidthPreset = curveTableMinWidth - curveActionWidth - layout.ColumnGap

	// emptyTableBody is the room the table keeps for its own empty sentence on
	// a chart with no rows: the sentence must not be cut, and it should not
	// push the totals band down.
	emptyTableBody float32 = 96
)

// chartTable is the list under the plot: one row per curve that says what the
// line is and what it adds up to, with its edit and delete commands. A pie has
// no curves, so its table lists the slices with their amounts and shares.
func (m *Model) chartTable(c ui.C, chart *domain.Chart) {
	// The table is as tall as the rows it holds, up to the height where it
	// starts to scroll: a short list leaves the chart the room, and a long one
	// never squeezes it below its minimum.
	rows := len(m.lines)
	if chart.ChartType == chartPie {
		rows = len(m.pieSlices)
	}
	const maxRows = 5
	body := float32(min(rows, maxRows)) * layout.RowHeight
	if rows == 0 {
		body = emptyTableBody
	}
	height := layout.SectionHeaderHeight + layout.TableHeaderHeight + body
	// On a window too short for the plot's minimum plus the whole table, the
	// table is what gives way: it shrinks first, down to its head and two rows,
	// and the rows it still holds scroll inside it. Otherwise the column would
	// push the plot's minimum past the card and the totals band would fall out
	// of the window.
	floorBody := float32(min(rows, 2)) * layout.RowHeight
	if rows == 0 {
		floorBody = emptyTableBody
	}
	floor := min(layout.SectionHeaderHeight+layout.TableHeaderHeight+floorBody, height)
	ui.Column(c).FillWidth().Basis(height).MinHeight(floor).Shrink(1).Children(func() {
		m.chartTableHead(c, chart)
		// The header scrolls with its rows, so a window too narrow for the
		// columns scrolls the table sideways instead of cutting one off.
		minWidth := curveTableMinWidth
		if chart.IsPreset {
			minWidth = curveTableMinWidthPreset
		}
		ui.Scroll(c).Grow(1).FillWidth().Sideways().Scrollbars().Children(func() {
			ui.Column(c).FillWidth().MinWidth(minWidth).Children(func() {
				if chart.ChartType == chartPie {
					m.pieTable(c, chart)
					return
				}
				m.curveTable(c, chart)
			})
		})
	})
}

// chartTableHead is the table's own header: what it lists, how many, and the
// command that adds one more.
func (m *Model) chartTableHead(c ui.C, chart *domain.Chart) {
	label, count := "曲线", len(m.lines)
	if chart.ChartType == chartPie {
		label, count = "分组", len(m.pieSlices)
	}
	ui.Row(c).Height(layout.SectionHeaderHeight).FillWidth().Items(ui.Center).
		Gap(layout.Gap).Children(func() {
		typography.Section(c, label)
		countBadge(c, count)
		ui.Spacer(c)
		if chart.ChartType == chartLine && !chart.IsPreset {
			if ui.Button(c, "添加曲线").Small().Ghost().Icon(icons.Plus).Clicked() {
				m.openLine(chart, -1)
			}
		}
	})
}

// curveTable is the line chart's series: a header row with the columns, then a
// row per curve. The rows share the table with the chart above them, so the
// page ends on the totals band instead of trailing into empty space.
func (m *Model) curveTable(c ui.C, chart *domain.Chart) {
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.BlockGap, Gap: layout.ColumnGap}, func(c ui.C) {
		typography.Caption(c, "名称").Width(curveNameWidth)
		// The type column is centered: its header and the capsule under it sit
		// in the middle of the same cell, so the column reads as one block.
		ui.Row(c).Width(curveTypeWidth).Justify(ui.Center).Children(func() {
			typography.Caption(c, "类型")
		})
		ui.Row(c).Width(curveOutlierWidth).Justify(ui.Center).Children(func() {
			typography.Caption(c, "包含离群值")
		})
		typography.Caption(c, "条件").Grow(1).MinWidth(curveConditionMin)
		ui.Row(c).Width(curveAmountWidth).Justify(ui.End).Children(func() {
			typography.Caption(c, "合计")
		})
		// A preset has no commands, so it has no 操作 column at all. Keeping
		// an empty cell there left a blank strip after 合计; without the
		// column, 合计 is the rightmost column and sits on the table edge.
		if !chart.IsPreset {
			ui.Row(c).Width(curveActionWidth).Justify(ui.Center).Children(func() {
				typography.Caption(c, "操作")
			})
		}
	})
	if len(m.lines) == 0 {
		ui.Row(c).FillWidth().Height(emptyTableBody).Pad(layout.Inset).
			Items(ui.Center).Children(func() {
			typography.CaptionParagraph(c, "还没有曲线。加一条之后，这张表会写出它包含哪些记录、合计多少。", 2).Muted()
		})
		return
	}
	for index := range m.lines {
		line := &m.lines[index]
		c.KeyInt(index)
		m.curveRow(c, chart, index, line)
	}
}

// curveRow is one curve of the line chart.
func (m *Model) curveRow(c ui.C, chart *domain.Chart, index int, line *domain.ChartLine) {
	theme := c.Theme()
	ui.Row(c).Height(layout.RowHeight).FillWidth().
		PadX(layout.BlockGap).Gap(layout.ColumnGap).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).Children(func() {
		ui.Row(c).Width(curveNameWidth).Gap(layout.Gap).Items(ui.Center).Children(func() {
			dot(c, seriesColor(*theme, index))
			typography.Body(c, line.Label).Grow(1).Ellipsis()
		})
		ui.Row(c).Width(curveTypeWidth).Justify(ui.Center).Children(func() {
			kindChip(c, lineTypeLabel(line.TransactionType))
		})
		ui.Row(c).Width(curveOutlierWidth).Justify(ui.Center).Children(func() {
			outlier := "否"
			if line.IncludeOutlier {
				outlier = "是"
			}
			typography.Body(c, outlier)
		})
		typography.Caption(c, conditionSummary(*line)).Grow(1).
			MinWidth(curveConditionMin).Ellipsis()
		ui.Row(c).Width(curveAmountWidth).Justify(ui.End).Children(func() {
			typography.Body(c, domain.CentsToYuan(m.seriesTotal(index))).Mono()
		})
		if !chart.IsPreset {
			ui.Row(c).Width(curveActionWidth).Gap(2).Justify(ui.Center).Children(func() {
				if ui.Button(c, "编辑曲线 "+line.Label).Ghost().Small().Icon(icons.Pencil).
					IconOnly().Tooltip("编辑曲线").Clicked() {
					m.openLine(chart, index)
				}
				if ui.Button(c, "删除曲线 "+line.Label).Ghost().Small().Icon(icons.Trash2).
					IconOnly().Tooltip("删除曲线").Clicked() {
					m.removeLine(chart, index)
				}
			})
		}
	})
}

// pieTable lists the slices: the same colors the donut draws, what each one
// adds up to, and its share of the total.
func (m *Model) pieTable(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	// The first column is whatever the pie groups by, so the header can never
	// disagree with the control in the page header.
	groupColumn := groupLabel(chart.GroupField)
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.BlockGap, Gap: layout.ColumnGap}, func(c ui.C) {
		typography.Caption(c, groupColumn).Grow(1)
		ui.Row(c).Width(curveAmountWidth).Justify(ui.End).Children(func() {
			typography.Caption(c, "金额")
		})
		ui.Row(c).Width(curveTypeWidth).Justify(ui.End).Children(func() {
			typography.Caption(c, "占比")
		})
	})
	if len(m.pieSlices) == 0 {
		ui.Row(c).FillWidth().Height(emptyTableBody).Pad(layout.Inset).
			Items(ui.Center).Children(func() {
			typography.CaptionParagraph(c, pieEmptyDescription(m.pie, *m.selectedChart()), 3).Muted()
		})
		return
	}
	for index, slice := range m.pieSlices {
		c.KeyInt(index)
		ui.Row(c).Height(layout.RowHeight).FillWidth().
			PadX(layout.BlockGap).Gap(layout.ColumnGap).Items(ui.Center).
			Radius(theme.Metrics.RadiusControl).Children(func() {
			ui.Row(c).Grow(1).Gap(layout.Gap).Items(ui.Center).Children(func() {
				dot(c, pieColor(*theme, index))
				typography.Body(c, slice.Label).Grow(1).Ellipsis()
			})
			ui.Row(c).Width(curveAmountWidth).Justify(ui.End).Children(func() {
				typography.Body(c, domain.CentsToYuan(slice.Amount)).Mono()
			})
			ui.Row(c).Width(curveTypeWidth).Justify(ui.End).Children(func() {
				typography.Caption(c, fmt.Sprintf("%.1f%%", m.sliceShare(index)))
			})
		})
	}
}

// footer is the band under the table: the range's totals, the same readout the
// records page carries at its own bottom.
func (m *Model) footer(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	ui.Row(c).Height(layout.FooterHeight).FillWidth().
		PadX(layout.Inset).Gap(layout.GroupGap).Items(ui.Center).Children(func() {
		if chart.ChartType == chartPie {
			footerStat(c, "合计", m.pie.Total, theme.Foreground)
			if len(m.pieSlices) > 0 {
				typography.Caption(c, fmt.Sprintf("共 %d 组", len(m.pieSlices)))
			}
			if m.pie.Total > m.pie.FilteredTotal {
				ui.Spacer(c)
				typography.Caption(c, "带多个标签的记录会分别计入每个标签")
			}
			return
		}
		footerStat(c, "收入", m.stats.Income, theme.Success)
		footerStat(c, "支出", m.stats.Expense, theme.Danger)
		footerStat(c, "转账", m.stats.Transfer, semantic.Kind(*theme, "transfer"))
	})
}

// footerStat is one total of the footer: its name, then its figure on the mono
// grid, colored like the series it sums.
func footerStat(c ui.C, label string, cents int64, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		typography.Caption(c, label)
		ui.Text(c, domain.CentsToYuan(cents)).FontSize(typography.SizeCaption).
			Mono().Weight(gfx.WeightMedium).Color(color)
	})
}

// lineRow is one curve: its color and name, its total on the mono grid, the
// commands that change it, and a second line with the type and the condition
// the curve matches. The whole row is 56 high, so the list reads as a table.

// seriesTotal is the sum of one loaded curve over the range. A curve the query
// did not return yet counts as zero, the same way its line draws.
func (m *Model) seriesTotal(index int) int64 {
	if index < 0 || index >= len(m.lineData) {
		return 0
	}
	total := int64(0)
	for _, point := range m.lineData[index].Data {
		total += point.Amount
	}
	return total
}

// kindChip is the transaction type of a curve. The table rows sit on the card,
// so the capsule takes the muted fill the other chips on a card use.
func kindChip(c ui.C, label string) {
	theme := c.Theme()
	ui.Text(c, label).FontSize(typography.SizeCaption).Color(theme.MutedForeground).
		Background(theme.Muted).Radius(theme.Metrics.RadiusItem()).
		PadX(6).PadY(1)
}

// countBadge is a count in a capsule: the same badge the rail's header uses.
func countBadge(c ui.C, count int) {
	theme := c.Theme()
	ui.Text(c, strconv.Itoa(count)).FontSize(typography.SizeCaption).
		Color(theme.MutedForeground).Background(theme.Background).
		Radius(theme.Metrics.RadiusItem() + 2).PadX(6).PadY(1)
}

func (m *Model) moveChart(rows []int, to int) {
	if len(rows) == 0 {
		return
	}
	m.charts = moveItem(m.charts, rows[0], to)
	if m.repo == nil {
		return
	}
	ids := make([]string, len(m.charts))
	for index, chart := range m.charts {
		ids[index] = chart.ChartID
	}
	if err := m.repo.ReorderCharts(context.Background(), m.ledgerID, ids); err != nil {
		m.toaster.Show("保存图表排序失败："+err.Error(), "", ui.ToastError)
		m.Reload()
	}
}

func moveItem[T any](items []T, from, to int) []T {
	if from < 0 || from >= len(items) || to < 0 || to > len(items) {
		return items
	}
	if to > from {
		to--
	}
	if to == from {
		return items
	}
	item := items[from]
	if from < to {
		copy(items[from:to], items[from+1:to+1])
	} else {
		copy(items[to+1:from+1], items[to:from])
	}
	items[to] = item
	return items
}

func conditionSummary(line domain.ChartLine) string {
	if len(line.Conditions) == 0 {
		return "不限"
	}
	parts := make([]string, 0, len(line.Conditions))
	for _, condition := range line.Conditions {
		fields := []string{}
		if condition.TransactionType != "" {
			fields = append(fields, lineTypeLabel(condition.TransactionType))
		}
		if condition.Category != "" {
			fields = append(fields, condition.Category)
		}
		if len(condition.Tags) > 0 {
			fields = append(fields, strings.Join(condition.Tags, "/"))
		}
		if condition.Description != "" {
			fields = append(fields, "“"+condition.Description+"”")
		}
		if len(fields) == 0 {
			fields = append(fields, "不限")
		}
		parts = append(parts, strings.Join(fields, " · "))
	}
	return strings.Join(parts, "；")
}

func (m *Model) askDelete(chart domain.Chart) {
	m.deleting = chart
	m.showModal(modalDelete)
}

func (m *Model) deleteChart() {
	if m.repo == nil {
		return
	}
	chart := m.deleting
	if err := m.repo.DeleteChart(context.Background(), chart.ChartID); err != nil {
		m.modal = modalNone
		m.toaster.Show("删除图表失败："+err.Error(), "", ui.ToastError)
		return
	}
	m.modal = modalNone
	if m.selectedID == chart.ChartID {
		m.selectedID = ""
	}
	m.Reload()
	m.toaster.Show("图表已删除", "", ui.ToastSuccess)
}

func foldPie(slices []repository.ChartPieSlice) []repository.ChartPieSlice {
	const maxSlices = 9
	if len(slices) <= maxSlices {
		return slices
	}
	head := append([]repository.ChartPieSlice(nil), slices[:maxSlices]...)
	other := repository.ChartPieSlice{Key: "__other__", Label: fmt.Sprintf("其他（%d 组）", len(slices)-maxSlices)}
	for _, slice := range slices[maxSlices:] {
		other.Amount += slice.Amount
	}
	return append(head, other)
}

func pieEmptyDescription(pie repository.ChartPieResult, chart domain.Chart) string {
	if pie.RangeRecords == 0 {
		return "这个时间范围里没有记录，换一个范围试试。"
	}
	if pie.FilteredRecords == 0 {
		// One shape for both groupings: 「括号里是被筛掉的那一类」的记录.
		switch chart.GroupField {
		case groupTag:
			return "这个范围里没有「" + lineTypeLabel(chart.GroupTransactionType) + " · " + chart.GroupCategory + "」的记录。"
		case groupCategory:
			return "这个范围里没有「" + lineTypeLabel(chart.GroupTransactionType) + "」的记录。"
		}
		return "这个范围里没有可统计的记录。"
	}
	if chart.GroupField == groupTag {
		return "这个范围里这些记录都没有标签，先给记录加上标签。"
	}
	return "这个范围里没有可统计的记录。"
}
