package stock

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/chartanim"
	"transactions/internal/components/datepicker"
	"transactions/internal/components/layout"
	"transactions/internal/components/pagination"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/semantic"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

// statisticsView is the two panes of the 统计 screen: the cards and curve, or
// the per-settlement table.
type statisticsView uint8

const (
	statisticsSummary statisticsView = iota
	statisticsDetail
)

// statisticsRange is how the settlements are narrowed in time.
type statisticsRange uint8

const (
	statisticsRangeAll statisticsRange = iota
	statisticsRangeTime
	statisticsRangeRecent
)

var (
	statisticsViewLabels     = [...]string{"统计", "明细"}
	statisticsViewValues     = [...]statisticsView{statisticsSummary, statisticsDetail}
	statisticsRangeLabels    = [...]string{"全部", "按时间", "最近 N 笔"}
	statisticsRangeValues    = [...]statisticsRange{statisticsRangeAll, statisticsRangeTime, statisticsRangeRecent}
	statisticsStrategyValues = [...]domain.StockStatisticsStrategy{
		domain.StockStatisticsCumulative, domain.StockStatisticsGrouped,
	}
)

const (
	// statisticsGroupSize is the old app's group size: every five settlements.
	statisticsGroupSize = 5
	// statisticsDetailSize is the detail table's page size.
	statisticsDetailSize = 15
	// statisticsChartMinHeight keeps the curve readable when the window is
	// short; otherwise the card takes all the height the summary leaves.
	statisticsChartMinHeight = 220
	// statisticsChartShortHeight is the card's height on a window too short to
	// give the summary its remaining height; the page scrolls then.
	statisticsChartShortHeight = 300
	// statisticsRecentWidth is the 最近 N 笔 field's width.
	statisticsRecentWidth = 90
	// statisticsTagWidth is the tag Select's width.
	statisticsTagWidth = 150
	// statisticsStrategyWidth is the 统计口径 Select's width.
	statisticsStrategyWidth = 130
)

// statisticsPanel is the 统计 screen: eight settlement cards over a switchable
// seven-metric curve, and the newest-first table of every settlement.
type statisticsPanel struct {
	accounts *accountStore
	repo     repository.StockRepository

	// The section wires these: an account switch asks every tab to reload, and
	// a stock name in the detail table opens its round on the 记录 screen.
	onChange    func()
	toaster     *ui.Toaster
	onOpenRound func(code, roundID string)

	view statisticsView

	rangeMode  statisticsRange
	rangePick  datepicker.State
	startDate  string
	endDate    string
	recentText string
	recent     int

	strategy   domain.StockStatisticsStrategy
	tag        string
	tagOptions []string

	metric domain.StockMetric
	page   int

	stats        domain.StockStatistics
	errorMessage string
	bounds       gfx.Rect

	// The curve is Viva's area chart. The state keeps the hovered point, the
	// plot turns the pointer into a point, and the custom tooltip adds the stock
	// name the old chart's tip carried.
	chartState   viva.ChartState
	chartPlot    viva.ChartPlot
	chartHit     gfx.ID
	chartAt      gfx.Point
	chartVersion uint32
	chartAnim    chartanim.State
}

var _ panel = (*statisticsPanel)(nil)

// newStatisticsPanel returns the screen before it has a workspace. The account
// store is shared with every other stock tab.
func newStatisticsPanel(accounts *accountStore) *statisticsPanel {
	return &statisticsPanel{
		accounts:   accounts,
		view:       statisticsSummary,
		rangeMode:  statisticsRangeAll,
		rangePick:  datepicker.NewState(),
		recentText: "5",
		recent:     5,
		strategy:   domain.StockStatisticsCumulative,
		metric:     domain.MetricTotalPnl,
		page:       1,
	}
}

// setRepository connects the screen to the workspace, the way the account
// screen does.
func (p *statisticsPanel) setRepository(repo repository.StockRepository) {
	p.repo = repo
	p.Reload()
}

func (p *statisticsPanel) active() string {
	if p.accounts == nil {
		return ""
	}
	return p.accounts.activeID()
}

// Reload rereads the account's tag list and its settlement statistics for the
// current filters. It also drops a hover that no longer names a point.
func (p *statisticsPanel) Reload() {
	p.errorMessage = ""
	p.chartState.Clear()
	p.chartHit = gfx.ID{}
	p.chartAt = gfx.Point{}
	p.chartVersion++
	if p.repo == nil || p.active() == "" {
		p.stats = domain.StockStatistics{}
		p.tagOptions = nil
		return
	}
	ctx := context.Background()
	tags, err := p.repo.StockTradeTags(ctx, p.active())
	if err != nil {
		// A missing tag list only narrows the filter; the statistics still load.
		tags = nil
	}
	p.tagOptions = tags
	if p.tag != "" && !containsString(tags, p.tag) {
		p.tag = ""
	}
	stats, err := p.repo.StockStatistics(ctx, p.query())
	if err != nil {
		p.errorMessage = "读取统计失败：" + err.Error()
		return
	}
	p.stats = stats
}

