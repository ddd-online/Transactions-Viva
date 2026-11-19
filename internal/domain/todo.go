package domain

// 待办：卡片是「主题」，事项挂在卡片下；完成不是搬表，只把 status 置成 done
// 并记 CompletedAt。卡片视图只取 doing，历史视图只取 done。
// Urgency / Importance 是六档之一：-5 / -3 / -1 / 1 / 3 / 5。

// TodoCard 是一个待办主题。
type TodoCard struct {
	ID        string
	LedgerID  string
	Title     string
	CreatedAt int64
	UpdatedAt int64
	SortOrder int
}

// TodoItem 是一个待办事项。
type TodoItem struct {
	ID          string
	LedgerID    string
	CardID      string
	Title       string
	StartDate   string // 日粒度 YYYY-MM-DD；空串 = 没填
	DueDate     string
	Urgency     int
	Importance  int
	Status      string // doing / done
	CompletedAt int64  // 完成时刻（Unix 秒；未完成 = 0）
	CreatedAt   int64
	UpdatedAt   int64
}

// TodoProgress 是一条进度记录。正文可以订正；订正不改变 CreatedAt 与 Done。
type TodoProgress struct {
	ID        string
	LedgerID  string
	ItemID    string
	Content   string
	Done      bool
	CreatedAt int64
}
