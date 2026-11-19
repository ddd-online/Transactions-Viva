// The 记录 screen of the 记账 section. Its layout follows the reference
// application's hosts page: a toolbar, a row of filter chips, then the
// scrolling content, all inside the workspace card.
//
// The data is mock for now (records_mock.go): filtering, sorting and adding a
// record work on the in-memory slice only. When internal/store lands, refilter
// becomes a tr_query and the mock file goes away.
package accounting

import (
	"sort"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

const (
	kindExpense  = "expense"
	kindIncome   = "income"
	kindTransfer = "transfer"
)

// Column widths shared by the table header and its rows.
const (
	recordDateWidth     = float32(78)
	recordTypeWidth     = float32(64)
	recordCategoryWidth = float32(112)
	recordTagsWidth     = float32(180)
	recordMarksWidth    = float32(92)
	recordAmountWidth   = float32(120)
)

// record is one row of the mock table. The store's TransactionRecord plus its
// tag rows becomes this shape once the data is real.
type record struct {
	id           string
	kind         string
	category     string
	description  string
	tags         []string
	amount       int64 // 分
	at           time.Time
	outlier      bool
	keyEventDate string // 关联的事件日期（空串 = 不关联）
}

type rangeMode uint8

const (
	rangeToday rangeMode = iota
	rangeMonth
	rangeYear
	rangeAll
)

type sortField uint8

const (
	sortDate sortField = iota
	sortAmount
)

var rangeChoices = [...]struct {
	label string
	mode  rangeMode
}{
	{"今天", rangeToday},
	{"本月", rangeMonth},
	{"今年", rangeYear},
	{"全部时间", rangeAll},
}

// recordsModel is the 记录 screen's state. The shell owns it for as long as the
// section exists, so its filters and scroll survive a section switch.
type recordsModel struct {
	records []record
	shown   []record // every row the current range leaves, already sorted

	stats [3]int64 // 收入 / 支出 / 转账, within the visible rows

	rangeMode rangeMode
	sortField sortField
	sortDesc  bool

	// page is the 1-based page of the table; pageSize is the mock page size.
	// pageStart is where the current page begins in shown, set by table.
	page      int
	pageSize  int
	pageStart int

	list      ui.ListState
	rangeMenu bool
	sortMenu  bool
	draft     recordDraft
	templates []recordTemplate

	notice        string
	deletingID    string
	deleteConfirm bool
}

// newRecordsModel returns the page on 今天, newest first, with the mock rows
// already filtered.
func newRecordsModel() *recordsModel {
	m := &recordsModel{
		records:   mockRecords(),
		templates: mockTemplates(),
		rangeMode: rangeToday,
		sortField: sortDate,
		sortDesc:  true,
		page:      1,
		pageSize:  15,
	}
	m.refilter()
	return m
}

// View declares the records page.
func (m *recordsModel) View(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		m.toolbar(c)
		ui.Divider(c)
		m.table(c)
		if len(m.shown) > 0 {
			m.pager(c)
		}
		ui.Divider(c)
		m.footer(c)
	})
	m.composerView(c)
	m.deleteView(c)
}

// toolbar is the page's actions, laid out like the reference hosts page: the
// time range and sort on the left, the primary action on the right.
func (m *recordsModel) toolbar(c ui.C) {
	ui.Row(c).Height(56).FillWidth().PadX(18).Gap(10).Items(ui.Center).Children(func() {
		rangeButton := ui.Button(c, rangeLabel(m.rangeMode)).Outline()
		if rangeButton.Clicked() {
			m.rangeMenu = !m.rangeMenu
		}
		ui.Menu(c, rangeButton, &m.rangeMenu, func(menu *ui.MenuItems) {
			for _, choice := range rangeChoices {
				item := menu.Item(choice.label)
				item.Checked(m.rangeMode == choice.mode)
				if item.Clicked() {
					m.rangeMode = choice.mode
					m.refilter()
				}
			}
		})

		sortButton := ui.Button(c, "排序").Outline()
		if sortButton.Clicked() {
			m.sortMenu = !m.sortMenu
		}
		ui.Menu(c, sortButton, &m.sortMenu, func(menu *ui.MenuItems) {
			m.sortItem(menu, "日期：从新到旧", sortDate, true)
			m.sortItem(menu, "日期：从旧到新", sortDate, false)
			m.sortItem(menu, "金额：从高到低", sortAmount, true)
			m.sortItem(menu, "金额：从低到高", sortAmount, false)
		})

		ui.Spacer(c)
		if ui.Button(c, "记一笔").Icon(icons.Plus).Clicked() {
			m.openComposer()
		}
	})
}

// sortItem declares one choice of the sort menu and applies it when chosen.
func (m *recordsModel) sortItem(menu *ui.MenuItems, label string, field sortField, desc bool) {
	item := menu.Item(label)
	item.Checked(m.sortField == field && m.sortDesc == desc)
	if item.Clicked() {
		m.sortField, m.sortDesc = field, desc
		m.refilter()
	}
}

