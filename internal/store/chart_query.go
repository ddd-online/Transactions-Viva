package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// ChartData aggregates each configured line into month or year buckets across
// the whole local-date range. Missing buckets are filled with 0 so every line
// shares one continuous x-axis.
func (w *Workspace) ChartData(ctx context.Context, query repository.ChartQuery) ([]repository.ChartLineData, repository.ChartStatistics, error) {
	var stats repository.ChartStatistics
	if strings.TrimSpace(query.LedgerID) == "" {
		return nil, stats, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	if query.Granularity != "month" && query.Granularity != "year" {
		return nil, stats, fmt.Errorf("%w：图表粒度「%s」无法识别", repository.ErrInvalid, query.Granularity)
	}
	start, end, err := chartRange(query.StartDate, query.EndDate)
	if err != nil {
		return nil, stats, err
	}
	labels, err := chartLabels(query.StartDate, query.EndDate, query.Granularity)
	if err != nil {
		return nil, stats, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, stats, err
	}
	format := "%Y-%m"
	if query.Granularity == "year" {
		format = "%Y"
	}
	lines := make([]repository.ChartLineData, 0, len(query.Lines))
	for _, line := range query.Lines {
		if !validTransactionType(line.TransactionType) {
			return nil, stats, fmt.Errorf("%w：曲线类型「%s」无法识别", repository.ErrInvalid, line.TransactionType)
		}
		where := []string{"ledger_id = ?", "transaction_at >= ?", "transaction_at <= ?", "transaction_type = ?"}
		args := []any{query.LedgerID, start, end, line.TransactionType}
		if !line.IncludeOutlier {
			where = append(where, "COALESCE(CASE WHEN json_valid(flags) THEN json_extract(flags, '$.outlier') END, 0) IS NOT 1")
		}
		if conditions := chartConditions(line.Conditions); conditions != "" {
			where = append(where, "("+conditions+")")
			for _, condition := range line.Conditions {
				args = append(args, conditionArgs(condition)...)
			}
		}
		rows, err := db.QueryContext(ctx,
			"SELECT strftime(?, transaction_at, 'unixepoch') AS time, SUM(amount_cents) AS amount "+
				"FROM transaction_record WHERE "+strings.Join(where, " AND ")+" GROUP BY time",
			append([]any{format}, args...)...)
		if err != nil {
			return nil, stats, err
		}
		byBucket := map[string]int64{}
		for rows.Next() {
			var bucket string
			var amount int64
			if err := rows.Scan(&bucket, &amount); err != nil {
				rows.Close()
				return nil, stats, err
			}
			byBucket[bucket] = amount
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, stats, err
		}
		rows.Close()
		data := make([]repository.ChartPoint, len(labels))
		for index, label := range labels {
			data[index] = repository.ChartPoint{Time: label, Amount: byBucket[label]}
		}
		lines = append(lines, repository.ChartLineData{
			Label: line.Label, TransactionType: line.TransactionType, Data: data,
		})
	}

	rows, err := db.QueryContext(ctx,
		"SELECT transaction_type, SUM(amount_cents) FROM transaction_record "+
			"WHERE ledger_id = ? AND transaction_at >= ? AND transaction_at <= ? GROUP BY transaction_type",
		query.LedgerID, start, end)
	if err != nil {
		return nil, stats, err
	}
	defer rows.Close()
	for rows.Next() {
		var transactionType string
		var amount int64
		if err := rows.Scan(&transactionType, &amount); err != nil {
			return nil, stats, err
		}
		switch transactionType {
		case "income":
			stats.Income = amount
		case "expense":
			stats.Expense = amount
		case "transfer":
			stats.Transfer = amount
		}
	}
	return lines, stats, rows.Err()
}

// ChartPie aggregates a range by transaction type, category or tag. Tags are a
// many-to-many relation, so one record can appear in several slices; the
// difference between Total and FilteredTotal says so.
func (w *Workspace) ChartPie(ctx context.Context, query repository.ChartPieQuery) (repository.ChartPieResult, error) {
	var result repository.ChartPieResult
	if strings.TrimSpace(query.LedgerID) == "" {
		return result, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	start, end, err := chartRange(query.StartDate, query.EndDate)
	if err != nil {
		return result, err
	}
	if query.GroupField != "transactionType" && query.GroupField != "category" && query.GroupField != "tag" {
		return result, fmt.Errorf("%w：饼图分组「%s」无法识别", repository.ErrInvalid, query.GroupField)
	}
	if query.GroupField != "transactionType" && !validTransactionType(query.GroupTransactionType) {
		return result, fmt.Errorf("%w：饼图交易类型「%s」无法识别", repository.ErrInvalid, query.GroupTransactionType)
	}
	if query.GroupField == "tag" && strings.TrimSpace(query.GroupCategory) == "" {
		return result, fmt.Errorf("%w：按标签分组时请先选择分类", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return result, err
	}

	where := []string{"r.ledger_id = ?", "r.transaction_at >= ?", "r.transaction_at <= ?"}
	args := []any{query.LedgerID, start, end}
	groupBy := "r.transaction_type"
	from := "transaction_record r"
	if query.GroupField == "category" {
		groupBy = "r.category"
		where = append(where, "r.transaction_type = ?")
		args = append(args, query.GroupTransactionType)
	}
	if query.GroupField == "tag" {
		groupBy = "t.tag"
		from = "transaction_record r JOIN transaction_record_tag t " +
			"ON t.ledger_id = r.ledger_id AND t.transaction_id = r.transaction_id"
		where = append(where, "r.transaction_type = ?", "r.category = ?")
		args = append(args, query.GroupTransactionType, query.GroupCategory)
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+groupBy+" AS key, SUM(r.amount_cents) AS amount FROM "+from+
			" WHERE "+strings.Join(where, " AND ")+" GROUP BY "+groupBy, args...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var key string
		var amount int64
		if err := rows.Scan(&key, &amount); err != nil {
			rows.Close()
			return result, err
		}
		if amount <= 0 {
			continue
		}
		result.Slices = append(result.Slices, repository.ChartPieSlice{
			Key: key, Label: chartSliceLabel(query.GroupField, key), Amount: amount,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	sort.SliceStable(result.Slices, func(i, j int) bool {
		if result.Slices[i].Amount != result.Slices[j].Amount {
			return result.Slices[i].Amount > result.Slices[j].Amount
		}
		return result.Slices[i].Label < result.Slices[j].Label
	})
	for _, slice := range result.Slices {
		result.Total += slice.Amount
	}

	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM transaction_record WHERE ledger_id = ? AND transaction_at >= ? AND transaction_at <= ?",
		query.LedgerID, start, end).Scan(&result.RangeRecords); err != nil {
		return result, err
	}
	filteredWhere := []string{"ledger_id = ?", "transaction_at >= ?", "transaction_at <= ?"}
	filteredArgs := []any{query.LedgerID, start, end}
	if query.GroupField != "transactionType" {
		filteredWhere = append(filteredWhere, "transaction_type = ?")
		filteredArgs = append(filteredArgs, query.GroupTransactionType)
	}
	if query.GroupField == "tag" {
		filteredWhere = append(filteredWhere, "category = ?")
		filteredArgs = append(filteredArgs, query.GroupCategory)
	}
	var filteredTotal sql.NullInt64
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*), SUM(amount_cents) FROM transaction_record WHERE "+strings.Join(filteredWhere, " AND "),
		filteredArgs...).Scan(&result.FilteredRecords, &filteredTotal); err != nil {
		return result, err
	}
	result.FilteredTotal = filteredTotal.Int64
	return result, nil
}

func chartRange(startDate, endDate string) (int64, int64, error) {
	if strings.TrimSpace(startDate) == "" || strings.TrimSpace(endDate) == "" {
		return 0, 0, fmt.Errorf("%w：请先选择图表的时间范围", repository.ErrInvalid)
	}
	start, err := localDayStart(startDate)
	if err != nil {
		return 0, 0, err
	}
	end, err := localDayEnd(endDate)
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		return 0, 0, fmt.Errorf("%w：结束日期不能早于开始日期", repository.ErrInvalid)
	}
	return start, end, nil
}

func chartLabels(startDate, endDate, granularity string) ([]string, error) {
	start, err := time.ParseInLocation("2006-01-02", startDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("%w：日期「%s」无法识别", repository.ErrInvalid, startDate)
	}
	end, err := time.ParseInLocation("2006-01-02", endDate, time.Local)
	if err != nil {
		return nil, fmt.Errorf("%w：日期「%s」无法识别", repository.ErrInvalid, endDate)
	}
	if end.Before(start) {
		return nil, fmt.Errorf("%w：结束日期不能早于开始日期", repository.ErrInvalid)
	}
	var labels []string
	if granularity == "year" {
		for year := start.Year(); year <= end.Year(); year++ {
			labels = append(labels, fmt.Sprintf("%04d", year))
		}
		return labels, nil
	}
	if granularity != "month" {
		return nil, fmt.Errorf("%w：图表粒度「%s」无法识别", repository.ErrInvalid, granularity)
	}
	year, month := start.Year(), start.Month()
	for year < end.Year() || (year == end.Year() && month <= end.Month()) {
		labels = append(labels, fmt.Sprintf("%04d-%02d", year, month))
		month++
		if month > 12 {
			month, year = 1, year+1
		}
	}
	return labels, nil
}

// chartConditions builds the OR of the line's condition items. An empty item
// matches every row, as the old query engine did.
func chartConditions(items []domain.QueryConditionItem) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		sub := []string{}
		if item.TransactionType != "" {
			sub = append(sub, "transaction_type = ?")
		}
		if item.Category != "" {
			sub = append(sub, "category = ?")
		}
		if item.Description != "" {
			sub = append(sub, "instr(description, ?) > 0")
		}
		if len(item.Tags) > 0 {
			negative := "NOT "
			if !item.TagNot {
				negative = ""
			}
			inList := "(" + placeholders(len(item.Tags)) + ")"
			if item.TagPolicy == "all" {
				sub = append(sub, negative+"((SELECT COUNT(DISTINCT t.tag) FROM transaction_record_tag t "+
					"WHERE t.transaction_id = transaction_record.transaction_id AND t.tag IN "+inList+") = ?)")
			} else {
				sub = append(sub, negative+"(EXISTS (SELECT 1 FROM transaction_record_tag t "+
					"WHERE t.transaction_id = transaction_record.transaction_id AND t.tag IN "+inList+"))")
			}
		}
		if len(sub) == 0 {
			parts = append(parts, "1 = 1")
			continue
		}
		parts = append(parts, "("+strings.Join(sub, " AND ")+")")
	}
	return strings.Join(parts, " OR ")
}

func conditionArgs(item domain.QueryConditionItem) []any {
	args := []any{}
	if item.TransactionType != "" {
		args = append(args, item.TransactionType)
	}
	if item.Category != "" {
		args = append(args, item.Category)
	}
	if item.Description != "" {
		args = append(args, item.Description)
	}
	if len(item.Tags) > 0 {
		for _, tag := range item.Tags {
			args = append(args, tag)
		}
		if item.TagPolicy == "all" {
			args = append(args, len(item.Tags))
		}
	}
	return args
}

func placeholders(count int) string {
	parts := make([]string, count)
	for index := range parts {
		parts[index] = "?"
	}
	return strings.Join(parts, ", ")
}

func chartSliceLabel(groupField, key string) string {
	switch groupField {
	case "category":
		if key == "" {
			return "未分类"
		}
	case "tag":
		if key == "" {
			return "未加标签"
		}
	default:
		switch key {
		case "income":
			return "收入"
		case "expense":
			return "支出"
		case "transfer":
			return "转账"
		}
	}
	return key
}
