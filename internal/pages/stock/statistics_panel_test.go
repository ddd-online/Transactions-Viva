package stock

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/semantic"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

// statisticsTestRepo answers only what the statistics screen reads; the
// embedded interface stands in for the rest of the stock contract.
type statisticsTestRepo struct {
	repository.StockRepository
	tags  []string
	stats domain.StockStatistics
	last  domain.StockStatisticsQuery
	err   error
}

func (r *statisticsTestRepo) StockTradeTags(context.Context, string) ([]string, error) {
	return r.tags, nil
}

func (r *statisticsTestRepo) StockStatistics(_ context.Context, query domain.StockStatisticsQuery) (domain.StockStatistics, error) {
	r.last = query
	return r.stats, r.err
}

func newStatisticsTestPanel(t *testing.T, repo *statisticsTestRepo) *statisticsPanel {
	t.Helper()
	accounts := newAccountStore()
	accounts.reset([]domain.StockAccount{{ID: "acc", Name: "主账户"}})
	p := newStatisticsPanel(accounts)
	p.setRepository(repo)
	return p
}

// statisticsTestPoints builds a settled history whose cumulative values move.
func statisticsTestPoints(count int) []domain.StockStatisticsPoint {
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local).Unix()
	points := make([]domain.StockStatisticsPoint, 0, count)
	var total, peak, maxDrawdown int64
	wins, losses := 0, 0
	for index := 0; index < count; index++ {
		pnl := int64(12000)
		if index%3 == 1 {
			pnl = -5000
		}
		total += pnl
		if total > peak {
			peak = total
		}
		if drawdown := peak - total; drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
		switch {
		case pnl > 0:
			wins++
		case pnl < 0:
			losses++
		}
		ratio := 2.4
		span := index + 1
		points = append(points, domain.StockStatisticsPoint{
			Sequence: span, Span: span,
			OpenedAt:  base + int64(index)*86400,
			ClosedAt:  base + int64(index+2)*86400,
			RoundID:   fmt.Sprintf("round-%02d", span),
			StockCode: "600519", StockName: "贵州茅台", StockRoundNo: 1,
			Tag: []string{"突破", "趋势"}[index%2],
			Pnl: pnl, TotalPnl: total, TradeCount: span,
			WinCount: wins, LossCount: losses, WinRate: domain.PercentOf(int64(wins), int64(span)),
			AvgWin: 12000, AvgLoss: 5000, PnlRatio: &ratio,
			Expectancy:  int64(domain.RoundHalfAwayFromZero(float64(total) / float64(span))),
			MaxDrawdown: maxDrawdown, MaxDrawdownPct: domain.PercentOf(maxDrawdown, 1000000),
		})
	}
	return points
}

// The 960 × 620 window leaves a stock page 749 × 580 after the shell's 204 px
// navigation, its 7 px margin and the 36 px title bar.
const (
	statisticsPageWidth  = 749
	statisticsPageHeight = 580
)

func newStatisticsDriver(t *testing.T, p *statisticsPanel) *app.Driver {
	t.Helper()
	return newStatisticsDriverSize(t, p, statisticsPageWidth, statisticsPageHeight)
}

func newStatisticsDriverSize(t *testing.T, p *statisticsPanel, width, height int) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: p.View})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: width, Height: height}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func snapshotTexts(driver *app.Driver) []string {
	snapshot := driver.Snapshot()
	values := make([]string, len(snapshot.Texts))
	for index, text := range snapshot.Texts {
		values[index] = text.Value
	}
	return values
}

func hasSnapshotText(driver *app.Driver, want string) bool {
	for _, value := range snapshotTexts(driver) {
		if value == want {
			return true
		}
	}
	return false
}

func snapshotHit(driver *app.Driver, label string) (gfx.Hit, bool) {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}

func TestStatisticsQueryFollowsFilters(t *testing.T) {
	repo := &statisticsTestRepo{tags: []string{"突破", "趋势"}}
	p := newStatisticsTestPanel(t, repo)

	if repo.last.AccountID != "acc" || repo.last.Strategy != domain.StockStatisticsCumulative ||
		repo.last.GroupSize != statisticsGroupSize || repo.last.StartMonth != "" ||
		repo.last.EndMonth != "" || repo.last.Recent != nil {
		t.Fatalf("initial query = %+v", repo.last)
	}

	p.rangeMode = statisticsRangeTime
	p.startDate, p.endDate = "2026-03-04", "2026-01-31"
	query := p.query()
	if query.StartMonth != "2026-01" || query.EndMonth != "2026-03" {
		t.Fatalf("time query months = %q..%q", query.StartMonth, query.EndMonth)
	}
	p.endDate = ""
	if query = p.query(); query.StartMonth != "" || query.EndMonth != "" {
		t.Fatalf("incomplete range = %q..%q", query.StartMonth, query.EndMonth)
	}

	p.rangeMode = statisticsRangeRecent
	p.recent = 8
	query = p.query()
	if query.Recent == nil || *query.Recent != 8 || query.StartMonth != "" {
		t.Fatalf("recent query = %+v", query)
	}

	p.rangeMode = statisticsRangeAll
	p.strategy = domain.StockStatisticsGrouped
	p.tag = "突破"
	query = p.query()
	if query.Strategy != domain.StockStatisticsGrouped || query.Tag != "突破" || query.Recent != nil {
		t.Fatalf("grouped query = %+v", query)
	}
}