// query is the request the filters add up to. The repository takes whole
// months, so a date range submits only its YYYY-MM halves; an incomplete range
// submits none and reads as 全部.
func (p *statisticsPanel) query() domain.StockStatisticsQuery {
	query := domain.StockStatisticsQuery{
		AccountID: p.active(),
		Tag:       p.tag,
		Strategy:  p.strategy,
		GroupSize: statisticsGroupSize,
	}
	switch p.rangeMode {
	case statisticsRangeTime:
		start, end := monthOf(p.startDate), monthOf(p.endDate)
		if start != "" && end != "" {
			if start > end {
				start, end = end, start
			}
			query.StartMonth, query.EndMonth = start, end
		}
	case statisticsRangeRecent:
		if p.recent > 0 {
			recent := p.recent
			query.Recent = &recent
		}
	}
	return query
}

// monthOf keeps the YYYY-MM half of a date, and treats anything shorter as "not
// picked".
func monthOf(date string) string {
	if len(date) < 7 {
		return ""
	}
	return date[:7]
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func (p *statisticsPanel) filterChanged() {
	p.page = 1
	p.Reload()
}

// CloseOverlays closes the month range's floating panel.
func (p *statisticsPanel) CloseOverlays() { p.rangePick.Open = false }

// View declares the screen: the shared account picker and filters in the
// toolbar, then the summary or the detail table.
func (p *statisticsPanel) View(c ui.C) {
	p.bounds = c.Bounds()
	if p.repo == nil || p.active() == "" {
		emptyAccountPage(c, "还没有股票账户", "工作空间里至少会有一个股票账户。")
		return
	}
	layout.Page(c, p.toolbar, func(c ui.C) {
		if p.errorMessage != "" {
			typography.CaptionParagraph(c, p.errorMessage, 2).Color(c.Theme().Danger).
				PadX(layout.Inset).PadY(6)
		}
		if len(p.stats.Points) == 0 {
			p.empty(c)
			return
		}
		if p.view == statisticsDetail {
			p.detail(c)
			return
		}
		p.summary(c)
	})
}

func (p *statisticsPanel) toolbar(c ui.C) {
	contents := func() {
		// The account and the 统计 / 明细 pane stay together: a wrapped toolbar
		// must not leave the divider alone at the end of a line.
		ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
			if p.accounts != nil && p.accounts.picker(c) {
				if p.onChange != nil {
					p.onChange()
				} else {
					p.filterChanged()
				}
			}
			ui.Divider(c).Height(24)
			if ui.Choice(c, &p.view, statisticsViewValues[:], func(value statisticsView) string {
				return statisticsViewLabels[value]
			}).Changed() {
				p.chartState.Clear()
				p.chartAt = gfx.Point{}
			}
		})
		// The account and pane stay left; the settlement filters sit at the
		// right edge of the toolbar.
		ui.Spacer(c)
		if ui.Choice(c, &p.rangeMode, statisticsRangeValues[:], func(value statisticsRange) string {
			return statisticsRangeLabels[value]
		}).Changed() {
			p.filterChanged()
		}
		switch p.rangeMode {
		case statisticsRangeTime:
			datepicker.Picker(c, &p.rangePick, datepicker.Spec{
				Label: "统计月份范围", Start: p.startDate, End: p.endDate,
				Grain: datepicker.ByMonth, Periods: true, Width: 210,
				Placeholder: "全部月份", Viewport: p.bounds,
				OnRange: func(start, end string) {
					p.startDate, p.endDate = start, end
					p.filterChanged()
				},
			})
		case statisticsRangeRecent:
			if ui.TextField(c, &p.recentText, "最近 N 笔", "N").
				Width(statisticsRecentWidth).Mono().Submitted() {
				p.applyRecent()
			}
		}
		// The detail table is always per settlement: the grouping only changes
		// the curve, so the control is only shown where it does something.
		if p.view == statisticsSummary {
			strategy := p.strategy
			if selectbox.Select(c, &strategy, statisticsStrategyValues[:], func(value domain.StockStatisticsStrategy) string {
				if value == domain.StockStatisticsGrouped {
					return "分组统计"
				}
				return "逐笔累计"
			}).Label("统计口径").Width(statisticsStrategyWidth).Changed() {
				p.strategy = strategy
				p.filterChanged()
			}
		}
		tag := p.tag
		choices := append([]string{""}, p.tagOptions...)
		if selectbox.Select(c, &tag, choices, func(value string) string {
			if value == "" {
				return "全部标签"
			}
			return value
		}).Label("标签").Width(statisticsTagWidth).Changed() {
			p.tag = tag
			p.filterChanged()
		}
	}
	if c.Bounds().Width >= 1100 {
		// One line: a Row centers the select boxes and the segmented choices
		// on the toolbar's mid-line instead of leaving them at the Flow's top.
		layout.Toolbar(c, contents)
		return
	}
	layout.FlowToolbar(c, contents)
}

