package analysis

import (
	"math"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/pages/accounting/ledger"
	"transactions/internal/repository"
	"transactions/internal/store/storetest"
)

type repositoryPieSlice = repository.ChartPieSlice

func newTestModel(t *testing.T) *Model {
	t.Helper()
	fixture := storetest.Open(t)
	m := New(ledger.NewStore())
	m.ledgers.Reset([]domain.Ledger{fixture.Home, fixture.Travel})
	m.SetDataSource(fixture.Workspace, nil)
	return m
}

func TestPresetsAndLineData(t *testing.T) {
	m := newTestModel(t)
	if len(m.charts) != 2 {
		t.Fatalf("charts = %d, want the two presets", len(m.charts))
	}
	chart := m.selectedChart()
	if chart == nil || !chart.IsPreset || chart.Title != "月度消费趋势" {
		t.Fatalf("selected chart = %+v", chart)
	}
	if len(m.lines) != 3 || len(m.lineData) != 3 {
		t.Fatalf("lines = %d, data = %d, want 3", len(m.lines), len(m.lineData))
	}
	if len(m.lineData[0].Data) != 12 {
		t.Fatalf("monthly buckets = %d, want 12", len(m.lineData[0].Data))
	}
	if m.stats.Expense == 0 {
		t.Fatal("the fixture's expenses did not reach the summary")
	}
}

func TestCreateLineChartAndAddCurve(t *testing.T) {
	m := newTestModel(t)
	m.openCreate()
	m.form.title = "餐饮支出"
	m.form.chartType = "折线图"
	m.form.granularity = granularityYear
	m.createChart()
	if len(m.charts) != 3 || m.selectedChart() == nil {
		t.Fatalf("charts after create = %+v", m.charts)
	}
	chart := m.selectedChart()
	if chart.Title != "餐饮支出" || chart.IsPreset || chart.Granularity != granularityYear {
		t.Fatalf("created chart = %+v", chart)
	}
	m.openLine(chart, -1)
	m.form.lineLabel = "餐饮"
	m.form.lineKind = "支出"
	m.form.lineCategory = "餐饮美食"
	m.form.lineTags = []string{"三餐", "外卖"}
	m.form.linePolicy = "全部"
	m.form.lineDescription = "午餐"
	m.saveLine()
	if len(m.lines) != 2 {
		t.Fatalf("lines after add = %+v", m.lines)
	}
	added := m.lines[1]
	if added.Label != "餐饮" || len(added.Conditions) != 1 || added.Conditions[0].TagPolicy != "all" {
		t.Fatalf("added line = %+v", added)
	}
	if len(added.Conditions[0].Tags) != 2 || added.Conditions[0].Tags[0] != "三餐" {
		t.Fatalf("added line tags = %+v", added.Conditions[0].Tags)
	}
	if len(m.lineData) != 2 {
		t.Fatalf("line data = %d", len(m.lineData))
	}
}

func TestCreatePieChart(t *testing.T) {
	m := newTestModel(t)
	m.openCreate()
	m.form.title = "支出构成"
	m.form.chartType = "饼图"
	m.form.groupField = "分类"
	m.createChart()
	chart := m.selectedChart()
	if chart == nil || chart.ChartType != chartPie || chart.GroupField != groupCategory {
		t.Fatalf("pie chart = %+v", chart)
	}
	if chart.GroupTransactionType != "expense" {
		t.Fatalf("pie type = %q", chart.GroupTransactionType)
	}
	if len(m.pieSlices) == 0 || m.pie.Total == 0 {
		t.Fatalf("pie data = %+v", m.pie)
	}
	for _, slice := range m.pieSlices {
		if strings.TrimSpace(slice.Label) == "" {
			t.Fatalf("slice without a label: %+v", slice)
		}
	}
}

func TestTagOptionsFollowTypeAndCategory(t *testing.T) {
	m := newTestModel(t)
	m.openLine(m.selectedChart(), -1)
	m.form.lineKind = "支出"
	m.form.lineCategory = "餐饮美食"
	options := m.lineTagOptions()
	if !contains(options, "饮料") || contains(options, "工资") {
		t.Fatalf("餐饮美食 options = %v", options)
	}
	m.form.lineCategory = ""
	options = m.lineTagOptions()
	if !contains(options, "饮料") || contains(options, "工资") {
		t.Fatalf("expense options = %v", options)
	}
	m.form.lineKind = "收入"
	options = m.lineTagOptions()
	if contains(options, "饮料") || !contains(options, "工资") {
		t.Fatalf("income options = %v", options)
	}
}

