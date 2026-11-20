// Package records is the 记录 screen of the 记账 section: the toolbar, the table,
// its pager and statistics, and the dialog a record is written in (the composer
// subpackage). Its layout follows the reference application's hosts page.
//
// The rows come from internal/store through repository.Repository: a date
// range is queried in SQL, then the page keeps its multi-column sorting,
// statistics and pagination in memory.
package records

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/pagination"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/pages/accounting/ledger"
	"transactions/internal/pages/accounting/records/composer"
	tpl "transactions/internal/pages/accounting/template"
	"transactions/internal/repository"
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

// record is one row of the table: the store's TransactionRecord folded with
// its tag names for painting.
type record struct {
	id           string
	ledgerID     string
	kind         string
	category     string
	description  string
	tags         []string
	amount       int64 // 分
	at           time.Time
	outlier      bool
	keyEventDate string // 关联的事件日期（空串 = 不关联）
}

// sortField is a column the table can be ordered by.
type sortField uint8

const (
	sortDate sortField = iota
	sortAmount
)

// sortKey is one active sort column. The slice keeps the order the user chose
// the columns in, so its first entry is the primary sort.
type sortKey struct {
	field sortField
	desc  bool
}

// Model is the 记录 screen's state. The shell owns it for as long as the section
// exists, so its filters and scroll survive a section switch.
type Model struct {
	records []record
	shown   []record // every row the current range leaves, already sorted

	stats [3]int64 // 收入 / 支出 / 转账, within the visible rows

	// ledgers is the section's shared ledger list; ledgerID caches its active
	// choice for filtering, saving and the table. A switch on any accounting
	// page runs the onLedgerChanged callback, which reloads this page.
	ledgers         *ledger.Store
	ledgerID        string
	repo            repository.Repository
	onLedgerChanged func()
	labels          repository.LabelSnapshot

	// rangeStart and rangeEnd are the visible time range as local dates, both
	// ends included; rangeGrain is the unit the picker chooses them in.
	rangeStart string
	rangeEnd   string
	rangeGrain datepicker.Grain
	// rangeStale says a range picked in a painted panel still has to be filtered.
	rangeStale bool

	// sorts are the active sort columns, most significant first. The table is
	// unsorted until a header is clicked; each click cycles its column.
	sorts []sortKey

	// page is the 1-based page of the table; pageSize is the page size.
	// pageStart is where the current page begins in shown, set by table.
	page      int
	pageSize  int
	pageStart int

	list      ui.ListState
	templates *tpl.Cache
	form      composer.State
	formSpec  composer.Spec
	bounds    gfx.Rect
	rangePick datepicker.State

	// categoryList and tagList are the composer's choices, rebuilt each paint
	// into the same slices (see categoryNames / tagOptions).
	categoryList []string
	tagList      []string

	notice        string
	deletingID    string
	deleteConfirm bool
}

// New returns an empty page with its own ledger and template stores. The
// accounting parent calls SetDataSource once a workspace is open.
func New() *Model { return NewWithStores(ledger.NewStore(), tpl.NewCache()) }

// NewWithStores returns the page over the section's shared ledger and template
// stores, so the 记账 pages switch the same ledger and see the same templates.
func NewWithStores(ledgers *ledger.Store, templates *tpl.Cache) *Model {
	if ledgers == nil {
		ledgers = ledger.NewStore()
	}
	if templates == nil {
		templates = tpl.NewCache()
	}
	m := &Model{
		ledgers:    ledgers,
		ledgerID:   ledgers.Active(),
		templates:  templates,
		rangeStart: todayDate(),
		rangeEnd:   todayDate(),
		rangeGrain: datepicker.ByDay,
		page:       1,
		pageSize:   15,
		form:       composer.New(),
		rangePick:  datepicker.NewState(),
	}
	m.formSpec = composer.Spec{
		Categories: m.categoryNames,
		TagOptions: m.tagOptions,
		Templates:  m.templateList,
		OnSave:     m.saveDraft,
		OnTemplate: m.saveTemplate,
	}
	m.refilter()
	return m
}

// SetDataSource connects the page to the workspace. Reload fills the current
// range and the composer's category/tag/template choices.
func (m *Model) SetDataSource(repo repository.Repository, onLedgerChanged func()) {
	m.repo = repo
	m.onLedgerChanged = onLedgerChanged
	if repo == nil {
		m.records = nil
		m.labels = repository.LabelSnapshot{}
		m.ledgerID = ""
		m.refilter()
		return
	}
	m.Reload()
}