// applyRecent is the field's Enter: only a positive whole number becomes the
// new limit; an empty field means no limit, as it did in the old page.
func (p *statisticsPanel) applyRecent() {
	text := strings.TrimSpace(p.recentText)
	if text == "" {
		p.recent = 0
		p.filterChanged()
		return
	}
	recent, err := strconv.Atoi(text)
	if err != nil || recent <= 0 {
		p.errorMessage = "最近 N 笔需要正整数"
		return
	}
	p.recent = recent
	p.filterChanged()
}

// empty is the one empty state both panes share; it fills the room below the
// toolbar so the icon and sentence are centered in the page.
func (p *statisticsPanel) empty(c ui.C) {
	ui.Slot(c).Grow(1).FillWidth().Draw(func(d *ui.DrawContext) {
		viva.Empty(d.Canvas, *d.Theme, d.Rect, viva.EmptyData{
			Media: viva.EmptyMediaIcon,
			Icon:  icons.ChartCandlestick,
			Title: "还没有已结算的轮次",
		})
	})
}

// summary is the cards and the curve, in a page that scrolls when the window
// is shorter than the content.
func (p *statisticsPanel) summary(c ui.C) {
	fill := c.Bounds().Height >= 800
	content := func() {
		typography.Section(c, "结算统计")
		p.cards(c)
		p.chartSection(c, fill)
	}
	if !fill {
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().PadX(layout.Inset).PadY(layout.BlockGap).
				Gap(layout.GroupGap).Children(content)
		})
		return
	}
	// The chart fills what the cards leave, so a tall window has no blank
	// strip under the curve.
	ui.Column(c).Grow(1).FillWidth().PadX(layout.Inset).PadY(layout.BlockGap).
		Gap(layout.GroupGap).Children(content)
}

// chartPoints are the points the curve reads: the grouped points in 分组统计,
// the per-settlement points in 逐笔累计.
func (p *statisticsPanel) chartPoints() []domain.StockStatisticsPoint {
	if p.strategy == domain.StockStatisticsGrouped && len(p.stats.Groups) > 0 {
		return p.stats.Groups
	}
	return p.stats.Points
}

// latest is the last point of the current pane's series. The 分组统计 cards read
// the latest group; until its first group arrives they keep the cumulative
// point, so the cards never blank out on a switch.
func (p *statisticsPanel) latest() (domain.StockStatisticsPoint, bool) {
	points := p.chartPoints()
	if len(points) == 0 {
		return domain.StockStatisticsPoint{}, false
	}
	return points[len(points)-1], true
}

// cards are the eight readings. All of them read the same point: the last one
// of the selected series, so a range or strategy switch changes all eight
// together.
func (p *statisticsPanel) cards(c ui.C) {
	latest, ok := p.latest()
	if !ok {
		return
	}
	theme := c.Theme()
	pnlLabel := "已实现盈亏"
	if p.strategy == domain.StockStatisticsGrouped {
		pnlLabel = "分组盈亏"
	}
	kelly := domain.KellyPercent(latest.Span, latest.WinCount, latest.PnlRatio)
	avgWinColor := semantic.Expense(*theme)
	avgLossColor := semantic.Income(*theme)
	cards := []struct {
		label, hint, value, caption string
		size                        float32
		color                       *gfx.Color
	}{
		{pnlLabel, "", money(latest.TotalPnl), statScope(p.strategy, latest.Span),
			typography.SizeTitle, statColor(*theme, latest.TotalPnl)},
		{"凯利值", "按胜率与盈亏比算出的下注比例", kellyText(kelly), kellyCaption(kelly),
			typography.SizeSection, nil},
		{"平均盈利", domain.MetricHint(domain.MetricAvgWin), money(latest.AvgWin),
			strconv.Itoa(latest.WinCount) + " 笔盈利", typography.SizeSection, &avgWinColor},
		{"平均亏损", domain.MetricHint(domain.MetricAvgLoss), money(latest.AvgLoss),
			strconv.Itoa(latest.LossCount) + " 笔亏损", typography.SizeSection, &avgLossColor},
		{"胜率", domain.MetricHint(domain.MetricWinRate), statPercent(latest.WinRate),
			winLossCaption(latest), typography.SizeSection, nil},
		{"盈亏比", domain.MetricHint(domain.MetricPnlRatio), ratioText(latest.PnlRatio),
			"", typography.SizeSection, nil},
		{"期望值", domain.MetricHint(domain.MetricExpectancy), money(latest.Expectancy),
			"", typography.SizeSection, statColor(*theme, latest.Expectancy)},
		{"最大回撤", domain.MetricHint(domain.MetricMaxDrawdown),
			statPercent(latest.MaxDrawdownPct), "从高点回落 " + money(latest.MaxDrawdown),
			typography.SizeSection, nil},
	}
	metricRows(c, len(cards), func(c ui.C, index int) {
		card := cards[index]
		p.card(c, card.label, card.hint, card.value, card.caption, card.size, card.color)
	})
}