func TestStatisticsTagFallsBackWhenTheAccountChanges(t *testing.T) {
	repo := &statisticsTestRepo{tags: []string{"趋势"}}
	p := newStatisticsTestPanel(t, repo)
	p.tag = "突破"
	p.Reload()
	if p.tag != "" {
		t.Fatalf("tag = %q, want it cleared with the account's tag list", p.tag)
	}
	if month := monthOf("2026-11-22"); month != "2026-11" {
		t.Fatalf("monthOf = %q", month)
	}
	if month := monthOf("2026"); month != "" {
		t.Fatalf("monthOf incomplete = %q", month)
	}
}

func TestStatisticsEmptyState(t *testing.T) {
	p := newStatisticsTestPanel(t, &statisticsTestRepo{})
	if !hasSnapshotText(newStatisticsDriver(t, p), "还没有已结算的轮次") {
		t.Fatal("the empty state did not say 还没有已结算的轮次")
	}
}

func TestStatisticsSummaryCardsAndCurve(t *testing.T) {
	repo := &statisticsTestRepo{
		tags: []string{"突破", "趋势"},
		stats: domain.StockStatistics{
			Principal: 1000000, RoundCount: 20,
			Points: statisticsTestPoints(20),
		},
	}
	p := newStatisticsTestPanel(t, repo)
	driver := newStatisticsDriver(t, p)

	for _, want := range []string{
		"结算统计", "已实现盈亏", "凯利值", "平均盈利", "平均亏损",
		"胜率", "盈亏比", "期望值", "最大回撤", "统计曲线", "累计盈亏",
	} {
		if !hasSnapshotText(driver, want) {
			t.Fatalf("summary is missing %q", want)
		}
	}
	if p.chartHit.IsZero() {
		t.Fatal("the chart registered no hit region")
	}
	if p.chartPlot.Count() != len(repo.stats.Points) {
		t.Fatalf("chart points = %d, want %d", p.chartPlot.Count(), len(repo.stats.Points))
	}
	if !driver.Transitioning() {
		t.Fatal("the curve did not start its reveal")
	}
	if err := driver.Advance(500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if driver.Transitioning() {
		t.Fatal("the curve did not settle after its reveal")
	}

	// At the window's minimum width every summary text stays inside the frame.
	snapshot := driver.Snapshot()
	for _, text := range snapshot.Texts {
		if text.Bounds.X < -0.5 || text.Bounds.X+text.Bounds.Width > float32(snapshot.Width)+0.5 {
			t.Fatalf("%q is painted at %v, outside the %v frame", text.Value, text.Bounds, snapshot.Width)
		}
	}
}

func TestStatisticsPointerHoversTheCurve(t *testing.T) {
	repo := &statisticsTestRepo{stats: domain.StockStatistics{Points: statisticsTestPoints(5)}}
	p := newStatisticsTestPanel(t, repo)
	driver := newStatisticsDriver(t, p)
	if p.chartHit.IsZero() || p.chartPlot.Count() < 2 {
		t.Fatalf("chart not painted: hit %v, points %d", p.chartHit, p.chartPlot.Count())
	}
	x := p.chartPlot.PointX(2)
	y := p.chartPlot.Plot.Y + p.chartPlot.Plot.Height/2
	event := gfx.PointerEvent{Kind: gfx.PointerMove, Target: p.chartHit, X: x, Y: y}
	if result := p.Pointer(event); !result.Handled || !result.Repaint {
		t.Fatalf("hover = %+v", result)
	}
	if index, active := p.chartState.Index(); !active || index != 2 {
		t.Fatalf("hovered index = %d, active %v; want 2", index, active)
	}
	driver.Paint()
	if !hasSnapshotText(driver, "第3笔 11-05 · 贵州茅台 600519") {
		t.Fatal("the tooltip did not name the hovered settlement")
	}

	if result := p.Pointer(gfx.PointerEvent{Kind: gfx.PointerExit, X: x, Y: y}); !result.Repaint {
		t.Fatalf("leaving the chart = %+v", result)
	}
	if _, active := p.chartState.Index(); active {
		t.Fatal("the hover survived the pointer leaving the chart")
	}
}

func TestStatisticsSingleTieCurve(t *testing.T) {
	point := domain.StockStatisticsPoint{
		Sequence: 1, Span: 1,
		OpenedAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local).Unix(),
		ClosedAt: time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local).Unix(),
		RoundID:  "round-tie", StockCode: "000001", StockName: "平安银行", TieCount: 1,
	}
	repo := &statisticsTestRepo{stats: domain.StockStatistics{Points: []domain.StockStatisticsPoint{point}}}
	p := newStatisticsTestPanel(t, repo)
	driver := newStatisticsDriver(t, p)
	if p.chartPlot.Count() != 1 {
		t.Fatalf("a single tie paints %d points", p.chartPlot.Count())
	}
	if !hasSnapshotText(driver, "凯利值") || !hasSnapshotText(driver, "样本不足") {
		t.Fatal("the tie's cards did not paint")
	}
}

