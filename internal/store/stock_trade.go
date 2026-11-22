package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

const stockTradeColumns = "id, account_id, stock_code, stock_name, trade_type, round_id, order_id, " +
	"order_seq, price_milli, lots, shares, amount_cents, fee_cents, commission_cents, " +
	"stamp_duty_cents, transfer_fee_cents, realized_pnl_cents, trade_time, remark, created_at, " +
	"round_commission_rate, round_min_commission_cents, round_stamp_duty_rate, round_transfer_fee_rate"

// CreateStockOrder records one order (one or more fills). It is the transactional
// counterpart of the stock_trade_create command.
func (w *Workspace) CreateStockOrder(ctx context.Context, request repository.StockOrderRequest) ([]domain.StockTrade, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	account, err := stockAccountByID(ctx, db, request.AccountID)
	if err != nil {
		return nil, err
	}
	fills := normalizeStockFills(request.Fills)
	buy, sell := domain.IsBuy(request.TradeType), domain.IsSell(request.TradeType)
	if !buy && !sell {
		return nil, fmt.Errorf("%w：无效的交易类型", repository.ErrInvalid)
	}
	archiveOnly := len(fills) == 0 && request.TradeType == domain.TradeClose
	if len(fills) == 0 && !archiveOnly {
		return nil, fmt.Errorf("%w：成交明细不能为空", repository.ErrInvalid)
	}
	for _, fill := range fills {
		if fill.PriceMilli <= 0 {
			return nil, fmt.Errorf("%w：成交价必须大于 0", repository.ErrInvalid)
		}
		if fill.Lots <= 0 {
			return nil, fmt.Errorf("%w：手数必须大于 0", repository.ErrInvalid)
		}
	}
	tradeTime := request.TradeTime
	if tradeTime <= 0 {
		tradeTime = nowUnix()
	}
	now := nowUnix()
	orderID := newID()
	trades := make([]domain.StockTrade, 0, len(fills)+1)
	amounts := make([]int64, 0, len(fills))
	var totalAmount int64
	var totalLots int
	for index, fill := range fills {
		shares := int(domain.SharesOfLots(int64(fill.Lots)))
		amount := domain.AmountOf(fill.PriceMilli, int64(shares))
		amounts = append(amounts, amount)
		totalAmount += amount
		totalLots += fill.Lots
		trades = append(trades, domain.StockTrade{
			ID: newID(), AccountID: request.AccountID, StockCode: request.StockCode,
			StockName: request.StockName, TradeType: request.TradeType,
			OrderID: orderID, OrderSeq: index + 1, PriceMilli: fill.PriceMilli,
			Lots: fill.Lots, Shares: shares, AmountCents: amount,
			TradeTime: tradeTime, Remark: request.Remark, CreatedAt: now + int64(index),
		})
	}
	if archiveOnly {
		trades = append(trades, domain.StockTrade{
			ID: newID(), AccountID: request.AccountID, StockCode: request.StockCode,
			StockName: request.StockName, TradeType: domain.TradeClose,
			OrderID: orderID, OrderSeq: 1, TradeTime: tradeTime,
			Remark: request.Remark, CreatedAt: now,
		})
	}

	err = inTx(ctx, db, func(tx *sql.Tx) error {
		tags, err := stockTradeTagsTx(ctx, tx, request.AccountID)
		if err != nil {
			return err
		}
		if request.Tag != "" && !containsString(tags, request.Tag) {
			return fmt.Errorf("%w：无效的交易标签", repository.ErrInvalid)
		}
		position, err := getOrCreateStockPositionTx(ctx, tx, request.AccountID, request.StockCode, request.StockName)
		if err != nil {
			return err
		}
		position.StockName = request.StockName
		if request.Tag != "" {
			position.Tag = request.Tag
		}
		first, err := firstUnattachedStockTradeTx(ctx, tx, request.AccountID, request.StockCode)
		if err != nil {
			return err
		}
		opening := buy && position.Quantity == 0 && first == nil
		setting, roundFee, err := resolveRoundFeeTx(ctx, tx, request.AccountID, request.RoundFee, opening, first)
		if err != nil {
			return err
		}
		instrument := domain.TradeInstrumentOf(request.StockCode)
		var orderFee domain.FeeBreakdown
		var allocated []domain.FeeBreakdown
		if archiveOnly {
			orderFee = domain.FeeBreakdown{}
			allocated = []domain.FeeBreakdown{{}}
		} else {
			orderFee = domain.ComputeOrderFee(amounts, instrument, setting, buy)
			allocated = domain.AllocateOrderFee(orderFee, amounts, instrument, setting, buy)
		}
		for i := range trades {
			round := roundFee
			trades[i].RoundFee = &round
		}
		if sell {
			soldShares := 0
			for _, trade := range trades {
				soldShares += trade.Shares
			}
			if soldShares > position.Quantity {
				return fmt.Errorf("%w：卖出数量超过持仓（当前 %d 股）", repository.ErrInvalid, position.Quantity)
			}
			if request.TradeType == domain.TradeClose && !archiveOnly && soldShares != position.Quantity {
				return fmt.Errorf("%w：清仓必须全部卖出（当前 %d 股）", repository.ErrInvalid, position.Quantity)
			}
		}
		if archiveOnly && position.Quantity != 0 {
			return fmt.Errorf("%w：清仓请填写成交明细（当前 %d 股）", repository.ErrInvalid, position.Quantity)
		}

		var realizedTotal int64
		for i := range trades {
			trade := &trades[i]
			if !archiveOnly {
				trade.FeeCents = allocated[i].Total
				trade.Commission = allocated[i].Commission
				trade.StampDuty = allocated[i].StampDuty
				trade.TransferFee = allocated[i].TransferFee
			}
			if buy {
				position.Quantity += trade.Shares
				position.TotalCostCents += trade.AmountCents + trade.FeeCents
				continue
			}
			if trade.Shares == 0 {
				zero := int64(0)
				trade.RealizedPnl = &zero
				continue
			}
			costBasis := domain.CostBasisOf(position.TotalCostCents, trade.Shares, position.Quantity)
			realized := trade.AmountCents - trade.FeeCents - costBasis
			trade.RealizedPnl = &realized
			realizedTotal += realized
			position.Quantity -= trade.Shares
			position.TotalCostCents -= costBasis
			position.RealizedPnlCents += realized
			if position.Quantity == 0 {
				position.TotalCostCents = 0
			}
		}

		if request.TradeType == domain.TradeClose && position.Quantity == 0 {
			roundID, err := closeStockRoundTx(ctx, tx, request.AccountID, request.StockCode,
				request.StockName, tradeTime, request.Tag, position.Review)
			if err != nil {
				return err
			}
			for i := range trades {
				trades[i].RoundID = roundID
			}
			position.Review = ""
			position.Tag = ""
		}
		if err := updateStockPositionTx(ctx, tx, position); err != nil {
			return err
		}
		if totalLots > 0 {
			amountChange := totalAmount - orderFee.Total
			eventType := domain.StockEventSell
			eventText := "卖出 " + request.StockName + " " + itoaInt(totalLots) + "手"
			var netPnl *int64
			if buy {
				amountChange = -(totalAmount + orderFee.Total)
				eventType = domain.StockEventBuy
				eventText = "买入 " + request.StockName + " " + itoaInt(totalLots) + "手"
			} else {
				netPnl = &realizedTotal
			}
			createdAt, err := nextFundCreatedAt(ctx, tx, request.AccountID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_fund_record ("+stockFundColumns+") VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?)",
				newID(), request.AccountID, unixToStockDate(tradeTime), eventType, eventText,
				amountChange, netPnl, domain.TradeOrderRemark(request.StockName, int64(totalLots), totalAmount, len(trades)),
				createdAt); err != nil {
				return fmt.Errorf("写入资金记录失败：%w", err)
			}
			if err := recalculateStockCashChain(ctx, tx, request.AccountID, account.PrincipalCents); err != nil {
				return err
			}
		}
		for i := range trades {
			if err := insertStockTradeTx(ctx, tx, trades[i]); err != nil {
				return err
			}
		}
		detail := request.StockName + " " + request.StockCode + " · " + itoaInt(totalLots) + " 手 · ¥" + domain.CentsToYuan(totalAmount)
		if archiveOnly {
			detail = request.StockName + " " + request.StockCode + " · 归档本轮（0 手）"
		}
		return createStockOperation(ctx, tx, request.AccountID, domain.StockOperationOrder,
			domain.TradeTypeLabel(request.TradeType), detail, orderID)
	})
	if err != nil {
		return nil, err
	}
	return trades, nil
}

