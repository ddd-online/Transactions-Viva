package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"transactions/internal/domain"
)

// StockLimitDay reads one day's ladder cache marker; nil when there is none.
func (w *Workspace) StockLimitDay(ctx context.Context, date string) (*domain.StockLimitDay, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	var day domain.StockLimitDay
	err = db.QueryRowContext(ctx,
		"SELECT trade_date, status, count, fetched_at FROM stock_limit_day WHERE trade_date = ?", date).
		Scan(&day.TradeDate, &day.Status, &day.Count, &day.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取涨停池缓存失败：%w", err)
	}
	return &day, nil
}

// SaveStockLimitDay writes the day marker without touching the pool.
func (w *Workspace) SaveStockLimitDay(ctx context.Context, date, status string, count int, fetchedAt int64) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO stock_limit_day (trade_date, status, count, fetched_at) VALUES (?, ?, ?, ?) "+
			"ON CONFLICT(trade_date) DO UPDATE SET status = excluded.status, count = excluded.count, "+
			"fetched_at = excluded.fetched_at",
		date, status, count, fetchedAt); err != nil {
		return fmt.Errorf("写入涨停池缓存失败：%w", err)
	}
	return nil
}

// StockLimitEntries reads one day's pool rows, earliest seal first.
func (w *Workspace) StockLimitEntries(ctx context.Context, date string) ([]domain.StockLimitEntry, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT stock_code, stock_name, market, price_milli, pct, amount_cents, float_mv_cents, turnover, "+
			"level, first_seal, last_seal, broken_count, seal_fund_cents, industry, stat_days, stat_count "+
			"FROM stock_limit_pool WHERE trade_date = ? ORDER BY first_seal ASC, stock_code ASC", date)
	if err != nil {
		return nil, fmt.Errorf("读取涨停池失败：%w", err)
	}
	defer rows.Close()
	var entries []domain.StockLimitEntry
	for rows.Next() {
		var entry domain.StockLimitEntry
		if err := rows.Scan(&entry.StockCode, &entry.StockName, &entry.Market, &entry.PriceMilli,
			&entry.Pct, &entry.AmountCents, &entry.FloatMvCents, &entry.Turnover,
			&entry.Level, &entry.FirstSeal, &entry.LastSeal, &entry.BrokenCount,
			&entry.SealFundCents, &entry.Industry, &entry.StatDays, &entry.StatCount); err != nil {
			return nil, fmt.Errorf("读取涨停池失败：%w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// SaveStockLimitEntries replaces one day's pool and marker in one transaction.
func (w *Workspace) SaveStockLimitEntries(ctx context.Context, date string, entries []domain.StockLimitEntry, fetchedAt int64) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM stock_limit_pool WHERE trade_date = ?", date); err != nil {
			return fmt.Errorf("写入涨停池失败：%w", err)
		}
		for _, entry := range entries {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_limit_pool (trade_date, stock_code, stock_name, market, price_milli, pct, "+
					"amount_cents, float_mv_cents, turnover, level, first_seal, last_seal, broken_count, "+
					"seal_fund_cents, industry, stat_days, stat_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
				date, entry.StockCode, entry.StockName, entry.Market, entry.PriceMilli, entry.Pct,
				entry.AmountCents, entry.FloatMvCents, entry.Turnover, entry.Level, entry.FirstSeal,
				entry.LastSeal, entry.BrokenCount, entry.SealFundCents, entry.Industry,
				entry.StatDays, entry.StatCount); err != nil {
				return fmt.Errorf("写入涨停池失败：%w", err)
			}
		}
		status := "empty"
		if len(entries) > 0 {
			status = "ok"
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stock_limit_day (trade_date, status, count, fetched_at) VALUES (?, ?, ?, ?) "+
				"ON CONFLICT(trade_date) DO UPDATE SET status = excluded.status, count = excluded.count, "+
				"fetched_at = excluded.fetched_at",
			date, status, len(entries), fetchedAt); err != nil {
			return fmt.Errorf("写入涨停池缓存失败：%w", err)
		}
		return nil
	})
}

// StockLimitQuotes reads the day's stored quote snapshot.
func (w *Workspace) StockLimitQuotes(ctx context.Context, date string) (map[string]domain.StockLimitQuote, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT stock_code, open_pct, close_pct, close_price_milli, fetched_at "+
			"FROM stock_limit_quote WHERE trade_date = ?", date)
	if err != nil {
		return nil, fmt.Errorf("读取天梯行情失败：%w", err)
	}
	defer rows.Close()
	result := make(map[string]domain.StockLimitQuote)
	for rows.Next() {
		var quote domain.StockLimitQuote
		var openPct, closePct sql.NullFloat64
		var closePrice sql.NullInt64
		if err := rows.Scan(&quote.StockCode, &openPct, &closePct, &closePrice, &quote.FetchedAt); err != nil {
			return nil, fmt.Errorf("读取天梯行情失败：%w", err)
		}
		quote.TradeDate = date
		if openPct.Valid {
			value := openPct.Float64
			quote.OpenPct = &value
		}
		if closePct.Valid {
			value := closePct.Float64
			quote.ClosePct = &value
		}
		if closePrice.Valid {
			value := closePrice.Int64
			quote.ClosePriceMilli = &value
		}
		result[quote.StockCode] = quote
	}
	return result, rows.Err()
}

// SaveStockLimitQuotes upserts the day's stored quote snapshot.
func (w *Workspace) SaveStockLimitQuotes(ctx context.Context, date string, quotes map[string]domain.StockLimitQuote, fetchedAt int64) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for code, quote := range quotes {
			var openPct, closePct, closePrice any
			if quote.OpenPct != nil {
				openPct = *quote.OpenPct
			}
			if quote.ClosePct != nil {
				closePct = *quote.ClosePct
			}
			if quote.ClosePriceMilli != nil {
				closePrice = *quote.ClosePriceMilli
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_limit_quote (trade_date, stock_code, open_pct, close_pct, close_price_milli, fetched_at) "+
					"VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(trade_date, stock_code) DO UPDATE SET "+
					"open_pct = excluded.open_pct, close_pct = excluded.close_pct, "+
					"close_price_milli = excluded.close_price_milli, fetched_at = excluded.fetched_at",
				date, code, openPct, closePct, closePrice, fetchedAt); err != nil {
				return fmt.Errorf("写入天梯行情失败：%w", err)
			}
		}
		return nil
	})
}
