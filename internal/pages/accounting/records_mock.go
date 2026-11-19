package accounting

import "time"

// mockSyncTargets are the other ledgers a row can be copied to. The real app
// reads them from the workspace; the mock has no ledger switcher, so syncing
// reports the chosen target instead of adding a second copy to this table.
var mockSyncTargets = [...]string{"家庭账本", "旅行账本"}

// mockTagOptions are the tags the composer's 标签 picker offers. The real app
// filters them by category and type.
var mockTagOptions = [...]string{
	"日常", "同事", "通勤", "家庭", "周末", "固定支出", "设计",
	"健康", "旅行", "社交", "书籍", "理财", "月薪", "储蓄",
}

// recordTemplate is one entry of the composer's 模板 Select. The real app reads
// them from the workspace; these are mock rows for the form.
type recordTemplate struct {
	id          string
	name        string
	kind        string
	category    string
	description string
	tags        []string
}

// mockTemplates returns the templates the composer starts with. Saving a new
// template appends to the page's copy for the rest of the run.
func mockTemplates() []recordTemplate {
	return []recordTemplate{
		{id: "tpl-lunch", name: "午餐 · 餐饮美食", kind: kindExpense, category: "餐饮美食", description: "午餐", tags: []string{"日常"}},
		{id: "tpl-commute", name: "地铁通勤 · 交通出行", kind: kindExpense, category: "交通出行", description: "地铁通勤", tags: []string{"通勤"}},
		{id: "tpl-salary", name: "工资到账 · 工资奖金", kind: kindIncome, category: "工资奖金", description: "工资到账", tags: []string{"月薪"}},
	}
}

// mockRecords is the 记录 page's seed while the SQLite store is not wired yet.
// The dates are relative to today so the ranges and the statistics have
// something to show on any day; they are never persisted. Replace this file
// with the tr_query path when internal/store lands.
func mockRecords() []record {
	now := time.Now()
	today := now.Format("2006-01-02")
	at := func(daysAgo, hour, minute int) time.Time {
		day := now.AddDate(0, 0, -daysAgo)
		return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.Local)
	}
	return []record{
		{id: "mock-1", kind: kindExpense, category: "餐饮美食", description: "早餐 · 豆浆油条", tags: []string{"日常"}, amount: 1200, at: at(0, 8, 40)},
		{id: "mock-2", kind: kindExpense, category: "餐饮美食", description: "午餐 · 公司楼下", tags: []string{"日常", "同事"}, amount: 3200, at: at(0, 12, 10), keyEventDate: today},
		{id: "mock-3", kind: kindExpense, category: "交通出行", description: "地铁通勤", tags: []string{"通勤"}, amount: 400, at: at(0, 18, 35)},
		{id: "mock-4", kind: kindIncome, category: "工资奖金", description: "工资到账", tags: []string{"月薪"}, amount: 1850000, at: at(0, 20, 5), outlier: true},
		{id: "mock-5", kind: kindTransfer, category: "账户转账", description: "转入储蓄账户", tags: []string{"储蓄"}, amount: 500000, at: at(0, 21, 10)},

		{id: "mock-6", kind: kindExpense, category: "日用百货", description: "超市采购", tags: []string{"家庭"}, amount: 12680, at: at(1, 9, 20)},
		{id: "mock-7", kind: kindExpense, category: "休闲娱乐", description: "电影票", tags: []string{"周末"}, amount: 8000, at: at(4, 19, 30)},
		{id: "mock-8", kind: kindExpense, category: "居住", description: "房租", tags: []string{"固定支出"}, amount: 450000, at: at(9, 10, 0)},
		{id: "mock-9", kind: kindIncome, category: "副业", description: "兼职设计费", tags: []string{"设计"}, amount: 120000, at: at(15, 15, 20)},

		{id: "mock-13", kind: kindExpense, category: "餐饮美食", description: "咖啡续杯", tags: []string{"日常"}, amount: 1800, at: at(2, 15, 0)},
		{id: "mock-14", kind: kindExpense, category: "交通出行", description: "打车回家", tags: []string{"通勤"}, amount: 3600, at: at(3, 22, 10)},
		{id: "mock-15", kind: kindExpense, category: "居住", description: "水费", tags: []string{"固定支出"}, amount: 6800, at: at(5, 9, 30)},
		{id: "mock-16", kind: kindExpense, category: "学习成长", description: "书店", tags: []string{"书籍"}, amount: 9800, at: at(6, 16, 20)},
		{id: "mock-17", kind: kindExpense, category: "餐饮美食", description: "朋友聚餐", tags: []string{"社交"}, amount: 26000, at: at(7, 19, 0)},
		{id: "mock-18", kind: kindExpense, category: "通讯", description: "话费", tags: []string{"固定支出"}, amount: 10000, at: at(8, 11, 0)},
		{id: "mock-19", kind: kindIncome, category: "投资收益", description: "理财收益", tags: []string{"理财"}, amount: 3250, at: at(11, 8, 30)},
		{id: "mock-20", kind: kindExpense, category: "运动健康", description: "健身月卡", tags: []string{"健康"}, amount: 29900, at: at(13, 18, 0)},

		{id: "mock-10", kind: kindExpense, category: "医疗健康", description: "体检", tags: []string{"健康"}, amount: 68000, at: at(40, 8, 0)},
		{id: "mock-11", kind: kindIncome, category: "工资奖金", description: "年终奖", tags: []string{"奖金"}, amount: 3000000, at: at(90, 9, 0)},
		{id: "mock-12", kind: kindExpense, category: "休闲娱乐", description: "旅行机票", tags: []string{"旅行"}, amount: 880000, at: at(400, 14, 0)},
	}
}
