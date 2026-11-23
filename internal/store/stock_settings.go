package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"transactions/internal/domain"
)

// StockFeeSetting returns the account's four fee rates, creating the default
// row when it is missing.
func (w *Workspace) StockFeeSetting(ctx context.Context, accountID string) (domain.StockFeeSetting, error) {
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockFeeSetting{}, err
	}
	var setting domain.StockFeeSetting
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		var err error
		setting, err = getOrCreateStockFeeSettingTx(ctx, tx, accountID)
		return err
	})
	return setting, err
}

// SaveStockFeeSetting validates and writes the four rates.
func (w *Workspace) SaveStockFeeSetting(ctx context.Context, accountID string, fee domain.RoundFee) (domain.StockFeeSetting, error) {
	if err := domain.ValidateRoundFee(fee); err != nil {
		return domain.StockFeeSetting{}, invalidf("%s", err.Error())
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockFeeSetting{}, err
	}
	var setting domain.StockFeeSetting
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		current, err := getOrCreateStockFeeSettingTx(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_fee_setting SET commission_rate = ?, min_commission_cents = ?, "+
				"stamp_duty_rate = ?, transfer_fee_rate = ?, updated_at = ? WHERE account_id = ?",
			fee.CommissionRate, fee.MinCommission, fee.StampDutyRate, fee.TransferFeeRate,
			nowUnix(), accountID); err != nil {
			return fmt.Errorf("保存交易费用设置失败：%w", err)
		}
		current.CommissionRate = fee.CommissionRate
		current.MinCommission = fee.MinCommission
		current.StampDutyRate = fee.StampDutyRate
		current.TransferFeeRate = fee.TransferFeeRate
		setting = current
		return nil
	})
	return setting, err
}

// StockTradeTags returns the account's ordered tags, repairing a broken stored
// value to the defaults.
func (w *Workspace) StockTradeTags(ctx context.Context, accountID string) ([]string, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	var tags []string
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		var err error
		tags, err = stockTradeTagsTx(ctx, tx, accountID)
		return err
	})
	return tags, err
}

// SaveStockTradeTags normalizes and validates the account's tags.
func (w *Workspace) SaveStockTradeTags(ctx context.Context, accountID string, tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 8 {
			return nil, invalidf("单个标签不能超过 8 个字")
		}
		for _, existing := range normalized {
			if existing == tag {
				return nil, invalidf("标签不能重复")
			}
		}
		if len(normalized) >= 20 {
			return nil, invalidf("最多 20 个标签")
		}
		normalized = append(normalized, tag)
	}
	if len(normalized) == 0 {
		return nil, invalidf("至少保留一个标签")
	}
	if !containsString(normalized, domain.DefaultStockTradeTag) {
		return nil, invalidf("默认标签「分析」不可删除")
	}
	encoded, _ := json.Marshal(normalized)
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		now := nowUnix()
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO stock_trade_tag_setting (id, account_id, tags, created_at, updated_at) "+
				"VALUES (?, ?, ?, ?, ?) ON CONFLICT(account_id) DO UPDATE SET tags = excluded.tags, updated_at = excluded.updated_at",
			newID(), accountID, string(encoded), now, now); err != nil {
			return fmt.Errorf("保存交易标签失败：%w", err)
		}
		return nil
	})
	return normalized, err
}

// StockReviewTemplates returns the effective templates (empty stored values
// become the built-in ones).
func (w *Workspace) StockReviewTemplates(ctx context.Context, accountID string) (domain.StockReviewSetting, error) {
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockReviewSetting{}, err
	}
	if _, err := stockAccountByID(ctx, db, accountID); err != nil {
		return domain.StockReviewSetting{}, err
	}
	var template, daily sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT template, daily_template FROM stock_review_setting WHERE account_id = ?", accountID).
		Scan(&template, &daily)
	if errors.Is(err, sql.ErrNoRows) {
		return effectiveStockReview("", ""), nil
	}
	if err != nil {
		return domain.StockReviewSetting{}, fmt.Errorf("读取复盘模板失败：%w", err)
	}
	return effectiveStockReview(template.String, daily.String), nil
}

// SaveStockReviewTemplate stores one or both templates; nil daily keeps it.
func (w *Workspace) SaveStockReviewTemplate(ctx context.Context, accountID, template string, daily *string) (domain.StockReviewSetting, error) {
	if utf8.RuneCountInString(template) > domain.StockReviewTemplateMaxChars {
		return domain.StockReviewSetting{}, invalidf("本轮复盘模板最多 %d 个字", domain.StockReviewTemplateMaxChars)
	}
	if daily != nil && utf8.RuneCountInString(*daily) > domain.StockReviewTemplateMaxChars {
		return domain.StockReviewSetting{}, invalidf("每日本轮复盘模板最多 %d 个字", domain.StockReviewTemplateMaxChars)
	}
	stored := ""
	if strings.TrimSpace(template) != "" {
		stored = template
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.StockReviewSetting{}, err
	}
	var result domain.StockReviewSetting
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		var currentTemplate, currentDaily string
		err := tx.QueryRowContext(ctx,
			"SELECT template, daily_template FROM stock_review_setting WHERE account_id = ?", accountID).
			Scan(&currentTemplate, &currentDaily)
		if errors.Is(err, sql.ErrNoRows) {
			currentTemplate, currentDaily = "", ""
			now := nowUnix()
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO stock_review_setting (id, account_id, template, daily_template, created_at, updated_at) "+
					"VALUES (?, ?, '', '', ?, ?)", newID(), accountID, now, now); err != nil {
				return fmt.Errorf("保存复盘模板失败：%w", err)
			}
		} else if err != nil {
			return fmt.Errorf("读取复盘模板失败：%w", err)
		}
		nextDaily := currentDaily
		if daily != nil {
			nextDaily = ""
			if strings.TrimSpace(*daily) != "" {
				nextDaily = *daily
			}
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE stock_review_setting SET template = ?, daily_template = ?, updated_at = ? WHERE account_id = ?",
			stored, nextDaily, nowUnix(), accountID); err != nil {
			return fmt.Errorf("保存复盘模板失败：%w", err)
		}
		result = effectiveStockReview(stored, nextDaily)
		return nil
	})
	return result, err
}

func effectiveStockReview(template, daily string) domain.StockReviewSetting {
	if strings.TrimSpace(template) == "" {
		template = domain.StockReviewTemplateDefault
	}
	if strings.TrimSpace(daily) == "" {
		daily = domain.StockDailyReviewTemplateDefault
	}
	return domain.StockReviewSetting{Template: template, DailyTemplate: daily}
}
