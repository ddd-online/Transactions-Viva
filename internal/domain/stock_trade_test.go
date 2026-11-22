package domain

import "testing"

func trade(id, tradeType string, amount, fee int64) StockTrade {
	return StockTrade{
		ID:          id,
		TradeType:   tradeType,
		AmountCents: amount,
		FeeCents:    fee,
		Shares:      100,
	}
}

func TestCurrentRoundTrades(t *testing.T) {
	trades := []StockTrade{
		trade("1", TradeOpen, 100, 1),
		trade("2", TradeAdd, 200, 2),
		trade("3", TradeReduce, 300, 3),
		trade("4", TradeClose, 400, 4),
		trade("5", TradeOpen, 500, 5),
	}
	current := CurrentRoundTrades(trades)
	if len(current) != 1 || current[0].ID != "5" {
		t.Fatalf("current round = %+v", current)
	}

	// 减仓到 0 不结束轮次：close 之前的 reduce 仍留在当前轮。
	trades = []StockTrade{
		trade("1", TradeOpen, 1000, 1),
		trade("2", TradeReduce, 900, 1),
	}
	if current := CurrentRoundTrades(trades); len(current) != 2 {
		t.Fatalf("reduce-to-zero round length = %d", len(current))
	}
}

func TestRoundFlowAndSettledPnl(t *testing.T) {
	trades := []StockTrade{
		trade("1", TradeOpen, 10_000, 5),
		trade("2", TradeAdd, 20_000, 10),
		trade("3", TradeReduce, 15_000, 5),
		trade("4", TradeClose, 20_000, 5),
	}
	flow := RoundFlowOf(trades[:3])
	if flow.BuyCost != 30_015 || flow.CashFlow != -15_020 {
		t.Fatalf("round flow = %+v", flow)
	}
	if got := SettledPnl(trades); got != 4_975 {
		t.Fatalf("settled pnl = %d", got)
	}
	buyCost, sellNet, pnl, rate := RoundPnl(trades)
	if buyCost != 30_015 || sellNet != 34_990 || pnl != 4_975 || rate != 16.58 {
		t.Fatalf("round pnl = %d %d %d %.2f", buyCost, sellNet, pnl, rate)
	}
}

func TestCostBasis(t *testing.T) {
	if got := CostBasisOf(300, 100, 300); got != 100 {
		t.Fatalf("cost basis = %d", got)
	}
	if got := CostBasisOf(100, 0, 100); got != 0 {
		t.Fatalf("zero-share cost basis = %d", got)
	}
	if got := CostBasisOf(100, 100, 0); got != 0 {
		t.Fatalf("zero-quantity cost basis = %d", got)
	}
}

func TestFloatingPnlRate(t *testing.T) {
	rate := FloatingPnlRate(-101_828, -480_828)
	if rate == nil || *rate != -21.18 {
		t.Fatalf("floating rate = %v", rate)
	}
	if rate := FloatingPnlRate(100, 0); rate != nil {
		t.Fatalf("zero cost rate = %v", rate)
	}
}

func TestTradeTypeHelpers(t *testing.T) {
	if !IsBuy(TradeOpen) || !IsBuy(TradeAdd) || IsBuy(TradeClose) {
		t.Fatal("IsBuy is wrong")
	}
	if !IsSell(TradeReduce) || !IsSell(TradeClose) || IsSell(TradeOpen) {
		t.Fatal("IsSell is wrong")
	}
	if got := TradeTypeLabel(TradeReduce); got != "减仓" {
		t.Fatalf("label = %q", got)
	}
	if got := EffectivePositionTag(""); got != DefaultStockTradeTag {
		t.Fatalf("effective tag = %q", got)
	}
	if got := EffectivePositionTag("打板"); got != "打板" {
		t.Fatalf("effective tag = %q", got)
	}
}

func TestQuoteChangePercent(t *testing.T) {
	value := QuoteChangePercent(11_000, 10_000)
	if value == nil || *value != 10 {
		t.Fatalf("quote change = %v", value)
	}
	if value := QuoteChangePercent(11_000, 0); value != nil {
		t.Fatalf("zero prev close = %v", value)
	}
}
