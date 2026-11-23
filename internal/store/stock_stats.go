package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"transactions/internal/domain"
)

// StockStatistics computes the application's settlements statistics:
// the time axis is the close time, win rate includes ties in the denominator,
// and max drawdown is a cash-flow-neutral cumulative P&L curve.
func (w *Workspace) StockStatistics(ctx context.Context, query domain.StockStatisticsQuery) (domain.StockStatistics, error) {
	if err := validateStockStatisticsQuery(&query); err != nil {
		return domain.StockStatistics{}, err
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockStatistics{}, err
	}
	account, err := stockAccountByID(ctx, db, query.AccountID)
	if err != nil {
		return domain.StockStatistics{}, err
	}
	rounds, err := w.StockRounds(ctx, query.AccountID)
	if err != nil {
		return domain.StockStatistics{}, err
	}
	trades, err := w.StockTrades(ctx, query.AccountID)
	if err != nil {
		return domain.StockStatistics{}, err
	}
	byRound := make(map[string][]domain.StockTrade)
	for _, trade := range trades {
		byRound[trade.RoundID] = append(byRound[trade.RoundID], trade)
	}
	histories, _ := w.StockHistories(ctx, query.AccountID)
	names := make(map[string]string, len(histories))
	for _, history := range histories {
		names[history.StockCode] = history.StockName
	}
	settlements := make([]domain.StockSettlement, 0, len(rounds))
	for _, round := range rounds {
		roundTrades := byRound[round.ID]
		_, _, pnl, rate := domain.RoundPnl(roundTrades)
		name := names[round.StockCode]
		if name == "" {
			name = round.StockCode
		}
		settlements = append(settlements, domain.StockSettlement{
			RoundID: round.ID, StockCode: round.StockCode, StockName: name,
			StockRoundNo: round.RoundNo, Tag: round.Tag, OpenedAt: round.OpenedAt,
			ClosedAt: round.ClosedAt, PnlCents: pnl, PnlRate: rate,
			TradeCount: domain.RoundTradeCount(roundTrades),
		})
	}
	sort.SliceStable(settlements, func(i, j int) bool {
		if settlements[i].ClosedAt != settlements[j].ClosedAt {
			return settlements[i].ClosedAt < settlements[j].ClosedAt
		}
		if settlements[i].OpenedAt != settlements[j].OpenedAt {
			return settlements[i].OpenedAt < settlements[j].OpenedAt
		}
		return settlements[i].RoundID < settlements[j].RoundID
	})
	filtered := filterStockSettlements(settlements, query)
	principal, addRecords := stockPrincipalTimeline(ctx, db, query.AccountID)
	result := domain.StockStatistics{Principal: account.PrincipalCents, RoundCount: len(filtered)}
	if len(filtered) == 0 {
		return result, nil
	}
	windowBase := stockPrincipalAt(filtered[0].ClosedAt, principal, addRecords)
	for i := range filtered {
		result.Points = append(result.Points,
			buildStockPoint(filtered[:i+1], windowBase, i+1, i+1))
	}
	if query.Strategy == domain.StockStatisticsGrouped {
		for start := 0; start < len(filtered); start += query.GroupSize {
			end := start + query.GroupSize
			if end > len(filtered) {
				end = len(filtered)
			}
			group := filtered[start:end]
			point := buildStockPoint(group, windowBase, end, len(group))
			result.Groups = append(result.Groups, point)
		}
	}
	return result, nil
}

func validateStockStatisticsQuery(query *domain.StockStatisticsQuery) error {
	if query.AccountID == "" {
		return invalidf("没有指定股票账户")
	}
	switch query.Strategy {
	case "":
		query.Strategy = domain.StockStatisticsCumulative
	case domain.StockStatisticsCumulative, domain.StockStatisticsGrouped:
	default:
		return invalidf("统计口径无法识别")
	}
	if query.Strategy == domain.StockStatisticsGrouped {
		if query.GroupSize < 2 || query.GroupSize > 1000 {
			return invalidf("分组大小必须在 2–1000 之间")
		}
	}
	hasStart, hasEnd := strings.TrimSpace(query.StartMonth) != "", strings.TrimSpace(query.EndMonth) != ""
	if hasStart != hasEnd {
		return invalidf("统计时间范围必须同时给出起止月份")
	}
	if hasStart {
		start, err := time.Parse("2006-01", query.StartMonth)
		if err != nil {
			return invalidf("统计月份格式应为 YYYY-MM")
		}
		end, err := time.Parse("2006-01", query.EndMonth)
		if err != nil || end.Before(start) {
			return invalidf("统计月份格式应为 YYYY-MM")
		}
	}
	if query.Recent != nil {
		if *query.Recent <= 0 {
			return invalidf("最近笔数必须为正整数")
		}
		if hasStart {
			return invalidf("按时间与最近 N 笔不能同时使用")
		}
	}
	return nil
}