// table is the column header and the scrolling rows.
func (m *recordsModel) table(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		ui.Row(c).Height(34).FillWidth().PadX(18).Gap(12).Items(ui.Center).
			Background(theme.Muted).Children(func() {
			ui.Text(c, "日期").Width(recordDateWidth).FontSize(11).Muted()
			ui.Text(c, "类型").Width(recordTypeWidth).FontSize(11).Muted()
			ui.Text(c, "分类").Width(recordCategoryWidth).FontSize(11).Muted()
			ui.Text(c, "标签").Width(recordTagsWidth).FontSize(11).Muted()
			ui.Text(c, "描述").Grow(1).FontSize(11).Muted()
			ui.Text(c, "标记").Width(recordMarksWidth).TextCenter().FontSize(11).Muted()
			ui.Text(c, "金额").Width(recordAmountWidth).TextEnd().FontSize(11).Muted()
		})
		ui.Divider(c)
		if len(m.shown) == 0 {
			m.empty(c)
			return
		}
		m.page = min(max(m.page, 1), m.pageCount())
		m.pageStart = (m.page - 1) * m.pageSize
		end := min(m.pageStart+m.pageSize, len(m.shown))
		ui.List(c, &m.list, end-m.pageStart, m.row).Label("记录").Grow(1)
	})
}

// row declares one record: date and time, type, category, tags, description and
// the signed amount.
func (m *recordsModel) row(r ui.ListRow) {
	rec := &m.shown[m.pageStart+r.Index]
	theme := r.C.Theme()
	r.Element.Height(48).PadX(18).Gap(12).Items(ui.Center).
		Hover(ui.Look{}.Background(theme.Hover))
	if r.Selected() {
		r.Element.Background(theme.Hover)
	}

	ui.Text(r.C, rec.at.Format("01-02")).Width(recordDateWidth).FontSize(13).Ellipsis()

	color := kindColor(*theme, rec.kind)
	ui.Text(r.C, kindLabel(rec.kind)).Width(recordTypeWidth).FontSize(12).
		Weight(gfx.WeightMedium).Color(color)
	ui.Text(r.C, rec.category).Width(recordCategoryWidth).FontSize(12).Ellipsis()

	tags := strings.Join(rec.tags, "、")
	if tags == "" {
		tags = "-"
	}
	ui.Text(r.C, tags).Width(recordTagsWidth).FontSize(12).Muted().Ellipsis()

	description := rec.description
	if description == "" {
		description = "-"
	}
	ui.Text(r.C, description).Grow(1).FontSize(13).Ellipsis()
	ui.Row(r.C).Width(recordMarksWidth).Items(ui.Center).Justify(ui.Center).Gap(6).
		Children(func() {
			if rec.outlier {
				mark := outlierColor(*theme)
				ui.Text(r.C, "离群值").FontSize(11).Weight(gfx.WeightMedium).Color(mark).
					Background(mark.WithAlpha(0.12)).PadSides(1, 6, 1, 6).Radius(4)
			}
			if rec.keyEventDate != "" {
				ui.Text(r.C, "★").FontSize(12).Color(outlierColor(*theme))
			}
		})
	ui.Text(r.C, rec.signedAmount()).Width(recordAmountWidth).TextEnd().
		FontSize(13).Weight(gfx.WeightMedium).Color(color)

	// The old operation column's three actions live on the row's context menu.
	id := rec.id
	ui.ContextMenu(r.C, r.Element, func(menu *ui.MenuItems) {
		if menu.Item("编辑记录").Clicked() {
			m.openEdit(id)
		}
		menu.Sub("同步到其他账本", func(sub *ui.MenuItems) {
			for _, target := range mockSyncTargets {
				if sub.Item(target).Clicked() {
					m.syncTo(id, target)
				}
			}
		})
		menu.Separator()
		if menu.Item("删除记录").Destructive().Clicked() {
			m.deletingID, m.deleteConfirm = id, true
		}
	})
}

// empty is what the content area says when the time range matches nothing.
func (m *recordsModel) empty(c ui.C) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
		ui.Text(c, "当前时间范围内没有记录").FontSize(16).Bold()
		ui.Text(c, "换个时间范围，也可以直接记一笔。").FontSize(13).Muted()
		if ui.Button(c, "查看全部时间").Outline().Clicked() {
			m.rangeMode = rangeAll
			m.refilter()
		}
	})
}

// footer is the records' 收入 / 支出 / 转账 summary and the visible row count.
func (m *recordsModel) footer(c ui.C) {
	theme := c.Theme()
	ui.Row(c).Height(40).FillWidth().PadX(18).Gap(24).Items(ui.Center).Children(func() {
		m.stat(c, "收入", m.stats[0], kindColor(*theme, kindIncome))
		m.stat(c, "支出", m.stats[1], kindColor(*theme, kindExpense))
		m.stat(c, "转账", m.stats[2], kindColor(*theme, kindTransfer))
		ui.Spacer(c)
		if m.notice != "" {
			ui.Text(c, m.notice).FontSize(11).Muted().MaxWidth(560).Ellipsis()
		}
	})
}

