package analysis

import (
	"math"
	"strconv"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

// canvas paints the selected chart into its card. A line chart is Viva's own
// area chart — axes, grid, dots and the hover tooltip are painted by the
// component — so the page keeps no chart geometry of its own and a hovered
// point needs no hand-written tip. A pie has no component, so it is still
// painted here, in the same card and from the same theme palette.
func (m *Model) canvas(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	if chart.ChartType == chartPie {
		// A pie has no hover model; forget the line chart's plot so a stale
		// hover is not carried over.
		m.chartHit = gfx.ID{}
		ui.Row(c).FillWidth().Grow(1).MinHeight(160).
			Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Background(theme.Card).Children(func() {
			ui.Slot(c).Fill().Draw(m.drawPie)
		})
		return
	}
	// The component registers the plot as a hit region with this id; the page's
	// Pointer turns the pointer over it into the hovered point, and its Key
	// steps that point with the arrows.
	m.chartHit = ui.ID(c)
	label := chart.Title + "的曲线"
	ui.Row(c).FillWidth().Grow(1).MinHeight(160).
		Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
		Background(theme.Card).Children(func() {
		ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) {
			if len(m.lineData) == 0 || !hasLineValues(m.lineData) {
				d.Canvas.LabelCenter("暂无数据", inset(d.Rect, 18),
					gfx.TextStyle{Size: typography.SizeBody, Color: d.Theme.MutedForeground})
				return
			}
			m.chartPlot = components.PaintAreaChart(d.Canvas, *d.Theme, inset(d.Rect, 12), m.chartHit,
				m.areaChart(*d.Theme, label), &m.chartState)
		})
	})
}

// areaChart is the selected chart's series as the component wants them: the
// buckets on the X axis as their labels, one row of yuan values per curve, a
// line with a dot per point and no fill, and a tooltip that writes money.
func (m *Model) areaChart(theme gfx.Theme, label string) components.AreaChart {
	buckets := []repository.ChartPoint(nil)
	if len(m.lineData) > 0 {
		buckets = m.lineData[0].Data
	}
	series := make([]components.ChartSeries, len(m.lineData))
	values := make([][]float32, len(m.lineData))
	for index, line := range m.lineData {
		series[index] = components.ChartSeries{
			Key: line.Label, Label: line.Label, Color: seriesColor(theme, index),
		}
		points := make([]float32, len(buckets))
		for i := range points {
			if i < len(line.Data) {
				points[i] = float32(line.Data[i].Amount) / 100
			}
		}
		values[index] = points
	}
	return components.AreaChart{
		Series: series, Values: values,
		Fill: components.ChartFillNone, Dots: true, Curve: components.ChartCurveLinear,
		Grid:   components.ChartGridHorizontal,
		Legend: components.ChartLegendNone,
		XAxis: components.ChartAxis{Format: func(dst []byte, v float32) []byte {
			index := int(v + 0.5)
			if index < 0 || index >= len(buckets) {
				return dst
			}
			text := buckets[index].Time
			if len(text) == 7 {
				text = text[5:]
			}
			return append(dst, text...)
		}},
		ValueFormat: func(dst []byte, v float32) []byte {
			return strconv.AppendFloat(dst, float64(v), 'f', 2, 32)
		},
		Label:   label,
		Version: m.chartVersion,
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

// Pointer moves the chart's hovered point under the pointer and clears it when
// the pointer leaves the plot, so the component paints (or drops) its tooltip.
func (m *Model) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerMove && event.Kind != gfx.PointerExit {
		return gfx.EventResult{}
	}
	previous, active := m.chartState.Index()
	if event.Kind == gfx.PointerExit || m.chartHit.IsZero() || event.Target != m.chartHit {
		m.chartState.Clear()
		// Only repaint: the move belongs to whatever the pointer is over now.
		return gfx.EventResult{Repaint: active}
	}
	index := m.chartPlot.IndexAt(event.X)
	if active && index == previous {
		return gfx.EventResult{Handled: true}
	}
	m.chartState.Hover(index)
	return gfx.EventResult{Handled: true, Repaint: true}
}

// chartKey steps the hovered point with the arrow keys, Home and End, the way
// the component's own demo does.
func (m *Model) chartKey(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || m.chartPlot.Count() == 0 {
		return gfx.EventResult{}
	}
	index, active := m.chartState.Index()
	if !active {
		index = -1
	}
	next, handled := components.ChartStepForKey(event.Key, index, m.chartPlot.Count())
	if !handled {
		return gfx.EventResult{}
	}
	m.chartState.Hover(next)
	return gfx.EventResult{Handled: true, Repaint: true}
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
