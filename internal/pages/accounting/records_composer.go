package accounting

import (
	"strconv"
	"strings"
	"time"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
	"transactions/internal/icons"
)

// kindLabels is the order of the type choice in the draft dialog; the record's
// stored kinds are the values in kindValue.
var kindLabels = []string{"收入", "支出", "转账"}

// markLabels are the composer's 标记 choices. 无标记 clears the outlier flag;
// an empty value shows the Select's placeholder.
var markLabels = []string{"无标记", "离群值"}

// recordFields are the FormField scratch values of the composer, one for each
// field; ui.FormField keeps its layout here between paints.
type recordFields struct {
	template    ui.FormFieldState
	date        ui.FormFieldState
	kind        ui.FormFieldState
	category    ui.FormFieldState
	tags        ui.FormFieldState
	mark        ui.FormFieldState
	eventDate   ui.FormFieldState
	description ui.FormFieldState
	amount      ui.FormFieldState
}

// recordTags is the Viva multi-select behind the 标签 field, with the stable
// identities its painted hit regions need.
type recordTags struct {
	tree    gfx.Tree
	feature gfx.Scope
	options gfx.Scope
	removes gfx.Scope
	input   gfx.ID
	trigger gfx.ID
	dismiss gfx.ID
	scope   gfx.ID
	state   viva.MultiSelectState
}

// newRecordTags builds the tags multi-select and its option list.
func newRecordTags() recordTags {
	var tags recordTags
	tags.feature = tags.tree.Feature("record-tags")
	tags.options = tags.tree.Sub(tags.feature, tags.tree.Name("option"))
	tags.removes = tags.tree.Sub(tags.feature, tags.tree.Name("remove"))
	tags.input = tags.tree.ID(tags.feature, "input")
	tags.trigger = tags.tree.ID(tags.feature, "trigger")
	tags.dismiss = tags.tree.ID(tags.feature, "dismiss")
	tags.scope = tags.tree.ID(tags.feature, "scope")
	tags.state.AllowCreate = true
	tags.state.MatchCaseInsensitive = true
	options := make([]viva.MultiSelectOption, len(mockTagOptions))
	for index, tag := range mockTagOptions {
		options[index] = viva.MultiSelectOption{Value: tag, Label: tag}
	}
	tags.state.SetOptions(options)
	return tags
}

// recordDraft is the 新增记录 / 编辑记录 dialog's copy of the fields it edits.
// It is mock too: saving writes to the page's in-memory rows, not to SQLite.
type recordDraft struct {
	open        bool
	editingID   string
	templateID  string
	templateMsg string
	kind        string
	amount      string
	category    string
	description string
	tags        string
	mark        string
	eventDate   string
	date        string
	dateOpen    bool
	eventOpen   bool
	dateMonth   time.Time
	eventMonth  time.Time
	err         string
}

// openComposer starts a new record on today's date.
func (m *recordsModel) openComposer() {
	m.draft = recordDraft{
		open: true,
		kind: "支出",
		date: time.Now().Format("2006-01-02"),
	}
	m.setTags(nil)
}

// openEdit fills the same dialog from an existing row.
func (m *recordsModel) openEdit(id string) {
	rec := m.find(id)
	if rec == nil {
		return
	}
	mark := ""
	if rec.outlier {
		mark = "离群值"
	}
	m.draft = recordDraft{
		open:        true,
		editingID:   rec.id,
		kind:        kindLabel(rec.kind),
		amount:      domain.CentsToYuan(rec.amount),
		category:    rec.category,
		description: rec.description,
		mark:        mark,
		eventDate:   rec.keyEventDate,
		date:        rec.at.Format("2006-01-02"),
	}
	m.setTags(rec.tags)
}

