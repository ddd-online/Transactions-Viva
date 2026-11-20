package analysis

import (
	"strings"
	"testing"

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
	m.form.lineTags = "三餐, 外卖"
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
