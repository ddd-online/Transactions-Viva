package domain

import "math"

// 股票成交、轮次与持仓。价格单位是厘，金额单位是分；成交时间 Unix 秒。

// RoundFee 是一轮持仓的费用快照：建仓时定下，本轮内不可改。
type RoundFee struct {
	CommissionRate  float64
	MinCommission   int64
	StampDutyRate   float64
	TransferFeeRate float64
}

// StockTrade 是一笔成交。
type StockTrade struct {
	ID          string
	AccountID   string
	StockCode   string
	StockName   string
	TradeType   string // open / add / reduce / close
	RoundID     string
	OrderID     string
	OrderSeq    int
	PriceMilli  int64 // 成交价（厘 / 股）
	Lots        int
	Shares      int
	AmountCents int64 // 成交金额（分）
	FeeCents    int64
	Commission  int64
	StampDuty   int64
	TransferFee int64
	// RealizedPnl 只有卖出非空。
	RealizedPnl *int64
	TradeTime   int64
	RoundFee    *RoundFee
	Remark      string
	CreatedAt   int64
}

// StockPosition 是持仓表的一行。数量、成本、已实现盈亏是派生值。
type StockPosition struct {
	ID               string
	AccountID        string
	StockCode        string
	StockName        string
	Quantity         int
	TotalCostCents   int64
	RealizedPnlCents int64
	Review           string
	Tag              string
	CreatedAt        int64
	UpdatedAt        int64
}

// StockPositionView 是持仓页的一行：持仓 + 本轮派生 + 行情（可选）。
type StockPositionView struct {
	Position        StockPosition
	RoundCashFlow   int64
	RoundOpenedDate string
	DayReviews      []StockDayReview
	RoundFee        *RoundFee
	Tag             string
	LatestPrice     *int64
	PrevClose       *int64
	QuoteTime       *int64
}

// StockTradeHistory 是每只已清仓股票的一条历史集合。
type StockTradeHistory struct {
	ID        string
	AccountID string
	StockCode string
	StockName string
	CreatedAt int64
	UpdatedAt int64
}

// StockTradeRound 是一轮持仓：一次建仓到清仓之间的所有成交。
type StockTradeRound struct {
	ID        string
	AccountID string
	StockCode string
	HistoryID string
	RoundNo   int
	OpenedAt  int64
	ClosedAt  int64
	Tag       string
	Review    string
	CreatedAt int64
}

// StockDayReview 是当前轮或已归档轮次里的一天复盘。
type StockDayReview struct {
	ID         string
	AccountID  string
	StockCode  string
	RoundID    string
	ReviewDate string
	Review     string
	CreatedAt  int64
	UpdatedAt  int64
}

// StockRoundOption 是心得弹窗里可选的一轮。
type StockRoundOption struct {
	RoundID   string
	StockCode string
	StockName string
	RoundNo   int
	OpenedAt  int64
	ClosedAt  int64
}

// StockLesson 是一条交易心得。来源字段是创建 / 更新时写下的快照。
type StockLesson struct {
	ID              string
	AccountID       string
	Text            string
	RoundID         string // 空串 = 未关联
	StockCode       string
	StockName       string
	RoundNo         int
	OpenedAt        int64
	ClosedAt        int64
	PnlCents        int64
	PnlRate         float64
	SourceAvailable bool
	CreatedAt       int64
	UpdatedAt       int64
}

// StockLedgerEntry 是交易台账的库行（数字由本轮成交实时算，不落库）。
type StockLedgerEntry struct {
	ID             string
	AccountID      string
	StockCode      string
	RoundID        string // 空串 = 持仓期
	WhyBuy         string
	StopLoss       string
	TakeProfit     string
	FollowedSystem string
	Summary        string
	CreatedAt      int64
	UpdatedAt      int64
}

// StockLedgerView 是台账 + 由成交算出的数字。
type StockLedgerView struct {
	Entry          StockLedgerEntry
	BuyPriceMilli  int64
	BuyCostCents   int64
	SellPriceMilli *int64
	SellNetCents   *int64
	NetPnlCents    *int64
}

// StockHistorySummary 是记录页底栏的汇总。
type StockHistorySummary struct {
	StockCount   int
	RoundCount   int
	WinCount     int
	LossCount    int
	TotalPnl     int64
	TotalPnlRate float64
}

// StockHistoryItem 是记录页左栏的一只股票。
type StockHistoryItem struct {
	History      StockTradeHistory
	RoundCount   int
	LastClosedAt int64
	TotalPnl     int64
	TotalPnlRate float64
	LatestPrice  *int64
}

// StockRoundDetail 是一轮展开后的全部内容。
type StockRoundDetail struct {
	Round      StockTradeRound
	Trades     []StockTrade
	DayReviews []StockDayReview
	Ledger     *StockLedgerView
	FeeCents   int64
	PnlCents   int64
	PnlRate    float64
}

// StockHistoryDetail 是记录页右栏。
type StockHistoryDetail struct {
	History      StockTradeHistory
	Rounds       []StockRoundDetail
	WinCount     int
	LossCount    int
	TotalPnl     int64
	TotalPnlRate float64
	LatestPrice  *int64
}

// RoundFlow 是当前轮次的资金口径。
type RoundFlow struct {
	BuyCost  int64
	CashFlow int64
}

// IsBuy 报告成交类型是不是买入方向（建仓 / 加仓）。
func IsBuy(tradeType string) bool {
	return tradeType == TradeOpen || tradeType == TradeAdd
}

