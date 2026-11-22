package domain

// 股票账户与资金记录。账户属于工作空间，与账本无关；一个工作空间可以有多个账户，
// 每个账户有自己的本金、资金链、持仓、成交与设置。

// StockAccount 是一个股票账户。
type StockAccount struct {
	ID             string
	Name           string
	Description    string
	PrincipalCents int64
	SortOrder      int
	CreatedAt      int64
	UpdatedAt      int64
}

// StockFeeSetting 是一个股票账户的费用参数。
type StockFeeSetting struct {
	ID              string
	AccountID       string
	CommissionRate  float64 // 佣金费率（万 2.354 → 0.0002354）
	MinCommission   int64   // 最低佣金（分 / 笔）
	StampDutyRate   float64 // 印花税率（卖出收取）
	TransferFeeRate float64 // 过户费率（双向，仅沪市股票）
	CreatedAt       int64
	UpdatedAt       int64
}

// DefaultStockFeeSetting 返回新账户的默认费用参数。
func DefaultStockFeeSetting() StockFeeSetting {
	return StockFeeSetting{
		CommissionRate:  0.0002354,
		MinCommission:   500,
		StampDutyRate:   0.0005,
		TransferFeeRate: 0.00001,
	}
}

// 股票资金事件类型。字符串是数据契约的一部分，不能改。
const (
	StockEventAddPrincipal      = "add_principal"
	StockEventWithdraw          = "withdraw"
	StockEventInterestPrincipal = "interest_principal"
	StockEventBuy               = "buy"
	StockEventSell              = "sell"

	StockOperationFund  = "fund"
	StockOperationOrder = "order"
)

// StockFundRecord 是资金链上的一条变化。现金余额是派生值，写库后按链序复算。
type StockFundRecord struct {
	ID                string
	AccountID         string
	RecordDate        string // 本地 YYYY-MM-DD
	EventType         string
	EventText         string
	AmountChangeCents int64
	CashBalanceCents  int64
	// NetPnlCents 只有卖出记录有值；买入 / 本金类为空。
	NetPnlCents *int64
	Remark      string
	CreatedAt   int64
}

// StockFundPage 是一页资金变化记录。
type StockFundPage struct {
	Items    []StockFundRecord
	Total    int
	Page     int
	PageSize int
}

// StockAccountSummary 是账户口径的现金汇总（不含行情）。
type StockAccountSummary struct {
	Principal       int64 // 累计本金
	AvailableCash   int64
	WithdrawnTotal  int64
	InterestTotal   int64
	PositionCost    int64
	SellNetPnlTotal int64 // 含未清仓轮次里已减仓的回款，仅用于推现金
}

// StockOverview 是账户页的总览，含行情相关字段。
type StockOverview struct {
	Principal           int64
	AvailableCash       int64
	PositionMarketValue int64
	WithdrawnTotal      int64
	InterestTotal       int64
	TotalAssets         int64
	RealizedPnl         int64
	UnrealizedPnl       int64
	QuoteFailedCount    int
	TotalPnlPercent     float64
	// PositionRatio 为 nil 表示分母 ≤ 0，界面显示「—」。
	PositionRatio *float64
}

// StockOperation 是一条可回滚操作，每个账户最多保留 10 条。
type StockOperation struct {
	ID        string
	AccountID string
	Kind      string
	Action    string
	Detail    string
	TargetID  string
	CreatedAt int64
}

// StockOperationRollback 是回滚一次的结果。
type StockOperationRollback struct {
	Skipped bool
	Action  string
}

// StockReviewSetting 是账户当前生效的两份复盘模板（空串在读取时已换成内置模板）。
type StockReviewSetting struct {
	Template      string
	DailyTemplate string
}
