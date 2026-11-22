package domain

// 全局常量。字符串字面量是数据既有的一部分（数据库里存的就是这些值），
// 修改即为破坏性变更；不要「顺手统一」大小写或拼写。

const (
	// All 是筛选器里「全部」的占位值。
	All = "all"

	// 交易类型。
	TypeIncome   = "income"
	TypeExpense  = "expense"
	TypeTransfer = "transfer"

	// 待办状态。
	TodoDoing = "doing"
	TodoDone  = "done"

	// 股票持仓交易类型。
	TradeOpen   = "open"
	TradeAdd    = "add"
	TradeReduce = "reduce"
	TradeClose  = "close"
)

// TransactionTypes 是交易类型的可选值。
var TransactionTypes = [...]string{TypeIncome, TypeExpense, TypeTransfer}

// TodoStatuses 是待办状态的可选值。
var TodoStatuses = [...]string{TodoDoing, TodoDone}

// TodoLevels 是待办紧急度 / 重要度的六档。
var TodoLevels = [...]int{-5, -3, -1, 1, 3, 5}

// TodoLevelLabels 与 TodoLevels 一一对应。
var TodoLevelLabels = [...]string{"低", "中低", "次低", "次高", "中高", "高"}

// DefaultTodoLevel 是没填时的档位。
const DefaultTodoLevel = -1

// 本轮复盘的默认模板。
const (
	StockReviewTemplateDefault      = "买入理由：\n卖出理由：\n买点审视：\n卖点审视：\n交易心得：\n"
	StockDailyReviewTemplateDefault = "今日盘面：\n操作计划：\n情绪与纪律：\n"
	StockReviewTemplateMaxChars     = 200
)

// 本轮交易标签。
var DefaultStockTradeTags = [...]string{"分析", "打板", "尾盘", "追涨", "蓄力"}

// DefaultStockTradeTag 是库里空串时生效的标签。
const DefaultStockTradeTag = "分析"

// SnapTodoLevel 就近取档：幅度向上取到相邻奇数，超出 ±5 夹到端点，0 落到默认档。
func SnapTodoLevel(value int) int {
	magnitude := value
	if magnitude < 0 {
		magnitude = -magnitude
	}
	var snapped int
	if magnitude >= 5 {
		snapped = 5
	} else {
		snapped = magnitude + (1 - magnitude%2)
	}
	switch {
	case value < 0:
		return -snapped
	case value == 0:
		return DefaultTodoLevel
	default:
		return snapped
	}
}

// TodoLevelLabel 返回档位文案；不在六档里的值先就近取档。
func TodoLevelLabel(value int) string {
	snapped := SnapTodoLevel(value)
	for i, level := range TodoLevels {
		if level == snapped {
			return TodoLevelLabels[i]
		}
	}
	return TodoLevelLabels[0]
}
