package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// replayOrder is one order accumulated while replaying an account's trades.
type replayOrder struct {
	key       string
	stockCode string
	stockName string
	isBuy     bool
	isClose   bool
	amount    int64
	fee       int64
	lots      int
	netPnl    int64
	firstTime int64
	lastTime  int64
	createdAt int64
	indexes   []int
}

// UpdateStockTrade edits one fill, recomputes the whole order's fees and replays
// the account. TradeTime <= 0 keeps the stored time.
func (w *Workspace) UpdateStockTrade(ctx context.Context, request repository.StockTradeUpdate) error {
	if request.TradeID == "" {
		return invalidf("交易记录不存在")
	}
	if request.PriceMilli <= 0 {
		return invalidf("成交价必须大于 0")
	}
	if request.Lots <= 0 {
		return invalidf("手数必须大于 0")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		trade, err := getStockTradeTx(ctx, tx, request.TradeID)
		if err != nil {
			return err
		}
		if trade.AccountID != request.AccountID {
			return notFoundf("交易记录不存在")
		}
		orderTrades, err := loadStockOrderTradesTx(ctx, tx, request.AccountID, stockOrderKey(trade))
		if err != nil {
			return err
		}
		if len(orderTrades) == 0 {
			orderTrades = []domain.StockTrade{trade}
		}
		setting := domain.DefaultStockFeeSetting()
		if trade.RoundFee != nil {
			setting = domain.SettingOfRoundFee(*trade.RoundFee)
		} else if current, err := getOrCreateStockFeeSettingTx(ctx, tx, request.AccountID); err == nil {
			setting = current
		}
		instrument := domain.TradeInstrumentOf(trade.StockCode)
		buy := domain.IsBuy(trade.TradeType)
		amounts := make([]int64, len(orderTrades))
		for i := range orderTrades {
			item := &orderTrades[i]
			if item.ID == request.TradeID {
				item.PriceMilli = request.PriceMilli
				item.Lots = request.Lots
				item.Shares = int(domain.SharesOfLots(int64(request.Lots)))
			}
			if request.TradeTime > 0 {
				item.TradeTime = request.TradeTime
			}
			item.AmountCents = domain.AmountOf(item.PriceMilli, int64(item.Shares))
			amounts[i] = item.AmountCents
		}
		allocated := domain.AllocateOrderFee(
			domain.ComputeOrderFee(amounts, instrument, setting, buy),
			amounts, instrument, setting, buy)
		for i := range orderTrades {
			item := &orderTrades[i]
			item.FeeCents = allocated[i].Total
			item.Commission = allocated[i].Commission
			item.StampDuty = allocated[i].StampDuty
			item.TransferFee = allocated[i].TransferFee
			if err := updateStockTradeRowTx(ctx, tx, *item); err != nil {
				return err
			}
		}
		return rebuildStockTrades(ctx, tx, request.AccountID)
	})
}

// DeleteStockOrder removes every fill of one order and replays the account.
func (w *Workspace) DeleteStockOrder(ctx context.Context, accountID, orderID string) error {
	if orderID == "" {
		return invalidf("交易记录不存在")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		return deleteStockOrderTx(ctx, tx, accountID, orderID)
	})
}

