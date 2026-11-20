// Package analysis is the 记账 · 分析 screen: saved line and pie charts over a
// date range. Charts live in the workspace's chart table; presets are seeded
// once per ledger and can be deleted. The layout is this application's own:
// a chart rail, a canvas, the curve configuration, and a right-hand summary.
package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/selectbox"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/pages/accounting/ledger"
	"transactions/internal/repository"
)

const (
	chartLine = "line"
	chartPie  = "pie"

	groupType     = "transactionType"
	groupCategory = "category"
	groupTag      = "tag"

	granularityMonth = "month"
	granularityYear  = "year"
)

var (
	chartTypeLabels   = [...]string{"折线图", "饼图"}
	groupLabels       = [...]string{"交易类型", "分类", "标签"}
	granularityLabels = [...]string{"月", "年"}
	lineTypeLabels    = [...]string{"支出", "收入", "转账"}
	lineTypeValues    = [...]string{"expense", "income", "transfer"}
	tagPolicyLabels   = [...]string{"任意", "全部"}
	tagPolicyValues   = [...]string{"any", "all"}
)

var (
	groupValues       = [...]string{groupType, groupCategory, groupTag}
	granularityValues = [...]string{granularityMonth, granularityYear}
)

// Model is the analysis page's state.
type Model struct {
	repo            repository.Repository
	ledgers         *ledger.Store
	onLedgerChanged func()
	ledgerID        string

	rangeStart string
	rangeEnd   string
	rangeGrain datepicker.Grain
	rangePick  datepicker.State
	rangeStale bool

	charts     []domain.Chart
	selectedID string
	lines      []domain.ChartLine
	lineData   []repository.ChartLineData
	stats      repository.ChartStatistics
	pie        repository.ChartPieResult
	pieSlices  []repository.ChartPieSlice
	list       ui.ListState

	categories []domain.Category
	tagsByKind map[string][]string
	tagKeyed   map[string][]string
	tagSelect  tagSelect
	loadErr    string
	toaster    ui.Toaster
	bounds     gfx.Rect

	modal      modalKind
	dialogOpen bool
	form       form
	deleting   domain.Chart
}

// New returns an empty analysis page over the section's shared ledger store.
func New(ledgers *ledger.Store) *Model {
	if ledgers == nil {
		ledgers = ledger.NewStore()
	}
	now := time.Now()
	year := now.Year()
	m := &Model{
		ledgers:    ledgers,
		ledgerID:   ledgers.Active(),
		rangeStart: fmt.Sprintf("%04d-01-01", year),
		rangeEnd:   fmt.Sprintf("%04d-12-31", year),
		rangeGrain: datepicker.ByMonth,
		rangePick:  datepicker.NewState(),
		list:       ui.ListState{},
	}
	m.list.Key = func(index int) int { return index }
	m.list.Label = func(index int) string {
		if index < 0 || index >= len(m.charts) {
			return ""
		}
		return m.charts[index].Title
	}
	m.list.Reorder = m.moveChart
	m.initTagSelect()
	return m
}

// SetDataSource connects the page to the workspace.
func (m *Model) SetDataSource(repo repository.Repository, onLedgerChanged func()) {
	m.repo = repo
	m.onLedgerChanged = onLedgerChanged
	if repo == nil {
		m.charts = nil
		m.selectedID = ""
		m.lines, m.lineData, m.pie, m.pieSlices = nil, nil, repository.ChartPieResult{}, nil
		return
	}
	m.Reload()
}

// CloseOverlays closes the analysis dialogs.
func (m *Model) CloseOverlays() {
	m.modal = modalNone
	m.dialogOpen = false
}

// Reload loads the active ledger's saved charts and the selected chart's data.
func (m *Model) Reload() {
	if m.repo == nil {
		return
	}
	active := m.ledgers.Active()
	if active != m.ledgerID {
		m.ledgerID = active
		m.selectedID = ""
		m.list.Select(-1)
	}
	m.ledgerID = active
	if active == "" {
		m.charts = nil
		m.loadErr = ""
		return
	}
	ctx := context.Background()
	if err := m.repo.EnsureChartPresets(ctx, active); err != nil {
		m.loadErr = "初始化预设图表失败：" + err.Error()
	}
	if snapshot, err := m.repo.Labels(ctx, active); err == nil {
		m.categories = snapshot.Categories
		m.tagsByKind = map[string][]string{}
		m.tagKeyed = map[string][]string{}
		for _, tag := range snapshot.Tags {
			category, kind, ok := splitTagCategory(tag.CategoryTransactionType)
			if !ok {
				continue
			}
			m.tagsByKind[kind] = append(m.tagsByKind[kind], tag.Name)
			key := kind + "\x00" + category
			m.tagKeyed[key] = append(m.tagKeyed[key], tag.Name)
		}
	}
	charts, err := m.repo.Charts(ctx, active)
	if err != nil {
		m.loadErr = "读取图表失败：" + err.Error()
		m.charts = nil
		return
	}
	m.loadErr = ""
	m.charts = charts
	found := false
	for index := range charts {
		if charts[index].ChartID == m.selectedID {
			m.list.Select(index)
			found = true
			break
		}
	}
	if !found {
		m.selectedID = ""
		if len(charts) > 0 {
			m.selectedID = charts[0].ChartID
			m.list.Select(0)
		} else {
			m.list.Select(-1)
		}
	}
	m.loadSelected()
}

