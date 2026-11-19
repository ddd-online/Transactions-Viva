package accounting

import (
	"strconv"
	"strings"
	"time"

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
	err         string
}

// openComposer starts a new record on today's date.
func (m *recordsModel) openComposer() {
	m.draft = recordDraft{
		open: true,
		kind: "支出",
		date: time.Now().Format("2006-01-02"),
	}
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
		tags:        strings.Join(rec.tags, ", "),
		mark:        mark,
		eventDate:   rec.keyEventDate,
		date:        rec.at.Format("2006-01-02"),
	}
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
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "模板").FontSize(12).Muted()
				ui.Row(c).Gap(8).Children(func() {
					if ui.Select(c, &m.draft.templateID, m.templateIDs(), m.templateName).
						Placeholder("选择模板").Grow(1).Changed() {
						m.applyTemplate()
					}
					if ui.Button(c, "保存为模板").Secondary().Clicked() {
						m.saveTemplate()
					}
				})
				if m.draft.templateMsg != "" {
					ui.Text(c, m.draft.templateMsg).FontSize(11).Color(c.Theme().Success)
				}
			})

			ui.Row(c).Gap(12).Children(func() {
				m.field(c, "日期", func() {
					ui.TextField(c, &m.draft.date, "日期", "YYYY-MM-DD").Icon(icons.Calendar)
				})
				m.field(c, "类型", func() {
					ui.Choice(c, &m.draft.kind, kindLabels, nil).Grow(1)
				})
			})

			ui.Row(c).Gap(12).Children(func() {
				m.field(c, "分类", func() {
					ui.TextField(c, &m.draft.category, "分类", "选择或输入分类")
				})
				m.field(c, "标签", func() {
					ui.TextField(c, &m.draft.tags, "标签", "选择一个或多个标签")
				})
			})

			ui.Row(c).Gap(12).Children(func() {
				m.field(c, "标记", func() {
					ui.Select(c, &m.draft.mark, markLabels, nil).Placeholder("选择标记")
				})
				m.field(c, "关联事件日期", func() {
					ui.TextField(c, &m.draft.eventDate, "关联事件日期", "请选择日期").Icon(icons.Calendar)
				})
			})

			m.wideField(c, "描述", func() {
				ui.TextField(c, &m.draft.description, "描述", "描述消费内容")
			})
			m.wideField(c, "金额", func() {
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

// field declares one half of a two-column form row.
func (m *recordsModel) field(c ui.C, label string, body func()) {
	ui.Column(c).Grow(1).Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).Muted()
		body()
	})
}

// wideField declares a field that takes the dialog's whole width.
func (m *recordsModel) wideField(c ui.C, label string, body func()) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).Muted()
		body()
	})
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
		m.draft.tags = strings.Join(template.tags, ", ")
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
		tags:        splitTags(m.draft.tags),
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
	tags := splitTags(m.draft.tags)
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

// splitTags reads the comma-separated tag field, accepting the Chinese comma
// and the enumeration comma as well.
func splitTags(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、'
	})
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if tag := strings.TrimSpace(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
