package domain

import "sort"

// 股票统计：一个「结算」= 一个已清仓轮次；时间轴取清仓时间。
// 统计点里的金额是分，百分比是百分数（66.67 表示 66.67%），比率是倍数。

// StockStatisticsStrategy 是统计口径。
type StockStatisticsStrategy string

const (
	StockStatisticsCumulative StockStatisticsStrategy = "cumulative"
	StockStatisticsGrouped    StockStatisticsStrategy = "grouped"
)

// StockStatisticsQuery 是统计页的一次查询。
type StockStatisticsQuery struct {
	AccountID  string
	StartMonth string // YYYY-MM；空串 = 不限
	EndMonth   string
	Recent     *int // 最近 N 笔；nil / 0 = 不限
	Tag        string
	Strategy   StockStatisticsStrategy
	GroupSize  int
}

// StockSettlement 是统计的一条输入（一个已清仓轮次）。
type StockSettlement struct {
	RoundID      string
	StockCode    string
	StockName    string
	StockRoundNo int
	Tag          string
	OpenedAt     int64
	ClosedAt     int64
	PnlCents     int64
	PnlRate      float64
	TradeCount   int
	BuyCostCents int64
}

// StockStatisticsPoint 是一个统计点。
type StockStatisticsPoint struct {
	Sequence       int
	Span           int
	OpenedAt       int64
	ClosedAt       int64
	RoundID        string
	StockCode      string
	StockName      string
	StockRoundNo   int
	Tag            string
	Pnl            int64
	PnlRate        float64
	TradeCount     int
	TotalPnl       int64
	WinCount       int
	LossCount      int
	TieCount       int
	WinRate        float64
	AvgWin         int64
	AvgLoss        int64
	PnlRatio       *float64
	Expectancy     int64
	MaxDrawdown    int64
	MaxDrawdownPct float64
}

// StockStatistics 是统计页的数据。
type StockStatistics struct {
	Principal  int64
	RoundCount int
	Points     []StockStatisticsPoint
	Groups     []StockStatisticsPoint
}

// StockMetric 是统计曲线的七档指标。
type StockMetric string

const (
	MetricTotalPnl    StockMetric = "total_pnl"
	MetricWinRate     StockMetric = "win_rate"
	MetricAvgWin      StockMetric = "avg_win"
	MetricAvgLoss     StockMetric = "avg_loss"
	MetricPnlRatio    StockMetric = "pnl_ratio"
	MetricExpectancy  StockMetric = "expectancy"
	MetricMaxDrawdown StockMetric = "max_drawdown"
)

// StockMetrics 是全部指标的顺序。
var StockMetrics = [...]StockMetric{
	MetricTotalPnl, MetricWinRate, MetricAvgWin, MetricAvgLoss,
	MetricPnlRatio, MetricExpectancy, MetricMaxDrawdown,
}

// MetricLabel 是逐笔累计口径下的显示名。
func MetricLabel(metric StockMetric) string {
	switch metric {
	case MetricTotalPnl:
		return "累计盈亏"
	case MetricWinRate:
		return "胜率"
	case MetricAvgWin:
		return "平均盈利"
	case MetricAvgLoss:
		return "平均亏损"
	case MetricPnlRatio:
		return "盈亏比"
	case MetricExpectancy:
		return "期望值"
	case MetricMaxDrawdown:
		return "最大回撤"
	}
	return string(metric)
}

// MetricLabelIn 是某个统计口径下的显示名；分组口径下累计盈亏叫「分组盈亏」。
func MetricLabelIn(metric StockMetric, strategy StockStatisticsStrategy) string {
	if metric == MetricTotalPnl && strategy == StockStatisticsGrouped {
		return "分组盈亏"
	}
	return MetricLabel(metric)
}

// MetricHint 是「这个指标在说什么」的一句话；累计盈亏没有。
func MetricHint(metric StockMetric) string {
	switch metric {
	case MetricWinRate:
		return "我赢的次数够不够多"
	case MetricAvgWin:
		return "赢的时候能赢多少"
	case MetricAvgLoss:
		return "输的时候会输多少"
	case MetricPnlRatio:
		return "赢一次的收获等于几次的代价"
	case MetricExpectancy:
		return "每开一笔，长期平均赚多少"
	case MetricMaxDrawdown:
		return "最坏的时候我要忍多少"
	}
	return ""
}

// ChartValueKind 是曲线值的格式化语义。
type ChartValueKind string

const (
	ChartValueMoney   ChartValueKind = "money"
	ChartValuePercent ChartValueKind = "percent"
	ChartValueRatio   ChartValueKind = "ratio"
)

// MetricKind 返回指标的取值语义。
func MetricKind(metric StockMetric) ChartValueKind {
	switch metric {
	case MetricWinRate, MetricMaxDrawdown:
		return ChartValuePercent
	case MetricPnlRatio:
		return ChartValueRatio
	}
	return ChartValueMoney
}

// MetricValue 从统计点取这一档的值；平均亏损取负画在 0 轴下方。
func MetricValue(metric StockMetric, point StockStatisticsPoint) float64 {
	switch metric {
	case MetricTotalPnl:
		return float64(point.TotalPnl)
	case MetricWinRate:
		return point.WinRate
	case MetricAvgWin:
		return float64(point.AvgWin)
	case MetricAvgLoss:
		return -float64(point.AvgLoss)
	case MetricPnlRatio:
		if point.PnlRatio == nil {
			return 0
		}
		return *point.PnlRatio
	case MetricExpectancy:
		return float64(point.Expectancy)
	case MetricMaxDrawdown:
		return point.MaxDrawdownPct
	}
	return 0
}

// MetricYBounds 只给天生有边界的指标：胜率 0..100（出现负数时 -100..100）。
func MetricYBounds(metric StockMetric, values []float64) *[2]float64 {
	if metric != MetricWinRate {
		return nil
	}
	low := 0.0
	for _, value := range values {
		if value < 0 {
			low = -100
			break
		}
	}
	return &[2]float64{low, 100}
}

// MetricHasReference 报告是否画 y=0 参考线。
func MetricHasReference(metric StockMetric) bool {
	return metric == MetricTotalPnl || metric == MetricExpectancy
}

// PointSpanLabel 是统计点在横轴上的覆盖范围文案。
func PointSpanLabel(strategy StockStatisticsStrategy, sequence, span int) string {
	first := sequence - span + 1
	if strategy == StockStatisticsGrouped && span > 1 && first >= 1 {
		return "第" + itoa(first) + "-" + itoa(sequence) + "笔"
	}
	return "第" + itoa(sequence) + "笔"
}

// DetailRowsNewestFirst 按统计序号倒序，最近结算的一行在最上面。
func DetailRowsNewestFirst(points []StockStatisticsPoint) []StockStatisticsPoint {
	result := append([]StockStatisticsPoint(nil), points...)
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Sequence > result[j].Sequence
	})
	return result
}

// KellyPercent 是凯利值（百分数）：f* = p − (1−p)/b。样本不足或没有盈亏比时为 nil。
func KellyPercent(span, winCount int, pnlRatio *float64) *float64 {
	if pnlRatio == nil || span <= 0 {
		return nil
	}
	p := float64(winCount) / float64(span)
	value := (p - (1-p)/(*pnlRatio)) * 100
	return &value
}

// itoa 是不引入 strconv 的小整数格式化（序号只到几万）。
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buffer [24]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buffer[index] = '-'
	}
	return string(buffer[index:])
}