// RollbackStockOperation undoes the newest operation. The log entry is popped
// even when its target is already gone.
func (w *Workspace) RollbackStockOperation(ctx context.Context, accountID string) (domain.StockOperationRollback, error) {
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockOperationRollback{}, err
	}
	var outcome domain.StockOperationRollback
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		var operation domain.StockOperation
		err := tx.QueryRowContext(ctx,
			"SELECT "+stockOperationColumns+" FROM stock_operation WHERE account_id = ? "+
				"ORDER BY created_at DESC, id DESC LIMIT 1", accountID).
			Scan(&operation.ID, &operation.AccountID, &operation.Kind, &operation.Action,
				&operation.Detail, &operation.TargetID, &operation.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return invalidf("没有可回滚的操作")
		}
		if err != nil {
			return fmt.Errorf("读取操作记录失败：%w", err)
		}
		skipped := false
		switch operation.Kind {
		case domain.StockOperationOrder:
			if err := deleteStockOrderTx(ctx, tx, accountID, operation.TargetID); err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					skipped = true
				} else {
					return err
				}
			}
		case domain.StockOperationFund:
			var eventType string
			var amountChange int64
			err := tx.QueryRowContext(ctx,
				"SELECT event_type, amount_change_cents FROM stock_fund_record WHERE id = ? AND account_id = ?",
				operation.TargetID, accountID).Scan(&eventType, &amountChange)
			if errors.Is(err, sql.ErrNoRows) {
				skipped = true
			} else if err != nil {
				return fmt.Errorf("读取资金记录失败：%w", err)
			} else {
				if eventType == domain.StockEventAddPrincipal {
					account, err := stockAccountByIDTx(ctx, tx, accountID)
					if err != nil {
						return err
					}
					restored := account.PrincipalCents - amountChange
					if restored < 0 {
						restored = 0
					}
					if _, err := tx.ExecContext(ctx,
						"UPDATE stock_account SET principal_cents = ?, updated_at = ? WHERE id = ?",
						restored, nowUnix(), accountID); err != nil {
						return fmt.Errorf("更新股票账户失败：%w", err)
					}
				}
				if _, err := tx.ExecContext(ctx,
					"DELETE FROM stock_fund_record WHERE id = ? AND account_id = ?",
					operation.TargetID, accountID); err != nil {
					return fmt.Errorf("删除资金记录失败：%w", err)
				}
				account, err := stockAccountByIDTx(ctx, tx, accountID)
				if err != nil {
					return err
				}
				if err := recalculateStockCashChain(ctx, tx, accountID, account.PrincipalCents); err != nil {
					return err
				}
			}
		default:
			skipped = true
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM stock_operation WHERE id = ?", operation.ID); err != nil {
			return fmt.Errorf("删除操作记录失败：%w", err)
		}
		outcome = domain.StockOperationRollback{Action: operation.Action, Skipped: skipped}
		return nil
	})
	if err != nil {
		return domain.StockOperationRollback{}, err
	}
	return outcome, nil
}