// card is one reading: a label with its one-line hint, a mono figure and an
// optional caption. The width is fixed so the Flow can fit four in a row. The
// hint hangs on the card's own pointer region (not on the label): a hint that
// needs no click must not add eight tab stops to the page.
func (p *statisticsPanel) card(c ui.C, label, hint, value, caption string, size float32, color *gfx.Color) {
	theme := c.Theme()
	var id gfx.ID
	if hint != "" {
		id = ui.ID(c)
	}
	column := ui.Column(c).Grow(1).Basis(0).Gap(4)
	if hint != "" {
		column = column.HitWith(id, label, gfx.HitOptions{SkipFocus: true}).
			Tooltip(hint)
	}
	column.Children(func() {
		typography.Caption(c, label)
		figure := ui.Text(c, value).FontSize(size).Bold().Mono().Ellipsis()
		if color != nil {
			figure.Color(*color)
		}
		if caption != "" {
			ui.Text(c, caption).FontSize(typography.SizeCaption).
				Color(theme.MutedForeground).MaxLines(2)
		}
	})
}

// chartSection is the 统计曲线 block: the metric switch above Viva's area chart.
func (p *statisticsPanel) chartSection(c ui.C, fill bool) {
	points := p.chartPoints()
	if len(points) == 0 {
		return
	}
	theme := c.Theme()
	section := ui.Column(c).Gap(layout.BlockGap)
	if fill {
		section = section.Grow(1)
	}
	section.Children(func() {
		ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
			typography.Section(c, "统计曲线")
			ui.Spacer(c)
			if ui.Choice(c, &p.metric, domain.StockMetrics[:], func(metric domain.StockMetric) string {
				return domain.MetricLabelIn(metric, p.strategy)
			}).Changed() {
				p.chartState.Clear()
				p.chartAt = gfx.Point{}
				p.chartVersion++
			}
		})
		p.chartHit = ui.ID(c)
		card := ui.Row(c).FillWidth()
		if fill {
			// The card takes the height the summary leaves and the whole width;
			// the plot stretches inside it with its own 12 px inset. A time
			// series reads better wide than locked to 16:9, so only a minimum
			// height is fixed, not an aspect ratio.
			card = card.Grow(1).MinHeight(statisticsChartMinHeight)
		} else {
			card = card.Height(statisticsChartShortHeight)
		}
		card.Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Background(theme.Card).Children(func() {
			ui.Slot(c).Fill().Draw(p.drawChart)
		})
	})
}

// drawChart paints the selected metric through Viva's area chart and then the
// hover mark and tooltip that carry the stock name.
func (p *statisticsPanel) drawChart(d *ui.DrawContext) {
	points := p.chartPoints()
	if len(points) == 0 {
		d.Canvas.LabelCenter("暂无数据", statInset(d.Rect, 18),
			gfx.TextStyle{Size: typography.SizeBody, Color: d.Theme.MutedForeground})
		return
	}
	area := statInset(d.Rect, 12)
	progress := p.chartAnim.Progress(d.Canvas,
		"stats:"+strconv.FormatUint(uint64(p.chartVersion), 10), chartanim.Duration)
	p.chartPlot = viva.PaintAreaChart(d.Canvas, *d.Theme, area,
		p.chartHit, p.areaChart(*d.Theme), &p.chartState)
	if progress < 1 {
		// Cover the un-revealed part of the plot instead of clipping the whole
		// chart: the axis labels and the grid's frame stay whole while the
		// data unfolds left to right.
		plot := p.chartPlot.Plot
		d.Canvas.Fill(gfx.Rect{
			X: plot.X + plot.Width*progress, Y: plot.Y,
			Width: plot.Width * (1 - progress), Height: plot.Height,
		}, d.Theme.Card)
	}
	p.drawHover(d)
}

// areaChart is the selected metric as one series, one dot per settlement, and
// the axis formatting the metric's unit asks for.
func (p *statisticsPanel) areaChart(theme gfx.Theme) viva.AreaChart {
	points := p.chartPoints()
	values := make([]float32, len(points))
	for index := range points {
		values[index] = float32(statChartValue(p.metric, points[index]))
	}
	label := domain.MetricLabelIn(p.metric, p.strategy)
	axis := viva.ChartAxis{Format: func(dst []byte, value float32) []byte {
		return statAppendYAxis(dst, p.metric, value)
	}}
	if bounds := metricYBounds(p.metric, points); bounds != nil {
		axis.Fixed = true
		axis.Min, axis.Max = float32(bounds[0]), float32(bounds[1])
	}
	return viva.AreaChart{
		Series: []viva.ChartSeries{{
			Key: label, Label: label, Color: p.chartColor(theme),
		}},
		Values: [][]float32{values},
		Curve:  viva.ChartCurveLinear,
		Fill:   viva.ChartFillGradient,
		Dots:   len(points) <= 80,
		XAxis: viva.ChartAxis{Format: func(dst []byte, value float32) []byte {
			return p.appendXAxis(dst, value)
		}},
		YAxis:       axis,
		Grid:        viva.ChartGridHorizontal,
		Legend:      viva.ChartLegendNone,
		HideTooltip: true,
		Label:       label + "统计曲线",
		Version:     p.chartVersion,
	}
}