func TestStatisticsTimeRangeToolbarFits(t *testing.T) {
	repo := &statisticsTestRepo{stats: domain.StockStatistics{Points: statisticsTestPoints(4)}}
	p := newStatisticsTestPanel(t, repo)
	p.rangeMode = statisticsRangeTime
	driver := newStatisticsDriver(t, p)
	if !hasSnapshotText(driver, "全部月份") {
		t.Fatal("the month range field did not show its placeholder")
	}
	snapshot := driver.Snapshot()
	for _, text := range snapshot.Texts {
		if text.Bounds.X < -0.5 || text.Bounds.X+text.Bounds.Width > float32(snapshot.Width)+0.5 {
			t.Fatalf("%q is painted at %v, outside the %v frame", text.Value, text.Bounds, snapshot.Width)
		}
	}
}

func TestStatisticsDetailPagesAndOpensTheRound(t *testing.T) {
	repo := &statisticsTestRepo{stats: domain.StockStatistics{Points: statisticsTestPoints(20)}}
	p := newStatisticsTestPanel(t, repo)
	p.view = statisticsDetail
	var openedCode, openedRound string
	p.onOpenRound = func(code, roundID string) {
		openedCode, openedRound = code, roundID
	}
	driver := newStatisticsDriver(t, p)

	if !hasSnapshotText(driver, "共 20 条") {
		t.Fatal("the detail footer did not count 20 settlements")
	}
	if count := countTexts(driver, "贵州茅台 600519"); count != statisticsDetailSize {
		t.Fatalf("page 1 lists %d rows, want %d", count, statisticsDetailSize)
	}
	if count := countHits(driver, "查看这一轮交易"); count == 0 || count > statisticsDetailSize {
		t.Fatalf("page 1 has %d visible stock targets", count)
	}
	cell, ok := snapshotHit(driver, "查看这一轮交易")
	if !ok {
		t.Fatal("the newest settlement's stock is not clickable")
	}
	if err := driver.Click(cell.Bounds.X+cell.Bounds.Width/2, cell.Bounds.Y+cell.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if openedCode != "600519" || openedRound != "round-20" {
		t.Fatalf("opened %q / %q, want the newest round", openedCode, openedRound)
	}

	p.page = 2
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if count := countTexts(driver, "贵州茅台 600519"); count != 5 {
		t.Fatalf("page 2 lists %d rows, want 5", count)
	}
	if count := countHits(driver, "查看这一轮交易"); count != 5 {
		t.Fatalf("page 2 has %d stock targets, want 5", count)
	}
}

func TestStatisticsDetailRejectsInvalidRecent(t *testing.T) {
	p := newStatisticsTestPanel(t, &statisticsTestRepo{})
	p.recentText = "0"
	p.applyRecent()
	if p.recent != 5 || !strings.Contains(p.errorMessage, "正整数") {
		t.Fatalf("recent = %d, error = %q", p.recent, p.errorMessage)
	}
	p.recentText = "  "
	p.applyRecent()
	if p.recent != 0 {
		t.Fatalf("empty recent = %d, want no limit", p.recent)
	}
}

func countHits(driver *app.Driver, label string) int {
	count := 0
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label {
			count++
		}
	}
	return count
}

func countTexts(driver *app.Driver, value string) int {
	count := 0
	for _, text := range snapshotTexts(driver) {
		if text == value {
			count++
		}
	}
	return count
}

// A negative cumulative PnL is still a normal reading, not an error: its curve,
// and the curves of the three other non-directional metrics, use the primary
// accent. Only the genuinely directional curves keep A-share semantics.
func TestStatisticsChartColor(t *testing.T) {
	theme := ui.LightTheme()
	base := domain.StockStatisticsPoint{TotalPnl: -100}
	for _, metric := range []domain.StockMetric{
		domain.MetricTotalPnl, domain.MetricWinRate, domain.MetricPnlRatio, domain.MetricExpectancy,
	} {
		panel := &statisticsPanel{metric: metric, stats: domain.StockStatistics{Points: []domain.StockStatisticsPoint{base}}}
		if got := panel.chartColor(theme); got != theme.Primary {
			t.Fatalf("%s curve = %v, want primary %v", metric, got, theme.Primary)
		}
	}
	for metric, want := range map[domain.StockMetric]gfx.Color{
		domain.MetricAvgWin:      semantic.Expense(theme),
		domain.MetricAvgLoss:     semantic.Income(theme),
		domain.MetricMaxDrawdown: semantic.Income(theme),
	} {
		panel := &statisticsPanel{metric: metric, stats: domain.StockStatistics{Points: []domain.StockStatisticsPoint{base}}}
		if got := panel.chartColor(theme); got != want {
			t.Fatalf("%s curve = %v, want %v", metric, got, want)
		}
	}
}
