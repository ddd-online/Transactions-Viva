package analysis

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// canvas paints the selected chart into its card. The drawing is a small,
// self-contained canvas chart rather than a browser chart: line and donut
// charts share the theme's chart palette and the page's summary.
func (m *Model) canvas(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Height(320).
		Radius(theme.Metrics.RadiusSurface).
		Border(1, theme.Border).
		Background(theme.Card).
		Children(func() {
			ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) {
				if chart.ChartType == chartPie {
					m.drawPie(d)
					return
				}
				m.drawLine(d)
			})
		})
}

func (m *Model) drawLine(d *ui.DrawContext) {
	theme := d.Theme
	inner := inset(d.Rect, 18)
	if len(m.lineData) == 0 || !hasLineValues(m.lineData) {
		d.Canvas.LabelCenter("暂无数据", inner, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		return
	}
	plot := gfx.Rect{
		X: inner.X + 52, Y: inner.Y + 16,
		Width: inner.Width - 68, Height: inner.Height - 54,
	}
	if plot.Width < 40 || plot.Height < 40 {
		return
	}
	maxAmount := int64(0)
	for _, line := range m.lineData {
		for _, point := range line.Data {
			if point.Amount > maxAmount {
				maxAmount = point.Amount
			}
		}
	}
	if maxAmount <= 0 {
		maxAmount = 1
	}
	// Horizontal grid and amount labels.
	for step := 0; step <= 4; step++ {
		y := plot.Y + plot.Height*float32(step)/4
		d.Canvas.Fill(gfx.Rect{X: plot.X, Y: y, Width: plot.Width, Height: 1}, theme.Border)
		value := float64(maxAmount) * float64(4-step) / 4 / 100
		label := fmt.Sprintf("%.0f", value)
		if value > 0 && value < 10 {
			label = fmt.Sprintf("%.1f", value)
		}
		d.Canvas.Label(label, gfx.Rect{X: inner.X, Y: y - 8, Width: 44, Height: 16},
			gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
	}

	labels := m.lineData[0].Data
	if len(labels) == 0 {
		return
	}
	maxLabels := max(2, int(plot.Width/64))
	labelStep := max(1, int(math.Ceil(float64(len(labels))/float64(maxLabels))))
	indices := make([]int, 0, maxLabels+1)
	for index := 0; index < len(labels); index += labelStep {
		indices = append(indices, index)
	}
	if last := len(labels) - 1; indices[len(indices)-1] != last {
		if last-indices[len(indices)-1] < labelStep {
			indices[len(indices)-1] = last
		} else {
			indices = append(indices, last)
		}
	}
	for _, index := range indices {
		x := plot.X
		if len(labels) > 1 {
			x += plot.Width * float32(index) / float32(len(labels)-1)
		} else {
			x += plot.Width / 2
		}
		label := labels[index].Time
		if len(label) == 7 {
			label = label[5:]
		}
		d.Canvas.LabelCenter(label, gfx.Rect{X: x - 24, Y: plot.Y + plot.Height + 8, Width: 48, Height: 16},
			gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
	}

	for lineIndex, line := range m.lineData {
		if len(line.Data) == 0 {
			continue
		}
		color := seriesColor(*theme, lineIndex)
		var path gfx.PathBuilder
		for index, point := range line.Data {
			x := plot.Width / 2
			if len(line.Data) > 1 {
				x = plot.Width * float32(index) / float32(len(line.Data)-1)
			}
			y := plot.Height * (1 - float32(point.Amount)/float32(maxAmount))
			if index == 0 {
				path.MoveTo(x, y)
			} else {
				path.LineTo(x, y)
			}
		}
		d.Canvas.DrawPath(path.Path(plot.Width, plot.Height), plot, gfx.PathStyle{
			Stroke: color, StrokeWidth: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		})
		for index, point := range line.Data {
			x := plot.X
			if len(line.Data) > 1 {
				x += plot.Width * float32(index) / float32(len(line.Data)-1)
			} else {
				x += plot.Width / 2
			}
			y := plot.Y + plot.Height*(1-float32(point.Amount)/float32(maxAmount))
			d.Canvas.Rounded(gfx.Rect{X: x - 2.5, Y: y - 2.5, Width: 5, Height: 5}, 2.5, color)
		}
	}
}

func (m *Model) drawPie(d *ui.DrawContext) {
	theme := d.Theme
	inner := inset(d.Rect, 18)
	if len(m.pieSlices) == 0 || m.pie.Total <= 0 {
		d.Canvas.LabelCenter("暂无数据", inner, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		return
	}
	size := min(inner.Width, inner.Height) - 12
	if size < 60 {
		return
	}
	pie := gfx.Rect{
		X: inner.X + (inner.Width-size)/2, Y: inner.Y + (inner.Height-size)/2,
		Width: size, Height: size,
	}
	center := size / 2
	radius := size/2 - 8
	start := float64(-math.Pi / 2)
	for index, slice := range m.pieSlices {
		angle := float64(slice.Amount) / float64(m.pie.Total) * 2 * math.Pi
		var path gfx.PathBuilder
		path.MoveTo(center, center)
		steps := max(3, int(angle/0.08))
		for step := 0; step <= steps; step++ {
			a := start + angle*float64(step)/float64(steps)
			path.LineTo(center+float32(math.Cos(a))*radius, center+float32(math.Sin(a))*radius)
		}
		path.Close()
		d.Canvas.DrawPath(path.Path(size, size), pie, gfx.PathStyle{
			Fill: pieColor(*theme, index), Stroke: theme.Card, StrokeWidth: 2,
		})
		start += angle
	}
	hole := radius * 0.52
	d.Canvas.Rounded(gfx.Rect{
		X: pie.X + center - hole, Y: pie.Y + center - hole,
		Width: hole * 2, Height: hole * 2,
	}, hole, theme.Card)
	d.Canvas.LabelCenter("合计", gfx.Rect{
		X: pie.X + center - radius/2, Y: pie.Y + center - 18,
		Width: radius, Height: 18,
	}, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
	d.Canvas.LabelCenter(domain.CentsToYuan(m.pie.Total), gfx.Rect{
		X: pie.X + center - radius/2, Y: pie.Y + center,
		Width: radius, Height: 22,
	}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
}

func inset(rect gfx.Rect, all float32) gfx.Rect {
	rect.X += all
	rect.Y += all
	rect.Width = max(rect.Width-2*all, 0)
	rect.Height = max(rect.Height-2*all, 0)
	return rect
}

func hasLineValues(lines []repository.ChartLineData) bool {
	for _, line := range lines {
		for _, point := range line.Data {
			if point.Amount != 0 {
				return true
			}
		}
	}
	return false
}