// ReloadData re-queries only the chart data (the range changed). Chart
// definitions and the rail stay as they are.
func (m *Model) ReloadData() {
	m.loadSelected()
}

// loadSelected decodes the selected chart's lines and fetches its data.
func (m *Model) loadSelected() {
	m.loadErr = ""
	m.lines, m.lineData, m.pie, m.pieSlices = nil, nil, repository.ChartPieResult{}, nil
	m.stats = repository.ChartStatistics{}
	chart := m.selectedChart()
	if chart == nil || m.repo == nil {
		return
	}
	lines, err := decodeLines(chart.ChartLines)
	if err != nil {
		m.loadErr = "图表曲线配置损坏：" + err.Error()
		return
	}
	m.lines = lines
	ctx := context.Background()
	if chart.ChartType == chartPie {
		result, err := m.repo.ChartPie(ctx, repository.ChartPieQuery{
			LedgerID: m.ledgerID, StartDate: m.rangeStart, EndDate: m.rangeEnd,
			GroupField: chart.GroupField, GroupTransactionType: chart.GroupTransactionType,
			GroupCategory: chart.GroupCategory,
		})
		if err != nil {
			m.loadErr = "读取饼图失败：" + err.Error()
			return
		}
		m.pie = result
		m.pieSlices = foldPie(result.Slices)
		return
	}
	data, stats, err := m.repo.ChartData(ctx, repository.ChartQuery{
		LedgerID: m.ledgerID, StartDate: m.rangeStart, EndDate: m.rangeEnd,
		Granularity: chart.Granularity, Lines: lines,
	})
	if err != nil {
		m.loadErr = "读取图表失败：" + err.Error()
		return
	}
	m.lineData, m.stats = data, stats
}

func (m *Model) selectedChart() *domain.Chart {
	for index := range m.charts {
		if m.charts[index].ChartID == m.selectedID {
			return &m.charts[index]
		}
	}
	return nil
}

// View paints the toolbars, chart rail, canvas and summary.
func (m *Model) View(c ui.C) {
	m.bounds = c.Bounds()
	if m.rangeStale {
		m.rangeStale = false
		m.loadSelected()
	}
	ui.Column(c).Fill().Children(func() {
		m.toolbar(c)
		ui.Divider(c)
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			m.chartRail(c)
			ui.Divider(c)
			m.detail(c)
		})
	})
	m.modals(c)
	ui.Toasts(c, &m.toaster)
}

func (m *Model) toolbar(c ui.C) {
	ui.Row(c).Height(56).FillWidth().PadX(18).Gap(8).Items(ui.Center).Children(func() {
		if ledger.Picker(c, m.ledgers) && m.onLedgerChanged != nil {
			m.onLedgerChanged()
		}
		ui.Divider(c).Height(24)
		datepicker.Picker(c, &m.rangePick, datepicker.Spec{
			Label: "选择分析范围", Start: m.rangeStart, End: m.rangeEnd,
			Grain: m.rangeGrain, Grains: true, Periods: true, Viewport: m.bounds,
			OnGrain: func(grain datepicker.Grain) {
				m.rangeGrain, m.rangeStale = grain, true
			},
			OnRange: func(start, end string) {
				m.rangeStart, m.rangeEnd, m.rangeStale = start, end, true
			},
		})
		ui.Spacer(c)
		if ui.Button(c, "新增图表").Icon(icons.Plus).Clicked() {
			m.openCreate()
		}
	})
}

