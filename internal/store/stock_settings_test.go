package store

import (
	"context"
	"errors"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

func TestStockSettingsDefaultsAndValidation(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	setting, err := workspace.StockFeeSetting(ctx, account.ID)
	if err != nil || setting.CommissionRate != 0.0002354 || setting.MinCommission != 500 {
		t.Fatalf("default fee = %+v err=%v", setting, err)
	}
	round := domain.RoundFeeOfSetting(setting)
	round.CommissionRate = 0.0001
	saved, err := workspace.SaveStockFeeSetting(ctx, account.ID, round)
	if err != nil || saved.CommissionRate != 0.0001 {
		t.Fatalf("saved fee = %+v err=%v", saved, err)
	}
	bad := round
	bad.CommissionRate = 0
	if _, err := workspace.SaveStockFeeSetting(ctx, account.ID, bad); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("bad fee error = %v", err)
	}

	tags, err := workspace.StockTradeTags(ctx, account.ID)
	if err != nil || len(tags) != 5 || tags[0] != "分析" {
		t.Fatalf("default tags = %v err=%v", tags, err)
	}
	if _, err := workspace.SaveStockTradeTags(ctx, account.ID, []string{"分析", "分析"}); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("duplicate tags error = %v", err)
	}
	if _, err := workspace.SaveStockTradeTags(ctx, account.ID, []string{"打板"}); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("missing default tag error = %v", err)
	}
	if _, err := workspace.SaveStockTradeTags(ctx, account.ID, []string{"分析", "低吸"}); err != nil {
		t.Fatalf("save tags: %v", err)
	}
	tags, _ = workspace.StockTradeTags(ctx, account.ID)
	if len(tags) != 2 || tags[1] != "低吸" {
		t.Fatalf("tags after save = %v", tags)
	}

	reviews, err := workspace.StockReviewTemplates(ctx, account.ID)
	if err != nil || reviews.Template == "" || reviews.DailyTemplate == "" {
		t.Fatalf("templates = %+v err=%v", reviews, err)
	}
	daily := "今天这样看"
	reviews, err = workspace.SaveStockReviewTemplate(ctx, account.ID, "自定义", &daily)
	if err != nil || reviews.Template != "自定义" || reviews.DailyTemplate != daily {
		t.Fatalf("saved templates = %+v err=%v", reviews, err)
	}
	reviews, err = workspace.SaveStockReviewTemplate(ctx, account.ID, "", nil)
	if err != nil || reviews.DailyTemplate != daily || reviews.Template == "" {
		t.Fatalf("cleared template = %+v err=%v", reviews, err)
	}
}

func TestStockDayReviewLifecycle(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	reviews, err := workspace.UpdateStockDayReview(ctx, account.ID, "000001", todayStockDate(), "观察一天")
	if err != nil {
		t.Fatalf("day review: %v", err)
	}
	if len(reviews) != 1 || reviews[0].Review != "观察一天" {
		t.Fatalf("reviews = %+v", reviews)
	}
	if _, err := workspace.UpdateStockDayReview(ctx, account.ID, "000001", "2999-01-01", "未来"); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("future review error = %v", err)
	}
}

func TestStockLessonAndLedger(t *testing.T) {
	workspace := testWorkspace(t)
	ctx := context.Background()
	account := stockTestAccount(t, workspace)
	seedStockCash(t, workspace, account.ID, 100_000)
	trades, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeOpen,
		Fills: []repository.StockFill{{PriceMilli: 5_000, Lots: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreateStockOrder(ctx, repository.StockOrderRequest{
		AccountID: account.ID, StockCode: "000001", StockName: "平安银行", TradeType: domain.TradeClose,
		Fills: []repository.StockFill{{PriceMilli: 6_000, Lots: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	options, err := workspace.StockRoundOptions(ctx, account.ID)
	if err != nil || len(options) != 1 {
		t.Fatalf("round options = %+v err=%v", options, err)
	}
	lesson, err := workspace.CreateStockLesson(ctx, account.ID, "做对了", options[0].RoundID)
	if err != nil {
		t.Fatalf("CreateStockLesson: %v", err)
	}
	if lesson.RoundID != options[0].RoundID || lesson.RoundNo != 1 || !lesson.SourceAvailable {
		t.Fatalf("lesson = %+v", lesson)
	}
	lessons, _ := workspace.StockLessons(ctx, account.ID)
	if len(lessons) != 1 || lessons[0].Text != "做对了" {
		t.Fatalf("lessons = %+v", lessons)
	}
	if _, err := workspace.CreateStockLesson(ctx, account.ID, "", options[0].RoundID); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty lesson error = %v", err)
	}

	entry, err := workspace.StockLedger(ctx, account.ID, "000001", options[0].RoundID)
	if err != nil || entry != nil {
		t.Fatalf("empty ledger = %+v err=%v", entry, err)
	}
	if _, err := workspace.SaveStockLedger(ctx, domain.StockLedgerEntry{
		AccountID: account.ID, StockCode: "000001", RoundID: options[0].RoundID,
	}); !errors.Is(err, repository.ErrInvalid) {
		t.Fatalf("empty ledger save error = %v", err)
	}
	saved, err := workspace.SaveStockLedger(ctx, domain.StockLedgerEntry{
		AccountID: account.ID, StockCode: "000001", RoundID: options[0].RoundID,
		WhyBuy: "趋势向上", StopLoss: "4.60", TakeProfit: "6.50",
	})
	if err != nil || saved.ID == "" {
		t.Fatalf("saved ledger = %+v err=%v", saved, err)
	}
	_ = trades
}