// IsSell 报告成交类型是不是卖出方向（减仓 / 清仓）。
func IsSell(tradeType string) bool {
	return tradeType == TradeReduce || tradeType == TradeClose
}

// CurrentRoundTrades 从按时间升序的成交流里切出当前在建轮次：
// open/add/reduce 属于当前轮，close 清空已收集；减仓到 0 也不结束轮次。
func CurrentRoundTrades(trades []StockTrade) []StockTrade {
	result := make([]StockTrade, 0, len(trades))
	for _, trade := range trades {
		switch trade.TradeType {
		case TradeOpen, TradeAdd, TradeReduce:
			result = append(result, trade)
		case TradeClose:
			result = result[:0]
		}
	}
	return result
}

// CurrentRoundFee 返回当前轮首笔的费用快照；老数据为 nil。
func CurrentRoundFee(trades []StockTrade) *RoundFee {
	current := CurrentRoundTrades(trades)
	if len(current) == 0 {
		return nil
	}
	return current[0].RoundFee
}

// RoundFlowOf 把当前轮折成买入成本与资金变动。
func RoundFlowOf(trades []StockTrade) RoundFlow {
	var flow RoundFlow
	for _, trade := range CurrentRoundTrades(trades) {
		switch {
		case IsBuy(trade.TradeType):
			cost := trade.AmountCents + trade.FeeCents
			flow.BuyCost += cost
			flow.CashFlow -= cost
		case IsSell(trade.TradeType):
			flow.CashFlow += trade.AmountCents - trade.FeeCents
		}
	}
	return flow
}

// SettledPnl 是已清仓轮次的盈亏合计；未清仓轮次不计。
func SettledPnl(trades []StockTrade) int64 {
	var total, buyCost, sellNet int64
	for _, trade := range trades {
		switch trade.TradeType {
		case TradeOpen, TradeAdd:
			buyCost += trade.AmountCents + trade.FeeCents
		case TradeReduce, TradeClose:
			sellNet += trade.AmountCents - trade.FeeCents
		default:
			continue
		}
		if trade.TradeType == TradeClose {
			total += sellNet - buyCost
			buyCost, sellNet = 0, 0
		}
	}
	return total
}

// CostBasisOf 是减仓时按剩余总成本的比例结转成本（四舍五入到分）。
func CostBasisOf(totalCost int64, shares, quantity int) int64 {
	if shares <= 0 || quantity <= 0 {
		return 0
	}
	return int64(RoundHalfAwayFromZero(float64(totalCost) * float64(shares) / float64(quantity)))
}

// RoundPnl 是一轮全部成交的盈亏与盈亏率（买入总额为分母）。
func RoundPnl(trades []StockTrade) (buyCost, sellNet, pnl int64, rate float64) {
	for _, trade := range trades {
		switch trade.TradeType {
		case TradeOpen, TradeAdd:
			buyCost += trade.AmountCents + trade.FeeCents
		case TradeReduce, TradeClose:
			sellNet += trade.AmountCents - trade.FeeCents
		}
	}
	pnl = sellNet - buyCost
	if buyCost > 0 {
		rate = RoundRate(float64(pnl) / float64(buyCost) * 100)
	}
	return buyCost, sellNet, pnl, rate
}

// EffectivePositionTag 是持仓行标签的生效值：空串按默认「分析」。
func EffectivePositionTag(raw string) string {
	if raw == "" {
		return DefaultStockTradeTag
	}
	return raw
}

// FloatingPnlRate 是浮动盈亏率（%）：浮动盈亏 ÷ 本轮净投入 × 100。
// 净投入 ≤ 0 时没有可算的比率，返回 nil。
func FloatingPnlRate(pnl, cashFlow int64) *float64 {
	cost := -cashFlow
	if cost <= 0 {
		return nil
	}
	value := RoundRate(float64(pnl) / float64(cost) * 100)
	return &value
}

// StockRoundFee 是一组成交的费用合计，只算真正成交的行（shares > 0）。
func StockRoundFee(trades []StockTrade) int64 {
	var total int64
	for _, trade := range trades {
		if trade.Shares > 0 {
			total += trade.FeeCents
		}
	}
	return total
}

// TradeTypeLabel 是成交类型的中文名。
func TradeTypeLabel(tradeType string) string {
	switch tradeType {
	case TradeOpen:
		return "建仓"
	case TradeAdd:
		return "加仓"
	case TradeReduce:
		return "减仓"
	case TradeClose:
		return "清仓"
	}
	return "交易"
}

// RoundTradeCount 是一轮里真正成交的笔数（0 手标记不算）。
func RoundTradeCount(trades []StockTrade) int {
	count := 0
	for _, trade := range trades {
		if trade.Shares > 0 {
			count++
		}
	}
	return count
}

// TradeOrderRemark 是资金记录备注：「名称 N手 @ 均价」，多笔补「（M笔成交）」。
func TradeOrderRemark(stockName string, totalLots, totalAmount int64, count int) string {
	shares := SharesOfLots(totalLots)
	avgPriceMilli := int64(0)
	if shares > 0 {
		avgPriceMilli = int64(RoundHalfAwayFromZero(float64(totalAmount) * 10 / float64(shares)))
	}
	remark := stockName + " " + itoa(int(totalLots)) + "手 @ " + MilliToYuan(avgPriceMilli)
	if count > 1 {
		remark += "（" + itoa(count) + "笔成交）"
	}
	return remark
}

// MaxInt64 是费用 / 金额运算里判断溢出的边界。
const MaxInt64 = int64(math.MaxInt64)
