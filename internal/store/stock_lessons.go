package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"transactions/internal/domain"
)

// StockLessons returns the account's lessons, newest first, with SourceAvailable
// reflecting whether the linked round still exists.
func (w *Workspace) StockLessons(ctx context.Context, accountID string) ([]domain.StockLesson, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rounds := make(map[string]bool)
	rows, err := db.QueryContext(ctx, "SELECT id FROM stock_trade_round WHERE account_id = ?", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取轮次失败：%w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		rounds[id] = true
	}
	rows.Close()
	rows, err = db.QueryContext(ctx,
		"SELECT id, account_id, text, round_id, stock_code, stock_name, round_no, opened_at, closed_at, "+
			"pnl_cents, pnl_rate, created_at, updated_at FROM stock_lesson WHERE account_id = ? "+
			"ORDER BY created_at DESC, id DESC", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取心得失败：%w", err)
	}
	defer rows.Close()
	var lessons []domain.StockLesson
	for rows.Next() {
		var lesson domain.StockLesson
		var roundID sql.NullString
		if err := rows.Scan(&lesson.ID, &lesson.AccountID, &lesson.Text, &roundID,
			&lesson.StockCode, &lesson.StockName, &lesson.RoundNo, &lesson.OpenedAt,
			&lesson.ClosedAt, &lesson.PnlCents, &lesson.PnlRate, &lesson.CreatedAt,
			&lesson.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取心得失败：%w", err)
		}
		if roundID.Valid && roundID.String != "" {
			lesson.RoundID = roundID.String
			lesson.SourceAvailable = rounds[lesson.RoundID]
		}
		lessons = append(lessons, lesson)
	}
	return lessons, rows.Err()
}

// StockRoundOptions lists closed rounds for the lesson dialog.
func (w *Workspace) StockRoundOptions(ctx context.Context, accountID string) ([]domain.StockRoundOption, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT r.id, r.stock_code, COALESCE(h.stock_name, ''), r.round_no, r.opened_at, r.closed_at "+
			"FROM stock_trade_round r LEFT JOIN stock_trade_history h ON h.id = r.history_id "+
			"WHERE r.account_id = ? ORDER BY r.closed_at DESC, r.stock_code ASC, r.round_no DESC", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取轮次失败：%w", err)
	}
	defer rows.Close()
	var options []domain.StockRoundOption
	for rows.Next() {
		var option domain.StockRoundOption
		if err := rows.Scan(&option.RoundID, &option.StockCode, &option.StockName,
			&option.RoundNo, &option.OpenedAt, &option.ClosedAt); err != nil {
			return nil, fmt.Errorf("读取轮次失败：%w", err)
		}
		options = append(options, option)
	}
	return options, rows.Err()
}

type stockRoundSnapshot struct {
	RoundID   string
	StockCode string
	StockName string
	RoundNo   int
	OpenedAt  int64
	ClosedAt  int64
	PnlCents  int64
	PnlRate   float64
}

func loadStockRoundSnapshot(ctx context.Context, db *sql.DB, accountID, roundID string) (stockRoundSnapshot, error) {
	var snapshot stockRoundSnapshot
	err := db.QueryRowContext(ctx,
		"SELECT r.id, r.stock_code, COALESCE(h.stock_name, ''), r.round_no, r.opened_at, r.closed_at "+
			"FROM stock_trade_round r LEFT JOIN stock_trade_history h ON h.id = r.history_id "+
			"WHERE r.id = ? AND r.account_id = ?", roundID, accountID).
		Scan(&snapshot.RoundID, &snapshot.StockCode, &snapshot.StockName,
			&snapshot.RoundNo, &snapshot.OpenedAt, &snapshot.ClosedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return snapshot, notFoundf("交易轮次不存在")
	}
	if err != nil {
		return snapshot, fmt.Errorf("读取轮次失败：%w", err)
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+stockTradeColumns+" FROM stock_trade WHERE account_id = ? AND round_id = ? "+
			"ORDER BY trade_time ASC, created_at ASC, id ASC", accountID, roundID)
	if err != nil {
		return snapshot, fmt.Errorf("读取成交失败：%w", err)
	}
	defer rows.Close()
	var trades []domain.StockTrade
	for rows.Next() {
		trade, err := scanStockTrade(rows)
		if err != nil {
			return snapshot, err
		}
		trades = append(trades, trade)
	}
	_, _, pnl, rate := domain.RoundPnl(trades)
	snapshot.PnlCents, snapshot.PnlRate = pnl, rate
	return snapshot, rows.Err()
}

