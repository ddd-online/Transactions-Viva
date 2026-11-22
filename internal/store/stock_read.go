package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// StockPositions returns the account's position rows (quantity may be zero for
// a 0-share round that is still open).
func (w *Workspace) StockPositions(ctx context.Context, accountID string) ([]domain.StockPosition, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	return loadStockPositionsDB(ctx, db, accountID)
}

func loadStockPositionsDB(ctx context.Context, db *sql.DB, accountID string) ([]domain.StockPosition, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, account_id, stock_code, stock_name, quantity, total_cost_cents, realized_pnl_cents, "+
			"review, tag, created_at, updated_at FROM stock_position WHERE account_id = ? "+
			"ORDER BY quantity DESC, created_at ASC", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取持仓失败：%w", err)
	}
	defer rows.Close()
	var positions []domain.StockPosition
	for rows.Next() {
		var position domain.StockPosition
		if err := rows.Scan(&position.ID, &position.AccountID, &position.StockCode, &position.StockName,
			&position.Quantity, &position.TotalCostCents, &position.RealizedPnlCents,
			&position.Review, &position.Tag, &position.CreatedAt, &position.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取持仓失败：%w", err)
		}
		positions = append(positions, position)
	}
	return positions, rows.Err()
}

// StockTrades returns every trade of one account, time ascending.
func (w *Workspace) StockTrades(ctx context.Context, accountID string) ([]domain.StockTrade, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+stockTradeColumns+" FROM stock_trade WHERE account_id = ? "+
			"ORDER BY trade_time ASC, created_at ASC, id ASC", accountID)
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

