// Package domain holds the product's pure rules and data shapes: money,
// records, ledgers, events, diary entries, todos and stock trades. Nothing in
// this package imports the UI, the database or the operating system, so the
// same rules are shared by every page and are testable on their own.
//
// 单位是硬契约，与旧工程一致：
//
//   - 金额恒为整数「分」，字段名以 Cents 结尾；
//   - 股票价格恒为整数「厘」（1/1000 元），字段名以 Milli 结尾；
//   - 时间恒为 Unix 秒，字段名以 At 结尾；
//   - 日期恒为本地日期字符串 "YYYY-MM-DD"，字段名以 Date 结尾。
package domain