// composerView declares the create/edit dialog while it is open.
func (m *recordsModel) composerView(c ui.C) {
	title := "新增记录"
	if m.draft.editingID != "" {
		title = "编辑记录"
	}
	ui.ModalWith(c, &m.draft.open, title, ui.ModalSpec{
		Width: 640,
	}, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			m.wideFormField(c, &m.fields.template, "模板", func() {
				ui.Row(c).Gap(8).Children(func() {
					if ui.Select(c, &m.draft.templateID, m.templateIDs(), m.templateName).
						Placeholder("选择模板").Grow(1).Changed() {
						m.applyTemplate()
					}
					if ui.Button(c, "保存为模板").Secondary().Clicked() {
						m.saveTemplate()
					}
				})
			})
			if m.draft.templateMsg != "" {
				ui.Text(c, m.draft.templateMsg).FontSize(11).Color(c.Theme().Success)
			}

			ui.Row(c).Gap(12).Children(func() {
				m.formField(c, &m.fields.date, "日期", func() {
					if ui.TextField(c, &m.draft.date, "日期", "YYYY-MM-DD").
						Action(icons.Calendar, "打开日期选择").ActionClicked() {
						m.openDatePicker()
					}
				})
				m.formField(c, &m.fields.kind, "类型", func() {
					ui.Choice(c, &m.draft.kind, kindLabels, nil).Grow(1)
				})
			})

			if m.draft.dateOpen {
				m.calendar(c, &m.draft.dateMonth, &m.draft.date, &m.draft.dateOpen)
			}

			ui.Row(c).Gap(12).Children(func() {
				m.formField(c, &m.fields.category, "分类", func() {
					ui.TextField(c, &m.draft.category, "分类", "选择或输入分类")
				})
				m.formField(c, &m.fields.tags, "标签", func() {
					m.multiSelect(c)
				})
			})

			ui.Row(c).Gap(12).Children(func() {
				m.formField(c, &m.fields.mark, "标记", func() {
					ui.Select(c, &m.draft.mark, markLabels, nil).Placeholder("选择标记")
				})
				m.formField(c, &m.fields.eventDate, "关联事件日期", func() {
					if ui.TextField(c, &m.draft.eventDate, "关联事件日期", "请选择日期").
						Action(icons.Calendar, "打开事件日期选择").ActionClicked() {
						m.openEventPicker()
					}
				})
			})

			if m.draft.eventOpen {
				m.calendar(c, &m.draft.eventMonth, &m.draft.eventDate, &m.draft.eventOpen)
			}

			m.wideFormField(c, &m.fields.description, "描述", func() {
				ui.TextField(c, &m.draft.description, "描述", "描述消费内容")
			})
			m.wideFormField(c, &m.fields.amount, "金额", func() {
				ui.TextField(c, &m.draft.amount, "金额", "0.00")
			})

			if m.draft.err != "" {
				ui.Text(c, m.draft.err).FontSize(12).Color(c.Theme().Danger)
			}
			ui.Divider(c)
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.draft.open = false
		}
		if ui.Button(c, "保存").Clicked() {
			m.saveDraft()
		}
	})
}

// formFieldHeight is the compact form row: the FormField title and its 32 px
// control. FormField reserves description and error rows by default, which this
// dialog does not use, so the fixed row keeps the old modal's density.
const formFieldHeight float32 = 64

// formField declares one half of a two-column form row with Viva's FormField,
// which owns the title and the control rhythm. Basis(0) makes both halves share
// the row equally instead of letting the wider control take more than its share.
func (m *recordsModel) formField(c ui.C, state *ui.FormFieldState, label string, body func()) {
	ui.FormField(c, state, label).Grow(1).Basis(0).Height(formFieldHeight).Children(body)
}

// wideFormField is a FormField that takes the dialog's whole width.
func (m *recordsModel) wideFormField(c ui.C, state *ui.FormFieldState, label string, body func()) {
	ui.FormField(c, state, label).FillWidth().Height(formFieldHeight).Children(body)
}

// openDatePicker prepares the calendar for the record date.
func (m *recordsModel) openDatePicker() {
	if m.draft.dateOpen {
		m.draft.dateOpen = false
		return
	}
	m.draft.dateOpen, m.draft.eventOpen = true, false
	m.draft.dateMonth = monthOf(m.draft.date)
}

// openEventPicker prepares the calendar for the linked event date.
func (m *recordsModel) openEventPicker() {
	if m.draft.eventOpen {
		m.draft.eventOpen = false
		return
	}
	m.draft.eventOpen, m.draft.dateOpen = true, false
	m.draft.eventMonth = monthOf(m.draft.eventDate)
}

// monthOf returns the first day of value's month, or of today when value is
// empty or malformed.
func monthOf(value string) time.Time {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
	if err != nil {
		day = time.Now()
	}
	return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)
}

