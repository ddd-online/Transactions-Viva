package domain

import (
	"errors"
	"math"
)

// 股票费用与成交口径。这里的规则与本项目 shared/domain/fee.ts 逐条对应：
// 品种决定收哪些费；佣金按整笔委托收一次并套最低佣金；印花税 / 过户费逐笔取整再相加；
// 分摊佣金时末笔吸收余数，保证 Σ分摊 == 委托合计。

// Instrument 是交易品种，由六位代码判定。
type Instrument string

const (
	InstrumentShanghaiStock Instrument = "shanghaiStock"
	InstrumentShenzhenStock Instrument = "shenzhenStock"
	InstrumentShanghaiFund  Instrument = "shanghaiFund"
	InstrumentShenzhenFund  Instrument = "shenzhenFund"
)

// InstrumentOf 由六位代码判定品种；不在支持范围的代码返回 false。
//
// 这是「代码是否合法」的唯一判据：沪 60/68、深 00/30、沪基金 50–58、深基金 15–18。
func InstrumentOf(stockCode string) (Instrument, bool) {
	if len(stockCode) != 6 {
		return "", false
	}
	for i := 0; i < len(stockCode); i++ {
		if stockCode[i] < '0' || stockCode[i] > '9' {
			return "", false
		}
	}
	switch stockCode[:2] {
	case "60", "68":
		return InstrumentShanghaiStock, true
	case "00", "30":
		return InstrumentShenzhenStock, true
	case "50", "51", "52", "53", "54", "55", "56", "57", "58":
		return InstrumentShanghaiFund, true
	case "15", "16", "17", "18":
		return InstrumentShenzhenFund, true
	}
	return "", false
}

// IsValidStockCode 是 InstrumentOf 的布尔形式。
func IsValidStockCode(stockCode string) bool {
	_, ok := InstrumentOf(stockCode)
	return ok
}

// IsFund 报告品种是不是场内基金（免印花税、免过户费）。
func IsFund(instrument Instrument) bool {
	return instrument == InstrumentShanghaiFund || instrument == InstrumentShenzhenFund
}

// IsShanghai 报告品种是不是沪市（行情前缀用；过户费另看 IsFund）。
func IsShanghai(instrument Instrument) bool {
	return instrument == InstrumentShanghaiStock || instrument == InstrumentShanghaiFund
}

// InstrumentLabel 是界面上的品种名。
func InstrumentLabel(instrument Instrument) string {
	if IsFund(instrument) {
		return "场内基金"
	}
	return "股票"
}

// MarketPrefixOf 返回行情接口的市场前缀：沪市 sh，深市 sz。
func MarketPrefixOf(instrument Instrument) string {
	if IsShanghai(instrument) {
		return "sh"
	}
	return "sz"
}

// MarketPrefix 按代码返回行情前缀；非法代码按深市处理（调用方通常会先校验）。
func MarketPrefix(stockCode string) string {
	instrument, ok := InstrumentOf(stockCode)
	if !ok {
		return "sz"
	}
	return MarketPrefixOf(instrument)
}

// TradeInstrumentOf 是费用 / 重放路径使用的品种判定：未知代码兜底为深市股票，
// 避免一条历史脏数据中断整轮重放。下单与名称刷新仍应先做 IsValidStockCode 校验。
func TradeInstrumentOf(stockCode string) Instrument {
	if instrument, ok := InstrumentOf(stockCode); ok {
		return instrument
	}
	return InstrumentShenzhenStock
}

// FeeBreakdown 是一笔成交或一笔委托的费用明细，单位分。
type FeeBreakdown struct {
	Commission  int64
	StampDuty   int64
	TransferFee int64
	Total       int64
}

// FeeTotal 返回三项之和。
func FeeTotal(commission, stampDuty, transferFee int64) int64 {
	return commission + stampDuty + transferFee
}

