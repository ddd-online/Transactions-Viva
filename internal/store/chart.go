package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"transactions/internal/domain"
)

const chartColumns = "chart_id, ledger_id, title, granularity, chart_lines, chart_type, group_field, " +
	"group_transaction_type, group_category, is_preset, sort_order, created_at, updated_at"

// EnsureChartPresets seeds the application's two preset charts for a
// ledger, once. app_meta keeps a marker so a deleted preset stays deleted.
func (w *Workspace) EnsureChartPresets(ctx context.Context, ledgerID string) error {
	if strings.TrimSpace(ledgerID) == "" {
		return invalidf("没有指定账本")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	marker := "chart_seeded:" + ledgerID
	return inTx(ctx, db, func(tx *sql.Tx) error {
		var markerValue string
		err := tx.QueryRowContext(ctx, "SELECT value FROM app_meta WHERE key = ?", marker).Scan(&markerValue)
		if err == nil {
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM chart WHERE ledger_id = ?", ledgerID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return writeMeta(ctx, tx, marker, "1")
		}
		now := nowUnix()
		presets := []struct {
			title       string
			granularity string
			outlier     bool
			sort        int
		}{
			{title: "月度消费趋势", granularity: "month", outlier: false, sort: 0},
			{title: "年度消费趋势", granularity: "year", outlier: true, sort: 1},
		}
		for _, preset := range presets {
			lines := []domain.ChartLine{
				{Label: "支出", TransactionType: "expense", IncludeOutlier: preset.outlier, Conditions: []domain.QueryConditionItem{}},
				{Label: "收入", TransactionType: "income", IncludeOutlier: preset.outlier, Conditions: []domain.QueryConditionItem{}},
				{Label: "转账", TransactionType: "transfer", IncludeOutlier: preset.outlier, Conditions: []domain.QueryConditionItem{}},
			}
			encoded, err := json.Marshal(lines)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO chart (chart_id, ledger_id, title, granularity, chart_lines, chart_type, "+
					"group_field, group_transaction_type, group_category, is_preset, sort_order, created_at, updated_at) "+
					"VALUES (?, ?, ?, ?, ?, 'line', '', '', '', 1, ?, ?, ?)",
				newID(), ledgerID, preset.title, preset.granularity, string(encoded), preset.sort, now, now); err != nil {
				return err
			}
		}
		return writeMeta(ctx, tx, marker, "1")
	})
}

// Charts lists one ledger's saved charts in display order.
func (w *Workspace) Charts(ctx context.Context, ledgerID string) ([]domain.Chart, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return nil, invalidf("没有指定账本")
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+chartColumns+" FROM chart WHERE ledger_id = ? ORDER BY sort_order ASC, created_at DESC", ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var charts []domain.Chart
	for rows.Next() {
		chart, err := scanChart(rows)
		if err != nil {
			return nil, err
		}
		charts = append(charts, chart)
	}
	return charts, rows.Err()
}

// CreateChart inserts a chart at the end of its ledger's order.
func (w *Workspace) CreateChart(ctx context.Context, chart domain.Chart) (string, error) {
	if err := validateChart(chart); err != nil {
		return "", err
	}
	db, err := w.dbHandle()
	if err != nil {
		return "", err
	}
	chart.ChartID = newID()
	now := nowUnix()
	chart.CreatedAt, chart.UpdatedAt = now, now
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		if err := requireLedger(ctx, tx, chart.LedgerID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(sort_order), 0) + 1 FROM chart WHERE ledger_id = ?", chart.LedgerID).Scan(&chart.SortOrder); err != nil {
			return err
		}
		return insertChart(ctx, tx, chart)
	})
	if err != nil {
		return "", err
	}
	return chart.ChartID, nil
}