// calendar paints one month inside a popover and applies a day (or a month
// step) back to the form. It is built from ordinary page buttons rather than a
// slot-painted component, so its rows stay clickable inside the popover.
func (m *recordsModel) calendar(c ui.C, month *time.Time, value *string, open *bool) {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
	if month.IsZero() {
		first = monthOf(*value)
		*month = first
	}
	offset := (int(first.Weekday()) + 6) % 7 // Monday is the first column
	start := first.AddDate(0, 0, -offset)
	theme := c.Theme()
	today := time.Now().Format("2006-01-02")

	ui.Column(c).Width(288).Pad(8).Gap(4).
		Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface).
		Background(theme.Popover).Shadow(gfx.ShadowSM).Children(func() {
		ui.Row(c).Height(28).Gap(4).Children(func() {
			if ui.Button(c, "‹").Ghost().Width(28).Clicked() {
				*month = first.AddDate(0, -1, 0)
			}
			ui.Text(c, first.Format("2006 年 1 月")).Grow(1).TextCenter().
				FontSize(13).Weight(gfx.WeightMedium)
			if ui.Button(c, "›").Ghost().Width(28).Clicked() {
				*month = first.AddDate(0, 1, 0)
			}
		})
		ui.Row(c).Gap(0).Children(func() {
			for _, weekday := range [7]string{"一", "二", "三", "四", "五", "六", "日"} {
				ui.Text(c, weekday).Width(34).TextCenter().FontSize(11).Muted()
			}
		})
		for week := 0; week < 6; week++ {
			ui.Row(c.KeyInt(week)).Gap(0).Children(func() {
				for column := 0; column < 7; column++ {
					day := start.AddDate(0, 0, week*7+column)
					date := day.Format("2006-01-02")
					selected := date == *value
					c.Key(date)
					dayButton := ui.Button(c, strconv.Itoa(day.Day())).Ghost().
						Width(34).Height(28)
					if day.Month() != first.Month() {
						dayButton.Color(theme.MutedForeground)
					}
					if date == today {
						dayButton.Border(1, theme.Primary)
					}
					if selected {
						dayButton.Background(theme.Selection).Color(theme.SelectionForeground)
					}
					if dayButton.Clicked() {
						*value = date
						*open = false
					}
				}
			})
		}
	})
}

// multiSelect paints Viva's multi-select in the 标签 field and routes its
// pointer choices back into the state and the draft's tag string.
func (m *recordsModel) multiSelect(c ui.C) {
	ui.Slot(c).FillWidth().Height(c.Theme().Metrics.ControlHeight).Draw(func(d *ui.DrawContext) {
		viva.PaintMultiSelect(d.Canvas, *d.Theme, d.Rect, &m.tags.state, viva.MultiSelectOptions{
			InputID: m.tags.input, TriggerID: m.tags.trigger, DismissID: m.tags.dismiss, Scope: m.tags.scope,
			OptionIDs: m.tags.options, RemoveIDs: m.tags.removes,
			Label: "标签", Placeholder: "选择一个或多个标签",
			EmptyLabel: "没有匹配项", CreatePrefix: "创建：",
			Viewport: m.bounds, Layer: 20, MaxRows: 7,
		})
		switch {
		case d.Canvas.Clicked(m.tags.input):
			m.tags.state.Open = true
		case d.Canvas.Clicked(m.tags.trigger):
			m.tags.state.Open = !m.tags.state.Open
		case d.Canvas.Clicked(m.tags.dismiss):
			m.tags.state.Open = false
		default:
			if index, ok := d.Canvas.ClickedIn(m.tags.options); ok {
				m.tags.state.Choose(index)
			}
			if index, ok := d.Canvas.ClickedIn(m.tags.removes); ok {
				m.tags.state.Remove(index)
			}
		}
		m.syncTags()
	})
}

// syncTags copies the multi-select's values into the draft's tag field.
func (m *recordsModel) syncTags() {
	m.draft.tags = strings.Join(m.tags.state.Values(), ", ")
}

// setTags replaces the multi-select's values when a dialog opens or a template
// is applied.
func (m *recordsModel) setTags(values []string) {
	m.tags.state.Query = text.Editor{}
	m.tags.state.Open = false
	m.tags.state.SetValues(values)
	m.syncTags()
}

// tagsEditor is the native editor of the tags query while the field is
// focused, for Config.TextInput.
func (m *recordsModel) tagsEditor(id gfx.ID) *text.Editor {
	if !m.draft.open || id != m.tags.input {
		return nil
	}
	return &m.tags.state.Query
}

// Key routes the tags popup's navigation keys and returns the words typed into
// it; Config.OnKey reaches this through the accounting page.
func (m *recordsModel) Key(event gfx.KeyEvent) gfx.EventResult {
	if !m.draft.open {
		return gfx.EventResult{}
	}
	if event.Target != m.tags.input &&
		(!m.tags.state.Open || event.Key != gfx.KeyEscape && event.Key != gfx.KeyTab) {
		return gfx.EventResult{}
	}
	result := m.tags.state.Key(event)
	m.syncTags()
	return result
}

