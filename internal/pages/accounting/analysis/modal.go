package analysis

import (
	"context"
	"strings"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/selectbox"
	"transactions/internal/domain"
)

type modalKind uint8

const (
	modalNone modalKind = iota
	modalCreate
	modalRename
	modalLine
	modalDelete
)

type form struct {
	title         string
	chartType     string // label: 折线图 / 饼图
	granularity   string
	groupField    string // label: 交易类型 / 分类 / 标签
	groupKind     string
	groupCategory string

	lineLabel       string
	lineKind        string
	lineCategory    string
	lineTags        string
	linePolicy      string // label: 任意 / 全部
	lineDescription string
	lineOutlier     bool
	editingLine     int

	err string
}

func (m *Model) showModal(kind modalKind) {
	m.modal = kind
	m.dialogOpen = true
}

func (m *Model) closeModal() {
	m.dialogOpen = false
	m.modal = modalNone
}

func (m *Model) openCreate() {
	m.form = form{
		chartType:   "折线图",
		granularity: granularityYear,
		groupField:  "分类",
		groupKind:   "支出",
		lineKind:    "支出",
		linePolicy:  "任意",
		lineOutlier: true,
	}
	m.showModal(modalCreate)
}

func (m *Model) openRename(chart *domain.Chart) {
	m.form = form{title: chart.Title}
	m.showModal(modalRename)
}

func (m *Model) openLine(chart *domain.Chart, index int) {
	m.form = form{
		lineKind:    "支出",
		linePolicy:  "任意",
		lineOutlier: true,
		editingLine: index,
	}
	if index >= 0 && index < len(m.lines) {
		line := m.lines[index]
		m.form.lineLabel = line.Label
		m.form.lineKind = lineTypeLabel(line.TransactionType)
		m.form.lineOutlier = line.IncludeOutlier
		if len(line.Conditions) > 0 {
			condition := line.Conditions[0]
			m.form.lineCategory = condition.Category
			m.form.lineTags = strings.Join(condition.Tags, ", ")
			if condition.TagPolicy == "all" {
				m.form.linePolicy = "全部"
			}
			m.form.lineDescription = condition.Description
		}
	}
	m.showModal(modalLine)
}

func (m *Model) createChart() {
	if m.repo == nil {
		m.form.err = "工作空间尚未打开"
		return
	}
	title := strings.TrimSpace(m.form.title)
	if title == "" {
		m.form.err = "请输入图表名称"
		return
	}
	chart := domain.Chart{
		LedgerID: m.ledgerID, Title: title,
		ChartType: chartTypeValue(m.form.chartType),
	}
	if chart.ChartType == chartLine {
		chart.Granularity = m.form.granularity
		if chart.Granularity != granularityYear {
			chart.Granularity = granularityMonth
		}
		chart.ChartLines = encodeLines([]domain.ChartLine{{
			Label: "支出", TransactionType: "expense", IncludeOutlier: true, Conditions: []domain.QueryConditionItem{},
		}})
	} else {
		field := groupFieldValue(m.form.groupField)
		chart.GroupField = field
		if field == groupType {
			chart.GroupTransactionType = ""
		} else {
			chart.GroupTransactionType = m.groupKindValue()
		}
		if field == groupTag {
			if strings.TrimSpace(m.form.groupCategory) == "" {
				m.form.err = "按标签分组时请选择分类"
				return
			}
			chart.GroupCategory = m.form.groupCategory
		}
		chart.ChartLines = "[]"
	}
	id, err := m.repo.CreateChart(context.Background(), chart)
	if err != nil {
		m.form.err = "创建图表失败：" + err.Error()
		return
	}
	m.closeModal()
	m.selectedID = id
	m.Reload()
	m.toaster.Show("图表已创建", "", ui.ToastSuccess)
}

func (m *Model) renameChart() {
	chart := m.selectedChart()
	if chart == nil || m.repo == nil {
		return
	}
	title := strings.TrimSpace(m.form.title)
	if title == "" {
		m.form.err = "请输入图表名称"
		return
	}
	updated := *chart
	updated.Title = title
	if err := m.repo.UpdateChart(context.Background(), updated); err != nil {
		m.form.err = "重命名失败：" + err.Error()
		return
	}
	m.closeModal()
	m.Reload()
	m.toaster.Show("图表已重命名", "", ui.ToastSuccess)
}