// UpdateChart writes a chart's editable fields. A preset may not change its
// content; only its sort order (through ReorderCharts) is mutable.
func (w *Workspace) UpdateChart(ctx context.Context, chart domain.Chart) error {
	if strings.TrimSpace(chart.ChartID) == "" {
		return invalidf("没有指定图表")
	}
	if err := validateChart(chart); err != nil {
		return err
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		existing, err := findChart(ctx, tx, chart.ChartID)
		if err != nil {
			return err
		}
		if existing == nil {
			return notFoundf("找不到该图表")
		}
		if existing.IsPreset && (existing.Title != chart.Title ||
			existing.Granularity != chart.Granularity ||
			existing.ChartLines != chart.ChartLines ||
			existing.ChartType != chart.ChartType ||
			existing.GroupField != chart.GroupField ||
			existing.GroupTransactionType != chart.GroupTransactionType ||
			existing.GroupCategory != chart.GroupCategory) {
			return invalidf("预设图表只能删除或排序")
		}
		chart.IsPreset = existing.IsPreset
		chart.CreatedAt = existing.CreatedAt
		chart.UpdatedAt = nowUnix()
		_, err = tx.ExecContext(ctx,
			"UPDATE chart SET title = ?, granularity = ?, chart_lines = ?, chart_type = ?, "+
				"group_field = ?, group_transaction_type = ?, group_category = ?, is_preset = ?, "+
				"sort_order = ?, created_at = ?, updated_at = ? WHERE chart_id = ?",
			chart.Title, chart.Granularity, chart.ChartLines, chart.ChartType,
			chart.GroupField, chart.GroupTransactionType, chart.GroupCategory,
			boolInt(chart.IsPreset), chart.SortOrder, chart.CreatedAt, chart.UpdatedAt, chart.ChartID)
		return err
	})
}

// DeleteChart removes one chart; a missing row is fine.
func (w *Workspace) DeleteChart(ctx context.Context, chartID string) error {
	if strings.TrimSpace(chartID) == "" {
		return invalidf("没有指定图表")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "DELETE FROM chart WHERE chart_id = ?", chartID)
	return err
}

// ReorderCharts writes sort_order 0..n-1 for the given charts.
func (w *Workspace) ReorderCharts(ctx context.Context, ledgerID string, ids []string) error {
	if strings.TrimSpace(ledgerID) == "" {
		return invalidf("没有指定账本")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for index, id := range ids {
			if _, err := tx.ExecContext(ctx,
				"UPDATE chart SET sort_order = ?, updated_at = ? WHERE chart_id = ? AND ledger_id = ?",
				index, nowUnix(), id, ledgerID); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateChart(chart domain.Chart) error {
	if strings.TrimSpace(chart.LedgerID) == "" {
		return invalidf("没有指定账本")
	}
	if strings.TrimSpace(chart.Title) == "" {
		return invalidf("图表名称不能为空")
	}
	if chart.ChartType != "line" && chart.ChartType != "pie" {
		return invalidf("图表类型「%s」无法识别", chart.ChartType)
	}
	return nil
}

func insertChart(ctx context.Context, tx *sql.Tx, chart domain.Chart) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO chart (chart_id, ledger_id, title, granularity, chart_lines, chart_type, "+
			"group_field, group_transaction_type, group_category, is_preset, sort_order, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		chart.ChartID, chart.LedgerID, chart.Title, chart.Granularity, chart.ChartLines, chart.ChartType,
		chart.GroupField, chart.GroupTransactionType, chart.GroupCategory, boolInt(chart.IsPreset),
		chart.SortOrder, chart.CreatedAt, chart.UpdatedAt)
	return err
}

type chartScanner interface {
	Scan(dest ...any) error
}

func scanChart(row chartScanner) (domain.Chart, error) {
	var chart domain.Chart
	var preset int
	err := row.Scan(&chart.ChartID, &chart.LedgerID, &chart.Title, &chart.Granularity, &chart.ChartLines,
		&chart.ChartType, &chart.GroupField, &chart.GroupTransactionType, &chart.GroupCategory,
		&preset, &chart.SortOrder, &chart.CreatedAt, &chart.UpdatedAt)
	chart.IsPreset = preset != 0
	return chart, err
}

func findChart(ctx context.Context, tx *sql.Tx, chartID string) (*domain.Chart, error) {
	row := tx.QueryRowContext(ctx, "SELECT "+chartColumns+" FROM chart WHERE chart_id = ?", chartID)
	chart, err := scanChart(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chart, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func readMeta(ctx context.Context, db *sql.DB, key string) (string, bool, error) {
	var value string
	err := db.QueryRowContext(ctx, "SELECT value FROM app_meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func writeMeta(ctx context.Context, tx *sql.Tx, key, value string) error {
	_, err := tx.ExecContext(ctx,
		"INSERT INTO app_meta(key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value)
	return err
}