func deleteStockOrderTx(ctx context.Context, tx *sql.Tx, accountID, orderID string) error {
	trades, err := loadStockOrderTradesTx(ctx, tx, accountID, orderID)
	if err != nil {
		return err
	}
	if len(trades) == 0 {
		trade, err := getStockTradeTx(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if trade.AccountID != accountID {
			return notFoundf("交易记录不存在")
		}
		trades = []domain.StockTrade{trade}
	}
	for _, trade := range trades {
		if _, err := tx.ExecContext(ctx, "DELETE FROM stock_trade WHERE id = ?", trade.ID); err != nil {
			return fmt.Errorf("删除成交失败：%w", err)
		}
	}
	return rebuildStockTrades(ctx, tx, accountID)
}

func stockOrderKey(trade domain.StockTrade) string {
	if trade.OrderID != "" {
		return trade.OrderID
	}
	return trade.ID
}

func getStockTradeTx(ctx context.Context, tx *sql.Tx, id string) (domain.StockTrade, error) {
	rows, err := tx.QueryContext(ctx, "SELECT "+stockTradeColumns+" FROM stock_trade WHERE id = ?", id)
	if err != nil {
		return domain.StockTrade{}, fmt.Errorf("读取成交失败：%w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.StockTrade{}, err
		}
		return domain.StockTrade{}, notFoundf("交易记录不存在")
	}
	return scanStockTrade(rows)
}

func loadStockOrderTradesTx(ctx context.Context, tx *sql.Tx, accountID, orderID string) ([]domain.StockTrade, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT "+stockTradeColumns+" FROM stock_trade WHERE account_id = ? AND order_id = ? "+
			"ORDER BY order_seq ASC, id ASC", accountID, orderID)
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

func updateStockTradeRowTx(ctx context.Context, tx *sql.Tx, trade domain.StockTrade) error {
	if _, err := tx.ExecContext(ctx,
		"UPDATE stock_trade SET price_milli = ?, lots = ?, shares = ?, amount_cents = ?, "+
			"fee_cents = ?, commission_cents = ?, stamp_duty_cents = ?, transfer_fee_cents = ?, "+
			"trade_time = ? WHERE id = ?",
		trade.PriceMilli, trade.Lots, trade.Shares, trade.AmountCents, trade.FeeCents,
		trade.Commission, trade.StampDuty, trade.TransferFee, trade.TradeTime, trade.ID); err != nil {
		return fmt.Errorf("更新成交失败：%w", err)
	}
	return nil
}

// rebuildStockTrades reconstructs positions, rounds, histories, buy/sell fund
// records and realized P&L from the account's stored trades.
func rebuildStockTrades(ctx context.Context, tx *sql.Tx, accountID string) error {
	trades, err := loadAllStockTradesTx(ctx, tx, accountID)
	if err != nil {
		return err
	}
	sortStockTradesForReplay(trades)

	existingRounds, err := loadStockRoundsTx(ctx, tx, accountID)
	if err != nil {
		return err
	}
	roundMeta := make(map[string]domain.StockTradeRound, len(existingRounds))
	for _, round := range existingRounds {
		roundMeta[stockRoundKey(round.StockCode, round.RoundNo)] = round
	}
	existingPositions, err := loadStockPositionsTx(ctx, tx, accountID)
	if err != nil {
		return err
	}
	positions := make(map[string]*domain.StockPosition, len(existingPositions))
	for i := range existingPositions {
		position := existingPositions[i]
		position.Quantity = 0
		position.TotalCostCents = 0
		position.RealizedPnlCents = 0
		positions[position.StockCode] = &position
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM stock_fund_record WHERE account_id = ? AND event_type IN (?, ?)",
		accountID, domain.StockEventBuy, domain.StockEventSell); err != nil {
		return fmt.Errorf("重建资金记录失败：%w", err)
	}

	histories := make(map[string]domain.StockTradeHistory)
	roundCounts := make(map[string]int)
	cycleOpenedAt := make(map[string]int64)
	cycleIndexes := make(map[string][]int)
	usedRounds := make(map[string]bool)
	var lastRecordCreatedAt int64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(created_at), 0) FROM stock_fund_record WHERE account_id = ?",
		accountID).Scan(&lastRecordCreatedAt); err != nil {
		return fmt.Errorf("重建资金记录失败：%w", err)
	}

	var current *replayOrder
	flush := func() error {
		if current == nil {
			return nil
		}
		order := current
		current = nil
		var amountChange int64
		eventType := domain.StockEventSell
		eventText := "卖出 " + order.stockName + " " + itoaInt(order.lots) + "手"
		var netPnl *int64
		if order.isBuy {
			amountChange = -(order.amount + order.fee)
			eventType = domain.StockEventBuy
			eventText = "买入 " + order.stockName + " " + itoaInt(order.lots) + "手"
		} else {
			amountChange = order.amount - order.fee
			value := order.netPnl
			netPnl = &value
		}
		if order.lots > 0 {
			createdAt := order.createdAt
			if createdAt <= lastRecordCreatedAt {
				createdAt = lastRecordCreatedAt + 1
			}
			lastRecordCreatedAt = createdAt
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_fund_record ("+stockFundColumns+") VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?)",
				newID(), accountID, unixToStockDate(order.lastTime), eventType, eventText,
				amountChange, netPnl, domain.TradeOrderRemark(order.stockName, int64(order.lots), order.amount, len(order.indexes)),
				createdAt); err != nil {
				return fmt.Errorf("重建资金记录失败：%w", err)
			}
		}
		roundID := ""
		position := positions[order.stockCode]
		if position != nil && order.isClose && position.Quantity == 0 {
			history, ok := histories[order.stockCode]
			if !ok {
				var err error
				history, err = getOrCreateStockHistoryTx(ctx, tx, accountID, order.stockCode, order.stockName)
				if err != nil {
					return err
				}
				histories[order.stockCode] = history
			}
			roundNo := roundCounts[order.stockCode] + 1
			openedAt := cycleOpenedAt[order.stockCode]
			if openedAt == 0 {
				openedAt = order.firstTime
			}
			key := stockRoundKey(order.stockCode, roundNo)
			existing, ok := roundMeta[key]
			if ok {
				if _, err := tx.ExecContext(ctx,
					"UPDATE stock_trade_round SET history_id = ?, opened_at = ?, closed_at = ? WHERE id = ?",
					history.ID, openedAt, order.lastTime, existing.ID); err != nil {
					return fmt.Errorf("更新轮次失败：%w", err)
				}
				roundID = existing.ID
			} else {
				roundID = newID()
				if _, err := tx.ExecContext(ctx,
					"INSERT INTO stock_trade_round (id, account_id, stock_code, history_id, round_no, "+
						"opened_at, closed_at, tag, review, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?)",
					roundID, accountID, order.stockCode, history.ID, roundNo, openedAt,
					order.lastTime, domain.DefaultStockTradeTag, nowUnix()); err != nil {
					return fmt.Errorf("创建轮次失败：%w", err)
				}
				roundMeta[key] = domain.StockTradeRound{
					ID: roundID, AccountID: accountID, StockCode: order.stockCode,
					HistoryID: history.ID, RoundNo: roundNo, OpenedAt: openedAt,
					ClosedAt: order.lastTime, Tag: domain.DefaultStockTradeTag,
				}
			}
			usedRounds[roundID] = true
			ids := make([]string, 0, len(cycleIndexes[order.stockCode]))
			for _, index := range cycleIndexes[order.stockCode] {
				ids = append(ids, trades[index].ID)
			}
			for _, id := range ids {
				if _, err := tx.ExecContext(ctx,
					"UPDATE stock_trade SET round_id = ? WHERE id = ?", roundID, id); err != nil {
					return fmt.Errorf("归档成交失败：%w", err)
				}
			}
			roundCounts[order.stockCode] = roundNo
			delete(cycleOpenedAt, order.stockCode)
			delete(cycleIndexes, order.stockCode)
		}
		for _, index := range order.indexes {
			trade := &trades[index]
			var realized any
			if trade.RealizedPnl != nil {
				realized = *trade.RealizedPnl
			}
			if _, err := tx.ExecContext(ctx,
				"UPDATE stock_trade SET round_id = ?, realized_pnl_cents = ? WHERE id = ?",
				roundID, realized, trade.ID); err != nil {
				return fmt.Errorf("更新成交失败：%w", err)
			}
		}
		return nil
	}

	for index := range trades {
		trade := &trades[index]
		key := stockOrderKey(*trade)
		if current == nil || current.key != key {
			if err := flush(); err != nil {
				return err
			}
			current = &replayOrder{
				key: key, stockCode: trade.StockCode, stockName: trade.StockName,
				isBuy: domain.IsBuy(trade.TradeType), isClose: trade.TradeType == domain.TradeClose,
				firstTime: trade.TradeTime, createdAt: trade.CreatedAt,
			}
		}
		if trade.StockName != "" {
			current.stockName = trade.StockName
		}
		current.amount += trade.AmountCents
		current.fee += trade.FeeCents
		current.lots += trade.Lots
		current.lastTime = trade.TradeTime
		current.indexes = append(current.indexes, index)
		if _, ok := positions[trade.StockCode]; !ok {
			now := nowUnix()
			position := &domain.StockPosition{
				ID: newID(), AccountID: accountID, StockCode: trade.StockCode,
				StockName: trade.StockName, CreatedAt: now, UpdatedAt: now,
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_position (id, account_id, stock_code, stock_name, quantity, "+
					"total_cost_cents, realized_pnl_cents, review, tag, created_at, updated_at) "+
					"VALUES (?, ?, ?, ?, 0, 0, 0, '', '', ?, ?)",
				position.ID, position.AccountID, position.StockCode, position.StockName,
				position.CreatedAt, position.UpdatedAt); err != nil {
				return fmt.Errorf("创建持仓失败：%w", err)
			}
			positions[trade.StockCode] = position
		}
		position := positions[trade.StockCode]
		if current.stockName != "" {
			position.StockName = current.stockName
		}
		trade.RoundID = ""
		trade.RealizedPnl = nil
		if domain.IsBuy(trade.TradeType) {
			if _, ok := cycleIndexes[trade.StockCode]; !ok {
				cycleOpenedAt[trade.StockCode] = trade.TradeTime
				cycleIndexes[trade.StockCode] = nil
			}
			position.Quantity += trade.Shares
			position.TotalCostCents += trade.AmountCents + trade.FeeCents
			cycleIndexes[trade.StockCode] = append(cycleIndexes[trade.StockCode], index)
			continue
		}
		if cycleOpenedAt[trade.StockCode] == 0 {
			cycleOpenedAt[trade.StockCode] = trade.TradeTime
		}
		if trade.Shares > position.Quantity {
			return invalidf("卖出数量超过持仓（当前 %d 股）", position.Quantity)
		}
		costBasis := domain.CostBasisOf(position.TotalCostCents, trade.Shares, position.Quantity)
		realized := trade.AmountCents - trade.FeeCents - costBasis
		trade.RealizedPnl = &realized
		current.netPnl += realized
		cycleIndexes[trade.StockCode] = append(cycleIndexes[trade.StockCode], index)
		position.Quantity -= trade.Shares
		position.TotalCostCents -= costBasis
		position.RealizedPnlCents += realized
		if position.Quantity == 0 {
			position.TotalCostCents = 0
		}
	}
	if err := flush(); err != nil {
		return err
	}

	for _, round := range existingRounds {
		if usedRounds[round.ID] {
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM stock_day_review WHERE round_id = ?", round.ID); err != nil {
			return fmt.Errorf("删除每日复盘失败：%w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM stock_trade_round WHERE id = ?", round.ID); err != nil {
			return fmt.Errorf("删除轮次失败：%w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM stock_trade_history WHERE account_id = ? AND id NOT IN ("+
			"SELECT history_id FROM stock_trade_round WHERE account_id = ?)",
		accountID, accountID); err != nil {
		return fmt.Errorf("清理交易历史失败：%w", err)
	}
	for _, position := range positions {
		if _, open := cycleIndexes[position.StockCode]; !open {
			position.Review = ""
		}
		if err := updateStockPositionTx(ctx, tx, *position); err != nil {
			return err
		}
	}
	account, err := stockAccountByIDTx(ctx, tx, accountID)
	if err != nil {
		return err
	}
	return recalculateStockCashChain(ctx, tx, accountID, account.PrincipalCents)
}

func stockRoundKey(stockCode string, roundNo int) string {
	return stockCode + "#" + itoaInt(roundNo)
}

func loadStockRoundsTx(ctx context.Context, tx *sql.Tx, accountID string) ([]domain.StockTradeRound, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id, account_id, stock_code, history_id, round_no, opened_at, closed_at, tag, review, created_at "+
			"FROM stock_trade_round WHERE account_id = ?", accountID)
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

func loadStockPositionsTx(ctx context.Context, tx *sql.Tx, accountID string) ([]domain.StockPosition, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id, account_id, stock_code, stock_name, quantity, total_cost_cents, realized_pnl_cents, "+
			"review, tag, created_at, updated_at FROM stock_position WHERE account_id = ?", accountID)
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