func (m *Model) saveLine() {
	chart := m.selectedChart()
	if chart == nil || m.repo == nil {
		return
	}
	label := strings.TrimSpace(m.form.lineLabel)
	if label == "" {
		m.form.err = "请输入曲线名称"
		return
	}
	for index, line := range m.lines {
		if index != m.form.editingLine && line.Label == label {
			m.form.err = "已经有名为「" + label + "」的曲线"
			return
		}
	}
	tags := splitTags(m.form.lineTags)
	condition := domain.QueryConditionItem{
		TransactionType: m.lineKindValue(),
		Category:        strings.TrimSpace(m.form.lineCategory),
		Tags:            tags,
		TagPolicy:       "any",
		Description:     strings.TrimSpace(m.form.lineDescription),
	}
	if m.form.linePolicy == "全部" {
		condition.TagPolicy = "all"
	}
	line := domain.ChartLine{
		Label: label, TransactionType: m.lineKindValue(),
		IncludeOutlier: m.form.lineOutlier,
	}
	if condition.Category != "" || len(condition.Tags) > 0 || condition.Description != "" {
		line.Conditions = []domain.QueryConditionItem{condition}
	} else {
		line.Conditions = []domain.QueryConditionItem{}
	}
	lines := append([]domain.ChartLine(nil), m.lines...)
	if m.form.editingLine >= 0 && m.form.editingLine < len(lines) {
		lines[m.form.editingLine] = line
	} else {
		lines = append(lines, line)
	}
	updated := *chart
	updated.ChartLines = encodeLines(lines)
	if err := m.repo.UpdateChart(context.Background(), updated); err != nil {
		m.form.err = "保存曲线失败：" + err.Error()
		return
	}
	m.closeModal()
	m.Reload()
	m.toaster.Show("曲线已保存", "", ui.ToastSuccess)
}

func (m *Model) removeLine(chart *domain.Chart, index int) {
	if m.repo == nil || index < 0 || index >= len(m.lines) {
		return
	}
	lines := append([]domain.ChartLine(nil), m.lines[:index]...)
	lines = append(lines, m.lines[index+1:]...)
	updated := *chart
	updated.ChartLines = encodeLines(lines)
	if err := m.repo.UpdateChart(context.Background(), updated); err != nil {
		m.toaster.Show("删除曲线失败："+err.Error(), "", ui.ToastError)
		return
	}
	m.Reload()
	m.toaster.Show("曲线已删除", "", ui.ToastSuccess)
}

func (m *Model) changeGranularity(chart *domain.Chart, value string) {
	if m.repo == nil {
		return
	}
	updated := *chart
	updated.Granularity = value
	if err := m.repo.UpdateChart(context.Background(), updated); err != nil {
		m.toaster.Show("保存粒度失败："+err.Error(), "", ui.ToastError)
		return
	}
	m.Reload()
}

func (m *Model) changeGrouping(chart *domain.Chart, field, kind, category string) {
	if m.repo == nil {
		return
	}
	updated := *chart
	updated.GroupField = field
	if field == groupType {
		updated.GroupTransactionType, updated.GroupCategory = "", ""
	} else {
		updated.GroupTransactionType = kind
		if field == groupTag {
			category = m.normalizedCategory(kind, category)
			updated.GroupCategory = category
		} else {
			updated.GroupCategory = ""
		}
	}
	if err := m.repo.UpdateChart(context.Background(), updated); err != nil {
		m.toaster.Show("保存分组失败："+err.Error(), "", ui.ToastError)
		return
	}
	m.Reload()
}

