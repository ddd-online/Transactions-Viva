package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func TestEnsureChartPresetsAndCRUD(t *testing.T) {
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)
	ctx := context.Background()
	if err := workspace.EnsureChartPresets(ctx, ledger.ID); err != nil {
		t.Fatalf("EnsureChartPresets: %v", err)
	}
	charts, err := workspace.Charts(ctx, ledger.ID)
	if err != nil || len(charts) != 2 {
		t.Fatalf("charts = %+v, err=%v", charts, err)
	}
	if !charts[0].IsPreset || charts[0].Title != "月度消费趋势" || charts[1].Granularity != "year" {
		t.Fatalf("presets = %+v", charts)
	}
	if err := workspace.DeleteChart(ctx, charts[0].ChartID); err != nil {
		t.Fatal(err)
	}
	if err := workspace.EnsureChartPresets(ctx, ledger.ID); err != nil {
		t.Fatal(err)
	}
	charts, err = workspace.Charts(ctx, ledger.ID)
	if err != nil || len(charts) != 1 {
		t.Fatalf("charts after delete = %+v, err=%v", charts, err)
	}

	line, err := workspace.CreateChart(ctx, domain.Chart{
		LedgerID: ledger.ID, Title: "餐饮趋势", Granularity: "month", ChartType: "line",
		ChartLines: `[{"label":"餐饮","transactionType":"expense","includeOutlier":true,"conditions":[]}]`,
	})
	if err != nil {
		t.Fatalf("CreateChart: %v", err)
	}
	charts, err = workspace.Charts(ctx, ledger.ID)
	if err != nil || len(charts) != 2 || charts[1].ChartID != line {
		t.Fatalf("charts after create = %+v, err=%v", charts, err)
	}
	custom := charts[1]
	custom.Title = "餐饮月度"
	if err := workspace.UpdateChart(ctx, custom); err != nil {
		t.Fatalf("UpdateChart: %v", err)
	}
	preset := charts[0]
	preset.Title = "改名"
	if err := workspace.UpdateChart(ctx, preset); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("preset update error = %v", err)
	}
	if err := workspace.ReorderCharts(ctx, ledger.ID, []string{custom.ChartID, preset.ChartID}); err != nil {
		t.Fatalf("ReorderCharts: %v", err)
	}
	charts, err = workspace.Charts(ctx, ledger.ID)
	if err != nil || charts[0].ChartID != custom.ChartID {
		t.Fatalf("reordered charts = %+v, err=%v", charts, err)
	}
	if err := workspace.DeleteChart(ctx, custom.ChartID); err != nil {
		t.Fatalf("DeleteChart: %v", err)
	}
}

func TestChartDataAndPie(t *testing.T) {
	workspace := testWorkspace(t)
	ledger := testLedger(t, workspace)
	ctx := context.Background()
	records := []struct {
		id       string
		kind     string
		category string
		amount   int64
		at       int64
		flags    string
		tags     []string
	}{
		{"jan100", "expense", "餐饮美食", 100, day(2026, time.January, 5, 12, 0, 0), `{"outlier":false}`, []string{"三餐"}},
		{"jan200", "expense", "餐饮美食", 200, day(2026, time.January, 10, 12, 0, 0), `{"outlier":true}`, []string{"外卖"}},
		{"feb500", "income", "工资奖金", 500, day(2026, time.February, 2, 12, 0, 0), `{"outlier":false}`, nil},
		{"feb300", "expense", "餐饮美食", 300, day(2026, time.February, 3, 12, 0, 0), `{"outlier":false}`, []string{"三餐", "外卖"}},
	}
	for _, row := range records {
		if _, err := workspace.CreateRecord(ctx, domain.TransactionRecord{
			TransactionID: row.id, LedgerID: ledger.ID, AmountCents: row.amount,
			TransactionType: row.kind, Category: row.category, Flags: row.flags, TransactionAt: row.at,
		}, row.tags); err != nil {
			t.Fatalf("CreateRecord %s: %v", row.id, err)
		}
	}
	line := func(includeOutlier bool, conditions []domain.QueryConditionItem) ([]repository.ChartPoint, repository.ChartStatistics) {
		t.Helper()
		lines, stats, err := workspace.ChartData(ctx, repository.ChartQuery{
			LedgerID: ledger.ID, StartDate: "2026-01-01", EndDate: "2026-02-28", Granularity: "month",
			Lines: []domain.ChartLine{{
				Label: "支出", TransactionType: "expense", IncludeOutlier: includeOutlier, Conditions: conditions,
			}},
		})
		if err != nil {
			t.Fatalf("ChartData: %v", err)
		}
		return lines[0].Data, stats
	}
	all, stats := line(true, nil)
	if len(all) != 2 || all[0].Time != "2026-01" || all[0].Amount != 300 || all[1].Amount != 300 {
		t.Fatalf("expense points = %+v", all)
	}
	if stats.Income != 500 || stats.Expense != 600 {
		t.Fatalf("statistics = %+v", stats)
	}
	withoutOutlier, _ := line(false, nil)
	if withoutOutlier[0].Amount != 100 || withoutOutlier[1].Amount != 300 {
		t.Fatalf("without outlier points = %+v", withoutOutlier)
	}
	anyTag, _ := line(false, []domain.QueryConditionItem{{Tags: []string{"三餐"}, TagPolicy: "any"}})
	if anyTag[0].Amount != 100 || anyTag[1].Amount != 300 {
		t.Fatalf("any-tag points = %+v", anyTag)
	}
	allTags, _ := line(false, []domain.QueryConditionItem{{Tags: []string{"三餐", "外卖"}, TagPolicy: "all"}})
	if allTags[0].Amount != 0 || allTags[1].Amount != 300 {
		t.Fatalf("all-tag points = %+v", allTags)
	}

	pie, err := workspace.ChartPie(ctx, repository.ChartPieQuery{
		LedgerID: ledger.ID, StartDate: "2026-01-01", EndDate: "2026-02-28", GroupField: "transactionType",
	})
	if err != nil {
		t.Fatalf("ChartPie type: %v", err)
	}
	if pie.Total != 1100 || len(pie.Slices) != 2 || pie.Slices[0].Label != "支出" || pie.RangeRecords != 4 {
		t.Fatalf("type pie = %+v", pie)
	}
	pie, err = workspace.ChartPie(ctx, repository.ChartPieQuery{
		LedgerID: ledger.ID, StartDate: "2026-01-01", EndDate: "2026-02-28",
		GroupField: "tag", GroupTransactionType: "expense", GroupCategory: "餐饮美食",
	})
	if err != nil {
		t.Fatalf("ChartPie tag: %v", err)
	}
	if pie.Total != 900 || pie.FilteredTotal != 600 || pie.FilteredRecords != 3 {
		t.Fatalf("tag pie = %+v", pie)
	}
	for _, slice := range pie.Slices {
		if slice.Label == "三餐" && slice.Amount != 400 {
			t.Fatalf("三餐 slice = %+v", slice)
		}
		if slice.Label == "外卖" && slice.Amount != 500 {
			t.Fatalf("外卖 slice = %+v", slice)
		}
	}
}
