package analysis

import (
	"fmt"
	"math"
	"strconv"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/chartanim"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

const (
	// pieHoleRatio is the hole's share of the radius: a donut, not a pie, so
	// the range's total can sit in the middle.
	pieHoleRatio = 0.52
	// noSlice is "no wedge here": the hole, the seam between two wedges, or the
	// card around the pie.
	noSlice = -1
	// pieTooltipGap is how far off a wedge's outer edge its tip hangs.
	pieTooltipGap = 12
	// pieTooltipLayer is the layer the area chart paints its own tooltip in, so
	// a pie's tip sits above the card exactly like a line chart's.
	pieTooltipLayer = 4
)

// canvas paints the selected chart into its card. A line chart is Viva's own
// area chart — axes, grid, dots and the hover tooltip are painted by the
// component — so the page keeps no chart geometry of its own and a hovered
// point needs no hand-written tip. A pie has no component, so its wedges are
// painted here, in the same card and from the same theme palette, and the page
// turns the pointer into the hovered slice itself (the tip is the component's
// tooltip shell, see pieTooltip).
func (m *Model) canvas(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	if chart.ChartType == chartPie {
		// The pie forgets the line chart's plot so a stale hover is not carried
		// over, and names its own hit region: the page's Pointer reads the slice
		// under the pointer from it.
		m.chartHit = gfx.ID{}
		m.pieHit = ui.ID(c)
		ui.Row(c).FillWidth().Grow(1).MinHeight(160).
			Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Background(theme.Card).Children(func() {
			ui.Slot(c).Fill().Draw(m.drawPie)
		})
		return
	}
	m.pieHit, m.pieHover = gfx.ID{}, noSlice
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
			plot := inset(d.Rect, 12)
			progress := m.chartProgress(d)
			m.chartPlot = components.PaintAreaChart(d.Canvas, *d.Theme, plot, m.chartHit,
				m.areaChart(*d.Theme, label), &m.chartState)
			if progress < 1 {
				// The axis labels stay whole while the data unfolds: cover the
				// un-revealed plot interior rather than clipping the chart.
				revealed := m.chartPlot.Plot
				d.Canvas.Fill(gfx.Rect{
					X: revealed.X + revealed.Width*progress, Y: revealed.Y,
					Width: revealed.Width * (1 - progress), Height: revealed.Height,
				}, d.Theme.Card)
			}
		})
	})
}

// chartProgress is the one-shot reveal keyed by the selected chart and its
// data version. The key changes on a new range, chart or chart type, so the
// curve/sweep starts again.
func (m *Model) chartProgress(d *ui.DrawContext) float32 {
	key := "analysis:" + m.selectedID + ":" + strconv.FormatUint(uint64(m.chartVersion), 10)
	return m.chartAnim.Progress(d.Canvas, key, chartanim.Duration)
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

// drawPie paints the donut: one wedge per slice from twelve o'clock clockwise,
// the hole with the range's total, and — over the hovered wedge — the same
// tooltip shell the line chart uses. Its geometry is kept for pieSliceAt, which
// the page's Pointer asks which wedge a point is in.
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
	m.pieCenter = gfx.Point{X: pie.X + center, Y: pie.Y + center}
	m.pieRadius = radius
	d.Canvas.HitWithOptions(m.pieHit, pie, "饼图", gfx.HitOptions{
		Role:   gfx.RoleGroup,
		Cursor: gfx.PointerCursorCrosshair,
		Value:  m.pieSliceValue(m.pieHover),
	})
	progress := m.chartProgress(d)
	sweep := 2 * math.Pi * float64(progress)
	cursor := 0.0
	start := float64(-math.Pi / 2)
	for index, slice := range m.pieSlices {
		angle := float64(slice.Amount) / float64(m.pie.Total) * 2 * math.Pi
		if cursor >= sweep {
			break
		}
		visible := angle
		if cursor+angle > sweep {
			visible = sweep - cursor
		}
		if visible > 0 {
			var path gfx.PathBuilder
			path.MoveTo(center, center)
			steps := max(3, int(visible/0.08))
			for step := 0; step <= steps; step++ {
				a := start + visible*float64(step)/float64(steps)
				path.LineTo(center+float32(math.Cos(a))*radius, center+float32(math.Sin(a))*radius)
			}
			path.Close()
			d.Canvas.DrawPath(path.Path(size, size), pie, gfx.PathStyle{
				Fill: pieColor(*theme, index), Stroke: theme.Card, StrokeWidth: 2,
			})
		}
		start += angle
		cursor += angle
	}
	hole := radius * pieHoleRatio
	d.Canvas.Rounded(gfx.Rect{
		X: pie.X + center - hole, Y: pie.Y + center - hole,
		Width: hole * 2, Height: hole * 2,
	}, hole, theme.Card)
	// The hole carries the total only where the hole is wider than the figure:
	// in a short card the number would spill across the ring, and the footer
	// already states the range's total. It fades in behind the sweeping ring;
	// painted at full strength on the first frame it looked like a number
	// floating in an empty card.
	labelFade := max(0, min(1, (progress-0.25)/0.5))
	total := domain.CentsToYuan(m.pie.Total)
	amount := gfx.TextStyle{
		Size: typography.SizeBody, Color: d.Fade(theme.Foreground.WithAlpha(labelFade)), Mono: true,
	}
	if d.Canvas.TextWidthStyle(total, amount) <= hole*2-8 {
		d.Canvas.LabelCenter("合计", gfx.Rect{
			X: pie.X + center - radius/2, Y: pie.Y + center - 18,
			Width: radius, Height: 18,
		}, gfx.TextStyle{
			Size: typography.SizeCaption, Color: d.Fade(theme.MutedForeground.WithAlpha(labelFade)),
		})
		d.Canvas.LabelCenter(total, gfx.Rect{
			X: pie.X + center - radius/2, Y: pie.Y + center,
			Width: radius, Height: 22,
		}, amount)
	}
	if progress >= 1 && m.pieHover >= 0 && m.pieHover < len(m.pieSlices) {
		m.pieTooltip(d)
	}
}