func (m *Model) normalizedCategory(kind, category string) string {
	names := m.categoryNames(kind)
	if contains(names, category) {
		return category
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func (m *Model) lineKindValue() string {
	for index, label := range lineTypeLabels {
		if label == m.form.lineKind {
			return lineTypeValues[index]
		}
	}
	return "expense"
}

func chartTypeValue(label string) string {
	if label == "饼图" {
		return chartPie
	}
	return chartLine
}

func groupFieldValue(label string) string {
	for index, value := range groupLabels {
		if value == label {
			return groupValues[index]
		}
	}
	return groupType
}

func splitTags(raw string) []string {
	raw = strings.ReplaceAll(raw, "，", ",")
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, tag := range parts {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// modals paints the create, rename, curve and delete dialogs.
func (m *Model) modals(c ui.C) {
	if !m.dialogOpen {
		m.modal = modalNone
		return
	}
	if m.modal == modalDelete {
		chart := m.deleting
		ui.ModalWith(c, &m.dialogOpen, "删除图表？", ui.ModalSpec{Width: 380}, func() {
			ui.Text(c, "「"+chart.Title+"」会从当前账本移除。").FontSize(13)
		}, func() {
			if ui.Button(c, "取消").Outline().Clicked() {
				m.closeModal()
			}
			if ui.Button(c, "删除").Destructive().Clicked() {
				m.deleteChart()
			}
		})
		return
	}
	if m.modal == modalNone {
		return
	}
	title := "新增图表"
	switch m.modal {
	case modalRename:
		title = "重命名图表"
	case modalLine:
		if m.form.editingLine < 0 {
			title = "添加曲线"
		} else {
			title = "编辑曲线"
		}
	}
	ui.ModalWith(c, &m.dialogOpen, title, ui.ModalSpec{Width: 540}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			switch m.modal {
			case modalCreate:
				m.createFields(c)
			case modalRename:
				m.primaryField(c, "图表名称", &m.form.title, "请输入图表名称")
			case modalLine:
				m.lineFields(c)
			}
			if m.form.err != "" {
				ui.Text(c, m.form.err).FontSize(12).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Secondary().Clicked() {
			m.closeModal()
		}
		label := "保存"
		if m.modal == modalCreate {
			label = "创建"
		}
		if ui.Button(c, label).Clicked() {
			switch m.modal {
			case modalCreate:
				m.createChart()
			case modalRename:
				m.renameChart()
			case modalLine:
				m.saveLine()
			}
		}
	})
}

func (m *Model) createFields(c ui.C) {
	m.primaryField(c, "图表名称", &m.form.title, "例如：年度支出")
	ui.Text(c, "图表类型").FontSize(c.Theme().Metrics.FontSizeS).Muted()
	ui.Choice(c, &m.form.chartType, chartTypeLabels[:], nil)
	if m.form.chartType == "饼图" {
		ui.Text(c, "分组方式").FontSize(c.Theme().Metrics.FontSizeS).Muted()
		ui.Choice(c, &m.form.groupField, groupLabels[:], nil)
		if m.form.groupField != "交易类型" {
			ui.Text(c, "交易类型").FontSize(c.Theme().Metrics.FontSizeS).Muted()
			if selectbox.Select(c, &m.form.groupKind, lineTypeLabels[:], nil).Label("交易类型").FillWidth().Changed() {
				m.form.groupCategory = ""
			}
		}
		if m.form.groupField == "标签" {
			ui.Text(c, "分类").FontSize(c.Theme().Metrics.FontSizeS).Muted()
			names := m.categoryNames(m.groupKindValue())
			selectbox.Select(c, &m.form.groupCategory, names, nil).Placeholder("选择分类").Label("分类").FillWidth()
		}
		return
	}
	ui.Text(c, "曲线粒度").FontSize(c.Theme().Metrics.FontSizeS).Muted()
	selectbox.Select(c, &m.form.granularity, granularityValues[:], func(value string) string {
		if value == granularityYear {
			return "年"
		}
		return "月"
	}).Label("曲线粒度").FillWidth()
}

func (m *Model) lineFields(c ui.C) {
	m.primaryField(c, "曲线名称", &m.form.lineLabel, "例如：餐饮支出")
	ui.Text(c, "交易类型").FontSize(c.Theme().Metrics.FontSizeS).Muted()
	if selectbox.Select(c, &m.form.lineKind, lineTypeLabels[:], nil).Label("交易类型").FillWidth().Changed() {
		m.form.lineCategory = ""
		m.form.lineTags = ""
	}
	ui.Text(c, "分类（可选）").FontSize(c.Theme().Metrics.FontSizeS).Muted()
	names := m.categoryNames(m.lineKindValueForForm())
	selectbox.Select(c, &m.form.lineCategory, names, nil).Placeholder("不限分类").Label("分类（可选）").FillWidth()
	m.field(c, "标签（可选，逗号分隔）", &m.form.lineTags, "三餐, 外卖")
	ui.Text(c, "标签匹配").FontSize(c.Theme().Metrics.FontSizeS).Muted()
	ui.Choice(c, &m.form.linePolicy, tagPolicyLabels[:], nil)
	m.field(c, "描述包含（可选）", &m.form.lineDescription, "输入关键词")
	ui.Checkbox(c, &m.form.lineOutlier, "包含离群值")
}

func (m *Model) lineKindValueForForm() string {
	for index, label := range lineTypeLabels {
		if label == m.form.lineKind {
			return lineTypeValues[index]
		}
	}
	return "expense"
}

func (m *Model) groupKindValue() string {
	for index, label := range lineTypeLabels {
		if label == m.form.groupKind {
			return lineTypeValues[index]
		}
	}
	return "expense"
}

func (m *Model) field(c ui.C, label string, value *string, placeholder string) {
	ui.Text(c, label).FontSize(c.Theme().Metrics.FontSizeS).Muted()
	ui.TextField(c, value, label, placeholder).FillWidth()
}

// primaryField is the first field of a dialog: it takes the keyboard as the
// dialog appears, so the user can type the name without clicking the field.
func (m *Model) primaryField(c ui.C, label string, value *string, placeholder string) {
	ui.Text(c, label).FontSize(c.Theme().Metrics.FontSizeS).Muted()
	ui.TextField(c, value, label, placeholder).FillWidth().AutoFocus()
}
