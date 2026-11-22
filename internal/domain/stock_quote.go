package domain

// StockQuote 是一笔行情。价格单位厘；QuoteDate 只在行情时间可解析时非空。
type StockQuote struct {
	StockCode   string
	LatestPrice int64
	PrevClose   int64
	OpenPrice   int64
	QuoteTime   int64
	QuoteDate   string
}

// QuoteChangePercent 是涨跌幅（%）：(最新价 − 昨收) ÷ 昨收 × 100，两位小数。
// 昨收非正或价格非法时返回 nil。
func QuoteChangePercent(latestPrice, prevClose int64) *float64 {
	if prevClose <= 0 || latestPrice <= 0 {
		return nil
	}
	value := RoundRate(float64(latestPrice-prevClose) / float64(prevClose) * 100)
	return &value
}

// StockNameDTO 是名称查询的结果。
type StockNameDTO struct {
	StockCode string
	StockName string
}
