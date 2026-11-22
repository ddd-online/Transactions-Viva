package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/repository"
)

// RefreshStockName updates every display copy of a stock name for one account.
// The history collection's updated_at stays put, so a rename does not reorder
// the records page; fund record text and operation logs keep the old snapshot.
func (w *Workspace) RefreshStockName(ctx context.Context, accountID, stockCode, stockName string) error {
	stockCode, stockName = strings.TrimSpace(stockCode), strings.TrimSpace(stockName)
	if stockCode == "" || stockName == "" {
		return fmt.Errorf("%w：股票代码与名称不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_position SET stock_name = ?, updated_at = ? WHERE account_id = ? AND stock_code = ?",
			stockName, nowUnix(), accountID, stockCode); err != nil {
			return fmt.Errorf("刷新股票名称失败：%w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_trade SET stock_name = ? WHERE account_id = ? AND stock_code = ?",
			stockName, accountID, stockCode); err != nil {
			return fmt.Errorf("刷新股票名称失败：%w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_trade_history SET stock_name = ? WHERE account_id = ? AND stock_code = ?",
			stockName, accountID, stockCode); err != nil {
			return fmt.Errorf("刷新股票名称失败：%w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_lesson SET stock_name = ?, updated_at = ? WHERE account_id = ? AND stock_code = ?",
			stockName, nowUnix(), accountID, stockCode); err != nil {
			return fmt.Errorf("刷新股票名称失败：%w", err)
		}
		return nil
	})
}