func normalizeStockFills(fills []repository.StockFill) []repository.StockFill {
	result := make([]repository.StockFill, 0, len(fills))
	for _, fill := range fills {
		if fill.PriceMilli <= 0 && fill.Lots <= 0 {
			continue
		}
		result = append(result, fill)
	}
	return result
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func itoaInt(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		digits[index] = '-'
	}
	return string(digits[index:])
}

func unixToStockDate(seconds int64) string {
	return time.Unix(seconds, 0).Format(domain.DiaryDateLayout)
}

func stockTradeTagsTx(ctx context.Context, tx *sql.Tx, accountID string) ([]string, error) {
	var raw string
	err := tx.QueryRowContext(ctx,
		"SELECT tags FROM stock_trade_tag_setting WHERE account_id = ?", accountID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		raw = ""
	} else if err != nil {
		return nil, fmt.Errorf("读取交易标签失败：%w", err)
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil && len(tags) > 0 && allStrings(tags) {
		return tags, nil
	}
	defaults := make([]string, 0, len(domain.DefaultStockTradeTags))
	defaults = append(defaults, domain.DefaultStockTradeTags[:]...)
	encoded, _ := json.Marshal(defaults)
	now := nowUnix()
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_trade_tag_setting (id, account_id, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?) "+
			"ON CONFLICT(account_id) DO UPDATE SET tags = excluded.tags, updated_at = excluded.updated_at",
		newID(), accountID, string(encoded), now, now); err != nil {
		return nil, fmt.Errorf("修复交易标签失败：%w", err)
	}
	return defaults, nil
}

