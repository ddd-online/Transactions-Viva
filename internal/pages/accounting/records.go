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
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

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
	countText string

	// page is the 1-based page of the table; pageSize is the mock page size.
	// pageStart is where the current page begins in shown, set by table.
	page      int
	pageSize  int
	pageStart int

	list      ui.ListState
	rangeMenu bool
	sortMenu  bool
	draft     recordDraft

	// The pagination component paints its own hits, so the page names its
	// items in the page tree once per paint: previous, seven page slots, next.
	pageIDs   [9]gfx.ID
	pageItems [7]pageItem
}

// pageItem is one slot of the pagination row: a page number, an ellipsis, or
// an unused slot when there are fewer than seven pages.
type pageItem struct {
	number   int
	ellipsis bool
}

// newRecordsModel returns the page on 今天, newest first, with the mock rows
// already filtered.
func newRecordsModel() *recordsModel {
	m := &recordsModel{
		records:   mockRecords(),
		rangeMode: rangeToday,
		sortField: sortDate,
		sortDesc:  true,
		page:      1,
		pageSize:  5,
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
			ui.Divider(c)
			m.pager(c)
		}
		ui.Divider(c)
		m.footer(c)
	})
	m.composerView(c)
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
	})
}

func (m *recordsModel) stat(c ui.C, label string, cents int64, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(11).Muted()
		ui.Text(c, domain.CentsToYuan(cents)).FontSize(12).
			Weight(gfx.WeightMedium).Color(color)
	})
}

// pager is the row under the table: the total on the left and Viva's
// pagination component on the right.
func (m *recordsModel) pager(c ui.C) {
	ui.Row(c).Height(48).FillWidth().PadX(18).Gap(12).Items(ui.Center).Children(func() {
		ui.Text(c, m.countText).FontSize(12).Muted()
		ui.Spacer(c)
		// The component paints its own hit regions, so their identities are
		// named here, before the slot that paints it.
		for index := range m.pageIDs {
			m.pageIDs[index] = ui.ID(c)
		}
		m.buildPageItems()
		ui.Slot(c).Width(480).Height(32).Draw(m.drawPagination)
	})
}

// pageCount is how many pages the current rows take.
func (m *recordsModel) pageCount() int {
	if len(m.shown) == 0 || m.pageSize <= 0 {
		return 1
	}
	pages := (len(m.shown) + m.pageSize - 1) / m.pageSize
	if pages < 1 {
		pages = 1
	}
	return pages
}

// buildPageItems fills the seven pagination slots: every page when they fit,
// otherwise the first and last page with an ellipsis and a window around the
// current one.
func (m *recordsModel) buildPageItems() {
	m.pageItems = [7]pageItem{}
	total := m.pageCount()
	if total <= len(m.pageItems) {
		for number := 1; number <= total; number++ {
			m.pageItems[number-1].number = number
		}
		return
	}
	switch {
	case m.page <= 4:
		for index := 0; index < 5; index++ {
			m.pageItems[index].number = index + 1
		}
		m.pageItems[5].ellipsis = true
		m.pageItems[6].number = total
	case m.page >= total-3:
		m.pageItems[0].number = 1
		m.pageItems[1].ellipsis = true
		for index := 0; index < 5; index++ {
			m.pageItems[2+index].number = total - 4 + index
		}
	default:
		m.pageItems[0].number = 1
		m.pageItems[1].ellipsis = true
		m.pageItems[2].number = m.page - 1
		m.pageItems[3].number = m.page
		m.pageItems[4].number = m.page + 1
		m.pageItems[5].ellipsis = true
		m.pageItems[6].number = total
	}
}

// drawPagination paints Viva's pagination component against the right edge of
// its slot and turns a click into a page change.
func (m *recordsModel) drawPagination(d *ui.DrawContext) {
	theme := *d.Theme
	metrics := theme.Metrics
	itemHeight := min(metrics.ControlHeight, d.Rect.Height)
	edge := metrics.GapItem
	inner := metrics.GapItem * 1.25
	gap := metrics.GapItem * 0.75
	spacing := metrics.GapItem / 2

	previousWidth := edge + 16 + gap +
		d.Canvas.TextWidthWeight("上一页", metrics.FontSizeM, gfx.WeightMedium) + inner
	nextWidth := edge + 16 + gap +
		d.Canvas.TextWidthWeight("下一页", metrics.FontSizeM, gfx.WeightMedium) + inner
	items := 0
	for _, item := range m.pageItems {
		if item.number > 0 || item.ellipsis {
			items++
		}
	}
	width := previousWidth + nextWidth + float32(items)*itemHeight +
		float32(items+1)*spacing
	if width > d.Rect.Width {
		width = d.Rect.Width
	}
	bounds := gfx.Rect{
		X: d.Rect.X + d.Rect.Width - width, Y: d.Rect.Y,
		Width: width, Height: itemHeight,
	}

	pagination := components.BeginPagination(d.Canvas, theme, bounds)
	pagination.AddPrevious(m.pageIDs[0], "上一页", m.page <= 1)
	for index, item := range m.pageItems {
		switch {
		case item.ellipsis:
			pagination.AddEllipsis()
		case item.number > 0:
			pagination.AddPage(m.pageIDs[index+1], strconv.Itoa(item.number), item.number == m.page)
		}
	}
	pagination.AddNext(m.pageIDs[8], "下一页", m.page >= m.pageCount())

	if id, ok := d.Canvas.ClickTarget(); ok && m.isPageID(id) {
		d.Canvas.AfterPaint(func() { m.choosePage(id) })
	}
}

// isPageID reports whether id belongs to the pagination row, so a click
// elsewhere does not reach choosePage.
func (m *recordsModel) isPageID(id gfx.ID) bool {
	if id == m.pageIDs[0] || id == m.pageIDs[8] {
		return true
	}
	for index, item := range m.pageItems {
		if item.number > 0 && m.pageIDs[index+1] == id {
			return true
		}
	}
	return false
}

// choosePage applies a click on one of the pagination items.
func (m *recordsModel) choosePage(id gfx.ID) {
	pages := m.pageCount()
	switch id {
	case m.pageIDs[0]:
		if m.page > 1 {
			m.page--
		}
	case m.pageIDs[8]:
		if m.page < pages {
			m.page++
		}
	default:
		for index, item := range m.pageItems {
			if item.number > 0 && m.pageIDs[index+1] == id {
				m.page = item.number
				break
			}
		}
	}
	m.page = min(max(m.page, 1), pages)
	m.list.Select(-1)
}

// refilter recomputes the visible rows and the statistics.
func (m *recordsModel) refilter() {
	start := m.rangeStart(time.Now())

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
	m.countText = "共 " + strconv.Itoa(len(m.shown)) + " 条"
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
