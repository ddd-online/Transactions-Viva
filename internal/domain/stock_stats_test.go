package domain

import "testing"

func TestStockStatisticsMetrics(t *testing.T) {
	point := StockStatisticsPoint{
		TotalPnl:       1_234,
		WinRate:        66.67,
		AvgWin:         500,
		AvgLoss:        200,
		PnlRatio:       floatPtr(2.5),
		Expectancy:     300,
		MaxDrawdownPct: 12.34,
	}
	if got := MetricValue(MetricAvgLoss, point); got != -200 {
		t.Fatalf("avg loss curve = %v", got)
	}
	if got := MetricValue(MetricTotalPnl, point); got != 1_234 {
		t.Fatalf("total pnl = %v", got)
	}
	if got := MetricLabelIn(MetricTotalPnl, StockStatisticsGrouped); got != "分组盈亏" {
		t.Fatalf("grouped label = %q", got)
	}
	if got := MetricLabelIn(MetricTotalPnl, StockStatisticsCumulative); got != "累计盈亏" {
		t.Fatalf("cumulative label = %q", got)
	}
	if got := MetricKind(MetricPnlRatio); got != ChartValueRatio {
		t.Fatalf("ratio kind = %q", got)
	}
	if bounds := MetricYBounds(MetricWinRate, []float64{10, 20}); bounds == nil || *bounds != [2]float64{0, 100} {
		t.Fatalf("win-rate bounds = %v", bounds)
	}
	if bounds := MetricYBounds(MetricWinRate, []float64{-10, 20}); bounds == nil || *bounds != [2]float64{-100, 100} {
		t.Fatalf("negative win-rate bounds = %v", bounds)
	}
	if !MetricHasReference(MetricExpectancy) {
		t.Fatal("expectancy should have a zero reference")
	}
	if got := PointSpanLabel(StockStatisticsGrouped, 10, 5); got != "第6-10笔" {
		t.Fatalf("span label = %q", got)
	}
	if got := PointSpanLabel(StockStatisticsCumulative, 10, 10); got != "第10笔" {
		t.Fatalf("span label = %q", got)
	}
}

func TestKellyPercent(t *testing.T) {
	ratio := 2.0
	value := KellyPercent(10, 6, &ratio)
	if value == nil || *value != 40 {
		t.Fatalf("kelly = %v", value)
	}
	if value := KellyPercent(10, 6, nil); value != nil {
		t.Fatalf("kelly without ratio = %v", value)
	}
	if value := KellyPercent(0, 0, &ratio); value != nil {
		t.Fatalf("kelly without samples = %v", value)
	}
}

func floatPtr(value float64) *float64 { return &value }

func intPtr(value int) *int { return &value }