func (m *Model) chartRail(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Width(240).FillHeight().Children(func() {
		ui.Row(c).Height(46).FillWidth().PadX(18).Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, "图表").FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
			ui.Text(c, fmt.Sprintf("%d", len(m.charts))).FontSize(theme.Metrics.FontSizeS).
				Color(theme.MutedForeground).Background(theme.Muted).
				Radius(theme.Metrics.RadiusItem()).PadX(6).PadY(1)
			ui.Spacer(c)
		})
		ui.Divider(c)
		if len(m.charts) == 0 {
			empty(c, "暂无图表")
			return
		}
		ui.List(c, &m.list, len(m.charts), m.chartRow).Label("图表").Grow(1)
		if m.list.Changed() {
			index := m.list.Selected()
			if index >= 0 && index < len(m.charts) {
				m.selectedID = m.charts[index].ChartID
				m.loadSelected()
			}
		}
	})
}

func (m *Model) chartRow(row ui.ListRow) {
	theme := row.C.Theme()
	if row.Index >= len(m.charts) {
		return
	}
	chart := m.charts[row.Index]
	row.Element.Height(40).PadX(8).Gap(8).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Background(theme.Background)
	if row.Selected() {
		row.Element.Background(theme.Accent)
	}
	ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	ui.Text(row.C, chart.Title).FontSize(theme.Metrics.FontSizeM).Grow(1).Ellipsis()
	if chart.ChartType == chartPie {
		ui.Text(row.C, "饼图").FontSize(theme.Metrics.FontSizeS).Color(theme.MutedForeground)
	}
	remove := ui.Button(row.C, "删除图表 "+chart.Title).
		Ghost().Icon(icons.Trash2).IconOnly().Tooltip("删除图表")
	if remove.Clicked() {
		m.askDelete(chart)
	}
}

func empty(c ui.C, text string) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		ui.Text(c, text).FontSize(theme.Metrics.FontSizeM).Muted().TextCenter()
	})
}

// ------------------------------------------------------------------ detail

func (m *Model) detail(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		chart := m.selectedChart()
		if chart == nil {
			empty(c, "选择左侧图表，或点右上角「新增图表」")
			return
		}
		m.header(c, chart)
		ui.Divider(c)
		if m.loadErr != "" {
			ui.Text(c, m.loadErr).FontSize(12).Color(theme.Danger).PadX(18).PadY(6)
		}
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(14).Pad(18).Children(func() {
					m.canvas(c, chart)
					m.linePanel(c, chart)
				})
			})
			ui.Divider(c)
			m.summary(c, chart)
		})
	})
}

func (m *Model) header(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	ui.Row(c).Height(56).FillWidth().PadX(18).Gap(8).Items(ui.Center).Children(func() {
		ui.Text(c, chart.Title).FontSize(theme.Metrics.FontSizeL).Weight(gfx.WeightSemibold).Shrink(1).Ellipsis()
		ui.Text(c, chartTypeLabel(chart.ChartType)).FontSize(theme.Metrics.FontSizeS).
			Color(theme.MutedForeground).Background(theme.Muted).
			Radius(theme.Metrics.RadiusItem()).PadX(8).PadY(2)
		if chart.IsPreset {
			ui.Text(c, "预设").FontSize(theme.Metrics.FontSizeS).Color(theme.MutedForeground)
		}
		ui.Spacer(c)
		if chart.ChartType == chartLine {
			granularity := chart.Granularity
			if granularity != granularityYear {
				granularity = granularityMonth
			}
			if selectbox.Select(c, &granularity, granularityValues[:], func(value string) string {
				if value == granularityYear {
					return "年"
				}
				return "月"
			}).Label("曲线粒度").Disabled(chart.IsPreset).Changed() {
				m.changeGranularity(chart, granularity)
			}
		} else {
			m.groupControls(c, chart)
		}
		if !chart.IsPreset {
			if ui.Button(c, "重命名").Small().Clicked() {
				m.openRename(chart)
			}
		}
	})
}

func (m *Model) groupControls(c ui.C, chart *domain.Chart) {
	field := chart.GroupField
	if field != groupCategory && field != groupTag {
		field = groupType
	}
	if selectbox.Select(c, &field, groupValues[:], groupLabel).Label("饼图分组").Disabled(chart.IsPreset).Changed() {
		m.changeGrouping(chart, field, chart.GroupTransactionType, chart.GroupCategory)
	}
	if field != groupType {
		kind := chart.GroupTransactionType
		if !validLineType(kind) {
			kind = "expense"
		}
		if selectbox.Select(c, &kind, lineTypeValues[:], lineTypeLabel).Label("交易类型").Disabled(chart.IsPreset).Changed() {
			m.changeGrouping(chart, field, kind, chart.GroupCategory)
		}
		if field == groupTag {
			names := m.categoryNames(kind)
			category := chart.GroupCategory
			if !contains(names, category) {
				category = ""
			}
			if selectbox.Select(c, &category, names, nil).Placeholder("选择分类").Label("分类").Disabled(chart.IsPreset).Changed() {
				m.changeGrouping(chart, field, kind, category)
			}
		}
	}
}

