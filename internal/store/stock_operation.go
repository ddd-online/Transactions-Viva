package store

import (
	"context"
	"database/sql"
	"fmt"

	"transactions/internal/domain"
)

const stockOperationLimit = 10

const stockOperationColumns = "id, account_id, kind, action, detail, target_id, created_at"

// createStockOperation writes one rollbackable operation and trims the account
// to its latest ten. It is called inside the caller's transaction.
func createStockOperation(ctx context.Context, tx *sql.Tx, accountID, kind, action, detail, targetID string) error {
	var latest int64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(created_at), 0) FROM stock_operation WHERE account_id = ?",
		accountID).Scan(&latest); err != nil {
		return fmt.Errorf("写入操作记录失败：%w", err)
	}
	createdAt := nowUnix()
	if createdAt <= latest {
		createdAt = latest + 1
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_operation ("+stockOperationColumns+") VALUES (?, ?, ?, ?, ?, ?, ?)",
		newID(), accountID, kind, action, detail, targetID, createdAt); err != nil {
		return fmt.Errorf("写入操作记录失败：%w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM stock_operation WHERE account_id = ? AND id NOT IN ("+
			"SELECT id FROM stock_operation WHERE account_id = ? "+
			"ORDER BY created_at DESC, id DESC LIMIT ?)",
		accountID, accountID, stockOperationLimit); err != nil {
		return fmt.Errorf("裁剪操作记录失败：%w", err)
	}
	return nil
}

// StockOperations reads an account's rollbackable operations, newest first.
func (w *Workspace) StockOperations(ctx context.Context, accountID string) ([]domain.StockOperation, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+stockOperationColumns+" FROM stock_operation WHERE account_id = ? "+
			"ORDER BY created_at DESC, id DESC", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取操作记录失败：%w", err)
	}
	defer rows.Close()
	var operations []domain.StockOperation
	for rows.Next() {
		var operation domain.StockOperation
		if err := rows.Scan(&operation.ID, &operation.AccountID, &operation.Kind,
			&operation.Action, &operation.Detail, &operation.TargetID, &operation.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取操作记录失败：%w", err)
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}