// Reload reads the active ledger's records in the current date range, plus the
// labels and templates the composer needs, and rebuilds the visible page.
func (m *Model) Reload() {
	if m.repo == nil {
		return
	}
	active := m.ledgers.Active()
	m.ledgerID = active
	if active == "" {
		m.records = nil
		m.refilter()
		return
	}
	ctx := context.Background()
	entries, err := m.repo.Records(ctx, repository.RecordQuery{
		LedgerID: active, StartDate: m.rangeStart, EndDate: m.rangeEnd,
	})
	if err != nil {
		m.records = nil
		m.refilter()
		m.notice = "读取记录失败：" + err.Error()
		return
	}
	m.records = make([]record, 0, len(entries))
	for _, entry := range entries {
		m.records = append(m.records, recordFromEntry(entry))
	}
	notice := ""
	if labels, err := m.repo.Labels(ctx, active); err == nil {
		m.labels = labels
	} else {
		notice = "读取分类标签失败：" + err.Error()
	}
	if templates, err := m.repo.Templates(ctx, active); err == nil {
		list := make([]tpl.Template, 0, len(templates))
		for _, one := range templates {
			list = append(list, tpl.FromDomain(one))
		}
		m.templates.Set(active, list)
	}
	m.refilter()
	if notice != "" {
		m.notice = notice
	}
}

// recordFromEntry folds a stored record and its tag names into a row.
func recordFromEntry(entry repository.RecordEntry) record {
	outlier := false
	if strings.HasPrefix(strings.TrimSpace(entry.Record.Flags), "{") {
		var flags struct {
			Outlier bool `json:"outlier"`
		}
		if json.Unmarshal([]byte(entry.Record.Flags), &flags) == nil {
			outlier = flags.Outlier
		}
	}
	return record{
		id:           entry.Record.TransactionID,
		ledgerID:     entry.Record.LedgerID,
		kind:         entry.Record.TransactionType,
		category:     entry.Record.Category,
		description:  entry.Record.Description,
		tags:         append([]string(nil), entry.Tags...),
		amount:       entry.Record.AmountCents,
		at:           time.Unix(entry.Record.TransactionAt, 0).In(time.Local),
		outlier:      outlier,
		keyEventDate: entry.Record.KeyEventDate,
	}
}

// View declares the records page.
func (m *Model) View(c ui.C) {
	m.bounds = c.Bounds()
	if m.rangeStale {
		m.rangeStale = false
		m.Reload()
	}
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
	m.form.View(c, m.formSpec)
	m.deleteView(c)
}

// toolbar is the page's actions, laid out like the old page's filter bar: the
// time range on the left, and the sort beside the primary action on the right.
func (m *Model) toolbar(c ui.C) {
	ui.Row(c).Height(56).FillWidth().PadX(18).Gap(8).Items(ui.Center).Children(func() {
		if ledger.Picker(c, m.ledgers) && m.onLedgerChanged != nil {
			m.onLedgerChanged()
		}
		// The same divider the tags and templates toolbars use: the ledger
		// selector stands apart from the page's own controls.
		ui.Divider(c).Height(24)
		if ui.Button(c, "今天").Outline().Clicked() {
			m.setRange(todayDate(), todayDate(), datepicker.ByDay)
		}
		datepicker.Picker(c, &m.rangePick, datepicker.Spec{
			Label: "选择时间范围", Start: m.rangeStart, End: m.rangeEnd,
			Grain: m.rangeGrain, Grains: true, Periods: true, Viewport: m.bounds,
			OnGrain: func(grain datepicker.Grain) {
				m.rangeGrain, m.rangeStale = grain, true
			},
			OnRange: func(start, end string) {
				m.rangeStart, m.rangeEnd, m.rangeStale = start, end, true
			},
		})

		ui.Spacer(c)
		if ui.Button(c, "记一笔").Icon(icons.Plus).Clicked() {
			m.openComposer()
		}
	})
}

// sortHeader is a column title with a square sort button beside it. The
// button cycles the column: none, descending, ascending. Columns sort in the
// order they were chosen, so the first one clicked is the primary sort.
func (m *Model) sortHeader(c ui.C, label string, field sortField, width float32, end bool) {
	row := ui.Row(c).Width(width).Height(34).Items(ui.Center).Gap(4)
	if end {
		row = row.Justify(ui.End)
	}
	row.Children(func() {
		if end {
			m.sortButton(c, label, field)
		}
		ui.Text(c, label).FontSize(11).Muted()
		if !end {
			m.sortButton(c, label, field)
		}
	})
}

