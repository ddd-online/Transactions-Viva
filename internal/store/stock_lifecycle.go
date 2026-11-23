package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
)

// ResetStockAccount clears the named account's stock data and principal. The
// account row stays: its name is what the settings list shows.
func (w *Workspace) ResetStockAccount(ctx context.Context, accountID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := stockAccountByIDTx(ctx, tx, accountID); err != nil {
			return err
		}
		for _, table := range stockAccountChildTables {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE account_id = ?", accountID); err != nil {
				return fmt.Errorf("重置股票数据失败：%w", err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_account SET principal_cents = 0, updated_at = ? WHERE id = ?",
			nowUnix(), accountID); err != nil {
			return fmt.Errorf("重置股票数据失败：%w", err)
		}
		return nil
	})
}

// ArchiveStockAccount moves one account's data into a new account. It refuses
// while any round is still open, mirroring the archive command.
func (w *Workspace) ArchiveStockAccount(ctx context.Context, accountID, name string) (domain.StockAccount, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.StockAccount{}, invalidf("股票账户名称不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockAccount{}, err
	}
	var created domain.StockAccount
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		account, err := stockAccountByIDTx(ctx, tx, accountID)
		if err != nil {
			return err
		}
		taken, err := stockAccountNameTakenTx(ctx, tx, name, "")
		if err != nil {
			return err
		}
		if taken {
			return duplicatef("股票账户名称已存在")
		}
		open, err := stockAccountOpenRounds(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if len(open) > 0 {
			return invalidf("仍有未清仓的轮次（%s），请先清仓归档后再归档账户", strings.Join(open, "、"))
		}
		next, err := nextStockAccountOrderTx(ctx, tx)
		if err != nil {
			return err
		}
		now := nowUnix()
		created = domain.StockAccount{
			ID: newID(), Name: name, Description: "股票归档 " + todayStockDate(),
			PrincipalCents: account.PrincipalCents, SortOrder: next,
			CreatedAt: now, UpdatedAt: now,
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stock_account ("+stockAccountColumns+") VALUES (?, ?, ?, ?, ?, ?, ?)",
			created.ID, created.Name, created.Description, created.PrincipalCents,
			created.SortOrder, created.CreatedAt, created.UpdatedAt); err != nil {
			return fmt.Errorf("创建归档账户失败：%w", err)
		}
		// Copy the fee setting; tags / review templates stay on the account.
		var feeID string
		var commission, stamp, transfer float64
		var minCommission int64
		var feeCreated, feeUpdated int64
		err = tx.QueryRowContext(ctx,
			"SELECT id, commission_rate, min_commission_cents, stamp_duty_rate, transfer_fee_rate, created_at, updated_at "+
				"FROM stock_fee_setting WHERE account_id = ?", accountID).
			Scan(&feeID, &commission, &minCommission, &stamp, &transfer, &feeCreated, &feeUpdated)
		if err == nil {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_fee_setting (id, account_id, commission_rate, min_commission_cents, "+
					"stamp_duty_rate, transfer_fee_rate, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				newID(), created.ID, commission, minCommission, stamp, transfer, now, now); err != nil {
				return fmt.Errorf("复制费用设置失败：%w", err)
			}
		} else if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("读取费用设置失败：%w", err)
		}
		for _, table := range []string{
			"stock_fund_record", "stock_position", "stock_trade", "stock_trade_history",
			"stock_trade_round", "stock_day_review", "stock_lesson", "stock_ledger",
		} {
			if _, err := tx.ExecContext(ctx,
				"UPDATE "+table+" SET account_id = ? WHERE account_id = ?", created.ID, accountID); err != nil {
				return fmt.Errorf("归档股票数据失败：%w", err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM stock_operation WHERE account_id = ?", accountID); err != nil {
			return fmt.Errorf("清理操作记录失败：%w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_account SET principal_cents = 0, updated_at = ? WHERE id = ?",
			now, accountID); err != nil {
			return fmt.Errorf("清理原账户失败：%w", err)
		}
		return nil
	})
	return created, err
}

func stockAccountNameTakenTx(ctx context.Context, tx *sql.Tx, name, excludeID string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM stock_account WHERE name = ?"
	args := []any{name}
	if excludeID != "" {
		query += " AND id <> ?"
		args = append(args, excludeID)
	}
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("读取股票账户失败：%w", err)
	}
	return count > 0, nil
}

func nextStockAccountOrderTx(ctx context.Context, tx *sql.Tx) (int, error) {
	var next int
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sort_order), -1) + 1 FROM stock_account").Scan(&next); err != nil {
		return 0, fmt.Errorf("读取股票账户失败：%w", err)
	}
	return next, nil
}

func stockAccountOpenRounds(ctx context.Context, tx *sql.Tx, accountID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT stock_code, stock_name, quantity FROM stock_position WHERE account_id = ?", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取持仓失败：%w", err)
	}
	defer rows.Close()
	type held struct {
		code, name string
		quantity   int
	}
	var positions []held
	for rows.Next() {
		var item held
		if err := rows.Scan(&item.code, &item.name, &item.quantity); err != nil {
			return nil, err
		}
		positions = append(positions, item)
	}
	var open []string
	for _, position := range positions {
		trades, err := loadStockTradesForCode(ctx, tx, accountID, position.code)
		if err != nil {
			return nil, err
		}
		current := domain.CurrentRoundTrades(trades)
		if position.quantity > 0 {
			name := position.name
			if name == "" {
				name = position.code
			}
			open = append(open, name)
		} else if len(current) > 0 {
			open = append(open, position.code+" （0 手）")
		}
	}
	return open, nil
}

func loadStockTradesForCode(ctx context.Context, tx *sql.Tx, accountID, stockCode string) ([]domain.StockTrade, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT "+stockTradeColumns+" FROM stock_trade WHERE account_id = ? AND stock_code = ? "+
			"ORDER BY trade_time ASC, created_at ASC, "+
			"CASE WHEN trade_type IN ('open','add') THEN 0 ELSE 1 END ASC, id ASC",
		accountID, stockCode)
	if err != nil {
		return nil, fmt.Errorf("读取成交失败：%w", err)
	}
	defer rows.Close()
	var trades []domain.StockTrade
	for rows.Next() {
		trade, err := scanStockTrade(rows)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, rows.Err()
}
