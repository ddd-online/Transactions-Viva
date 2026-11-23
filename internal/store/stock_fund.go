package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"transactions/internal/domain"
)

const stockFundColumns = "id, account_id, record_date, event_type, event_text, amount_change_cents, " +
	"cash_balance_cents, net_pnl_cents, remark, created_at"

// StockAccountSummary is the cash formula the application used:
// principal + interest + sell net − withdrawn − position cost.
func (w *Workspace) StockAccountSummary(ctx context.Context, accountID string) (domain.StockAccountSummary, error) {
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockAccountSummary{}, err
	}
	account, err := stockAccountByID(ctx, db, accountID)
	if err != nil {
		return domain.StockAccountSummary{}, err
	}
	var summary domain.StockAccountSummary
	summary.Principal = account.PrincipalCents
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(net_pnl_cents), 0) FROM stock_fund_record "+
			"WHERE account_id = ? AND event_type = ?",
		accountID, domain.StockEventSell,
	).Scan(&summary.SellNetPnlTotal); err != nil {
		return domain.StockAccountSummary{}, fmt.Errorf("读取股票账户失败：%w", err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(-amount_change_cents), 0) FROM stock_fund_record "+
			"WHERE account_id = ? AND event_type = ?",
		accountID, domain.StockEventWithdraw,
	).Scan(&summary.WithdrawnTotal); err != nil {
		return domain.StockAccountSummary{}, fmt.Errorf("读取股票账户失败：%w", err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(amount_change_cents), 0) FROM stock_fund_record "+
			"WHERE account_id = ? AND event_type = ?",
		accountID, domain.StockEventInterestPrincipal,
	).Scan(&summary.InterestTotal); err != nil {
		return domain.StockAccountSummary{}, fmt.Errorf("读取股票账户失败：%w", err)
	}
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(total_cost_cents), 0) FROM stock_position "+
			"WHERE account_id = ? AND quantity > 0",
		accountID,
	).Scan(&summary.PositionCost); err != nil {
		return domain.StockAccountSummary{}, fmt.Errorf("读取股票账户失败：%w", err)
	}
	summary.AvailableCash = summary.Principal + summary.InterestTotal +
		summary.SellNetPnlTotal - summary.WithdrawnTotal - summary.PositionCost
	return summary, nil
}

