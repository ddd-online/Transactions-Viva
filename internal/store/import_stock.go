package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type stockImportCounts struct {
	Accounts   int
	Funds      int
	Positions  int
	Trades     int
	Rounds     int
	Lessons    int
	LadderDays int
}

type stockAccountSeed struct {
	ledgerID   string
	accountID  string
	name       string
	principal  int64
	createdAt  int64
	updatedAt  int64
	known      bool
	ledgerName string
	order      int
}

// copyStock maps every old ledger's stock rows into a workspace stock account.
func copyStock(source *sql.DB, tx *sql.Tx) (stockImportCounts, error) {
	var counts stockImportCounts
	ledgerNames := oldLedgerNames(source)
	ledgerOrder := oldLedgerOrder(source)
	seeds, err := stockAccountSeeds(source, ledgerNames, ledgerOrder)
	if err != nil {
		return counts, err
	}
	if len(seeds) == 0 {
		return counts, nil
	}
	usedNames := make(map[string]bool)
	accountByLedger := make(map[string]string)
	now := nowUnix()
	for i := range seeds {
		seed := &seeds[i]
		name := strings.TrimSpace(seed.name)
		if name == "" {
			if seed.ledgerID == "" {
				name = "未归属股票账户"
			} else {
				name = "股票账户 " + seed.ledgerID
			}
		}
		base := name
		for suffix := 2; usedNames[name]; suffix++ {
			name = fmt.Sprintf("%s（%d）", base, suffix)
		}
		usedNames[name] = true
		description := ""
		if seed.ledgerID == "" {
			description = "旧库中没有对应账本的数据"
		} else if seed.known {
			description = "从旧账本「" + seed.ledgerName + "」导入"
		} else {
			description = "旧库中没有对应账本的数据"
		}
		createdAt, updatedAt := seed.createdAt, seed.updatedAt
		if createdAt == 0 {
			createdAt = now
		}
		if updatedAt == 0 {
			updatedAt = createdAt
		}
		if _, err := tx.Exec(
			"INSERT INTO stock_account (id, name, description, principal_cents, sort_order, created_at, updated_at) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?)",
			seed.accountID, name, description, seed.principal, seed.order, createdAt, updatedAt); err != nil {
			return counts, fmt.Errorf("导入股票账户失败：%w", err)
		}
		accountByLedger[seed.ledgerID] = seed.accountID
		counts.Accounts++
	}

	if exists, _ := tableExists(source, "stock_fee_setting"); exists {
		count, err := copyStockFeeSettings(source, tx, accountByLedger)
		if err != nil {
			return counts, err
		}
		_ = count
	}
	if exists, _ := tableExists(source, "stock_trade_tag_setting"); exists {
		if err := copyStockTagSettings(source, tx, accountByLedger); err != nil {
			return counts, err
		}
	}
	if exists, _ := tableExists(source, "stock_review_setting"); exists {
		if err := copyStockReviewSettings(source, tx, accountByLedger); err != nil {
			return counts, err
		}
	}
	if exists, _ := tableExists(source, "stock_fund_record"); exists {
		count, err := copyStockFundRecords(source, tx, accountByLedger)
		if err != nil {
			return counts, err
		}
		counts.Funds = count
	}
	if exists, _ := tableExists(source, "stock_position"); exists {
		count, err := copyStockPositions(source, tx, accountByLedger)
		if err != nil {
			return counts, err
		}
		counts.Positions = count
	}
	if exists, _ := tableExists(source, "stock_trade_history"); exists {
		if err := copyStockHistories(source, tx, accountByLedger); err != nil {
			return counts, err
		}
	}
	if exists, _ := tableExists(source, "stock_trade_round"); exists {
		count, err := copyStockRounds(source, tx, accountByLedger)
		if err != nil {
			return counts, err
		}
		counts.Rounds = count
	}
	if exists, _ := tableExists(source, "stock_trade"); exists {
		count, err := copyStockTrades(source, tx, accountByLedger)
		if err != nil {
			return counts, err
		}
		counts.Trades = count
	}
	if exists, _ := tableExists(source, "stock_day_review"); exists {
		if err := copyStockDayReviews(source, tx, accountByLedger); err != nil {
			return counts, err
		}
	}
	if exists, _ := tableExists(source, "stock_operation"); exists {
		if err := copyStockOperations(source, tx, accountByLedger); err != nil {
			return counts, err
		}
	}
	if exists, _ := tableExists(source, "stock_lesson"); exists {
		count, err := copyStockLessons(source, tx, accountByLedger)
		if err != nil {
			return counts, err
		}
		counts.Lessons = count
	}
	if exists, _ := tableExists(source, "stock_ledger"); exists {
		if err := copyStockLedgers(source, tx, accountByLedger); err != nil {
			return counts, err
		}
	}
	if exists, _ := tableExists(source, "stock_limit_day"); exists {
		count, err := copyStockLimitCache(source, tx)
		if err != nil {
			return counts, err
		}
		counts.LadderDays = count
	}
	return counts, nil
}