// WithTotal 用三项重算合计，避免「合计与明细不一致」。
func WithTotal(commission, stampDuty, transferFee int64) FeeBreakdown {
	return FeeBreakdown{
		Commission:  commission,
		StampDuty:   stampDuty,
		TransferFee: transferFee,
		Total:       FeeTotal(commission, stampDuty, transferFee),
	}
}

// RoundToCents 按费率计算费用并四舍五入到分。金额恒为正，半值远离零。
func RoundToCents(amount int64, rate float64) int64 {
	if amount <= 0 || rate <= 0 {
		return 0
	}
	return int64(RoundHalfAwayFromZero(float64(amount) * rate))
}

// ComputeCommission 是委托级佣金：max(金额 × 费率, 最低佣金)。
func ComputeCommission(amount int64, setting StockFeeSetting) int64 {
	return computeCommission(amount, ratesOfSetting(setting))
}

func computeCommission(amount int64, setting stockFeeRates) int64 {
	commission := RoundToCents(amount, setting.commissionRate)
	if commission < setting.minCommission {
		return setting.minCommission
	}
	return commission
}

// stockFeeRates 是费用算法需要的四项；StockFeeSetting 与 RoundFee 都折成它。
type stockFeeRates struct {
	commissionRate  float64
	minCommission   int64
	stampDutyRate   float64
	transferFeeRate float64
}

func ratesOfSetting(setting StockFeeSetting) stockFeeRates {
	return stockFeeRates{
		commissionRate:  setting.CommissionRate,
		minCommission:   setting.MinCommission,
		stampDutyRate:   setting.StampDutyRate,
		transferFeeRate: setting.TransferFeeRate,
	}
}

func ratesOfRound(round RoundFee) stockFeeRates {
	return stockFeeRates{
		commissionRate:  round.CommissionRate,
		minCommission:   round.MinCommission,
		stampDutyRate:   round.StampDutyRate,
		transferFeeRate: round.TransferFeeRate,
	}
}

// computeFillFee 是一笔成交的费用：佣金只按费率算，不套最低佣金。
func computeFillFee(amount int64, instrument Instrument, setting stockFeeRates, isBuy bool) FeeBreakdown {
	commission := RoundToCents(amount, setting.commissionRate)
	stamp := int64(0)
	if !isBuy && !IsFund(instrument) {
		stamp = RoundToCents(amount, setting.stampDutyRate)
	}
	transfer := int64(0)
	if instrument == InstrumentShanghaiStock {
		transfer = RoundToCents(amount, setting.transferFeeRate)
	}
	return WithTotal(commission, stamp, transfer)
}

// ComputeOrderFee 是委托级费用：佣金按委托总额收一次并套最低佣金；
// 印花税与过户费逐笔算好再相加。
func ComputeOrderFee(fills []int64, instrument Instrument, setting StockFeeSetting, isBuy bool) FeeBreakdown {
	rates := ratesOfSetting(setting)
	var totalAmount, stamp, transfer int64
	for _, amount := range fills {
		totalAmount += amount
		perFill := computeFillFee(amount, instrument, rates, isBuy)
		stamp += perFill.StampDuty
		transfer += perFill.TransferFee
	}
	return WithTotal(computeCommission(totalAmount, rates), stamp, transfer)
}

// ComputeFillFees 返回各笔成交自己的费用，不套最低佣金。
func ComputeFillFees(fills []int64, instrument Instrument, setting StockFeeSetting, isBuy bool) []FeeBreakdown {
	rates := ratesOfSetting(setting)
	result := make([]FeeBreakdown, len(fills))
	for i, amount := range fills {
		result[i] = computeFillFee(amount, instrument, rates, isBuy)
	}
	return result
}

// AllocateOrderFee 把委托费用拆到各笔成交：佣金按成交额比例分摊，
// 印花税 / 过户费就是逐笔值。Σ分摊 == 委托合计。
func AllocateOrderFee(fee FeeBreakdown, amounts []int64, instrument Instrument, setting StockFeeSetting, isBuy bool) []FeeBreakdown {
	if len(amounts) == 0 {
		return nil
	}
	commissions := AllocateByAmount(fee.Commission, amounts)
	perFill := ComputeFillFees(amounts, instrument, setting, isBuy)
	result := make([]FeeBreakdown, len(amounts))
	for i := range amounts {
		result[i] = WithTotal(commissions[i], perFill[i].StampDuty, perFill[i].TransferFee)
	}
	return result
}