// StockRoundTrades reads the trades of one round (or the current round when
// roundID is empty), time ascending.
func (w *Workspace) StockRoundTrades(ctx context.Context, accountID, roundID string) ([]domain.StockTrade, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	query := "SELECT " + stockTradeColumns + " FROM stock_trade WHERE account_id = ? AND round_id = ? ORDER BY trade_time ASC, created_at ASC, id ASC"
	args := []any{accountID, roundID}
	if roundID == "" {
		query = "SELECT " + stockTradeColumns + " FROM stock_trade WHERE account_id = ? AND (round_id = '' OR round_id IS NULL) ORDER BY trade_time ASC, created_at ASC, id ASC"
		args = []any{accountID}
	}
	rows, err := db.QueryContext(ctx, query, args...)
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

// StockRounds returns every round of the account.
func (w *Workspace) StockRounds(ctx context.Context, accountID string) ([]domain.StockTradeRound, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, account_id, stock_code, history_id, round_no, opened_at, closed_at, tag, review, created_at "+
			"FROM stock_trade_round WHERE account_id = ? ORDER BY stock_code ASC, round_no ASC", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取轮次失败：%w", err)
	}
	defer rows.Close()
	var rounds []domain.StockTradeRound
	for rows.Next() {
		var round domain.StockTradeRound
		if err := rows.Scan(&round.ID, &round.AccountID, &round.StockCode, &round.HistoryID,
			&round.RoundNo, &round.OpenedAt, &round.ClosedAt, &round.Tag, &round.Review,
			&round.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取轮次失败：%w", err)
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

// StockHistories returns the account's history collections.
func (w *Workspace) StockHistories(ctx context.Context, accountID string) ([]domain.StockTradeHistory, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, account_id, stock_code, stock_name, created_at, updated_at "+
			"FROM stock_trade_history WHERE account_id = ?", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取交易历史失败：%w", err)
	}
	defer rows.Close()
	var histories []domain.StockTradeHistory
	for rows.Next() {
		var history domain.StockTradeHistory
		if err := rows.Scan(&history.ID, &history.AccountID, &history.StockCode,
			&history.StockName, &history.CreatedAt, &history.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取交易历史失败：%w", err)
		}
		histories = append(histories, history)
	}
	return histories, rows.Err()
}

// StockDayReviews returns every day review of the account, newest first.
func (w *Workspace) StockDayReviews(ctx context.Context, accountID string) ([]domain.StockDayReview, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, account_id, stock_code, round_id, review_date, review, created_at, updated_at "+
			"FROM stock_day_review WHERE account_id = ? ORDER BY review_date DESC, created_at DESC", accountID)
	if err != nil {
		return nil, fmt.Errorf("读取每日复盘失败：%w", err)
	}
	defer rows.Close()
	var reviews []domain.StockDayReview
	for rows.Next() {
		var review domain.StockDayReview
		if err := rows.Scan(&review.ID, &review.AccountID, &review.StockCode, &review.RoundID,
			&review.ReviewDate, &review.Review, &review.CreatedAt, &review.UpdatedAt); err != nil {
			return nil, fmt.Errorf("读取每日复盘失败：%w", err)
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

// UpdateStockPositionReview saves the current round's review draft.
func (w *Workspace) UpdateStockPositionReview(ctx context.Context, accountID, stockCode, review string) error {
	review = strings.TrimSpace(review)
	if utf8.RuneCountInString(review) > 500 {
		return fmt.Errorf("%w：交易复盘不能超过 500 字", repository.ErrInvalid)
	}
	return w.updateStockPositionField(ctx, accountID, stockCode, "review", review)
}

// UpdateStockPositionTag saves the current round's tag draft; empty returns to
// the default tag.
func (w *Workspace) UpdateStockPositionTag(ctx context.Context, accountID, stockCode, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = domain.DefaultStockTradeTag
	}
	tags, err := w.StockTradeTags(ctx, accountID)
	if err != nil {
		return err
	}
	if !containsString(tags, tag) {
		return fmt.Errorf("%w：无效的交易标签", repository.ErrInvalid)
	}
	return w.updateStockPositionField(ctx, accountID, stockCode, "tag", tag)
}

func (w *Workspace) updateStockPositionField(ctx context.Context, accountID, stockCode, column, value string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return err
	}
	open, err := stockHasOpenRoundDB(ctx, db, accountID, stockCode)
	if err != nil {
		return err
	}
	if !open {
		return fmt.Errorf("%w：该股票已清仓，请在「交易历史」中编辑本轮复盘", repository.ErrNotFound)
	}
	result, err := db.ExecContext(ctx,
		"UPDATE stock_position SET "+column+" = ?, updated_at = ? WHERE account_id = ? AND stock_code = ?",
		value, nowUnix(), accountID, stockCode)
	if err != nil {
		return fmt.Errorf("保存持仓失败：%w", err)
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return fmt.Errorf("%w：持仓不存在", repository.ErrNotFound)
	}
	return nil
}

func stockHasOpenRoundDB(ctx context.Context, db *sql.DB, accountID, stockCode string) (bool, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT trade_type FROM stock_trade WHERE account_id = ? AND stock_code = ? "+
			"ORDER BY trade_time ASC, created_at ASC, id ASC", accountID, stockCode)
	if err != nil {
		return false, fmt.Errorf("读取成交失败：%w", err)
	}
	defer rows.Close()
	var trades []domain.StockTrade
	for rows.Next() {
		var tradeType string
		if err := rows.Scan(&tradeType); err != nil {
			return false, err
		}
		trades = append(trades, domain.StockTrade{TradeType: tradeType})
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return len(domain.CurrentRoundTrades(trades)) > 0, nil
}

// UpdateStockDayReview writes or clears one current-round day review.
func (w *Workspace) UpdateStockDayReview(ctx context.Context, accountID, stockCode, date, review string) ([]domain.StockDayReview, error) {
	if _, ok := domain.ParseDiaryDate(date); !ok {
		return nil, fmt.Errorf("%w：日期格式应为 YYYY-MM-DD", repository.ErrInvalid)
	}
	today := todayStockDate()
	if date > today {
		return nil, fmt.Errorf("%w：不能写未来日期的复盘", repository.ErrInvalid)
	}
	review = strings.TrimSpace(review)
	if utf8.RuneCountInString(review) > 500 {
		return nil, fmt.Errorf("%w：每日复盘不能超过 500 字", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return nil, err
	}
	open, err := stockHasOpenRoundDB(ctx, db, accountID, stockCode)
	if err != nil {
		return nil, err
	}
	if !open {
		return nil, fmt.Errorf("%w：该股票已清仓，请在「交易历史」中查看本轮复盘", repository.ErrNotFound)
	}
	opened, err := currentRoundOpenedDate(ctx, db, accountID, stockCode)
	if err != nil {
		return nil, err
	}
	if opened != "" && date < opened {
		return nil, fmt.Errorf("%w：不能早于本轮建仓日", repository.ErrInvalid)
	}
	if review == "" {
		_, err = db.ExecContext(ctx,
			"DELETE FROM stock_day_review WHERE account_id = ? AND stock_code = ? AND round_id = '' AND review_date = ?",
			accountID, stockCode, date)
	} else {
		now := nowUnix()
		_, err = db.ExecContext(ctx,
			"INSERT INTO stock_day_review (id, account_id, stock_code, round_id, review_date, review, created_at, updated_at) "+
				"VALUES (?, ?, ?, '', ?, ?, ?, ?) ON CONFLICT(account_id, stock_code, round_id, review_date) "+
				"DO UPDATE SET review = excluded.review, updated_at = excluded.updated_at",
			newID(), accountID, stockCode, date, review, now, now)
	}
	if err != nil {
		return nil, fmt.Errorf("保存每日复盘失败：%w", err)
	}
	return stockCurrentDayReviews(ctx, db, accountID, stockCode)
}

func currentRoundOpenedDate(ctx context.Context, db *sql.DB, accountID, stockCode string) (string, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT trade_type, trade_time FROM stock_trade WHERE account_id = ? AND stock_code = ? "+
			"ORDER BY trade_time ASC, created_at ASC, id ASC", accountID, stockCode)
	if err != nil {
		return "", fmt.Errorf("读取成交失败：%w", err)
	}
	defer rows.Close()
	var trades []domain.StockTrade
	for rows.Next() {
		var trade domain.StockTrade
		if err := rows.Scan(&trade.TradeType, &trade.TradeTime); err != nil {
			return "", err
		}
		trades = append(trades, trade)
	}
	current := domain.CurrentRoundTrades(trades)
	if len(current) == 0 {
		return "", nil
	}
	return unixToStockDate(current[0].TradeTime), nil
}

func stockCurrentDayReviews(ctx context.Context, db *sql.DB, accountID, stockCode string) ([]domain.StockDayReview, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, account_id, stock_code, round_id, review_date, review, created_at, updated_at "+
			"FROM stock_day_review WHERE account_id = ? AND stock_code = ? AND round_id = '' "+
			"ORDER BY review_date DESC", accountID, stockCode)
	if err != nil {
		return nil, fmt.Errorf("读取每日复盘失败：%w", err)
	}
	defer rows.Close()
	var reviews []domain.StockDayReview
	for rows.Next() {
		var review domain.StockDayReview
		if err := rows.Scan(&review.ID, &review.AccountID, &review.StockCode, &review.RoundID,
			&review.ReviewDate, &review.Review, &review.CreatedAt, &review.UpdatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

// UpdateStockRoundReview edits an archived round's review.
func (w *Workspace) UpdateStockRoundReview(ctx context.Context, accountID, roundID, review string) error {
	review = strings.TrimSpace(review)
	if utf8.RuneCountInString(review) > 500 {
		return fmt.Errorf("%w：交易复盘不能超过 500 字", repository.ErrInvalid)
	}
	return w.updateStockRoundField(ctx, accountID, roundID, "review", review)
}

// UpdateStockRoundTag edits an archived round's tag.
func (w *Workspace) UpdateStockRoundTag(ctx context.Context, accountID, roundID, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = domain.DefaultStockTradeTag
	}
	tags, err := w.StockTradeTags(ctx, accountID)
	if err != nil {
		return err
	}
	if !containsString(tags, tag) {
		return fmt.Errorf("%w：无效的交易标签", repository.ErrInvalid)
	}
	return w.updateStockRoundField(ctx, accountID, roundID, "tag", tag)
}

func (w *Workspace) updateStockRoundField(ctx context.Context, accountID, roundID, column, value string) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
		"UPDATE stock_trade_round SET "+column+" = ? WHERE id = ? AND account_id = ?",
		value, roundID, accountID)
	if err != nil {
		return fmt.Errorf("保存轮次失败：%w", err)
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return fmt.Errorf("%w：轮次不存在", repository.ErrNotFound)
	}
	return nil
}

func todayStockDate() string { return time.Now().Format(domain.DiaryDateLayout) }