func oldLedgerOrder(source *sql.DB) map[string]int {
	order := make(map[string]int)
	rows, err := source.Query("SELECT id FROM ledger ORDER BY created_at ASC, id ASC")
	if err != nil {
		return order
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		order[id] = index
		index++
	}
	return order
}

func stockAccountSeeds(source *sql.DB, ledgerNames map[string]string, ledgerOrder map[string]int) ([]stockAccountSeed, error) {
	byLedger := make(map[string]*stockAccountSeed)
	for _, table := range []string{
		"stock_account", "stock_fund_record", "stock_position", "stock_trade",
		"stock_trade_history", "stock_trade_round", "stock_trade_tag_setting",
		"stock_review_setting", "stock_operation", "stock_day_review",
		"stock_lesson", "stock_ledger",
	} {
		exists, err := tableExists(source, table)
		if err != nil || !exists {
			continue
		}
		rows, err := source.Query("SELECT DISTINCT ledger_id FROM " + table)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var ledgerID string
			if err := rows.Scan(&ledgerID); err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := byLedger[ledgerID]; !ok {
				name, known := ledgerNames[ledgerID]
				byLedger[ledgerID] = &stockAccountSeed{
					ledgerID: ledgerID, name: name, known: known, ledgerName: name,
					order: 100000 + len(byLedger),
				}
				if index, ok := ledgerOrder[ledgerID]; ok {
					byLedger[ledgerID].order = index
				}
			}
		}
		rows.Close()
	}
	if exists, _ := tableExists(source, "stock_account"); exists {
		rows, err := source.Query("SELECT id, ledger_id, principal_cents, created_at, updated_at FROM stock_account")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, ledgerID string
			var principal, createdAt, updatedAt int64
			if err := rows.Scan(&id, &ledgerID, &principal, &createdAt, &updatedAt); err != nil {
				rows.Close()
				return nil, err
			}
			seed, ok := byLedger[ledgerID]
			if !ok {
				seed = &stockAccountSeed{ledgerID: ledgerID, order: 100000 + len(byLedger)}
				byLedger[ledgerID] = seed
			}
			seed.accountID = strings.TrimSpace(id)
			seed.principal = principal
			seed.createdAt = createdAt
			seed.updatedAt = updatedAt
			if len(seed.name) == 0 {
				seed.name = ledgerNames[ledgerID]
				seed.known = seed.name != ""
				seed.ledgerName = seed.name
			}
		}
		rows.Close()
	}
	seeds := make([]stockAccountSeed, 0, len(byLedger))
	for _, seed := range byLedger {
		if seed.accountID == "" {
			if seed.ledgerID == "" {
				seed.accountID = "stock-orphan"
			} else {
				seed.accountID = "stock-" + seed.ledgerID
			}
		}
		seeds = append(seeds, *seed)
	}
	sort.SliceStable(seeds, func(i, j int) bool {
		if seeds[i].order != seeds[j].order {
			return seeds[i].order < seeds[j].order
		}
		return seeds[i].ledgerID < seeds[j].ledgerID
	})
	// Account ids must be unique; old ids are UUIDs and derived ids carry the
	// ledger id, but a defensive suffix keeps a corrupted source importable.
	usedIDs := make(map[string]bool)
	for i := range seeds {
		base := seeds[i].accountID
		for suffix := 2; usedIDs[base]; suffix++ {
			base = fmt.Sprintf("%s-%d", seeds[i].accountID, suffix)
		}
		seeds[i].accountID = base
		usedIDs[base] = true
	}
	return seeds, nil
}