func (m *recordsModel) stat(c ui.C, label string, cents int64, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(11).Muted()
		ui.Text(c, domain.CentsToYuan(cents)).FontSize(12).
			Weight(gfx.WeightMedium).Color(color)
	})
}

// pager is the shared pagination bar: total, page-size select and Viva pages.
func (m *recordsModel) pager(c ui.C) {
	components.Pagination(c, components.PaginationSpec{
		Total:    len(m.shown),
		Page:     m.page,
		PageSize: m.pageSize,
		OnPage: func(page int) {
			m.page = page
			m.list.Select(-1)
		},
		OnPageSize: func(size int) {
			m.pageSize = size
			m.refilter()
		},
	})
}

// pageCount is how many pages the current rows take.
func (m *recordsModel) pageCount() int {
	if len(m.shown) == 0 || m.pageSize <= 0 {
		return 1
	}
	return max((len(m.shown)+m.pageSize-1)/m.pageSize, 1)
}

// refilter recomputes the visible rows and the statistics.
func (m *recordsModel) refilter() {
	start := m.rangeStart(time.Now())

	m.notice = ""
	m.shown = m.shown[:0]
	m.stats = [3]int64{}
	for _, rec := range m.records {
		if !start.IsZero() && rec.at.Before(start) {
			continue
		}
		m.shown = append(m.shown, rec)
		switch rec.kind {
		case kindIncome:
			m.stats[0] += rec.amount
		case kindExpense:
			m.stats[1] += rec.amount
		case kindTransfer:
			m.stats[2] += rec.amount
		}
	}

	sort.SliceStable(m.shown, func(i, j int) bool {
		left, right := m.shown[i], m.shown[j]
		if m.sortField == sortAmount {
			if left.amount == right.amount {
				return left.at.After(right.at)
			}
			if m.sortDesc {
				return left.amount > right.amount
			}
			return left.amount < right.amount
		}
		if left.at.Equal(right.at) {
			return left.id < right.id
		}
		if m.sortDesc {
			return left.at.After(right.at)
		}
		return left.at.Before(right.at)
	})
	m.page = 1
	m.list.Select(-1)
}

// rangeStart is the first instant the chosen range includes; the zero time
// means no lower bound.
func (m *recordsModel) rangeStart(now time.Time) time.Time {
	switch m.rangeMode {
	case rangeToday:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	case rangeMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	case rangeYear:
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
	}
	return time.Time{}
}

func rangeLabel(mode rangeMode) string {
	for _, choice := range rangeChoices {
		if choice.mode == mode {
			return choice.label
		}
	}
	return ""
}

func (r record) signedAmount() string {
	switch r.kind {
	case kindIncome:
		return "+" + domain.CentsToYuan(r.amount)
	case kindExpense:
		return "-" + domain.CentsToYuan(r.amount)
	}
	return domain.CentsToYuan(r.amount)
}

func kindLabel(kind string) string {
	switch kind {
	case kindIncome:
		return "收入"
	case kindExpense:
		return "支出"
	case kindTransfer:
		return "转账"
	}
	return kind
}

func kindValue(label string) string {
	switch label {
	case "收入":
		return kindIncome
	case "支出":
		return kindExpense
	case "转账":
		return kindTransfer
	}
	return kindExpense
}

// The old project's semantic data colors (DESIGN.md / tokens.css), kept exact
// in both appearances: Viva's Success and Danger are close, but the records
// table and its footer must match the old project's income / expense /
// transfer hues.
var (
	incomeLight   = gfx.RGB24(0x16a34a)
	expenseLight  = gfx.RGB24(0xdc2626)
	transferLight = gfx.RGB24(0x3b82f6)
	incomeDark    = gfx.RGB24(0x4ade80)
	expenseDark   = gfx.RGB24(0xf87171)
	transferDark  = gfx.RGB24(0x60a5fa)
	outlierLight  = gfx.RGB24(0xd97706)
	outlierDark   = gfx.RGB24(0xfbbf24)
)

func outlierColor(theme gfx.Theme) gfx.Color {
	if theme.Background.Red < 0.5 {
		return outlierDark
	}
	return outlierLight
}

func kindColor(theme gfx.Theme, kind string) gfx.Color {
	dark := theme.Background.Red < 0.5
	switch kind {
	case kindIncome:
		if dark {
			return incomeDark
		}
		return incomeLight
	case kindExpense:
		if dark {
			return expenseDark
		}
		return expenseLight
	case kindTransfer:
		if dark {
			return transferDark
		}
		return transferLight
	}
	return theme.MutedForeground
}