// ------------------------------------------------------------------ summaries and small labels

func (m *Model) summary(c ui.C, chart *domain.Chart) {
	theme := c.Theme()
	ui.Column(c).Width(248).FillHeight().Background(theme.Muted).Children(func() {
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(10).Pad(16).Children(func() {
				if chart.ChartType == chartPie {
					m.pieSummary(c)
				} else {
					m.lineSummary(c)
				}
			})
		})
	})
}

func (m *Model) lineSummary(c ui.C) {
	theme := c.Theme()
	ui.Text(c, "曲线合计").FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
	ui.Row(c).Gap(14).Children(func() {
		ui.Text(c, "收入 "+domain.CentsToYuan(m.stats.Income)).FontSize(theme.Metrics.FontSizeS).Color(theme.Success)
		ui.Text(c, "支出 "+domain.CentsToYuan(m.stats.Expense)).FontSize(theme.Metrics.FontSizeS).Color(theme.Danger)
	})
	for index, line := range m.lineData {
		total := int64(0)
		for _, point := range line.Data {
			total += point.Amount
		}
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			dot(c, seriesColor(*theme, index))
			ui.Text(c, line.Label).FontSize(theme.Metrics.FontSizeM).Grow(1).Ellipsis()
			ui.Text(c, domain.CentsToYuan(total)).FontSize(theme.Metrics.FontSizeM).Mono()
		})
	}
	if len(m.lineData) == 0 {
		ui.Text(c, "暂无曲线").FontSize(theme.Metrics.FontSizeS).Muted()
	}
}

func (m *Model) pieSummary(c ui.C) {
	theme := c.Theme()
	ui.Text(c, "分组合计").FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
	ui.Text(c, domain.CentsToYuan(m.pie.Total)).FontSize(theme.Metrics.FontSizeL).Weight(gfx.WeightSemibold)
	for index, slice := range m.pieSlices {
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			dot(c, pieColor(*theme, index))
			ui.Text(c, slice.Label).FontSize(theme.Metrics.FontSizeM).Grow(1).Ellipsis()
			ui.Text(c, domain.CentsToYuan(slice.Amount)).FontSize(theme.Metrics.FontSizeS).Mono()
			share := float64(0)
			if m.pie.Total > 0 {
				share = float64(slice.Amount) / float64(m.pie.Total) * 100
			}
			ui.Text(c, fmt.Sprintf("%.1f%%", share)).FontSize(theme.Metrics.FontSizeS).Muted()
		})
	}
	if len(m.pieSlices) == 0 {
		ui.Text(c, pieEmptyDescription(m.pie, *m.selectedChart())).FontSize(theme.Metrics.FontSizeS).Muted()
	}
	if m.pie.Total > m.pie.FilteredTotal {
		ui.Text(c, "多标签的记录会重复计入").FontSize(theme.Metrics.FontSizeS).Muted()
	}
}

func dot(c ui.C, color gfx.Color) {
	ui.Row(c).Size(8, 8).Radius(4).Background(color)
}

func seriesColor(theme gfx.Theme, index int) gfx.Color {
	colors := theme.Chart
	if len(colors) == 0 {
		return theme.Primary
	}
	return colors[index%len(colors)]
}

func pieColor(theme gfx.Theme, index int) gfx.Color {
	if index >= 9 {
		return theme.MutedForeground
	}
	return seriesColor(theme, index)
}

func chartTypeLabel(value string) string {
	if value == chartPie {
		return "饼图"
	}
	return "折线图"
}

func groupLabel(value string) string {
	switch value {
	case groupCategory:
		return "分类"
	case groupTag:
		return "标签"
	}
	return "交易类型"
}

func lineTypeLabel(value string) string {
	for index, stored := range lineTypeValues {
		if stored == value {
			return lineTypeLabels[index]
		}
	}
	return value
}

func validLineType(value string) bool {
	for _, stored := range lineTypeValues {
		if stored == value {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	for _, one := range values {
		if one == value {
			return true
		}
	}
	return false
}

func (m *Model) categoryNames(kind string) []string {
	names := make([]string, 0)
	for _, category := range m.categories {
		if category.TransactionType == kind {
			names = append(names, category.Name)
		}
	}
	return names
}

func decodeLines(raw string) ([]domain.ChartLine, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var lines []domain.ChartLine
	if err := json.Unmarshal([]byte(raw), &lines); err != nil {
		return nil, err
	}
	return lines, nil
}

func encodeLines(lines []domain.ChartLine) string {
	if len(lines) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(lines)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}