func copyStockFeeSettings(source *sql.DB, tx *sql.Tx, accounts map[string]string) (int, error) {
	rows, err := source.Query("SELECT id, ledger_id, commission_rate, min_commission_cents, stamp_duty_rate, " +
		"transfer_fee_rate, created_at, updated_at FROM stock_fee_setting")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, ledgerID string
		var commission, stamp, transfer float64
		var minCommission, createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &commission, &minCommission, &stamp, &transfer, &createdAt, &updatedAt); err != nil {
			return count, err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_fee_setting (id, account_id, commission_rate, min_commission_cents, "+
			"stamp_duty_rate, transfer_fee_rate, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			id, accountID, commission, minCommission, stamp, transfer, createdAt, updatedAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyStockTagSettings(source *sql.DB, tx *sql.Tx, accounts map[string]string) error {
	rows, err := source.Query("SELECT id, ledger_id, tags, created_at, updated_at FROM stock_trade_tag_setting")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ledgerID string
		var tags sql.NullString
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &tags, &createdAt, &updatedAt); err != nil {
			return err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_trade_tag_setting (id, account_id, tags, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			id, accountID, tags.String, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyStockReviewSettings(source *sql.DB, tx *sql.Tx, accounts map[string]string) error {
	hasDaily, _ := tableHasColumn(source, "stock_review_setting", "daily_template")
	daily := "''"
	if hasDaily {
		daily = "daily_template"
	}
	rows, err := source.Query("SELECT id, ledger_id, template, " + daily + ", created_at, updated_at FROM stock_review_setting")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ledgerID string
		var template, dailyTemplate sql.NullString
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &template, &dailyTemplate, &createdAt, &updatedAt); err != nil {
			return err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_review_setting (id, account_id, template, daily_template, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?)", id, accountID, template.String, dailyTemplate.String, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyStockFundRecords(source *sql.DB, tx *sql.Tx, accounts map[string]string) (int, error) {
	rows, err := source.Query("SELECT id, ledger_id, record_date, event_type, event_text, amount_change_cents, " +
		"cash_balance_cents, net_pnl_cents, remark, created_at FROM stock_fund_record")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, ledgerID, date, eventType, eventText, remark string
		var amount, balance int64
		var netPnl sql.NullInt64
		var createdAt int64
		if err := rows.Scan(&id, &ledgerID, &date, &eventType, &eventText, &amount, &balance, &netPnl, &remark, &createdAt); err != nil {
			return count, err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_fund_record (id, account_id, record_date, event_type, event_text, "+
			"amount_change_cents, cash_balance_cents, net_pnl_cents, remark, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, accountID, date, eventType, eventText, amount, balance, nullableInt(netPnl), remark, createdAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyStockPositions(source *sql.DB, tx *sql.Tx, accounts map[string]string) (int, error) {
	rows, err := source.Query("SELECT id, ledger_id, stock_code, stock_name, quantity, total_cost_cents, " +
		"realized_pnl_cents, review, created_at, updated_at, tag FROM stock_position")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, ledgerID, code, name, review, tag string
		var quantity int
		var cost, realized, createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &code, &name, &quantity, &cost, &realized, &review, &createdAt, &updatedAt, &tag); err != nil {
			return count, err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_position (id, account_id, stock_code, stock_name, quantity, "+
			"total_cost_cents, realized_pnl_cents, review, created_at, updated_at, tag) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, accountID, code, name, quantity, cost, realized, review, createdAt, updatedAt, tag); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyStockHistories(source *sql.DB, tx *sql.Tx, accounts map[string]string) error {
	rows, err := source.Query("SELECT id, ledger_id, stock_code, stock_name, created_at, updated_at FROM stock_trade_history")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ledgerID, code, name string
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &code, &name, &createdAt, &updatedAt); err != nil {
			return err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_trade_history (id, account_id, stock_code, stock_name, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?)", id, accountID, code, name, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyStockRounds(source *sql.DB, tx *sql.Tx, accounts map[string]string) (int, error) {
	rows, err := source.Query("SELECT id, ledger_id, stock_code, history_id, round_no, opened_at, closed_at, " +
		"tag, review, created_at FROM stock_trade_round")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, ledgerID, code, historyID, tag, review string
		var roundNo int
		var openedAt, closedAt, createdAt int64
		if err := rows.Scan(&id, &ledgerID, &code, &historyID, &roundNo, &openedAt, &closedAt, &tag, &review, &createdAt); err != nil {
			return count, err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_trade_round (id, account_id, stock_code, history_id, round_no, "+
			"opened_at, closed_at, tag, review, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, accountID, code, historyID, roundNo, openedAt, closedAt, tag, review, createdAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyStockTrades(source *sql.DB, tx *sql.Tx, accounts map[string]string) (int, error) {
	rows, err := source.Query("SELECT " + strings.ReplaceAll(stockTradeColumns, "account_id", "ledger_id") + " FROM stock_trade")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var trade domainScanTrade
		if err := trade.scan(rows); err != nil {
			return count, err
		}
		accountID, ok := accounts[trade.ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_trade ("+stockTradeColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			trade.id, accountID, trade.code, trade.name, trade.tradeType, trade.roundID, trade.orderID,
			trade.orderSeq, trade.price, trade.lots, trade.shares, trade.amount, trade.fee,
			trade.commission, trade.stamp, trade.transfer, trade.realized, trade.tradeTime,
			trade.remark, trade.createdAt, trade.roundCommission, nullableInt(trade.roundMin),
			trade.roundStamp, trade.roundTransfer); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

type domainScanTrade struct {
	id, ledgerID, code, name, tradeType, roundID, orderID, remark string
	orderSeq, lots, shares                                        int
	price, amount, fee, commission, stamp, transfer, tradeTime    int64
	createdAt                                                     int64
	realized                                                      sql.NullInt64
	roundCommission, roundStamp, roundTransfer                    sql.NullFloat64
	roundMin                                                      sql.NullInt64
}

func (trade *domainScanTrade) scan(rows *sql.Rows) error {
	return rows.Scan(&trade.id, &trade.ledgerID, &trade.code, &trade.name, &trade.tradeType,
		&trade.roundID, &trade.orderID, &trade.orderSeq, &trade.price, &trade.lots, &trade.shares,
		&trade.amount, &trade.fee, &trade.commission, &trade.stamp, &trade.transfer,
		&trade.realized, &trade.tradeTime, &trade.remark, &trade.createdAt,
		&trade.roundCommission, &trade.roundMin, &trade.roundStamp, &trade.roundTransfer)
}

func copyStockDayReviews(source *sql.DB, tx *sql.Tx, accounts map[string]string) error {
	rows, err := source.Query("SELECT id, ledger_id, stock_code, round_id, review_date, review, created_at, updated_at FROM stock_day_review")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ledgerID, code, roundID, date, review string
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &code, &roundID, &date, &review, &createdAt, &updatedAt); err != nil {
			return err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_day_review (id, account_id, stock_code, round_id, review_date, review, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?)", id, accountID, code, roundID, date, review, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyStockOperations(source *sql.DB, tx *sql.Tx, accounts map[string]string) error {
	rows, err := source.Query("SELECT id, ledger_id, kind, action, detail, target_id, created_at FROM stock_operation")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ledgerID, kind, action, detail, targetID string
		var createdAt int64
		if err := rows.Scan(&id, &ledgerID, &kind, &action, &detail, &targetID, &createdAt); err != nil {
			return err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_operation (id, account_id, kind, action, detail, target_id, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)", id, accountID, kind, action, detail, targetID, createdAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyStockLessons(source *sql.DB, tx *sql.Tx, accounts map[string]string) (int, error) {
	rows, err := source.Query("SELECT id, ledger_id, text, round_id, stock_code, stock_name, round_no, opened_at, closed_at, pnl_cents, pnl_rate, created_at, updated_at FROM stock_lesson")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, ledgerID, text, code, name string
		var roundID sql.NullString
		var roundNo int
		var openedAt, closedAt, pnl, createdAt, updatedAt int64
		var rate float64
		if err := rows.Scan(&id, &ledgerID, &text, &roundID, &code, &name, &roundNo, &openedAt, &closedAt, &pnl, &rate, &createdAt, &updatedAt); err != nil {
			return count, err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_lesson (id, account_id, text, round_id, stock_code, stock_name, "+
			"round_no, opened_at, closed_at, pnl_cents, pnl_rate, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, accountID, text, nullableString(roundID), code, name, roundNo, openedAt, closedAt, pnl, rate, createdAt, updatedAt); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

func copyStockLedgers(source *sql.DB, tx *sql.Tx, accounts map[string]string) error {
	rows, err := source.Query("SELECT id, ledger_id, stock_code, round_id, why_buy, stop_loss, take_profit, followed_system, summary, created_at, updated_at FROM stock_ledger")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ledgerID, code string
		var roundID sql.NullString
		var whyBuy, stopLoss, takeProfit, followed, summary string
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &ledgerID, &code, &roundID, &whyBuy, &stopLoss, &takeProfit, &followed, &summary, &createdAt, &updatedAt); err != nil {
			return err
		}
		accountID, ok := accounts[ledgerID]
		if !ok {
			continue
		}
		if _, err := tx.Exec("INSERT INTO stock_ledger (id, account_id, stock_code, round_id, why_buy, stop_loss, "+
			"take_profit, followed_system, summary, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, accountID, code, nullableString(roundID), whyBuy, stopLoss, takeProfit, followed, summary, createdAt, updatedAt); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyStockLimitCache(source *sql.DB, tx *sql.Tx) (int, error) {
	rows, err := source.Query("SELECT trade_date, status, count, fetched_at FROM stock_limit_day")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var date, status string
		var days int
		var fetchedAt int64
		if err := rows.Scan(&date, &status, &days, &fetchedAt); err != nil {
			return count, err
		}
		if _, err := tx.Exec("INSERT INTO stock_limit_day (trade_date, status, count, fetched_at) VALUES (?, ?, ?, ?)",
			date, status, days, fetchedAt); err != nil {
			return count, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	if exists, _ := tableExists(source, "stock_limit_pool"); exists {
		entries, err := source.Query("SELECT trade_date, stock_code, stock_name, market, price_milli, pct, amount_cents, " +
			"float_mv_cents, turnover, level, first_seal, last_seal, broken_count, seal_fund_cents, industry, stat_days, stat_count FROM stock_limit_pool")
		if err == nil {
			for entries.Next() {
				var date, code, name, industry string
				var market, level, firstSeal, lastSeal, brokenCount, statDays, statCount int
				var price, amount, floatMv, sealFund int64
				var pct, turnover float64
				if err := entries.Scan(&date, &code, &name, &market, &price, &pct, &amount, &floatMv, &turnover,
					&level, &firstSeal, &lastSeal, &brokenCount, &sealFund, &industry, &statDays, &statCount); err != nil {
					entries.Close()
					return count, err
				}
				if _, err := tx.Exec("INSERT INTO stock_limit_pool (trade_date, stock_code, stock_name, market, price_milli, "+
					"pct, amount_cents, float_mv_cents, turnover, level, first_seal, last_seal, broken_count, seal_fund_cents, "+
					"industry, stat_days, stat_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					date, code, name, market, price, pct, amount, floatMv, turnover, level, firstSeal, lastSeal,
					brokenCount, sealFund, industry, statDays, statCount); err != nil {
					entries.Close()
					return count, err
				}
			}
			entries.Close()
		}
	}
	if exists, _ := tableExists(source, "stock_limit_quote"); exists {
		quotes, err := source.Query("SELECT trade_date, stock_code, open_pct, close_pct, close_price_milli, fetched_at FROM stock_limit_quote")
		if err == nil {
			for quotes.Next() {
				var date, code string
				var openPct, closePct sql.NullFloat64
				var closePrice sql.NullInt64
				var fetchedAt int64
				if err := quotes.Scan(&date, &code, &openPct, &closePct, &closePrice, &fetchedAt); err != nil {
					quotes.Close()
					return count, err
				}
				if _, err := tx.Exec("INSERT INTO stock_limit_quote (trade_date, stock_code, open_pct, close_pct, close_price_milli, fetched_at) "+
					"VALUES (?, ?, ?, ?, ?, ?)", date, code, nullableFloat(openPct), nullableFloat(closePct), nullableInt(closePrice), fetchedAt); err != nil {
					quotes.Close()
					return count, err
				}
			}
			quotes.Close()
		}
	}
	return count, nil
}

func nullableInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullableFloat(value sql.NullFloat64) any {
	if value.Valid {
		return value.Float64
	}
	return nil
}

func nullableString(value sql.NullString) any {
	if value.Valid && value.String != "" {
		return value.String
	}
	return nil
}