// pieTooltip describes the hovered slice where the pointer is: the slice's
// color, its name, what it adds up to and its share. It is the component's
// tooltip shell, so the pie's tip and the line chart's are the same object.
func (m *Model) pieTooltip(d *ui.DrawContext) {
	slice := m.pieSlices[m.pieHover]
	series := []components.ChartSeries{{
		Key: slice.Key, Label: slice.Label, Color: pieColor(*d.Theme, m.pieHover),
	}}
	rows := []components.ChartTooltipRow{{Series: 0, Value: m.pieSliceValue(m.pieHover)}}
	options := components.ChartTooltipOptions{
		Indicator: components.ChartTooltipDot, Layer: pieTooltipLayer,
	}
	size := components.ChartTooltipSize(d.Canvas, *d.Theme, series, rows, options)
	// Beside the pointer, flipped to the other side of it where the card runs
	// out, so the tip neither covers the ring under the pointer nor leaves the
	// card.
	x, y := m.pieAt.X+pieTooltipGap, m.pieAt.Y+pieTooltipGap
	bounds := inset(d.Rect, 8)
	if x+size.Width > bounds.X+bounds.Width {
		x = m.pieAt.X - size.Width - pieTooltipGap
	}
	if y+size.Height > bounds.Y+bounds.Height {
		y = m.pieAt.Y - size.Height - pieTooltipGap
	}
	x = max(min(x, bounds.X+bounds.Width-size.Width), bounds.X)
	y = max(min(y, bounds.Y+bounds.Height-size.Height), bounds.Y)
	components.PaintChartTooltip(d.Canvas, *d.Theme, gfx.Rect{X: x, Y: y}, series, rows, options)
}

// pieSliceAt names the wedge a point is in, or noSlice over the hole, outside
// the pie and on a slice too thin to name.
func (m *Model) pieSliceAt(x, y float32) int {
	if len(m.pieSlices) == 0 || m.pie.Total <= 0 || m.pieRadius <= 0 {
		return noSlice
	}
	dx, dy := x-m.pieCenter.X, y-m.pieCenter.Y
	distance := float32(math.Hypot(float64(dx), float64(dy)))
	if distance > m.pieRadius || distance < m.pieRadius*pieHoleRatio {
		return noSlice
	}
	// The wedges start at twelve o'clock and sweep clockwise; atan2 measures
	// the same direction, so shifting it by a quarter turn is the offset into
	// the pie, and each slice takes its own span off that.
	angle := math.Atan2(float64(dy), float64(dx)) + math.Pi/2
	for angle < 0 {
		angle += 2 * math.Pi
	}
	for index, slice := range m.pieSlices {
		span := float64(slice.Amount) / float64(m.pie.Total) * 2 * math.Pi
		if angle < span {
			return index
		}
		angle -= span
	}
	return noSlice
}

// pieSliceValue writes one slice the way the table and the tip do: what it adds
// up to, then its share of the pie.
func (m *Model) pieSliceValue(index int) string {
	if index < 0 || index >= len(m.pieSlices) {
		return ""
	}
	return fmt.Sprintf("%s · %.1f%%", domain.CentsToYuan(m.pieSlices[index].Amount), m.sliceShare(index))
}

// sliceShare is a slice's part of the pie, in percent.
func (m *Model) sliceShare(index int) float64 {
	if m.pie.Total <= 0 || index < 0 || index >= len(m.pieSlices) {
		return 0
	}
	return float64(m.pieSlices[index].Amount) / float64(m.pie.Total) * 100
}

// Pointer moves the hover the pointer is over — a line chart's point or a pie's
// slice — and clears it when the pointer leaves the chart, so the chart paints
// (or drops) its tooltip.
func (m *Model) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerMove && event.Kind != gfx.PointerExit {
		return gfx.EventResult{}
	}
	if chart := m.selectedChart(); chart != nil && chart.ChartType == chartPie {
		return m.piePointer(event)
	}
	return m.linePointer(event)
}

// piePointer turns the pointer over the pie into the hovered slice; the tip
// follows the pointer, so a move within a wedge repaints too, and leaving the
// pie drops the tip.
func (m *Model) piePointer(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind == gfx.PointerExit || m.pieHit.IsZero() || event.Target != m.pieHit {
		// Only repaint: the move belongs to whatever the pointer is over now.
		changed := m.pieHover != noSlice
		m.pieHover = noSlice
		return gfx.EventResult{Repaint: changed}
	}
	m.pieAt = gfx.Point{X: event.X, Y: event.Y}
	index := m.pieSliceAt(event.X, event.Y)
	if index == noSlice && m.pieHover == noSlice {
		// Over the hole or the seam: nothing is described there.
		return gfx.EventResult{Handled: true}
	}
	m.pieHover = index
	return gfx.EventResult{Handled: true, Repaint: true}
}

// linePointer is the same for the area chart's point: the component keeps the
// hover, the plot turns the pointer into an index.
func (m *Model) linePointer(event gfx.PointerEvent) gfx.EventResult {
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