// StockFundRecords reads one page in the order: date desc, created desc.
func (w *Workspace) StockFundRecords(ctx context.Context, accountID string, page, pageSize int) (domain.StockFundPage, error) {
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockFundPage{}, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return domain.StockFundPage{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_fund_record WHERE account_id = ?", accountID).Scan(&total); err != nil {
		return domain.StockFundPage{}, fmt.Errorf("读取资金记录失败：%w", err)
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+stockFundColumns+" FROM stock_fund_record WHERE account_id = ? "+
			"ORDER BY record_date DESC, created_at DESC, id DESC LIMIT ? OFFSET ?",
		accountID, pageSize, (page-1)*pageSize)
	if err != nil {
		return domain.StockFundPage{}, fmt.Errorf("读取资金记录失败：%w", err)
	}
	defer rows.Close()
	items := make([]domain.StockFundRecord, 0, pageSize)
	for rows.Next() {
		record, err := scanStockFundRecord(rows)
		if err != nil {
			return domain.StockFundPage{}, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return domain.StockFundPage{}, err
	}
	return domain.StockFundPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// AddStockPrincipal raises the principal and writes one fund record.
func (w *Workspace) AddStockPrincipal(ctx context.Context, accountID string, amountCents int64, date string) error {
	if amountCents <= 0 {
		return invalidf("追加金额必须大于 0")
	}
	recordDate, err := normalizeStockRecordDate(date)
	if err != nil {
		return err
	}
	return w.writePrincipalChange(ctx, accountID, amountCents, recordDate, domain.StockEventAddPrincipal, "追加本金", func(account domain.StockAccount) (int64, string) {
		next := account.PrincipalCents + amountCents
		return next, "本金 " + domain.CentsToYuan(account.PrincipalCents) + " → " + domain.CentsToYuan(next)
	})
}

// WithdrawStock takes money out; the principal does not change.
func (w *Workspace) WithdrawStock(ctx context.Context, accountID string, amountCents int64, date string) error {
	if amountCents <= 0 {
		return invalidf("支取金额必须大于 0")
	}
	recordDate, err := normalizeStockRecordDate(date)
	if err != nil {
		return err
	}
	return w.writePrincipalChange(ctx, accountID, -amountCents, recordDate, domain.StockEventWithdraw, "支取", func(account domain.StockAccount) (int64, string) {
		return account.PrincipalCents, "支取 " + domain.CentsToYuan(amountCents) + " 元"
	})
}

// AddStockInterest adds interest; the principal does not change.
func (w *Workspace) AddStockInterest(ctx context.Context, accountID string, amountCents int64, date string) error {
	if amountCents <= 0 {
		return invalidf("利息金额必须大于 0")
	}
	recordDate, err := normalizeStockRecordDate(date)
	if err != nil {
		return err
	}
	return w.writePrincipalChange(ctx, accountID, amountCents, recordDate, domain.StockEventInterestPrincipal, "利息归本", func(account domain.StockAccount) (int64, string) {
		return account.PrincipalCents, "利息 " + domain.CentsToYuan(amountCents) + " 元"
	})
}

// writePrincipalChange writes one of the three principal-class events. The
// update of the account, the fund record, the cash chain and the operation log
// share one transaction.
func (w *Workspace) writePrincipalChange(
	ctx context.Context,
	accountID string,
	amount int64,
	recordDate, eventType, eventText string,
	principalAfter func(domain.StockAccount) (int64, string),
) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		account, err := stockAccountByIDTx(ctx, tx, accountID)
		if err != nil {
			return err
		}
		// Withdrawal may not exceed the chain's last balance.
		if eventType == domain.StockEventWithdraw {
			balance, err := latestStockCashBalance(ctx, tx, accountID)
			if err != nil {
				return err
			}
			if balance < -amount {
				return invalidf("支取金额不能超过可用现金（%s 元）", domain.CentsToYuan(balance))
			}
		}
		nextPrincipal, remark := principalAfter(account)
		if nextPrincipal != account.PrincipalCents {
			if _, err := tx.ExecContext(ctx,
				"UPDATE stock_account SET principal_cents = ?, updated_at = ? WHERE id = ?",
				nextPrincipal, nowUnix(), accountID); err != nil {
				return fmt.Errorf("更新股票账户失败：%w", err)
			}
		}
		createdAt, err := nextFundCreatedAt(ctx, tx, accountID)
		if err != nil {
			return err
		}
		recordID := newID()
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stock_fund_record ("+stockFundColumns+") VALUES (?, ?, ?, ?, ?, ?, 0, NULL, ?, ?)",
			recordID, accountID, recordDate, eventType, eventText, amount, remark, createdAt); err != nil {
			return fmt.Errorf("写入资金记录失败：%w", err)
		}
		if err := recalculateStockCashChain(ctx, tx, accountID, nextPrincipal); err != nil {
			return err
		}
		return createStockOperation(ctx, tx, accountID, domain.StockOperationFund,
			eventText, fundAmountText(amount), recordID)
	})
}

// recalculateStockCashChain rebuilds the derived balances of one account.
// fundAmountText is the amount the operation log records: the symbol, with a
// withdrawal's minus in front of it (-¥500.00), so the log reads like every
// money cell in the application.
func fundAmountText(cents int64) string {
	text := domain.CentsToYuan(cents)
	if strings.HasPrefix(text, "-") {
		return "-¥" + text[1:]
	}
	return "¥" + text
}