func allStrings(values []string) bool {
	for _, value := range values {
		if value == "" {
			return false
		}
	}
	return true
}

func getOrCreateStockFeeSettingTx(ctx context.Context, tx *sql.Tx, accountID string) (domain.StockFeeSetting, error) {
	var setting domain.StockFeeSetting
	err := tx.QueryRowContext(ctx,
		"SELECT id, account_id, commission_rate, min_commission_cents, stamp_duty_rate, "+
			"transfer_fee_rate, created_at, updated_at FROM stock_fee_setting WHERE account_id = ?",
		accountID).Scan(&setting.ID, &setting.AccountID, &setting.CommissionRate,
		&setting.MinCommission, &setting.StampDutyRate, &setting.TransferFeeRate,
		&setting.CreatedAt, &setting.UpdatedAt)
	if err == nil {
		return setting, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.StockFeeSetting{}, fmt.Errorf("读取交易费用设置失败：%w", err)
	}
	setting = domain.DefaultStockFeeSetting()
	setting.ID, setting.AccountID, setting.CreatedAt, setting.UpdatedAt = newID(), accountID, nowUnix(), nowUnix()
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_fee_setting (id, account_id, commission_rate, min_commission_cents, "+
			"stamp_duty_rate, transfer_fee_rate, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		setting.ID, setting.AccountID, setting.CommissionRate, setting.MinCommission,
		setting.StampDutyRate, setting.TransferFeeRate, setting.CreatedAt, setting.UpdatedAt); err != nil {
		return domain.StockFeeSetting{}, fmt.Errorf("创建交易费用设置失败：%w", err)
	}
	return setting, nil
}

