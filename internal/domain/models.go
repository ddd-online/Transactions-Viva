package domain

// 核心记账模型。字段名沿用旧工程的 camelCase 语义，Go 里用导出的 PascalCase；
// 与 SQLite 列名（snake_case）的映射放在 store 层，模型本身不认识数据库。
//
// 能空的值用指针或空串表达：金额与时间的 0 是真实值，不能拿 0 当 NULL。

// Ledger 是一个账本。每个工作空间可以有多个账本，业务数据按账本隔离。
type Ledger struct {
	ID          string
	Name        string
	Description string
	CreatedAt   int64
	UpdatedAt   int64
}

// TransactionRecord 是一笔收支记录。
type TransactionRecord struct {
	TransactionID   string
	LedgerID        string
	AmountCents     int64  // 交易金额（分；源库列名 price）
	TransactionType string // income / expense / transfer
	Category        string
	Description     string
	Flags           string // 标记集 JSON，保持旧库的字符串形态
	KeyEventDate    string // 关联的事件日期（空串 = 不关联）
	TransactionAt   int64  // 发生时间（Unix 秒）
	CreatedAt       int64
	UpdatedAt       int64
}

// TransactionRecordTag 是记录与标签的关联行。
type TransactionRecordTag struct {
	LedgerID      string
	TransactionID string
	Tag           string
}

// Category 是一个分类。账本间共享的分类用空 LedgerID。
type Category struct {
	LedgerID        string
	Name            string
	TransactionType string
	SortOrder       int
}

// Tag 是一个标签；CategoryTransactionType 形如 "餐饮美食:expense"。
type Tag struct {
	LedgerID                string
	Name                    string
	CategoryTransactionType string
	SortOrder               int
}

// TransactionTemplate 是一份可复用的记录模板。
type TransactionTemplate struct {
	TemplateID      string
	LedgerID        string
	TemplateName    string
	TransactionType string
	Category        string
	Tags            string // 标签集 JSON 数组字符串
	Flags           string
	Description     string
	SortOrder       int
	CreatedAt       int64
	UpdatedAt       int64
}

// QueryConditionItem 是图表曲线的一段筛选条件。
type QueryConditionItem struct {
	TransactionType string
	Category        string
	Tags            []string
	TagPolicy       string
	TagNot          bool
	Description     string
}

// ChartLine 是图表里的一条曲线。
type ChartLine struct {
	Label           string
	TransactionType string
	IncludeOutlier  bool
	Conditions      []QueryConditionItem
}

// Chart 是一张图。折线图用 ChartLines，饼图用 GroupField / GroupTransactionType / GroupCategory。
type Chart struct {
	ChartID              string
	LedgerID             string
	Title                string
	Granularity          string // year / month；饼图没有时间粒度，存空串
	ChartLines           string // 曲线配置 JSON；饼图恒为 "[]"
	ChartType            string // line / pie
	GroupField           string // 饼图分组字段：transactionType / category
	GroupTransactionType string
	GroupCategory        string
	IsPreset             bool
	SortOrder            int
	CreatedAt            int64
	UpdatedAt            int64
}

// KeyEvent 是一个事件（按日期一条）。
type KeyEvent struct {
	ID        string
	Date      string
	Title     string
	Content   string
	Color     string
	CreatedAt int64
	UpdatedAt int64
	LedgerID  string
}

// KeyEventImage 是事件的一张图片。路径都相对工作空间的 data/assets。
type KeyEventImage struct {
	ID           string
	LedgerID     string
	EventDate    string
	FilePath     string // 展示图；HEIC 导入时是转码后的 JPEG
	ThumbPath    string
	OriginalPath string // HEIC/HEIF 原图；其它格式为空
	MotionPath   string // 实况照片的动态部分；不是实况时为空
	SortOrder    int
	CreatedAt    int64
}

// DiaryEntry 是一篇日记（按账本、按日期一条）。
type DiaryEntry struct {
	ID        string
	Date      string
	Content   string
	WordCount int // Unicode 字符数，不是字节数
	Mood      string
	CreatedAt int64
	UpdatedAt int64
	LedgerID  string
}

// DiaryDateItem 是日记日期树的一项。
type DiaryDateItem struct {
	Date      string
	WordCount int
	Mood      string
}
