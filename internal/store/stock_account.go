package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

const stockAccountColumns = "id, name, description, principal_cents, sort_order, created_at, updated_at"

// StockAccounts lists the workspace's stock accounts in display order.
func (w *Workspace) StockAccounts(ctx context.Context) ([]domain.StockAccount, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+stockAccountColumns+" FROM stock_account ORDER BY sort_order ASC, created_at ASC, id ASC")
	if err != nil {
		return nil, fmt.Errorf("读取股票账户失败：%w", err)
	}
	defer rows.Close()
	var accounts []domain.StockAccount
	for rows.Next() {
		account, err := scanStockAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

// CreateStockAccount appends an account at the end of the list.
func (w *Workspace) CreateStockAccount(ctx context.Context, name, description string) (domain.StockAccount, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.StockAccount{}, fmt.Errorf("%w：股票账户名称不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockAccount{}, err
	}
	taken, err := stockAccountNameTaken(ctx, db, name, "")
	if err != nil {
		return domain.StockAccount{}, err
	}
	if taken {
		return domain.StockAccount{}, fmt.Errorf("%w：股票账户名称已存在", repository.ErrDuplicate)
	}
	next, err := nextStockAccountOrder(ctx, db)
	if err != nil {
		return domain.StockAccount{}, err
	}
	now := nowUnix()
	account := domain.StockAccount{
		ID: newID(), Name: name, Description: strings.TrimSpace(description),
		SortOrder: next, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO stock_account ("+stockAccountColumns+") VALUES (?, ?, ?, ?, ?, ?, ?)",
		account.ID, account.Name, account.Description, account.PrincipalCents,
		account.SortOrder, account.CreatedAt, account.UpdatedAt,
	); err != nil {
		return domain.StockAccount{}, fmt.Errorf("创建股票账户失败：%w", err)
	}
	return account, nil
}

// UpdateStockAccount rewrites an account's name and description.
func (w *Workspace) UpdateStockAccount(ctx context.Context, id, name, description string) error {
	id, name = strings.TrimSpace(id), strings.TrimSpace(name)
	if id == "" {
		return fmt.Errorf("%w：股票账户 id 不能为空", repository.ErrInvalid)
	}
	if name == "" {
		return fmt.Errorf("%w：股票账户名称不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	taken, err := stockAccountNameTaken(ctx, db, name, id)
	if err != nil {
		return err
	}
	if taken {
		return fmt.Errorf("%w：股票账户名称已存在", repository.ErrDuplicate)
	}
	result, err := db.ExecContext(ctx,
		"UPDATE stock_account SET name = ?, description = ?, updated_at = ? WHERE id = ?",
		name, strings.TrimSpace(description), nowUnix(), id)
	if err != nil {
		return fmt.Errorf("更新股票账户失败：%w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%w：股票账户不存在", repository.ErrNotFound)
	}
	return nil
}

// DeleteStockAccount removes the account and every stock row it owns. The
// market tables are workspace-level and stay.
func (w *Workspace) DeleteStockAccount(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w：股票账户 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stock_account").Scan(&count); err != nil {
		return fmt.Errorf("读取股票账户失败：%w", err)
	}
	if count <= 1 {
		return fmt.Errorf("%w：至少保留一个股票账户", repository.ErrInvalid)
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for _, table := range stockAccountChildTables {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE account_id = ?", id); err != nil {
				return fmt.Errorf("删除股票账户数据失败：%w", err)
			}
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM stock_account WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("删除股票账户失败：%w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("%w：股票账户不存在", repository.ErrNotFound)
		}
		return nil
	})
}

var stockAccountChildTables = [...]string{
	"stock_fund_record",
	"stock_fee_setting",
	"stock_trade_tag_setting",
	"stock_review_setting",
	"stock_ledger",
	"stock_lesson",
	"stock_trade",
	"stock_trade_round",
	"stock_day_review",
	"stock_trade_history",
	"stock_position",
	"stock_operation",
}

func scanStockAccount(scanner interface{ Scan(...any) error }) (domain.StockAccount, error) {
	var account domain.StockAccount
	if err := scanner.Scan(
		&account.ID, &account.Name, &account.Description, &account.PrincipalCents,
		&account.SortOrder, &account.CreatedAt, &account.UpdatedAt,
	); err != nil {
		return domain.StockAccount{}, fmt.Errorf("读取股票账户失败：%w", err)
	}
	return account, nil
}

func stockAccountNameTaken(ctx context.Context, db *sql.DB, name, excludeID string) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM stock_account WHERE name = ?"
	args := []any{name}
	if excludeID != "" {
		query += " AND id <> ?"
		args = append(args, excludeID)
	}
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("读取股票账户失败：%w", err)
	}
	return count > 0, nil
}

func nextStockAccountOrder(ctx context.Context, db *sql.DB) (int, error) {
	var next int
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sort_order), -1) + 1 FROM stock_account").Scan(&next); err != nil {
		return 0, fmt.Errorf("读取股票账户失败：%w", err)
	}
	return next, nil
}
