package tags

// mockCategory is one category of the mock workspace: how many records it has
// and the tags filed under it. The store will compute both from the ledger.
type mockCategory struct {
	name  string
	count int
	tags  []mockTag
}

type mockTag struct {
	name  string
	count int
}

// mockGroups are the default categories of the old application
// (electron/services/category.ts DEFAULT_DATA), with a representative set of
// their tags and made-up record counts. The categories are the real defaults;
// the tag lists are trimmed to keep the mock readable, and the counts stand in
// for the store's queries.
var mockGroups = [kindCount][]mockCategory{
	kindExpense: {
		{name: "餐饮美食", count: 8, tags: []mockTag{
			{"三餐", 5}, {"外卖", 3}, {"咖啡", 2}, {"买菜", 1},
		}},
		{name: "购物消费", count: 3, tags: []mockTag{
			{"书籍", 1}, {"快递", 1}, {"数码", 1},
		}},
		{name: "交通出行", count: 2, tags: []mockTag{
			{"地铁", 2}, {"打车", 0},
		}},
		{name: "生活缴费", count: 4, tags: []mockTag{
			{"房租", 1}, {"水费", 1}, {"电费", 1}, {"通讯", 1},
		}},
		{name: "贷款还款"},
		{name: "医疗健康", tags: []mockTag{{"医药", 0}}},
		{name: "娱乐休闲"},
		{name: "人情往来", count: 1, tags: []mockTag{{"红包", 1}}},
		{name: "教育学习"},
	},
	kindIncome: {
		{name: "工资奖金", count: 1, tags: []mockTag{
			{"工资", 1}, {"奖金", 0},
		}},
		{name: "补贴补助"},
		{name: "退税退款"},
		{name: "二手转卖"},
		{name: "彩票收入"},
		{name: "投资理财", tags: []mockTag{{"理财", 0}}},
		{name: "借贷借款"},
		{name: "红包转账"},
	},
	kindTransfer: {
		{name: "五险一金", count: 1, tags: []mockTag{
			{"养老", 1}, {"医疗", 0}, {"住房", 0},
		}},
		{name: "税费党费", tags: []mockTag{{"交税", 0}}},
	},
}
