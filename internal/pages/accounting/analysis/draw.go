package analysis

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

// canvas paints the selected chart into its card. The drawing is a small,
// self-contained canvas chart rather than a browser chart: line and donut
// charts share the theme's chart palette and the page's summary.
// canvas is the plot surface. It takes the height the column has left, so the
// chart grows with the window instead of sitting in a fixed box; the series it
// draws are listed once, in the inspector.
func (m *Model) canvas(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Grow(1).MinHeight(160).
		Radius(theme.Metrics.RadiusSurface).
		Border(1, theme.Border).
		Background(theme.Card).Children(func() {
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
		d.Canvas.LabelCenter("暂无数据", inner, gfx.TextStyle{Size: typography.SizeBody, Color: theme.MutedForeground})
		return
	}
	plot := gfx.Rect{
		X: inner.X + 60, Y: inner.Y + 16,
		Width: inner.Width - 76, Height: inner.Height - 54,
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
	// The axis is rounded to a step a reader can hold (0 / 1500 / 3000), so the
	// top line is a number, not whatever the largest point happened to be.
	axisTop, axisStep := niceAxis(float64(maxAmount) / 100)
	for tick := 0; tick <= axisTicks; tick++ {
		y := plot.Y + plot.Height*float32(tick)/float32(axisTicks)
		d.Canvas.Fill(gfx.Rect{X: plot.X, Y: y, Width: plot.Width, Height: 1}, theme.Border)
		value := axisTop - axisStep*float64(tick)
		d.Canvas.LabelAligned(axisText(value), gfx.Rect{X: inner.X, Y: y - 8, Width: 52, Height: 16},
			gfx.TextStyle{Size: typography.SizeCaption, Color: theme.MutedForeground, Mono: true}, gfx.AlignRight)
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
			gfx.TextStyle{Size: typography.SizeCaption, Color: theme.MutedForeground})
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
			y := plot.Y + plot.Height*(1-float32(point.Amount)/100/float32(axisTop))
			d.Canvas.Rounded(gfx.Rect{X: x - 2.5, Y: y - 2.5, Width: 5, Height: 5}, 2.5, color)
		}
	}
}

func (m *Model) drawPie(d *ui.DrawContext) {
	theme := d.Theme
	inner := inset(d.Rect, 18)
	if len(m.pieSlices) == 0 || m.pie.Total <= 0 {
		d.Canvas.LabelCenter("暂无数据", inner, gfx.TextStyle{Size: typography.SizeBody, Color: theme.MutedForeground})
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
	}, gfx.TextStyle{Size: typography.SizeCaption, Color: theme.MutedForeground})
	d.Canvas.LabelCenter(domain.CentsToYuan(m.pie.Total), gfx.Rect{
		X: pie.X + center - radius/2, Y: pie.Y + center,
		Width: radius, Height: 22,
	}, gfx.TextStyle{Size: typography.SizeBody, Color: theme.Foreground, Mono: true})
}

// axisTicks is the number of intervals the amount axis is divided into.
const axisTicks = 4

// niceAxis picks a rounded top and step for the amount axis: four intervals
// that cover the range with numbers a reader can hold, so the ticks read
// 0 / 1500 / 3000 / 4500 / 6000 rather than 0 / 1334 / 2667 / 4001 / 5335.
func niceAxis(maxValue float64) (top, step float64) {
	if maxValue <= 0 {
		return 1, 1 / axisTicks
	}
	rough := maxValue / axisTicks
	magnitude := math.Pow(10, math.Floor(math.Log10(rough)))
	for _, candidate := range []float64{1, 1.5, 2, 2.5, 3, 4, 5, 10} {
		if candidate*magnitude >= rough {
			step = candidate * magnitude
			break
		}
	}
	if step == 0 {
		step = 10 * magnitude
	}
	return step * axisTicks, step
}

// axisText writes an axis value without trailing zeros: 6000 stays 6000 and
// 4.5 keeps its decimal.
func axisText(value float64) string {
	switch {
	case math.Abs(value-math.Round(value)) < 0.005:
		return fmt.Sprintf("%.0f", value)
	case math.Abs(value*10-math.Round(value*10)) < 0.005:
		return fmt.Sprintf("%.1f", value)
	}
	return fmt.Sprintf("%.2f", value)
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