// AllocateByAmount 按权重比例拆分一个金额，末项吸收取整余数。
func AllocateByAmount(total int64, weights []int64) []int64 {
	result := make([]int64, len(weights))
	if len(weights) == 0 {
		return result
	}
	var sum int64
	for _, weight := range weights {
		if weight > 0 {
			sum += weight
		}
	}
	if sum <= 0 {
		result[len(result)-1] = total
		return result
	}
	var allocated int64
	for i := 0; i < len(weights)-1; i++ {
		value := int64(RoundHalfAwayFromZero(float64(total) * float64(weights[i]) / float64(sum)))
		result[i] = value
		allocated += value
	}
	result[len(result)-1] = total - allocated
	return result
}

// ValidateStockFeeSetting 校验四项费率：佣金费率必须为正，其余不能为负。
func ValidateStockFeeSetting(setting StockFeeSetting) error {
	if setting.CommissionRate <= 0 ||
		setting.MinCommission < 0 ||
		setting.StampDutyRate < 0 ||
		setting.TransferFeeRate < 0 {
		return errors.New("佣金费率必须大于 0，最低佣金与费率不能为负")
	}
	return nil
}

// ValidateRoundFee 与 ValidateStockFeeSetting 同一套判据，给本轮快照用。
func ValidateRoundFee(round RoundFee) error {
	return ValidateStockFeeSetting(StockFeeSetting{
		CommissionRate:  round.CommissionRate,
		MinCommission:   round.MinCommission,
		StampDutyRate:   round.StampDutyRate,
		TransferFeeRate: round.TransferFeeRate,
	})
}

// RoundFeeOfSetting 只留费用算法要的四项。
func RoundFeeOfSetting(setting StockFeeSetting) RoundFee {
	return RoundFee{
		CommissionRate:  setting.CommissionRate,
		MinCommission:   setting.MinCommission,
		StampDutyRate:   setting.StampDutyRate,
		TransferFeeRate: setting.TransferFeeRate,
	}
}

// SettingOfRoundFee 把快照折成费用算法要的形态；账户与 id 留空。
func SettingOfRoundFee(round RoundFee) StockFeeSetting {
	return StockFeeSetting{
		CommissionRate:  round.CommissionRate,
		MinCommission:   round.MinCommission,
		StampDutyRate:   round.StampDutyRate,
		TransferFeeRate: round.TransferFeeRate,
	}
}

// SharesOfLots 把手数折成股数（A 股每手 100 股）。
func SharesOfLots(lots int64) int64 { return lots * 100 }

// AmountOf 把「厘 / 股 × 股数」折成「分」：round(priceMilli × shares / 10)。
// 整手 × 整数厘一定得到整数分；只有零股才真正四舍五入。
func AmountOf(priceMilli, shares int64) int64 {
	if priceMilli <= 0 || shares <= 0 {
		return 0
	}
	if priceMilli > (math.MaxInt64-5)/shares {
		return math.MaxInt64
	}
	return (priceMilli*shares + 5) / 10
}

// AmountOfLots 是 AmountOf 的手数版本。
func AmountOfLots(priceMilli, lots int64) int64 {
	return AmountOf(priceMilli, SharesOfLots(lots))
}

// PercentOf 返回 part / whole 的百分数，两位小数；whole ≤ 0 时返回 0。
func PercentOf(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(int64(RoundHalfAwayFromZero(float64(part)/float64(whole)*10_000))) / 100
}

// RoundRate 把比率四舍五入到两位小数。
func RoundRate(value float64) float64 {
	return float64(int64(RoundHalfAwayFromZero(value*100))) / 100
}