// CreateStockLesson writes a one-line lesson, optionally linked to a round.
func (w *Workspace) CreateStockLesson(ctx context.Context, accountID, text, roundID string) (domain.StockLesson, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.StockLesson{}, invalidf("写一句心得再保存")
	}
	if utf8.RuneCountInString(text) > 80 {
		return domain.StockLesson{}, invalidf("心得最多 80 个字")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockLesson{}, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return domain.StockLesson{}, err
	}
	snapshot := stockRoundSnapshot{}
	if roundID != "" {
		snapshot, err = loadStockRoundSnapshot(ctx, db, accountID, roundID)
		if err != nil {
			return domain.StockLesson{}, err
		}
	}
	now := nowUnix()
	lesson := domain.StockLesson{
		ID: newID(), AccountID: accountID, Text: text, RoundID: snapshot.RoundID,
		StockCode: snapshot.StockCode, StockName: snapshot.StockName, RoundNo: snapshot.RoundNo,
		OpenedAt: snapshot.OpenedAt, ClosedAt: snapshot.ClosedAt,
		PnlCents: snapshot.PnlCents, PnlRate: snapshot.PnlRate,
		SourceAvailable: roundID != "", CreatedAt: now, UpdatedAt: now,
	}
	roundValue := any(nil)
	if lesson.RoundID != "" {
		roundValue = lesson.RoundID
	}
	if _, err := db.ExecContext(ctx,
		"INSERT INTO stock_lesson (id, account_id, text, round_id, stock_code, stock_name, round_no, "+
			"opened_at, closed_at, pnl_cents, pnl_rate, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		lesson.ID, lesson.AccountID, lesson.Text, roundValue, lesson.StockCode,
		lesson.StockName, lesson.RoundNo, lesson.OpenedAt, lesson.ClosedAt,
		lesson.PnlCents, lesson.PnlRate, lesson.CreatedAt, lesson.UpdatedAt); err != nil {
		return domain.StockLesson{}, fmt.Errorf("保存心得失败：%w", err)
	}
	return lesson, nil
}

// UpdateStockLesson rewrites a lesson and its round snapshot.
func (w *Workspace) UpdateStockLesson(ctx context.Context, accountID, lessonID, text, roundID string) (domain.StockLesson, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.StockLesson{}, invalidf("写一句心得再保存")
	}
	if utf8.RuneCountInString(text) > 80 {
		return domain.StockLesson{}, invalidf("心得最多 80 个字")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockLesson{}, err
	}
	var existing domain.StockLesson
	err = db.QueryRowContext(ctx,
		"SELECT id, account_id, text, COALESCE(round_id, ''), stock_code, stock_name, round_no, "+
			"opened_at, closed_at, pnl_cents, pnl_rate, created_at, updated_at FROM stock_lesson "+
			"WHERE id = ? AND account_id = ?", lessonID, accountID).
		Scan(&existing.ID, &existing.AccountID, &existing.Text, &existing.RoundID,
			&existing.StockCode, &existing.StockName, &existing.RoundNo, &existing.OpenedAt,
			&existing.ClosedAt, &existing.PnlCents, &existing.PnlRate, &existing.CreatedAt,
			&existing.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StockLesson{}, notFoundf("心得不存在")
	}
	if err != nil {
		return domain.StockLesson{}, fmt.Errorf("读取心得失败：%w", err)
	}
	snapshot := stockRoundSnapshot{}
	if roundID != "" {
		snapshot, err = loadStockRoundSnapshot(ctx, db, accountID, roundID)
		if err != nil {
			return domain.StockLesson{}, err
		}
	}
	roundValue := any(nil)
	if snapshot.RoundID != "" {
		roundValue = snapshot.RoundID
	}
	if _, err := db.ExecContext(ctx,
		"UPDATE stock_lesson SET text = ?, round_id = ?, stock_code = ?, stock_name = ?, round_no = ?, "+
			"opened_at = ?, closed_at = ?, pnl_cents = ?, pnl_rate = ?, updated_at = ? WHERE id = ? AND account_id = ?",
		text, roundValue, snapshot.StockCode, snapshot.StockName, snapshot.RoundNo,
		snapshot.OpenedAt, snapshot.ClosedAt, snapshot.PnlCents, snapshot.PnlRate,
		nowUnix(), lessonID, accountID); err != nil {
		return domain.StockLesson{}, fmt.Errorf("保存心得失败：%w", err)
	}
	existing.Text, existing.RoundID = text, snapshot.RoundID
	existing.StockCode, existing.StockName, existing.RoundNo = snapshot.StockCode, snapshot.StockName, snapshot.RoundNo
	existing.OpenedAt, existing.ClosedAt = snapshot.OpenedAt, snapshot.ClosedAt
	existing.PnlCents, existing.PnlRate = snapshot.PnlCents, snapshot.PnlRate
	existing.SourceAvailable = snapshot.RoundID != ""
	return existing, nil
}

// DeleteStockLesson removes one lesson.
func (w *Workspace) DeleteStockLesson(ctx context.Context, accountID, lessonID string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
		"DELETE FROM stock_lesson WHERE id = ? AND account_id = ?", lessonID, accountID)
	if err != nil {
		return fmt.Errorf("删除心得失败：%w", err)
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return notFoundf("心得不存在")
	}
	return nil
}

