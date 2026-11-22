package store

// defaultCategory is one seeded category and its tags. The list is 19
// categories and 57 tags. Changing it changes the data every new ledger starts
// with, so it is a data contract, not a convenience.
type defaultCategory struct {
	name string
	tags []string
}

var defaultGroups = []struct {
	transactionType string
	categories      []defaultCategory
}{
	{
		transactionType: "expense",
		categories: []defaultCategory{
			{name: "餐饮美食", tags: []string{"三餐", "零食", "商场", "外卖", "饮料", "奶茶", "咖啡", "水果", "茶叶", "买菜"}},
			{name: "购物消费", tags: []string{"衣物", "数码", "家居", "书籍", "礼物", "玩具", "宠物", "游戏", "快递", "彩票", "电影", "运动", "酒店", "烟酒", "充值", "汽车", "还款"}},
			{name: "交通出行", tags: []string{"打车", "地铁", "公交", "高铁", "油费", "停车", "ETC", "车险"}},
			{name: "生活缴费", tags: []string{"房租", "物业", "燃气", "水费", "电费", "通讯", "还款", "网费", "理发"}},
			{name: "贷款还款"},
			{name: "医疗健康", tags: []string{"医药", "医险"}},
			{name: "娱乐休闲"},
			{name: "人情往来", tags: []string{"红包", "请客", "礼金"}},
			{name: "教育学习"},
		},
	},
	{
		transactionType: "income",
		categories: []defaultCategory{
			{name: "工资奖金", tags: []string{"工资", "奖金"}},
			{name: "补贴补助"},
			{name: "退税退款"},
			{name: "二手转卖"},
			{name: "彩票收入"},
			{name: "投资理财"},
			{name: "借贷借款"},
			{name: "红包转账"},
		},
	},
	{
		transactionType: "transfer",
		categories: []defaultCategory{
			{name: "五险一金", tags: []string{"养老", "医疗", "失业", "住房"}},
			{name: "税费党费", tags: []string{"团费", "交税"}},
		},
	},
}

// validTransactionType is the domain's three stored values.
func validTransactionType(value string) bool {
	switch value {
	case "income", "expense", "transfer":
		return true
	}
	return false
}