// Scroll moves the tags popup while it is open.
func (m *recordsModel) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	if !m.draft.open || !m.tags.state.Open || !m.tags.state.Wheel(event) {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// templateIDs are the choices of the 模板 Select, in the form's model order.
func (m *recordsModel) templateIDs() []string {
	ids := make([]string, len(m.templates))
	for index := range m.templates {
		ids[index] = m.templates[index].id
	}
	return ids
}

// templateName is the label of one 模板 option; an unknown id shows itself.
func (m *recordsModel) templateName(id string) string {
	for index := range m.templates {
		if m.templates[index].id == id {
			return m.templates[index].name
		}
	}
	return id
}

// applyTemplate copies the type, category, description and tags of the chosen
// template into the draft; the date, amount and marks stay as the user set them.
func (m *recordsModel) applyTemplate() {
	for index := range m.templates {
		template := &m.templates[index]
		if template.id != m.draft.templateID {
			continue
		}
		m.draft.kind = kindLabel(template.kind)
		m.draft.category = template.category
		m.draft.description = template.description
		m.setTags(template.tags)
		return
	}
}

// saveTemplate adds the current draft's repeating fields as a template. The
// real app persists it; here it only lives in this page's model.
func (m *recordsModel) saveTemplate() {
	category := strings.TrimSpace(m.draft.category)
	name := kindLabel(kindValue(m.draft.kind)) + " · " + category
	if category == "" {
		name = kindLabel(kindValue(m.draft.kind)) + " · 模板"
	}
	template := recordTemplate{
		id:          "tpl-new-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		name:        name,
		kind:        kindValue(m.draft.kind),
		category:    category,
		description: strings.TrimSpace(m.draft.description),
		tags:        append([]string(nil), m.tags.state.Values()...),
	}
	m.templates = append(m.templates, template)
	m.draft.templateID = template.id
	m.draft.templateMsg = "模板已保存"
}

// saveDraft validates the dialog's values and writes one mock record.
func (m *recordsModel) saveDraft() {
	category := strings.TrimSpace(m.draft.category)
	if category == "" {
		m.draft.err = "请填写分类"
		return
	}
	amount, err := domain.YuanToCents(m.draft.amount)
	if err != nil {
		m.draft.err = "金额格式不正确"
		return
	}
	if amount <= 0 {
		m.draft.err = "金额需要大于 0"
		return
	}
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(m.draft.date), time.Local)
	if err != nil {
		m.draft.err = "日期格式应为 YYYY-MM-DD"
		return
	}
	eventDate := strings.TrimSpace(m.draft.eventDate)
	if eventDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", eventDate, time.Local); err != nil {
			m.draft.err = "关联事件日期格式应为 YYYY-MM-DD"
			return
		}
	}
	at := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.Local)
	kind := kindValue(m.draft.kind)
	tags := append([]string(nil), m.tags.state.Values()...)
	description := strings.TrimSpace(m.draft.description)

	notice := "已记一笔"
	if m.draft.editingID != "" {
		if rec := m.find(m.draft.editingID); rec != nil {
			// Keep the clock time when the date did not change, so editing a
			// field does not reshuffle rows within the same day.
			if rec.at.Format("2006-01-02") == day.Format("2006-01-02") {
				at = time.Date(day.Year(), day.Month(), day.Day(), rec.at.Hour(), rec.at.Minute(), 0, 0, time.Local)
			}
			rec.kind, rec.category, rec.description = kind, category, description
			rec.tags, rec.amount, rec.at = tags, amount, at
			rec.outlier, rec.keyEventDate = m.draft.mark == "离群值", eventDate
			notice = "记录已更新"
		}
	} else {
		m.records = append(m.records, record{
			id:           "mock-new-" + strconv.FormatInt(time.Now().UnixNano(), 10),
			kind:         kind,
			category:     category,
			description:  description,
			tags:         tags,
			amount:       amount,
			at:           at,
			outlier:      m.draft.mark == "离群值",
			keyEventDate: eventDate,
		})
	}
	m.refilter()
	m.notice = notice
	m.draft.open = false
}

// find returns the mock row with id, or nil.
func (m *recordsModel) find(id string) *record {
	for index := range m.records {
		if m.records[index].id == id {
			return &m.records[index]
		}
	}
	return nil
}

// deleteView declares the delete confirmation while one is pending.
func (m *recordsModel) deleteView(c ui.C) {
	if m.deletingID == "" {
		return
	}
	ui.ModalWith(c, &m.deleteConfirm, "删除这条消费记录？", ui.ModalSpec{
		Width:       380,
		Description: "删除后不会保留；当前为 mock 数据。",
	}, func() {
		ui.Text(c, "这条记录会从当前列表移除。").FontSize(13).Muted()
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteConfirm = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			m.deleteRecord(m.deletingID)
		}
	})
}

// deleteRecord removes one mock row and reports it in the footer notice.
func (m *recordsModel) deleteRecord(id string) {
	for index := range m.records {
		if m.records[index].id != id {
			continue
		}
		m.records = append(m.records[:index], m.records[index+1:]...)
		break
	}
	m.deletingID, m.deleteConfirm = "", false
	m.refilter()
	m.notice = "记录已删除"
}

// syncTo reports copying the row to another ledger. The mock has one table and
// no ledger switcher, so the target is reported instead of duplicated visibly.
func (m *recordsModel) syncTo(id, target string) {
	rec := m.find(id)
	if rec == nil {
		return
	}
	description := rec.description
	if description == "" {
		description = rec.category
	}
	m.notice = "「" + description + "」已同步到「" + target + "」（mock）"
}
