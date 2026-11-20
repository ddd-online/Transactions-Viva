package templates

import "transactions/internal/pages/accounting/template"

// mockCategory is one 分类 the 新建模板 form offers, with the tags its 标签
// checkboxes list. They mirror the old application's default categories (and
// the 标签 page's mock groups); internal/store replaces them.
type mockCategory struct {
	name string
	tags []string
}

var mockCategories = map[string][]mockCategory{
	template.KindExpense: {
		{name: "餐饮美食", tags: []string{"三餐", "外卖", "咖啡", "买菜"}},
		{name: "购物消费", tags: []string{"书籍", "快递", "数码"}},
		{name: "交通出行", tags: []string{"地铁", "打车"}},
		{name: "生活缴费", tags: []string{"房租", "水费", "电费", "通讯"}},
		{name: "贷款还款"},
		{name: "医疗健康", tags: []string{"医药"}},
		{name: "娱乐休闲"},
		{name: "人情往来", tags: []string{"红包"}},
		{name: "教育学习"},
	},
	template.KindIncome: {
		{name: "工资奖金", tags: []string{"工资", "奖金"}},
		{name: "补贴补助"},
		{name: "退税退款"},
		{name: "二手转卖"},
		{name: "彩票收入"},
		{name: "投资理财", tags: []string{"理财"}},
		{name: "借贷借款"},
		{name: "红包转账"},
	},
	template.KindTransfer: {
		{name: "五险一金", tags: []string{"养老", "医疗", "住房"}},
		{name: "税费党费", tags: []string{"交税"}},
	},
}
