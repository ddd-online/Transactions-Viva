package accounting

import "time"

// mockRecords is the 记录 page's seed while the SQLite store is not wired yet.
// The dates are relative to today so the ranges and the statistics have
// something to show on any day; they are never persisted. Replace this file
// with the tr_query path when internal/store lands.
func mockRecords() []record {
	now := time.Now()
	at := func(daysAgo, hour, minute int) time.Time {
		day := now.AddDate(0, 0, -daysAgo)
		return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.Local)
	}
	return []record{
		{id: "mock-1", kind: kindExpense, category: "餐饮美食", description: "早餐 · 豆浆油条", tags: []string{"日常"}, amount: 1200, at: at(0, 8, 40)},
		{id: "mock-2", kind: kindExpense, category: "餐饮美食", description: "午餐 · 公司楼下", tags: []string{"日常", "同事"}, amount: 3200, at: at(0, 12, 10)},
		{id: "mock-3", kind: kindExpense, category: "交通出行", description: "地铁通勤", tags: []string{"通勤"}, amount: 400, at: at(0, 18, 35)},
		{id: "mock-4", kind: kindIncome, category: "工资奖金", description: "工资到账", tags: []string{"月薪"}, amount: 1850000, at: at(0, 20, 5)},
		{id: "mock-5", kind: kindTransfer, category: "账户转账", description: "转入储蓄账户", tags: []string{"储蓄"}, amount: 500000, at: at(0, 21, 10)},

		{id: "mock-6", kind: kindExpense, category: "日用百货", description: "超市采购", tags: []string{"家庭"}, amount: 12680, at: at(1, 9, 20)},
		{id: "mock-7", kind: kindExpense, category: "休闲娱乐", description: "电影票", tags: []string{"周末"}, amount: 8000, at: at(4, 19, 30)},
		{id: "mock-8", kind: kindExpense, category: "居住", description: "房租", tags: []string{"固定支出"}, amount: 450000, at: at(9, 10, 0)},
		{id: "mock-9", kind: kindIncome, category: "副业", description: "兼职设计费", tags: []string{"设计"}, amount: 120000, at: at(15, 15, 20)},

		{id: "mock-10", kind: kindExpense, category: "医疗健康", description: "体检", tags: []string{"健康"}, amount: 68000, at: at(40, 8, 0)},
		{id: "mock-11", kind: kindIncome, category: "工资奖金", description: "年终奖", tags: []string{"奖金"}, amount: 3000000, at: at(90, 9, 0)},
		{id: "mock-12", kind: kindExpense, category: "休闲娱乐", description: "旅行机票", tags: []string{"旅行"}, amount: 880000, at: at(400, 14, 0)},
	}
}