func filterStockSettlements(settlements []domain.StockSettlement, query domain.StockStatisticsQuery) []domain.StockSettlement {
	filtered := make([]domain.StockSettlement, 0, len(settlements))
	for _, settlement := range settlements {
		if query.Tag != "" && settlement.Tag != query.Tag {
			continue
		}
		if query.StartMonth != "" {
			month := time.Unix(settlement.ClosedAt, 0).Format("2006-01")
			if month < query.StartMonth || month > query.EndMonth {
				continue
			}
		}
		filtered = append(filtered, settlement)
	}
	if query.Recent != nil && *query.Recent < len(filtered) {
		filtered = filtered[len(filtered)-*query.Recent:]
	}
	return filtered
}

type stockAddRecord struct {
	date   string
	amount int64
}

func stockPrincipalTimeline(ctx context.Context, db *sql.DB, accountID string) (int64, []stockAddRecord) {
	account, err := stockAccountByID(ctx, db, accountID)
	if err != nil {
		return 0, nil
	}
	rows, err := db.QueryContext(ctx,
		"SELECT record_date, amount_change_cents FROM stock_fund_record WHERE account_id = ? AND event_type = ? "+
			"ORDER BY record_date ASC", accountID, domain.StockEventAddPrincipal)
	if err != nil {
		return account.PrincipalCents, nil
	}
	defer rows.Close()
	var records []stockAddRecord
	for rows.Next() {
		var record stockAddRecord
		if err := rows.Scan(&record.date, &record.amount); err == nil {
			records = append(records, record)
		}
	}
	return account.PrincipalCents, records
}

func stockPrincipalAt(closedAt, principal int64, records []stockAddRecord) int64 {
	date := unixToStockDate(closedAt)
	// Current principal minus additions after the window start. This keeps an
	// imported principal (no add records) working as the base.
	value := principal
	for _, record := range records {
		if record.date > date {
			value -= record.amount
		}
	}
	if value < 0 {
		return 0
	}
	return value
}

func buildStockPoint(settlements []domain.StockSettlement, windowBase int64, sequence, span int) domain.StockStatisticsPoint {
	last := settlements[len(settlements)-1]
	point := domain.StockStatisticsPoint{
		Sequence: sequence, Span: span, OpenedAt: last.OpenedAt, ClosedAt: last.ClosedAt,
		RoundID: last.RoundID, StockCode: last.StockCode, StockName: last.StockName,
		StockRoundNo: last.StockRoundNo, Tag: last.Tag, Pnl: last.PnlCents,
		PnlRate: last.PnlRate, TradeCount: last.TradeCount,
	}
	var winSum, lossSum int64
	var peak, maxDrawdown, peakAtDrawdown int64
	var cum int64
	for _, settlement := range settlements {
		point.TotalPnl += settlement.PnlCents
		switch {
		case settlement.PnlCents > 0:
			point.WinCount++
			winSum += settlement.PnlCents
		case settlement.PnlCents < 0:
			point.LossCount++
			lossSum -= settlement.PnlCents
		}
		cum += settlement.PnlCents
		if cum > peak {
			peak = cum
		}
		if drawdown := peak - cum; drawdown > maxDrawdown {
			maxDrawdown = drawdown
			peakAtDrawdown = peak
		}
	}
	point.TieCount = span - point.WinCount - point.LossCount
	point.WinRate = domain.PercentOf(int64(point.WinCount), int64(span))
	if point.WinCount > 0 {
		point.AvgWin = int64(domain.RoundHalfAwayFromZero(float64(winSum) / float64(point.WinCount)))
	}
	if point.LossCount > 0 {
		point.AvgLoss = int64(domain.RoundHalfAwayFromZero(float64(lossSum) / float64(point.LossCount)))
	}
	if point.WinCount > 0 && point.LossCount > 0 {
		ratio := (float64(winSum) / float64(point.WinCount)) / (float64(lossSum) / float64(point.LossCount))
		point.PnlRatio = &ratio
	}
	point.Expectancy = int64(domain.RoundHalfAwayFromZero(float64(point.TotalPnl) / float64(span)))
	point.MaxDrawdown = maxDrawdown
	point.MaxDrawdownPct = domain.PercentOf(maxDrawdown, windowBase+peakAtDrawdown)
	return point
}

func roundHistoryName(round domain.StockTradeRound, rounds []domain.StockTradeRound) string {
	// Kept for callers that already ha round slice; the statistics path uses
	// the history names loaded above.
	return round.StockCode
}