func resolveRoundFeeTx(
	ctx context.Context, tx *sql.Tx, accountID string, requested *domain.RoundFee,
	opening bool, first *domain.StockTrade,
) (domain.StockFeeSetting, domain.RoundFee, error) {
	if opening {
		if requested != nil {
			if err := domain.ValidateRoundFee(*requested); err != nil {
				return domain.StockFeeSetting{}, domain.RoundFee{}, fmt.Errorf("%w：%s", repository.ErrInvalid, err.Error())
			}
			return domain.SettingOfRoundFee(*requested), *requested, nil
		}
		setting, err := getOrCreateStockFeeSettingTx(ctx, tx, accountID)
		if err != nil {
			return domain.StockFeeSetting{}, domain.RoundFee{}, err
		}
		return setting, domain.RoundFeeOfSetting(setting), nil
	}
	if requested != nil {
		return domain.StockFeeSetting{}, domain.RoundFee{}, fmt.Errorf("%w：本轮的费用设置在建仓时确定，不可更改", repository.ErrInvalid)
	}
	setting, err := getOrCreateStockFeeSettingTx(ctx, tx, accountID)
	if err != nil {
		return domain.StockFeeSetting{}, domain.RoundFee{}, err
	}
	if first != nil && first.RoundFee != nil {
		return domain.SettingOfRoundFee(*first.RoundFee), *first.RoundFee, nil
	}
	return setting, domain.RoundFeeOfSetting(setting), nil
}