func (p *statisticsPanel) appendXAxis(dst []byte, value float32) []byte {
	points := p.chartPoints()
	index := int(math.Round(float64(value)))
	if index < 0 || index >= len(points) {
		return dst
	}
	return append(dst, domain.PointSpanLabel(p.strategy,
		points[index].Sequence, points[index].Span)...)
}

// chartColor uses the primary accent for readings that have no red/green
// meaning: cumulative PnL can be negative without becoming an "error", and a
// win rate, PnL ratio or expectancy is just a number. The three genuinely
// directional curves keep A-share semantics.
func (p *statisticsPanel) chartColor(theme gfx.Theme) gfx.Color {
	switch p.metric {
	case domain.MetricTotalPnl, domain.MetricWinRate, domain.MetricPnlRatio, domain.MetricExpectancy:
		return theme.Primary
	case domain.MetricAvgWin:
		return semantic.Expense(theme)
	case domain.MetricAvgLoss, domain.MetricMaxDrawdown:
		return semantic.Income(theme)
	}
	return theme.Primary
}

// drawHover paints the crosshair, the point and the tooltip of the hovered
// settlement. The component's own tooltip is off because the old tip also named
// the stock, which the axis label has no room for.
func (p *statisticsPanel) drawHover(d *ui.DrawContext) {
	points := p.chartPoints()
	index, active := p.chartState.Index()
	if !active || index < 0 || index >= len(points) || p.chartPlot.Count() == 0 {
		return
	}
	theme := *d.Theme
	plot := p.chartPlot.Plot
	x := p.chartPlot.PointX(index)
	if x < plot.X || x > plot.X+plot.Width {
		return
	}
	d.Canvas.Fill(gfx.Rect{X: x - 0.5, Y: plot.Y, Width: 1, Height: plot.Height}, theme.Border)
	value := float32(statChartValue(p.metric, points[index]))
	if y := p.chartPlot.ValueY(value); y >= plot.Y && y <= plot.Y+plot.Height {
		d.Canvas.Rounded(gfx.Rect{X: x - 6, Y: y - 6, Width: 12, Height: 12}, 6, theme.Background)
		d.Canvas.Rounded(gfx.Rect{X: x - 4, Y: y - 4, Width: 8, Height: 8}, 4, p.chartColor(theme))
	}
	p.drawTooltip(d, points[index], x)
}

// drawTooltip places the component's tooltip shell beside the pointer, flipped
// to the other side where the card runs out.
func (p *statisticsPanel) drawTooltip(d *ui.DrawContext, point domain.StockStatisticsPoint, pointX float32) {
	theme := *d.Theme
	label := domain.MetricLabelIn(p.metric, p.strategy)
	series := []viva.ChartSeries{{Key: label, Label: label, Color: p.chartColor(theme)}}
	rows := []viva.ChartTooltipRow{{
		Series: 0,
		Value:  statPointValue(p.metric, point),
	}}
	options := viva.ChartTooltipOptions{
		Indicator: viva.ChartTooltipDot,
		Layer:     4,
		Label:     statTooltipTitle(p.strategy, point),
	}
	size := viva.ChartTooltipSize(d.Canvas, theme, series, rows, options)
	anchorX, anchorY := p.chartAt.X, p.chartAt.Y
	if anchorX == 0 && anchorY == 0 {
		anchorX, anchorY = pointX, p.chartPlot.Plot.Y+16
	}
	gap := float32(12)
	x, y := anchorX+gap, anchorY+gap
	bounds := statInset(d.Rect, 8)
	if x+size.Width > bounds.X+bounds.Width {
		x = anchorX - size.Width - gap
	}
	if y+size.Height > bounds.Y+bounds.Height {
		y = anchorY - size.Height - gap
	}
	x = max(min(x, bounds.X+bounds.Width-size.Width), bounds.X)
	y = max(min(y, bounds.Y+bounds.Height-size.Height), bounds.Y)
	viva.PaintChartTooltip(d.Canvas, theme, gfx.Rect{X: x, Y: y}, series, rows, options)
}

