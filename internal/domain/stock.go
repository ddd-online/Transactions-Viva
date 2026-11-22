// Package domain 的股票部分：账户、资金、费用、成交、轮次、持仓、统计与天梯。
//
// 金额恒为整数「分」，股价恒为整数「厘」（1/1000 元）；时间 Unix 秒，日期本地
// YYYY-MM-DD。股票账户属于工作空间，不属于账本。
//
// 纯规则在这里实现一次，页面与 store 共用：
//   - stock_fee.go    品种、费用、金额换算；
//   - stock_trade.go  成交、当前轮、轮次盈亏、成本结转；
//   - stock_account.go 账户、资金记录、总览形状；
//   - stock_stats.go  统计点与七档指标；
//   - stock_limit.go  连板天梯；
//   - stock_quote.go  行情。
package domain