// sortButton is the square button of a sortable column: a neutral up/down icon
// while the column is off, a direction icon in the primary color on an active
// tint once it sorts.
func (m *Model) sortButton(c ui.C, label string, field sortField) {
	theme := c.Theme()
	desc, rank := m.sortState(field)
	glyph, color := icons.ArrowUpDown, theme.MutedForeground
	switch {
	case rank == 0:
	case desc:
		glyph, color = icons.ArrowUpWideNarrow, theme.Primary
	default:
		glyph, color = icons.ArrowDownNarrowWide, theme.Primary
	}
	button := ui.Row(c).Size(24, 24).Items(ui.Center).Justify(ui.Center).
		Radius(theme.Metrics.RadiusItem()).
		HitWith(ui.ID(c), "按"+label+"排序", gfx.HitOptions{
			Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
			Hint: "点击切换 未选 / 降序 / 升序",
		}).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	if rank == 0 {
		button = button.Hover(ui.Look{}.Background(theme.Accent)).
			Pressed(ui.Look{}.Background(theme.Border))
	} else {
		tint, hover, pressed := theme.Primary, theme.Primary, theme.Primary
		tint.Alpha, hover.Alpha, pressed.Alpha = 0.14, 0.22, 0.3
		button = button.Background(tint).
			Hover(ui.Look{}.Background(hover)).
			Pressed(ui.Look{}.Background(pressed))
	}
	button.Children(func() {
		ui.Icon(c, glyph).Size(14, 14).Color(color)
	})
	if button.Clicked() {
		m.cycleSort(field)
	}
}

// sortState returns the column's direction and its 1-based priority, or rank 0
// while the column is not sorting.
func (m *Model) sortState(field sortField) (desc bool, rank int) {
	for index, key := range m.sorts {
		if key.field == field {
			return key.desc, index + 1
		}
	}
	return false, 0
}

// cycleSort moves a column through none, descending and ascending. Turning a
// column on appends it, so the first column clicked is the primary sort.
func (m *Model) cycleSort(field sortField) {
	for index, key := range m.sorts {
		if key.field != field {
			continue
		}
		if key.desc {
			m.sorts[index].desc = false
		} else {
			m.sorts = append(m.sorts[:index], m.sorts[index+1:]...)
		}
		m.refilter()
		return
	}
	m.sorts = append(m.sorts, sortKey{field: field, desc: true})
	m.refilter()
}

// table is the column header and the scrolling rows.
func (m *Model) table(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		ui.Row(c).Height(34).FillWidth().PadX(18).Gap(12).Items(ui.Center).
			Background(theme.Muted).Children(func() {
			m.sortHeader(c, "日期", sortDate, recordDateWidth, false)
			ui.Text(c, "类型").Width(recordTypeWidth).FontSize(11).Muted()
			ui.Text(c, "分类").Width(recordCategoryWidth).FontSize(11).Muted()
			ui.Text(c, "标签").Width(recordTagsWidth).FontSize(11).Muted()
			ui.Text(c, "描述").Grow(1).FontSize(11).Muted()
			ui.Text(c, "标记").Width(recordMarksWidth).TextCenter().FontSize(11).Muted()
			m.sortHeader(c, "金额", sortAmount, recordAmountWidth, true)
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
func (m *Model) row(r ui.ListRow) {
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
			for _, target := range m.ledgers.List() {
				if target.ID == m.ledgerID {
					continue
				}
				if sub.Item(target.Name).Clicked() {
					m.syncTo(id, target.ID)
				}
			}
		})
		menu.Separator()
		if menu.Item("删除记录").Destructive().Clicked() {
			m.deletingID, m.deleteConfirm = id, true
		}
	})
}

// emptyActions are the buttons the empty state offers, in order.
const emptyActionCount = 3

// emptyActions are their labels: the last one is the primary action.
var emptyActions = [emptyActionCount]string{"上月", "今年", "记一笔"}

// empty is what the content area says when the time range matches nothing: the
// theme's empty state, with the actions the old page offered underneath it.
func (m *Model) empty(c ui.C) {
	theme := c.Theme()
	var ids [emptyActionCount]gfx.ID
	for index := range ids {
		ids[index] = ui.ID(c)
	}
	row := viva.BeginButton(nil, *theme, gfx.ID{}, gfx.Rect{}).BoundsHeight()
	ui.Slot(c).Grow(1).FillWidth().Draw(func(d *ui.DrawContext) {
		content := viva.Empty(d.Canvas, *d.Theme, d.Rect, viva.EmptyData{
			Media:         viva.EmptyMediaIcon,
			Icon:          icons.Wallet,
			Title:         "今天还没有记录",
			Description:   "换个时间范围，或直接记一笔。",
			ContentHeight: row,
		})
		m.paintEmptyActions(d, ids, content)
	})
}