// Pointer turns the pointer over the curve's plot into its settlement, the way
// the analysis page does. The tooltip follows the pointer, so a move within
// one point repaints too.
func (p *statisticsPanel) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if p.view != statisticsSummary || p.repo == nil {
		return gfx.EventResult{}
	}
	if event.Kind != gfx.PointerMove && event.Kind != gfx.PointerExit {
		return gfx.EventResult{}
	}
	previous, active := p.chartState.Index()
	if event.Kind == gfx.PointerExit || p.chartHit.IsZero() || event.Target != p.chartHit {
		p.chartState.Clear()
		p.chartAt = gfx.Point{}
		return gfx.EventResult{Repaint: active}
	}
	index := p.chartPlot.IndexAt(event.X)
	if index == previous && p.chartAt.X == event.X && p.chartAt.Y == event.Y {
		return gfx.EventResult{Handled: true}
	}
	p.chartAt = gfx.Point{X: event.X, Y: event.Y}
	p.chartState.Hover(index)
	return gfx.EventResult{Handled: true, Repaint: true}
}

// ---------------- 明细 ----------------

var detailWidths = [...]float32{78, 78, 112, 52, 82, 82, 80, 52, 78, 78, 50, 82, 86}

const (
	detailOpened = iota
	detailClosed
	detailStock
	detailTag
	detailPnl
	detailTotal
	detailWinLoss
	detailWinRate
	detailAvgWin
	detailAvgLoss
	detailRatio
	detailExpectancy
	detailDrawdown
	detailColumnCount
)

const (
	detailGap  float32 = 4
	detailPadX float32 = 16
)

// detailTableWidth is the table's minimum width: wider than the 960 px
// window's content column, so the table scrolls sideways instead of squeezing
// numbers, while a wider window stretches the stock column to fill it.
func detailTableWidth() float32 {
	total := detailPadX * 2
	for _, width := range detailWidths {
		total += width
	}
	return total + detailGap*(detailColumnCount-1)
}

// detailTableSize fits the table to the available width. All columns grow by
// the same factor, so the extra room is shared instead of collecting in one
// wide column; below the minimum the table keeps its width and scrolls
// sideways.
func detailTableSize(available float32) (table float32, widths [detailColumnCount]float32) {
	table = detailTableWidth()
	if available > table {
		table = available
	}
	fixed := detailPadX*2 + detailGap*(detailColumnCount-1)
	total := table - fixed
	base := detailTableWidth() - fixed
	scale := total / base
	for index, width := range detailWidths {
		widths[index] = width * scale
	}
	return table, widths
}

// detail is the newest-first table of every settlement, 15 rows a page.
func (p *statisticsPanel) detail(c ui.C) {
	rows := domain.DetailRowsNewestFirst(p.stats.Points)
	pages := max((len(rows)+statisticsDetailSize-1)/statisticsDetailSize, 1)
	page := min(max(p.page, 1), pages)
	start := (page - 1) * statisticsDetailSize
	end := min(start+statisticsDetailSize, len(rows))
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		// The table is its own scroll area: rows scroll vertically, the 13
		// columns scroll sideways at the minimum window width, and the pagination
		// stays on the page's bottom edge. The viewport width, not the window,
		// decides how far the stock column stretches, so the last column ends at
		// the scroll edge instead of under the page padding.
		area := ui.Scroll(c).Grow(1).Sideways().Scrollbars()
		tableWidth, widths := detailTableSize(area.ScrollState().ViewportWidth)
		area.Children(func() {
			ui.Column(c).Width(tableWidth).Children(func() {
				p.detailHeader(c, tableWidth, widths)
				for index := start; index < end; index++ {
					p.detailRow(c, rows[index], tableWidth, widths)
				}
			})
		})
		ui.Divider(c)
		pagination.Bar(c, pagination.Spec{
			Total: len(rows), Page: page, PageSize: statisticsDetailSize,
			PageSizes: []int{statisticsDetailSize},
			OnPage: func(next int) {
				p.page = next
			},
		})
	})
}

func (p *statisticsPanel) detailHeader(c ui.C, tableWidth float32, widths [detailColumnCount]float32) {
	headers := [...]string{
		"建仓", "清仓", "股票", "标签", "盈亏", "累计", "胜负", "胜率",
		"平均盈利", "平均亏损", "盈亏比", "期望值", "回撤",
	}
	layout.TableHeader(c, layout.TableHeaderSpec{Width: tableWidth, PadX: detailPadX, Gap: detailGap}, func(c ui.C) {
		for index, header := range headers {
			cell := typography.Caption(c, header).Width(widths[index]).Ellipsis()
			switch index {
			case detailOpened, detailClosed, detailTag:
				cell.TextCenter()
			default:
				cell.TextEnd()
			}
		}
	})
}

