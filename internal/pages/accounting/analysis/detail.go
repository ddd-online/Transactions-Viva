package analysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

func (m *Model) linePanel(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(8).Children(func() {
			typography.Section(c, "曲线")
			ui.Text(c, fmt.Sprintf("%d", len(m.lines))).FontSize(typography.SizeCaption).
				Color(theme.MutedForeground).Background(theme.Muted).
				Radius(theme.Metrics.RadiusItem()).PadX(6).PadY(1)
			ui.Spacer(c)
			if !chart.IsPreset {
				if ui.Button(c, "添加曲线").Small().Icon(icons.Plus).Clicked() {
					m.openLine(chart, -1)
				}
			}
		})
		if len(m.lines) == 0 {
			typography.BodyParagraph(c, "还没有曲线，添加一条之后这里会画出趋势。", 2).Muted()
			return
		}
		for index := range m.lines {
			line := &m.lines[index]
			c.KeyInt(index)
			ui.Row(c).FillWidth().MinHeight(40).PadX(8).Gap(8).Items(ui.Center).
				Radius(theme.Metrics.RadiusControl).Hover(ui.Look{}.Background(theme.Hover)).Children(func() {
				dot(c, seriesColor(*theme, index))
				typography.Body(c, line.Label)
				ui.Text(c, lineTypeLabel(line.TransactionType)).FontSize(typography.SizeCaption).
					Color(theme.MutedForeground).Background(theme.Muted).
					Radius(theme.Metrics.RadiusItem()).PadX(6).PadY(1)
				typography.Caption(c, conditionSummary(*line)).Grow(1).Ellipsis()
				if !chart.IsPreset {
					if ui.Button(c, "编辑曲线 "+line.Label).Ghost().Small().Clicked() {
						m.openLine(chart, index)
					}
					if ui.Button(c, "删除曲线 "+line.Label).Ghost().Icon(icons.Trash2).IconOnly().Tooltip("删除曲线").Clicked() {
						m.removeLine(chart, index)
					}
				}
			})
		}
	})
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
		switch chart.GroupField {
		case groupTag:
			return "这个范围里没有「" + lineTypeLabel(chart.GroupTransactionType) + " · " + chart.GroupCategory + "」的记录。"
		case groupCategory:
			return "这个范围里没有「" + lineTypeLabel(chart.GroupTransactionType) + "」记录。"
		}
		return "这个范围里没有可统计的记录。"
	}
	if chart.GroupField == groupTag {
		return "这个范围里这些记录都没有标签，先给记录加上标签。"
	}
	return "这个范围里没有可统计的记录。"
}