// paintEmptyActions paints the empty state's buttons into the row the native
// empty state reserved, and applies a press on them after the paint.
func (m *Model) paintEmptyActions(d *ui.DrawContext, ids [emptyActionCount]gfx.ID, content gfx.Rect) {
	if content.Width <= 0 || content.Height <= 0 {
		return
	}
	theme := *d.Theme
	metrics := theme.Metrics
	var widths [emptyActionCount]float32
	total := float32(0)
	for index, label := range emptyActions {
		widths[index] = viva.BeginButton(d.Canvas, theme, ids[index], content).
			SizeToFit(label).BoundsWidth()
		total += widths[index]
	}
	total += float32(emptyActionCount-1) * metrics.GapItem
	x := content.X + max((content.Width-total)/2, 0)
	for index, label := range emptyActions {
		button := viva.BeginButton(d.Canvas, theme, ids[index], gfx.Rect{
			X: x, Y: content.Y, Width: widths[index], Height: content.Height,
		})
		if index == emptyActionCount-1 {
			button.Default(label)
		} else {
			button.Secondary(label)
		}
		x += widths[index] + metrics.GapItem
	}
	switch {
	case d.Canvas.Clicked(ids[0]):
		d.Canvas.AfterPaint(m.lastMonth)
	case d.Canvas.Clicked(ids[1]):
		d.Canvas.AfterPaint(m.thisYear)
	case d.Canvas.Clicked(ids[2]):
		d.Canvas.AfterPaint(m.openComposer)
	}
}

// CloseOverlays closes the panels this page floats over itself: the range picker
// and the composer with everything it floats.
func (m *Model) CloseOverlays() {
	m.rangePick.Open = false
	m.form.Close()
	m.deletingID, m.deleteConfirm = "", false
}

// lastMonth shows the whole previous month.
func (m *Model) lastMonth() {
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, -1, 0)
	m.setRange(first.Format(dateLayout), first.AddDate(0, 1, -1).Format(dateLayout), datepicker.ByMonth)
}

// thisYear shows the whole current year.
func (m *Model) thisYear() {
	now := time.Now()
	m.setRange(time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local).Format(dateLayout),
		time.Date(now.Year(), 12, 31, 0, 0, 0, 0, time.Local).Format(dateLayout), datepicker.ByYear)
}

// footer is the records' 收入 / 支出 / 转账 summary and the visible row count.
func (m *Model) footer(c ui.C) {
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

func (m *Model) stat(c ui.C, label string, cents int64, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(11).Muted()
		ui.Text(c, domain.CentsToYuan(cents)).FontSize(12).
			Weight(gfx.WeightMedium).Color(color)
	})
}

// pager is the shared pagination bar: total, page-size select and Viva pages.
func (m *Model) pager(c ui.C) {
	pagination.Bar(c, pagination.Spec{
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
func (m *Model) pageCount() int {
	if len(m.shown) == 0 || m.pageSize <= 0 {
		return 1
	}
	return max((len(m.shown)+m.pageSize-1)/m.pageSize, 1)
}

// refilter recomputes the visible rows and the statistics.
func (m *Model) refilter() {

	m.notice = ""
	m.shown = m.shown[:0]
	m.stats = [3]int64{}
	for _, rec := range m.records {
		if rec.ledgerID != m.ledgerID {
			continue
		}
		day := rec.at.Format(dateLayout)
		if m.rangeStart != "" && day < m.rangeStart {
			continue
		}
		if m.rangeEnd != "" && day > m.rangeEnd {
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
		for _, key := range m.sorts {
			order := compareRecords(left, right, key.field)
			if order == 0 {
				continue
			}
			if key.desc {
				return order > 0
			}
			return order < 0
		}
		return false
	})
	m.page = 1
	m.list.Select(-1)
}

// compareRecords orders two rows by one column, ascending: -1 when left comes
// first, +1 when right does, 0 when they tie. The amount is compared as the row
// shows it, so an expense (a negative number) sorts below income and transfers.
func compareRecords(left, right record, field sortField) int {
	if field == sortAmount {
		leftAmount, rightAmount := signedAmount(left), signedAmount(right)
		switch {
		case leftAmount < rightAmount:
			return -1
		case leftAmount > rightAmount:
			return 1
		}
		return 0
	}
	switch {
	case left.at.Before(right.at):
		return -1
	case left.at.After(right.at):
		return 1
	}
	return 0
}

// signedAmount is a row's amount with the sign it reads with: an expense spends,
// so it is negative; income and transfers stay positive.
func signedAmount(rec record) int64 {
	if rec.kind == kindExpense {
		return -rec.amount
	}
	return rec.amount
}

// dateLayout is the local YYYY-MM-DD form the page compares its dates in.
const dateLayout = "2006-01-02"

// todayDate is today as the page's ranges hold it.
func todayDate() string { return time.Now().Format(dateLayout) }

// setRange applies a new visible range from a control of the page. The next
// paint reloads the rows from the workspace; reloading here would query SQLite
// while a callback is still painting.
func (m *Model) setRange(start, end string, grain datepicker.Grain) {
	m.rangeStart, m.rangeEnd, m.rangeGrain = start, end, grain
	m.rangeStale = true
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