// TestPieHoverPicksTheSlice checks the page's own pie geometry: the wedges
// start at twelve o'clock and sweep clockwise, the hole and the card around the
// pie name no slice, and the pointer reports a repaint only when the hovered
// wedge changes.
func TestPieHoverPicksTheSlice(t *testing.T) {
	m := newTestModel(t)
	m.openCreate()
	m.form.title = "支出构成"
	m.form.chartType = "饼图"
	m.form.groupField = "分类"
	m.createChart()
	if chart := m.selectedChart(); chart == nil || chart.ChartType != chartPie {
		t.Fatalf("selected chart = %+v, want the pie", chart)
	}
	// What the paint would hand the page: a hit region and the geometry.
	const (
		cx, cy = 200, 200
		radius = 120
	)
	var tree gfx.Tree
	m.pieHit = tree.ID(tree.Feature("analysis-test"), "pie")
	m.pieCenter = gfx.Point{X: cx, Y: cy}
	m.pieRadius = radius
	if got := m.pieSliceAt(cx, cy); got != noSlice {
		t.Fatalf("slice in the hole = %d, want none", got)
	}
	if got := m.pieSliceAt(cx, cy-radius-20); got != noSlice {
		t.Fatalf("slice outside the pie = %d, want none", got)
	}
	start := -math.Pi / 2
	for index, slice := range m.pieSlices {
		span := float64(slice.Amount) / float64(m.pie.Total) * 2 * math.Pi
		mid := start + span/2
		x := cx + float32(math.Cos(mid))*(radius*0.8)
		y := cy + float32(math.Sin(mid))*(radius*0.8)
		if got := m.pieSliceAt(x, y); got != index {
			t.Fatalf("slice at the middle of %d = %d", index, got)
		}
		start += span
	}

	// The pointer over the first wedge hovers it and asks for a repaint; a move
	// within the same wedge repaints too, because the tip follows the pointer.
	top := gfx.PointerEvent{Kind: gfx.PointerMove, Target: m.pieHit, X: cx, Y: cy - radius*0.8}
	if result := m.Pointer(top); !result.Repaint || m.pieHover != 0 {
		t.Fatalf("hover over the top wedge = %d, repaint %v", m.pieHover, result.Repaint)
	}
	if m.pieAt != (gfx.Point{X: top.X, Y: top.Y}) {
		t.Fatalf("tip follows %+v, want %+v", m.pieAt, top)
	}
	near := top
	near.Y -= 10
	if result := m.Pointer(near); !result.Repaint || m.pieHover != 0 {
		t.Fatalf("move within the wedge = %d, repaint %v", m.pieHover, result.Repaint)
	}
	// The hole describes no slice: it drops the tip, then asks for nothing while
	// there is nothing to drop.
	hole := gfx.PointerEvent{Kind: gfx.PointerMove, Target: m.pieHit, X: cx, Y: cy}
	if result := m.Pointer(hole); !result.Repaint || m.pieHover != noSlice {
		t.Fatalf("hover over the hole = %d, repaint %v", m.pieHover, result.Repaint)
	}
	if result := m.Pointer(hole); result.Repaint {
		t.Fatal("the hole repainted with no tip showing")
	}
	// A move off the pie clears the tip too.
	m.Pointer(top)
	left := gfx.PointerEvent{Kind: gfx.PointerMove, X: 0, Y: 0}
	if result := m.Pointer(left); !result.Repaint || m.pieHover != noSlice {
		t.Fatalf("hover after leaving the pie = %d, repaint %v", m.pieHover, result.Repaint)
	}
	if !strings.Contains(m.pieSliceValue(0), "·") || !strings.HasSuffix(m.pieSliceValue(0), "%") {
		t.Fatalf("slice value = %q", m.pieSliceValue(0))
	}
	if value := m.pieSliceValue(noSlice); value != "" {
		t.Fatalf("value of no slice = %q", value)
	}
}

// TestShortFrameKeepsTheTotalsBandInside paints the page at the window's
// minimum height with a pie whose table is longer than the frame can hold. The
// table gives way there: the totals band has to stay inside the page, because a
// band pushed past the frame takes the shell's own bottom rows with it.
func TestShortFrameKeepsTheTotalsBandInside(t *testing.T) {
	m := newTestModel(t)
	m.openCreate()
	m.form.title = "支出构成"
	m.form.chartType = "饼图"
	m.form.groupField = "分类"
	m.createChart()
	if len(m.pieSlices) < 6 {
		t.Fatalf("pie slices = %d, want a table longer than the frame can hold", len(m.pieSlices))
	}
	page := ui.NewPage(ui.PageConfig{View: m.View})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 960, Height: 620}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	frame := driver.Frame()
	if frame.Height != 620 {
		t.Fatalf("frame height = %v, want the configured 620", frame.Height)
	}
	found := 0
	for _, text := range driver.Snapshot().Texts {
		if text.Value != "合计" && !strings.HasPrefix(text.Value, "共 ") {
			continue
		}
		found++
		if bottom := text.Bounds.Y + text.Bounds.Height; bottom > frame.Height {
			t.Fatalf("%q is painted at %v, below the %v frame", text.Value, text.Bounds, frame.Height)
		}
	}
	if found == 0 {
		t.Fatal("the totals band did not paint")
	}
}

func TestFoldPieSlices(t *testing.T) {
	slices := make([]repositoryPieSlice, 0)
	for index := 0; index < 12; index++ {
		slices = append(slices, repositoryPieSlice{Label: string(rune('a' + index)), Amount: int64(12 - index)})
	}
	folded := foldPie(slices)
	if len(folded) != 10 || folded[9].Amount != 1+2+3 {
		t.Fatalf("folded = %+v", folded)
	}
	if !strings.Contains(folded[9].Label, "其他") {
		t.Fatalf("folded label = %q", folded[9].Label)
	}
}