func (p *statisticsPanel) detailRow(c ui.C, point domain.StockStatisticsPoint, tableWidth float32, widths [detailColumnCount]float32) {
	theme := c.Theme()
	row := ui.Row(c.KeyInt(point.Sequence)).Height(layout.RowHeight).
		Width(tableWidth).PadX(detailPadX).Gap(detailGap).Items(ui.Center)
	row.Draw(func(d *ui.DrawContext) {
		width := d.Rect.Width - 2*detailPadX
		if width <= 0 {
			return
		}
		d.Canvas.Fill(gfx.Rect{
			X: d.Rect.X + detailPadX, Y: d.Rect.Bottom() - 1,
			Width: width, Height: 1,
		}, d.Theme.Border)
	})
	row.Children(func() {
		detailDate(c, point.OpenedAt, widths[detailOpened])
		detailDate(c, point.ClosedAt, widths[detailClosed])
		p.detailStock(c, point, widths[detailStock])
		tag := point.Tag
		if tag == "" {
			tag = "—"
		}
		ui.Text(c, tag).FontSize(typography.SizeBody).Muted().
			Width(widths[detailTag]).TextCenter().Ellipsis()
		detailMoney(c, point.Pnl, widths[detailPnl], statColor(*theme, point.Pnl))
		detailMoney(c, point.TotalPnl, widths[detailTotal], nil)
		ui.Text(c, winLossText(point)).FontSize(typography.SizeCaption).
			Width(widths[detailWinLoss]).TextEnd().Ellipsis()
		ui.Text(c, statPercent(point.WinRate)).FontSize(typography.SizeBody).Mono().
			Width(widths[detailWinRate]).TextEnd()
		if point.AvgWin > 0 {
			color := semantic.Expense(*theme)
			detailMoney(c, point.AvgWin, widths[detailAvgWin], &color)
		} else {
			detailDash(c, widths[detailAvgWin])
		}
		if point.AvgLoss > 0 {
			color := semantic.Income(*theme)
			detailMoneyText(c, signedMoney(-point.AvgLoss), widths[detailAvgLoss], &color)
		} else {
			detailDash(c, widths[detailAvgLoss])
		}
		ui.Text(c, ratioText(point.PnlRatio)).FontSize(typography.SizeBody).Mono().
			Width(widths[detailRatio]).TextEnd()
		detailMoney(c, point.Expectancy, widths[detailExpectancy],
			statColor(*theme, point.Expectancy))
		ui.Column(c).Width(widths[detailDrawdown]).Gap(0).Items(ui.End).
			Children(func() {
				ui.Text(c, statPercent(point.MaxDrawdownPct)).FontSize(typography.SizeCaption).
					Mono().Color(semantic.Income(*theme))
				ui.Text(c, money(point.MaxDrawdown)).FontSize(typography.SizeCaption).
					Mono().Color(semantic.Income(*theme))
			})
	})
}

// detailStock is the one cell that does something: it opens the round on the
// 记录 screen when the section has wired that jump.
func (p *statisticsPanel) detailStock(c ui.C, point domain.StockStatisticsPoint, width float32) {
	label := strings.TrimSpace(point.StockName + " " + point.StockCode)
	clickable := point.RoundID != "" && p.onOpenRound != nil
	var id gfx.ID
	if clickable {
		// ui.ID names the next element, so it is asked for before the text.
		id = ui.ID(c)
	}
	cell := ui.Text(c, label).FontSize(typography.SizeBody).
		Width(width).TextEnd().Ellipsis()
	if !clickable {
		return
	}
	cell.Color(c.Theme().Primary)
	cell.HitWith(id, "查看这一轮交易", gfx.HitOptions{Cursor: gfx.PointerCursorHand})
	if cell.Clicked() {
		code, roundID := point.StockCode, point.RoundID
		c.After(func() { p.onOpenRound(code, roundID) })
	}
}

// ---------------- formatting ----------------

func detailDate(c ui.C, at int64, width float32) {
	ui.Text(c, statDate(at)).FontSize(typography.SizeCaption).Mono().
		Width(width).TextCenter()
}

func detailMoney(c ui.C, cents int64, width float32, color *gfx.Color) {
	detailMoneyText(c, money(cents), width, color)
}

func detailMoneyText(c ui.C, text string, width float32, color *gfx.Color) {
	cell := ui.Text(c, text).FontSize(typography.SizeBody).Mono().
		Width(width).TextEnd()
	if color != nil {
		cell.Color(*color)
	}
}

func detailDash(c ui.C, width float32) {
	ui.Text(c, "—").FontSize(typography.SizeBody).Mono().Width(width).TextEnd()
}

// statDate is a settlement time as the app's local date.
func statDate(at int64) string {
	if at <= 0 {
		return "—"
	}
	return time.Unix(at, 0).Format("2006-01-02")
}

// statShortDate is the tooltip's month and day.
func statShortDate(at int64) string {
	if at <= 0 {
		return ""
	}
	return time.Unix(at, 0).Format("01-02")
}

func statTooltipTitle(strategy domain.StockStatisticsStrategy, point domain.StockStatisticsPoint) string {
	title := domain.PointSpanLabel(strategy, point.Sequence, point.Span)
	if date := statShortDate(point.ClosedAt); date != "" {
		title += " " + date
	}
	if name := strings.TrimSpace(point.StockName + " " + point.StockCode); name != "" {
		title += " · " + name
	}
	return title
}

