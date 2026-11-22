package stock

import (
	"testing"

	"transactions/internal/domain"
)

// TestAssembleRecordHistory locks the two accounting bases the records screen
// relies on: the left cards and footer use every trade of a round (the 0-share
// close marker costs nothing), while the expanded detail counts and shows only
// fills that really traded.
func TestAssembleRecordHistory(t *testing.T) {
	history := domain.StockTradeHistory{
		ID: "h1", StockCode: "600000", StockName: "浦发银行",
	}
	roundOne := domain.StockTradeRound{
		ID: "r1", StockCode: "600000", HistoryID: "h1", RoundNo: 1,
		OpenedAt: 1_700_000_000, ClosedAt: 1_700_100_000, Tag: "分析",
	}
	roundTwo := domain.StockTradeRound{
		ID: "r2", StockCode: "600000", HistoryID: "h1", RoundNo: 2,
		OpenedAt: 1_700_200_000, ClosedAt: 1_700_300_000, Tag: "打板",
	}
	trades := []domain.StockTrade{
		{ID: "t1", RoundID: "r1", TradeType: domain.TradeOpen, PriceMilli: 1000,
			Lots: 10, Shares: 1000, AmountCents: 100_000, FeeCents: 500},
		{ID: "t2", RoundID: "r1", TradeType: domain.TradeClose, PriceMilli: 1200,
			Lots: 10, Shares: 1000, AmountCents: 120_000, FeeCents: 600},
		{ID: "t3", RoundID: "r2", TradeType: domain.TradeOpen, PriceMilli: 1000,
			Lots: 5, Shares: 500, AmountCents: 50_000, FeeCents: 500},
		// The close marker written when a round is archived: no shares, no money.
		{ID: "t4", RoundID: "r2", TradeType: domain.TradeClose, Shares: 0},
	}
	days := []domain.StockDayReview{
		{ID: "d1", StockCode: "600000", RoundID: "r1", ReviewDate: "2026-03-02", Review: "放量"},
		{ID: "d2", StockCode: "600000", RoundID: "", ReviewDate: "2026-03-03", Review: "当前轮"},
	}
	ledger := domain.StockLedgerEntry{
		ID: "l1", StockCode: "600000", RoundID: "r1",
		WhyBuy: "突破", StopLoss: "4.60", TakeProfit: "6.50",
		FollowedSystem: "是", Summary: "按计划",
	}

	items, details, summary := assembleRecordHistory(
		[]domain.StockTradeHistory{history}, trades,
		[]domain.StockTradeRound{roundOne, roundTwo}, days,
		[]domain.StockLedgerEntry{ledger})

	if len(items) != 1 || len(details) != 1 {
		t.Fatalf("items=%d details=%d, want 1 each", len(items), len(details))
	}
	item := items[0]
	if item.RoundCount != 2 || item.LastClosedAt != roundTwo.ClosedAt {
		t.Fatalf("left card = %+v", item)
	}
	// Round 1 earns 189.00, round 2 loses 505.00.
	if item.TotalPnl != -31_600 {
		t.Fatalf("left total pnl = %d, want -31600", item.TotalPnl)
	}
	if item.TotalPnlRate != -20.93 {
		t.Fatalf("left total rate = %v, want -20.93", item.TotalPnlRate)
	}

	detail := details["600000"]
	if detail == nil || len(detail.Rounds) != 2 {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.WinCount != 1 || detail.LossCount != 1 || detail.TotalPnl != -31_600 {
		t.Fatalf("detail summary = %+v", detail)
	}
	first := detail.Rounds[0]
	if len(first.Trades) != 2 || first.FeeCents != 1_100 {
		t.Fatalf("round 1 = %+v", first)
	}
	if first.PnlCents != 18_900 || first.PnlRate != 18.81 {
		t.Fatalf("round 1 pnl = %d / %v", first.PnlCents, first.PnlRate)
	}
	if len(first.DayReviews) != 1 || first.DayReviews[0].Review != "放量" {
		t.Fatalf("round 1 day reviews = %+v", first.DayReviews)
	}
	if first.Ledger == nil {
		t.Fatal("round 1 ledger is nil")
	}
	if first.Ledger.BuyPriceMilli != 1000 || first.Ledger.BuyCostCents != 100_500 {
		t.Fatalf("ledger buy = %+v", first.Ledger)
	}
	if first.Ledger.SellPriceMilli == nil || *first.Ledger.SellPriceMilli != 1200 {
		t.Fatalf("ledger sell price = %+v", first.Ledger.SellPriceMilli)
	}
	if first.Ledger.SellNetCents == nil || *first.Ledger.SellNetCents != 119_400 {
		t.Fatalf("ledger sell net = %+v", first.Ledger.SellNetCents)
	}
	if first.Ledger.NetPnlCents == nil || *first.Ledger.NetPnlCents != 18_900 {
		t.Fatalf("ledger net pnl = %+v", first.Ledger.NetPnlCents)
	}

	second := detail.Rounds[1]
	// Only the real fill is shown; the 0-share close marker is not a trade.
	if len(second.Trades) != 1 || second.Trades[0].ID != "t3" {
		t.Fatalf("round 2 trades = %+v", second.Trades)
	}
	if second.PnlCents != -50_500 {
		t.Fatalf("round 2 pnl = %d, want -50500", second.PnlCents)
	}
	if second.Ledger != nil {
		t.Fatalf("round 2 ledger = %+v, want nil", second.Ledger)
	}

	if summary.StockCount != 1 || summary.RoundCount != 2 ||
		summary.WinCount != 1 || summary.LossCount != 1 || summary.TotalPnl != -31_600 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.TotalPnlRate != -20.93 {
		t.Fatalf("summary rate = %v, want -20.93", summary.TotalPnlRate)
	}
}

// TestRecordsPanelSortsCardsByLastClose keeps the left rail in the old page's
// order: the most recently closed stock first, DAO order on ties.
func TestRecordsPanelSortsCardsByLastClose(t *testing.T) {
	older := domain.StockTradeHistory{ID: "h1", StockCode: "600000", StockName: "浦发银行"}
	newer := domain.StockTradeHistory{ID: "h2", StockCode: "000001", StockName: "平安银行"}
	rounds := []domain.StockTradeRound{
		{ID: "r1", StockCode: "600000", HistoryID: "h1", RoundNo: 1, ClosedAt: 100},
		{ID: "r2", StockCode: "000001", HistoryID: "h2", RoundNo: 1, ClosedAt: 200},
	}
	items, _, _ := assembleRecordHistory(
		[]domain.StockTradeHistory{older, newer}, nil, rounds, nil, nil)
	if len(items) != 2 || items[0].History.StockCode != "000001" {
		t.Fatalf("items = %+v", items)
	}
}

// TestLessonsChoicesPreselectsLockedRound covers the 记心得 path: the jump's
// stock and round stay in the lists (and selected) even when the options read
// has not caught up.
func TestLessonsChoicesPreselectsLockedRound(t *testing.T) {
	panel := &lessonsPanel{
		options: []domain.StockRoundOption{
			{RoundID: "r1", StockCode: "600000", StockName: "浦发银行", RoundNo: 1},
			{RoundID: "r2", StockCode: "600000", StockName: "浦发银行", RoundNo: 2},
			{RoundID: "r3", StockCode: "000001", StockName: "平安银行", RoundNo: 1},
		},
		stockCode: "600000",
		roundID:   "r2",
	}
	codes, labels := panel.stockChoices()
	if len(codes) != 2 || labels["600000"] != "浦发银行 600000" {
		t.Fatalf("stock choices = %v / %v", codes, labels)
	}
	rounds, roundLabels := panel.roundChoices()
	if len(rounds) != 2 || roundLabels["r2"] != "第 2 轮" {
		t.Fatalf("round choices = %v / %v", rounds, roundLabels)
	}

	locked := &lessonsPanel{stockCode: "600000", roundID: "missing"}
	rounds, roundLabels = locked.roundChoices()
	if len(rounds) != 1 || roundLabels["missing"] != "当前轮次" {
		t.Fatalf("locked round choices = %v / %v", rounds, roundLabels)
	}
}

func TestLessonJumpTip(t *testing.T) {
	unlinked := domain.StockLesson{}
	if got := lessonJumpTip(&unlinked, true); got != "未关联交易轮次" {
		t.Fatalf("unlinked tip = %q", got)
	}
	missing := domain.StockLesson{RoundID: "r1", SourceAvailable: false}
	if got := lessonJumpTip(&missing, true); got != "来源轮次已删除" {
		t.Fatalf("missing tip = %q", got)
	}
	live := domain.StockLesson{RoundID: "r1", SourceAvailable: true}
	if got := lessonJumpTip(&live, false); got != "暂时无法跳转" {
		t.Fatalf("no callback tip = %q", got)
	}
	if got := lessonJumpTip(&live, true); got != "查看来源轮次" {
		t.Fatalf("live tip = %q", got)
	}
}
