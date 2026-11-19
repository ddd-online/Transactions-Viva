package domain

// 股票域：一笔成交的价格单位是**厘**，金额与费用单位是**分**。
// 费用算法只在费用函数里实现一次（迁移旧工程时整段搬过来），页面不自己乘汇率。

// StockFeeSetting 是一个账本的费用参数。
type StockFeeSetting struct {
	ID              string
	LedgerID        string
	CommissionRate  float64 // 佣金费率（万 2.354 → 0.0002354）
	MinCommission   int64   // 最低佣金（分 / 笔）
	StampDutyRate   float64 // 印花税率（卖出收取）
	TransferFeeRate float64 // 过户费率（双向，仅沪市）
	CreatedAt       int64
	UpdatedAt       int64
}

// DefaultStockFeeSetting 返回新账本的默认费用参数。
func DefaultStockFeeSetting() StockFeeSetting {
	return StockFeeSetting{
		CommissionRate:  0.0002354,
		MinCommission:   500,
		StampDutyRate:   0.0005,
		TransferFeeRate: 0.00001,
	}
}

// RoundFee 是一轮持仓的费用快照：建仓时定下，本轮内不可改。
// 四列全零表示旧数据，沿用账本当前设置。
type RoundFee struct {
	CommissionRate  float64
	MinCommission   int64
	StampDutyRate   float64
	TransferFeeRate float64
}

// StockTrade 是一笔成交。
type StockTrade struct {
	ID          string
	LedgerID    string
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
	RealizedPnl *int64 // 卖出净盈亏（分），仅减仓 / 清仓非空
	TradeAt     int64
	RoundFee    *RoundFee
	Remark      string
	CreatedAt   int64
}

// StockTradeRound 是一轮持仓：一次建仓到清仓之间的所有成交。
type StockTradeRound struct {
	ID        string
	LedgerID  string
	StockCode string
	HistoryID string
	RoundNo   int
	OpenedAt  int64
	ClosedAt  int64
	Tag       string
	Review    string
	CreatedAt int64
}

// StockPosition 是持仓表的一行（价格派生值不落库，读时按行情合并）。
type StockPosition struct {
	ID             string
	LedgerID       string
	StockCode      string
	StockName      string
	Quantity       int
	TotalCostCents int64
	RealizedPnl    int64
	Review         string
	Tag            string
	CreatedAt      int64
	UpdatedAt      int64
}