func recalculateStockCashChain(ctx context.Context, tx *sql.Tx, accountID string, principal int64) error {
	var added int64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(amount_change_cents), 0) FROM stock_fund_record "+
			"WHERE account_id = ? AND event_type = ?",
		accountID, domain.StockEventAddPrincipal,
	).Scan(&added); err != nil {
		return fmt.Errorf("复算现金链失败：%w", err)
	}
	base := principal - added
	rows, err := tx.QueryContext(ctx,
		"SELECT id, record_date, cash_balance_cents, amount_change_cents FROM stock_fund_record "+
			"WHERE account_id = ? ORDER BY record_date ASC, created_at ASC, id ASC", accountID)
	if err != nil {
		return fmt.Errorf("复算现金链失败：%w", err)
	}
	defer rows.Close()
	type balanceChange struct {
		id      string
		balance int64
	}
	var changes []balanceChange
	cash := base
	for rows.Next() {
		var id, date string
		var oldBalance, amount int64
		if err := rows.Scan(&id, &date, &oldBalance, &amount); err != nil {
			return err
		}
		cash += amount
		if oldBalance != cash {
			changes = append(changes, balanceChange{id: id, balance: cash})
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, change := range changes {
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_fund_record SET cash_balance_cents = ? WHERE id = ?",
			change.balance, change.id); err != nil {
			return fmt.Errorf("复算现金链失败：%w", err)
		}
	}
	return nil
}

func latestStockCashBalance(ctx context.Context, tx *sql.Tx, accountID string) (int64, error) {
	var balance int64
	err := tx.QueryRowContext(ctx,
		"SELECT cash_balance_cents FROM stock_fund_record WHERE account_id = ? "+
			"ORDER BY record_date DESC, created_at DESC, id DESC LIMIT 1", accountID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		account, err := stockAccountByIDTx(ctx, tx, accountID)
		if err != nil {
			return 0, err
		}
		return account.PrincipalCents, nil
	}
	if err != nil {
		return 0, fmt.Errorf("读取资金记录失败：%w", err)
	}
	return balance, nil
}

func nextFundCreatedAt(ctx context.Context, tx *sql.Tx, accountID string) (int64, error) {
	var latest int64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(created_at), 0) FROM stock_fund_record WHERE account_id = ?",
		accountID).Scan(&latest); err != nil {
		return 0, fmt.Errorf("写入资金记录失败：%w", err)
	}
	now := nowUnix()
	if now <= latest {
		return latest + 1, nil
	}
	return now, nil
}

func normalizeStockRecordDate(date string) (string, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return time.Now().Format(domain.DiaryDateLayout), nil
	}
	parsed, ok := domain.ParseDiaryDate(date)
	if !ok {
		return "", invalidf("日期要写成 YYYY-MM-DD")
	}
	return parsed.Format(domain.DiaryDateLayout), nil
}

func stockAccountByID(ctx context.Context, db *sql.DB, id string) (domain.StockAccount, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.StockAccount{}, invalidf("没有指定股票账户")
	}
	row := db.QueryRowContext(ctx, "SELECT "+stockAccountColumns+" FROM stock_account WHERE id = ?", id)
	account, err := scanStockAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StockAccount{}, notFoundf("股票账户不存在")
	}
	return account, err
}

func stockAccountByIDTx(ctx context.Context, tx *sql.Tx, id string) (domain.StockAccount, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.StockAccount{}, invalidf("没有指定股票账户")
	}
	row := tx.QueryRowContext(ctx, "SELECT "+stockAccountColumns+" FROM stock_account WHERE id = ?", id)
	account, err := scanStockAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StockAccount{}, notFoundf("股票账户不存在")
	}
	return account, err
}

func scanStockFundRecord(scanner interface{ Scan(...any) error }) (domain.StockFundRecord, error) {
	var record domain.StockFundRecord
	var netPnl sql.NullInt64
	if err := scanner.Scan(
		&record.ID, &record.AccountID, &record.RecordDate, &record.EventType,
		&record.EventText, &record.AmountChangeCents, &record.CashBalanceCents,
		&netPnl, &record.Remark, &record.CreatedAt,
	); err != nil {
		return domain.StockFundRecord{}, fmt.Errorf("读取资金记录失败：%w", err)
	}
	if netPnl.Valid {
		value := netPnl.Int64
		record.NetPnlCents = &value
	}
	return record, nil
}
