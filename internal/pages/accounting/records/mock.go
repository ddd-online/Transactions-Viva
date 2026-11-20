package records

import (
	"time"

	"transactions/internal/pages/accounting/ledger"
)

// The ledger ids the mock rows belong to, aliased for the tests.
const (
	ledgerHome   = ledger.HomeID
	ledgerTravel = ledger.TravelID
)

// mockCategory is one 分类 of the composer's 分类 Select: its name and the type
// it belongs to.
type mockCategory struct {
	name string
	kind string
}

// mockCategories are the default categories of the old app
// (electron/services/category.ts, DEFAULT_DATA), in its order. The real app reads
// the workspace's own categories, which include the ones the user added; until
// the store lands these are the only choices.
var mockCategories = [...]mockCategory{
	{name: "餐饮美食", kind: kindExpense},
	{name: "购物消费", kind: kindExpense},
	{name: "交通出行", kind: kindExpense},
	{name: "生活缴费", kind: kindExpense},
	{name: "贷款还款", kind: kindExpense},
	{name: "医疗健康", kind: kindExpense},
	{name: "娱乐休闲", kind: kindExpense},
	{name: "人情往来", kind: kindExpense},
	{name: "教育学习", kind: kindExpense},
	{name: "工资奖金", kind: kindIncome},
	{name: "补贴补助", kind: kindIncome},
	{name: "退税退款", kind: kindIncome},
	{name: "二手转卖", kind: kindIncome},
	{name: "彩票收入", kind: kindIncome},
	{name: "投资理财", kind: kindIncome},
	{name: "借贷借款", kind: kindIncome},
	{name: "红包转账", kind: kindIncome},
	{name: "五险一金", kind: kindTransfer},
	{name: "税费党费", kind: kindTransfer},
}

// mockTagOptions are the tags the composer's 标签 picker offers. The real app
// filters them by category and type.
var mockTagOptions = [...]string{
	"日常", "同事", "通勤", "家庭", "周末", "固定支出", "设计",
	"健康", "旅行", "社交", "书籍", "理财", "月薪", "储蓄",
}

// mockRecords is the 记录 page's seed while the SQLite store is not wired yet.
// The dates are relative to today so the ranges and the statistics have
// something to show on any day; they are never persisted. Replace this file
// with the tr_query path when internal/store lands.
func mockRecords() []record {
	today := time.Now().Format("2006-01-02")
	at := mockAt
	records := []record{
		{id: "mock-1", kind: kindExpense, category: "餐饮美食", description: "早餐 · 豆浆油条", tags: []string{"日常"}, amount: 1200, at: at(0, 8, 40)},
		{id: "mock-2", kind: kindExpense, category: "餐饮美食", description: "午餐 · 公司楼下", tags: []string{"日常", "同事"}, amount: 3200, at: at(0, 12, 10), keyEventDate: today},
		{id: "mock-3", kind: kindExpense, category: "交通出行", description: "地铁通勤", tags: []string{"通勤"}, amount: 400, at: at(0, 18, 35)},
		{id: "mock-4", kind: kindIncome, category: "工资奖金", description: "工资到账", tags: []string{"月薪"}, amount: 1850000, at: at(0, 20, 5), outlier: true},
		{id: "mock-5", kind: kindTransfer, category: "五险一金", description: "社保代扣", tags: []string{"固定支出"}, amount: 500000, at: at(0, 21, 10)},

		{id: "mock-6", kind: kindExpense, category: "购物消费", description: "超市采购", tags: []string{"家庭"}, amount: 12680, at: at(1, 9, 20)},
		{id: "mock-7", kind: kindExpense, category: "娱乐休闲", description: "电影票", tags: []string{"周末"}, amount: 8000, at: at(4, 19, 30)},
		{id: "mock-8", kind: kindExpense, category: "生活缴费", description: "房租", tags: []string{"固定支出"}, amount: 450000, at: at(9, 10, 0)},
		{id: "mock-9", kind: kindIncome, category: "二手转卖", description: "转卖旧相机", tags: []string{"日常"}, amount: 120000, at: at(15, 15, 20)},

		{id: "mock-13", kind: kindExpense, category: "餐饮美食", description: "咖啡续杯", tags: []string{"日常"}, amount: 1800, at: at(2, 15, 0)},
		{id: "mock-14", kind: kindExpense, category: "交通出行", description: "打车回家", tags: []string{"通勤"}, amount: 3600, at: at(3, 22, 10)},
		{id: "mock-15", kind: kindExpense, category: "生活缴费", description: "水费", tags: []string{"固定支出"}, amount: 6800, at: at(5, 9, 30)},
		{id: "mock-16", kind: kindExpense, category: "教育学习", description: "书店", tags: []string{"书籍"}, amount: 9800, at: at(6, 16, 20)},
		{id: "mock-17", kind: kindExpense, category: "餐饮美食", description: "朋友聚餐", tags: []string{"社交"}, amount: 26000, at: at(7, 19, 0)},
		{id: "mock-18", kind: kindExpense, category: "生活缴费", description: "话费", tags: []string{"固定支出"}, amount: 10000, at: at(8, 11, 0)},
		{id: "mock-19", kind: kindIncome, category: "投资理财", description: "理财收益", tags: []string{"理财"}, amount: 3250, at: at(11, 8, 30)},
		{id: "mock-20", kind: kindExpense, category: "医疗健康", description: "健身月卡", tags: []string{"健康"}, amount: 29900, at: at(13, 18, 0)},

		{id: "mock-10", kind: kindExpense, category: "医疗健康", description: "体检", tags: []string{"健康"}, amount: 68000, at: at(40, 8, 0)},
		{id: "mock-11", kind: kindIncome, category: "工资奖金", description: "年终奖", tags: []string{"奖金"}, amount: 3000000, at: at(90, 9, 0)},
		{id: "mock-12", kind: kindExpense, category: "休闲娱乐", description: "旅行机票", tags: []string{"旅行"}, amount: 880000, at: at(400, 14, 0)},
	}
	// Every seed row above is the family ledger's; the travel ledger has its
	// own rows so switching the 账本 Select visibly changes the table.
	for index := range records {
		records[index].ledgerID = ledgerHome
	}
	return append(records, mockTravelRecords()...)
}

// mockTravelRecords are the rows of the mock workspace's second ledger, dated
// inside the default (today) range so the switch shows them without a range
// change.
func mockTravelRecords() []record {
	return []record{
		{id: "mock-travel-1", ledgerID: ledgerTravel, kind: kindExpense, category: "交通出行", description: "去程机票", tags: []string{"旅行"}, amount: 128000, at: mockAt(0, 9, 15)},
		{id: "mock-travel-2", ledgerID: ledgerTravel, kind: kindExpense, category: "娱乐休闲", description: "民宿两晚", tags: []string{"旅行"}, amount: 86000, at: mockAt(0, 18, 40)},
		{id: "mock-travel-3", ledgerID: ledgerTravel, kind: kindIncome, category: "红包转账", description: "差旅报销", tags: []string{"旅行"}, amount: 30000, at: mockAt(1, 12, 0)},
	}
}

// mockAt is a local clock time daysAgo days back, for the relative mock dates.
func mockAt(daysAgo, hour, minute int) time.Time {
	now := time.Now()
	day := now.AddDate(0, 0, -daysAgo)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.Local)
}