// StockLedgerEntries returns every ledger row of the account.
func (w *Workspace) StockLedgerEntries(ctx context.Context, accountID string) ([]domain.StockLedgerEntry, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, account_id, stock_code, COALESCE(round_id, ''), why_buy, stop_loss, take_profit, "+
			"followed_system, summary, created_at, updated_at FROM stock_ledger WHERE account_id = ?",
		accountID)
	if err != nil {
		return nil, fmt.Errorf("读取交易台账失败：%w", err)
	}
	defer rows.Close()
	var entries []domain.StockLedgerEntry
	for rows.Next() {
		var entry domain.StockLedgerEntry
		if err := rows.Scan(&entry.ID, &entry.AccountID, &entry.StockCode, &entry.RoundID,
			&entry.WhyBuy, &entry.StopLoss, &entry.TakeProfit, &entry.FollowedSystem,
			&entry.Summary, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取交易台账失败：%w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// StockLedger reads one stock's open ledger or one round's ledger.
func (w *Workspace) StockLedger(ctx context.Context, accountID, stockCode, roundID string) (*domain.StockLedgerEntry, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	query := "SELECT id, account_id, stock_code, COALESCE(round_id, ''), why_buy, stop_loss, take_profit, " +
		"followed_system, summary, created_at, updated_at FROM stock_ledger WHERE account_id = ? AND stock_code = ? AND round_id IS NULL"
	args := []any{accountID, stockCode}
	if roundID != "" {
		query = "SELECT id, account_id, stock_code, COALESCE(round_id, ''), why_buy, stop_loss, take_profit, " +
			"followed_system, summary, created_at, updated_at FROM stock_ledger WHERE account_id = ? AND round_id = ?"
		args = []any{accountID, roundID}
	}
	var entry domain.StockLedgerEntry
	err = db.QueryRowContext(ctx, query, args...).Scan(&entry.ID, &entry.AccountID,
		&entry.StockCode, &entry.RoundID, &entry.WhyBuy, &entry.StopLoss,
		&entry.TakeProfit, &entry.FollowedSystem, &entry.Summary,
		&entry.CreatedAt, &entry.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取交易台账失败：%w", err)
	}
	return &entry, nil
}

// SaveStockLedger writes one stock's ledger. At least one text field is needed.
func (w *Workspace) SaveStockLedger(ctx context.Context, entry domain.StockLedgerEntry) (domain.StockLedgerEntry, error) {
	entry.WhyBuy = strings.TrimSpace(entry.WhyBuy)
	entry.StopLoss = strings.TrimSpace(entry.StopLoss)
	entry.TakeProfit = strings.TrimSpace(entry.TakeProfit)
	entry.FollowedSystem = strings.TrimSpace(entry.FollowedSystem)
	entry.Summary = strings.TrimSpace(entry.Summary)
	fields := []string{entry.WhyBuy, entry.StopLoss, entry.TakeProfit, entry.FollowedSystem, entry.Summary}
	nonEmpty := false
	for _, field := range fields {
		if field != "" {
			nonEmpty = true
		}
		if utf8.RuneCountInString(field) > 2000 {
			return domain.StockLedgerEntry{}, invalidf("台账每一项最多 2000 个字")
		}
	}
	if !nonEmpty {
		return domain.StockLedgerEntry{}, invalidf("台账至少填写一项")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockLedgerEntry{}, err
	}
	existing, err := w.StockLedger(ctx, entry.AccountID, entry.StockCode, entry.RoundID)
	if err != nil {
		return domain.StockLedgerEntry{}, err
	}
	now := nowUnix()
	if existing == nil {
		entry.ID = newID()
		entry.CreatedAt, entry.UpdatedAt = now, now
		var roundValue any
		if entry.RoundID != "" {
			roundValue = entry.RoundID
		}
		if _, err := db.ExecContext(ctx,
			"INSERT INTO stock_ledger (id, account_id, stock_code, round_id, why_buy, stop_loss, "+
				"take_profit, followed_system, summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			entry.ID, entry.AccountID, entry.StockCode, roundValue, entry.WhyBuy,
			entry.StopLoss, entry.TakeProfit, entry.FollowedSystem, entry.Summary,
			entry.CreatedAt, entry.UpdatedAt); err != nil {
			return domain.StockLedgerEntry{}, fmt.Errorf("保存交易台账失败：%w", err)
		}
		return entry, nil
	}
	entry.ID, entry.CreatedAt = existing.ID, existing.CreatedAt
	entry.UpdatedAt = now
	if _, err := db.ExecContext(ctx,
		"UPDATE stock_ledger SET why_buy = ?, stop_loss = ?, take_profit = ?, followed_system = ?, "+
			"summary = ?, updated_at = ? WHERE id = ? AND account_id = ?",
		entry.WhyBuy, entry.StopLoss, entry.TakeProfit, entry.FollowedSystem,
		entry.Summary, now, entry.ID, entry.AccountID); err != nil {
		return domain.StockLedgerEntry{}, fmt.Errorf("保存交易台账失败：%w", err)
	}
	return entry, nil
}