// statPercent writes a percentage with the old table's two decimals.
func statPercent(value float64) string {
	if math.Abs(value) < 0.005 {
		value = 0
	}
	return strconv.FormatFloat(value, 'f', 2, 64) + "%"
}

func ratioText(value *float64) string {
	if value == nil {
		return "—"
	}
	return strconv.FormatFloat(*value, 'f', 2, 64)
}

// kellyText rounds to the same two decimals as the win rate and folds negative
// zero into zero, so a nil sample is the only dash.
func kellyText(value *float64) string {
	if value == nil {
		return "—"
	}
	rounded := math.Round(*value*100) / 100
	return statPercent(rounded)
}

func kellyCaption(value *float64) string {
	if value == nil {
		return "样本不足"
	}
	caption := "没有正期望"
	if *value > 0 {
		caption = "建议仓位"
	}
	return caption + " · 半凯利 " + kellyText(ptrOf(*value/2))
}

func ptrOf(value float64) *float64 { return &value }

func statScope(strategy domain.StockStatisticsStrategy, span int) string {
	if strategy == domain.StockStatisticsGrouped {
		return "最近一组 " + strconv.Itoa(span) + " 笔"
	}
	return "共 " + strconv.Itoa(span) + " 笔"
}

func winLossCaption(point domain.StockStatisticsPoint) string {
	caption := strconv.Itoa(point.WinCount) + " 胜 · " +
		strconv.Itoa(point.LossCount) + " 负"
	if point.TieCount > 0 {
		caption += " · " + strconv.Itoa(point.TieCount) + " 平"
	}
	return caption
}

// winLossText is the detail table's spelling: the old table separated the
// counts with spaces, not the summary's middle dots.
func winLossText(point domain.StockStatisticsPoint) string {
	text := strconv.Itoa(point.WinCount) + " 胜 " + strconv.Itoa(point.LossCount) + " 负"
	if point.TieCount > 0 {
		text += " " + strconv.Itoa(point.TieCount) + " 平"
	}
	return text
}

func statColor(theme gfx.Theme, value int64) *gfx.Color {
	color := pnlColor(theme, value)
	return &color
}

// statChartValue is a metric in the curve's unit: money is drawn in yuan (the
// points hold cents), everything else keeps its own unit.
func statChartValue(metric domain.StockMetric, point domain.StockStatisticsPoint) float64 {
	value := domain.MetricValue(metric, point)
	if domain.MetricKind(metric) == domain.ChartValueMoney {
		return value / 100
	}
	return value
}

func metricYBounds(metric domain.StockMetric, points []domain.StockStatisticsPoint) *[2]float64 {
	values := make([]float64, len(points))
	for index := range points {
		values[index] = statChartValue(metric, points[index])
	}
	return domain.MetricYBounds(metric, values)
}

// statPointValue is the tooltip's value line, in the metric's own format. Money
// keeps the old chart tip's sign in front of the symbol (-¥1.00).
func statPointValue(metric domain.StockMetric, point domain.StockStatisticsPoint) string {
	switch domain.MetricKind(metric) {
	case domain.ChartValuePercent:
		return statPercent(domain.MetricValue(metric, point))
	case domain.ChartValueRatio:
		return ratioText(point.PnlRatio)
	}
	cents := int64(domain.MetricValue(metric, point))
	if cents < 0 {
		return "-¥" + domain.CentsToYuan(-cents)
	}
	return "¥" + domain.CentsToYuan(cents)
}

// statAppendYAxis writes the value axis without a currency sign or grouping, as
// the old chart did: whole numbers, percent as a percent, and a drawdown with
// its meaningful two decimals.
func statAppendYAxis(dst []byte, metric domain.StockMetric, value float32) []byte {
	switch domain.MetricKind(metric) {
	case domain.ChartValuePercent:
		if metric == domain.MetricMaxDrawdown {
			text := strconv.FormatFloat(float64(value), 'f', 2, 64)
			text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
			if text == "-0" {
				text = "0"
			}
			dst = append(dst, text...)
			return append(dst, '%')
		}
		rounded := math.Round(float64(value))
		if rounded == 0 {
			rounded = 0
		}
		dst = strconv.AppendFloat(dst, rounded, 'f', 0, 64)
		return append(dst, '%')
	}
	rounded := math.Round(float64(value))
	if rounded == 0 {
		rounded = 0
	}
	return strconv.AppendFloat(dst, rounded, 'f', 0, 64)
}

// statInset trims a rectangle on every side, the same small inset the analysis
// page gives its chart.
func statInset(rect gfx.Rect, all float32) gfx.Rect {
	rect.X += all
	rect.Y += all
	rect.Width = max(rect.Width-2*all, 0)
	rect.Height = max(rect.Height-2*all, 0)
	return rect
}