func firstUnattachedStockTradeTx(ctx context.Context, tx *sql.Tx, accountID, stockCode string) (*domain.StockTrade, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT "+stockTradeColumns+" FROM stock_trade WHERE account_id = ? AND stock_code = ? "+
			"AND (round_id = '' OR round_id IS NULL) ORDER BY trade_time ASC, created_at ASC, id ASC LIMIT 1",
		accountID, stockCode)
	if err != nil {
		return nil, fmt.Errorf("读取成交失败：%w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	trade, err := scanStockTrade(rows)
	if err != nil {
		return nil, err
	}
	return &trade, nil
}

func getOrCreateStockPositionTx(ctx context.Context, tx *sql.Tx, accountID, stockCode, stockName string) (domain.StockPosition, error) {
	var position domain.StockPosition
	err := tx.QueryRowContext(ctx,
		"SELECT id, account_id, stock_code, stock_name, quantity, total_cost_cents, realized_pnl_cents, "+
			"review, tag, created_at, updated_at FROM stock_position WHERE account_id = ? AND stock_code = ?",
		accountID, stockCode).Scan(&position.ID, &position.AccountID, &position.StockCode,
		&position.StockName, &position.Quantity, &position.TotalCostCents,
		&position.RealizedPnlCents, &position.Review, &position.Tag,
		&position.CreatedAt, &position.UpdatedAt)
	if err == nil {
		return position, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.StockPosition{}, fmt.Errorf("读取持仓失败：%w", err)
	}
	now := nowUnix()
	position = domain.StockPosition{
		ID: newID(), AccountID: accountID, StockCode: stockCode, StockName: stockName,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_position (id, account_id, stock_code, stock_name, quantity, total_cost_cents, "+
			"realized_pnl_cents, review, tag, created_at, updated_at) VALUES (?, ?, ?, ?, 0, 0, 0, '', '', ?, ?)",
		position.ID, position.AccountID, position.StockCode, position.StockName,
		position.CreatedAt, position.UpdatedAt); err != nil {
		return domain.StockPosition{}, fmt.Errorf("创建持仓失败：%w", err)
	}
	return position, nil
}

func updateStockPositionTx(ctx context.Context, tx *sql.Tx, position domain.StockPosition) error {
	if _, err := tx.ExecContext(ctx,
		"UPDATE stock_position SET stock_name = ?, quantity = ?, total_cost_cents = ?, "+
			"realized_pnl_cents = ?, review = ?, tag = ?, updated_at = ? WHERE id = ?",
		position.StockName, position.Quantity, position.TotalCostCents,
		position.RealizedPnlCents, position.Review, position.Tag, nowUnix(), position.ID); err != nil {
		return fmt.Errorf("更新持仓失败：%w", err)
	}
	return nil
}

func closeStockRoundTx(ctx context.Context, tx *sql.Tx, accountID, stockCode, stockName string, closedAt int64, tag, review string) (string, error) {
	history, err := getOrCreateStockHistoryTx(ctx, tx, accountID, stockCode, stockName)
	if err != nil {
		return "", err
	}
	var count int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stock_trade_round WHERE history_id = ?", history.ID).Scan(&count); err != nil {
		return "", fmt.Errorf("读取轮次失败：%w", err)
	}
	var openedAt int64
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MIN(trade_time), 0) FROM stock_trade WHERE account_id = ? AND stock_code = ? "+
			"AND (round_id = '' OR round_id IS NULL)", accountID, stockCode).Scan(&openedAt); err != nil {
		return "", fmt.Errorf("读取成交失败：%w", err)
	}
	if openedAt == 0 {
		openedAt = closedAt
	}
	if tag == "" {
		tag = domain.DefaultStockTradeTag
	}
	roundID := newID()
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_trade_round (id, account_id, stock_code, history_id, round_no, "+
			"opened_at, closed_at, tag, review, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		roundID, accountID, stockCode, history.ID, count+1, openedAt, closedAt,
		tag, strings.TrimSpace(review), nowUnix()); err != nil {
		return "", fmt.Errorf("创建轮次失败：%w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE stock_trade SET round_id = ? WHERE account_id = ? AND stock_code = ? "+
			"AND (round_id = '' OR round_id IS NULL)", roundID, accountID, stockCode); err != nil {
		return "", fmt.Errorf("归档成交失败：%w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE stock_day_review SET round_id = ? WHERE account_id = ? AND stock_code = ? AND round_id = ''",
		roundID, accountID, stockCode); err != nil {
		return "", fmt.Errorf("归档每日复盘失败：%w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE stock_ledger SET round_id = ?, updated_at = ? WHERE account_id = ? AND stock_code = ? AND round_id IS NULL",
		roundID, nowUnix(), accountID, stockCode); err != nil {
		return "", fmt.Errorf("归档交易台账失败：%w", err)
	}
	return roundID, nil
}

func getOrCreateStockHistoryTx(ctx context.Context, tx *sql.Tx, accountID, stockCode, stockName string) (domain.StockTradeHistory, error) {
	var history domain.StockTradeHistory
	err := tx.QueryRowContext(ctx,
		"SELECT id, account_id, stock_code, stock_name, created_at, updated_at "+
			"FROM stock_trade_history WHERE account_id = ? AND stock_code = ?",
		accountID, stockCode).Scan(&history.ID, &history.AccountID, &history.StockCode,
		&history.StockName, &history.CreatedAt, &history.UpdatedAt)
	if err == nil {
		if history.StockName != stockName {
			if _, err := tx.ExecContext(ctx,
				"UPDATE stock_trade_history SET stock_name = ? WHERE id = ?", stockName, history.ID); err != nil {
				return domain.StockTradeHistory{}, fmt.Errorf("更新历史股票名称失败：%w", err)
			}
			history.StockName = stockName
		}
		return history, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.StockTradeHistory{}, fmt.Errorf("读取交易历史失败：%w", err)
	}
	now := nowUnix()
	history = domain.StockTradeHistory{
		ID: newID(), AccountID: accountID, StockCode: stockCode, StockName: stockName,
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_trade_history (id, account_id, stock_code, stock_name, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?)",
		history.ID, history.AccountID, history.StockCode, history.StockName,
		history.CreatedAt, history.UpdatedAt); err != nil {
		return domain.StockTradeHistory{}, fmt.Errorf("创建交易历史失败：%w", err)
	}
	return history, nil
}

func insertStockTradeTx(ctx context.Context, tx *sql.Tx, trade domain.StockTrade) error {
	var roundCommission, roundMin, roundStamp, roundTransfer any
	if trade.RoundFee != nil {
		roundCommission = trade.RoundFee.CommissionRate
		roundMin = trade.RoundFee.MinCommission
		roundStamp = trade.RoundFee.StampDutyRate
		roundTransfer = trade.RoundFee.TransferFeeRate
	}
	var realized any
	if trade.RealizedPnl != nil {
		realized = *trade.RealizedPnl
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO stock_trade ("+stockTradeColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		trade.ID, trade.AccountID, trade.StockCode, trade.StockName, trade.TradeType,
		trade.RoundID, trade.OrderID, trade.OrderSeq, trade.PriceMilli, trade.Lots,
		trade.Shares, trade.AmountCents, trade.FeeCents, trade.Commission,
		trade.StampDuty, trade.TransferFee, realized, trade.TradeTime, trade.Remark,
		trade.CreatedAt, roundCommission, roundMin, roundStamp, roundTransfer); err != nil {
		return fmt.Errorf("写入成交失败：%w", err)
	}
	return nil
}

func scanStockTrade(scanner interface{ Scan(...any) error }) (domain.StockTrade, error) {
	var trade domain.StockTrade
	var realized sql.NullInt64
	var roundCommission, roundStamp, roundTransfer sql.NullFloat64
	var roundMin sql.NullInt64
	if err := scanner.Scan(
		&trade.ID, &trade.AccountID, &trade.StockCode, &trade.StockName, &trade.TradeType,
		&trade.RoundID, &trade.OrderID, &trade.OrderSeq, &trade.PriceMilli, &trade.Lots,
		&trade.Shares, &trade.AmountCents, &trade.FeeCents, &trade.Commission,
		&trade.StampDuty, &trade.TransferFee, &realized, &trade.TradeTime, &trade.Remark,
		&trade.CreatedAt, &roundCommission, &roundMin, &roundStamp, &roundTransfer,
	); err != nil {
		return domain.StockTrade{}, fmt.Errorf("读取成交失败：%w", err)
	}
	if realized.Valid {
		value := realized.Int64
		trade.RealizedPnl = &value
	}
	if roundCommission.Valid && roundMin.Valid && roundStamp.Valid && roundTransfer.Valid {
		trade.RoundFee = &domain.RoundFee{
			CommissionRate: roundCommission.Float64, MinCommission: roundMin.Int64,
			StampDutyRate: roundStamp.Float64, TransferFeeRate: roundTransfer.Float64,
		}
	}
	return trade, nil
}

// loadAllStockTradesTx reads every trade of one account in time order.
func loadAllStockTradesTx(ctx context.Context, tx *sql.Tx, accountID string) ([]domain.StockTrade, error) {
	rows, err := tx.QueryContext(ctx,
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

// sortStockTradesForReplay is the canonical replay order: time, creation, buys
// before sells at the same instant, then id.
func sortStockTradesForReplay(trades []domain.StockTrade) {
	sort.SliceStable(trades, func(i, j int) bool {
		left, right := trades[i], trades[j]
		if left.TradeTime != right.TradeTime {
			return left.TradeTime < right.TradeTime
		}
		if left.CreatedAt != right.CreatedAt {
			return left.CreatedAt < right.CreatedAt
		}
		leftBuy, rightBuy := domain.IsBuy(left.TradeType), domain.IsBuy(right.TradeType)
		if leftBuy != rightBuy {
			return leftBuy
		}
		return left.ID < right.ID
	})
}
