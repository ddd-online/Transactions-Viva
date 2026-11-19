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

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
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
	recordAmountWidth   = float32(120)
)

// record is one row of the mock table. The store's TransactionRecord plus its
// tag rows becomes this shape once the data is real.
type record struct {
	id          string
	kind        string
	category    string
	description string
	tags        []string
	amount      int64 // 分
	at          time.Time
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

var typeChoices = [...]struct {
	label string
	kind  string
}{
	{"全部", ""},
	{"支出", kindExpense},
	{"收入", kindIncome},
	{"转账", kindTransfer},
}

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
// section exists, so the filters and scroll survive a section switch.
type recordsModel struct {
	records []record
	shown   []record // the rows the current filters leave, already sorted

	counts [4]int   // 全部 / 支出 / 收入 / 转账, within the time range
	stats  [3]int64 // 收入 / 支出 / 转账, within the visible rows

	kind      string // "" means every type
	rangeMode rangeMode
	sortField sortField
	sortDesc  bool
	countText string

	list      ui.ListState
	rangeMenu bool
	sortMenu  bool
	draft     recordDraft
}

// newRecordsModel returns the page on 今天, newest first, with the mock rows
// already filtered.
func newRecordsModel() *recordsModel {
	m := &recordsModel{
		records:   mockRecords(),
		rangeMode: rangeToday,
		sortField: sortDate,
		sortDesc:  true,
	}
	m.refilter()
	return m
}

// View declares the records page.
func (m *recordsModel) View(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		m.toolbar(c)
		ui.Divider(c)
		m.filters(c)
		ui.Divider(c)
		m.table(c)
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
		if ui.Button(c, "记一笔").Clicked() {
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

// filters is the row under the toolbar: one chip per transaction type, with the
// number of rows the time range leaves, and the range in words.
func (m *recordsModel) filters(c ui.C) {
	theme := c.Theme()
	ui.Row(c).Height(44).FillWidth().PadX(18).Gap(6).Items(ui.Center).Children(func() {
		for index, choice := range typeChoices {
			selected := m.kind == choice.kind
			c.KeyInt(index)
			chip := ui.Row(c).Height(28).Shrink(0).PadX(10).Gap(6).
				Items(ui.Center).Radius(999)
			if selected {
				chip = chip.Background(theme.Accent)
			}
			chip = chip.HitWith(ui.ID(c), choice.label,
				gfx.HitOptions{Role: gfx.RoleRadio, Selected: selected}).
				Hover(ui.Look{}.Background(theme.Hover))
			foreground := theme.MutedForeground
			if selected {
				foreground = theme.AccentForeground
			}
			chip.Children(func() {
				ui.Text(c, choice.label).FontSize(12).Weight(gfx.WeightMedium).Color(foreground)
				ui.Text(c, strconv.Itoa(m.counts[index])).FontSize(11).Color(theme.MutedForeground)
			})
			if chip.Clicked() {
				m.kind = choice.kind
				m.refilter()
			}
		}
		ui.Spacer(c)
		ui.Text(c, m.rangeSummary(time.Now())).FontSize(11).Muted()
	})
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
			ui.Text(c, "金额").Width(recordAmountWidth).TextEnd().FontSize(11).Muted()
		})
		ui.Divider(c)
		if len(m.shown) == 0 {
			m.empty(c)
			return
		}
		ui.List(c, &m.list, len(m.shown), m.row).Label("记录").Grow(1)
	})
}

// row declares one record: date and time, type, category, tags, description and
// the signed amount.
func (m *recordsModel) row(r ui.ListRow) {
	rec := &m.shown[r.Index]
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
	ui.Text(r.C, rec.signedAmount()).Width(recordAmountWidth).TextEnd().
		FontSize(13).Weight(gfx.WeightMedium).Color(color)
}

// empty is what the content area says when the filters match nothing.
func (m *recordsModel) empty(c ui.C) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Children(func() {
		ui.Text(c, "没有匹配的记录").FontSize(16).Bold()
		ui.Text(c, "换个时间范围或筛选条件，也可以直接记一笔。").FontSize(13).Muted()
		if ui.Button(c, "清除筛选").Outline().Clicked() {
			m.kind, m.rangeMode = "", rangeToday
			m.refilter()
		}
	})
}

// footer is the records' 收入 / 支出 / 转账 summary and the visible row count.
func (m *recordsModel) footer(c ui.C) {
	theme := c.Theme()
	ui.Row(c).Height(40).FillWidth().PadX(18).Gap(24).Items(ui.Center).Children(func() {
		m.stat(c, "收入", m.stats[0], theme.Success)
		m.stat(c, "支出", m.stats[1], theme.Danger)
		m.stat(c, "转账", m.stats[2], theme.MutedForeground)
		ui.Spacer(c)
		ui.Text(c, m.countText).FontSize(11).Muted()
	})
}

func (m *recordsModel) stat(c ui.C, label string, cents int64, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(11).Muted()
		ui.Text(c, domain.CentsToYuan(cents)).FontSize(12).
			Weight(gfx.WeightMedium).Color(color)
	})
}

// refilter recomputes the visible rows, the chip counts and the statistics.
func (m *recordsModel) refilter() {
	start := m.rangeStart(time.Now())

	m.shown = m.shown[:0]
	m.counts = [4]int{}
	m.stats = [3]int64{}
	for _, rec := range m.records {
		if !start.IsZero() && rec.at.Before(start) {
			continue
		}
		m.counts[0]++
		switch rec.kind {
		case kindExpense:
			m.counts[1]++
		case kindIncome:
			m.counts[2]++
		case kindTransfer:
			m.counts[3]++
		}
		if m.kind != "" && rec.kind != m.kind {
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

// rangeSummary names the chosen range and the number of rows it leaves.
func (m *recordsModel) rangeSummary(now time.Time) string {
	var label string
	switch m.rangeMode {
	case rangeToday:
		label = now.Format("2006-01-02")
	case rangeMonth:
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		label = start.Format("2006-01-02") + " 至 " + start.AddDate(0, 1, -1).Format("2006-01-02")
	case rangeYear:
		label = strconv.Itoa(now.Year()) + " 年"
	default:
		label = "全部时间"
	}
	return label + " · " + m.countText
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

func kindColor(theme gfx.Theme, kind string) gfx.Color {
	switch kind {
	case kindIncome:
		return theme.Success
	case kindExpense:
		return theme.Danger
	}
	return theme.MutedForeground
}
