package accounting

import (
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
)

// kindLabels is the order of the type choice in the draft dialog; the record's
// stored kinds are the values in kindValue.
var kindLabels = []string{"支出", "收入", "转账"}

// recordDraft is the 记一笔 dialog's copy of the fields it edits. It is mock
// too: saving appends to the page's in-memory rows, not to SQLite.
type recordDraft struct {
	open        bool
	kind        string
	amount      string
	category    string
	description string
	tags        string
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

// composerView declares the 记一笔 dialog while it is open.
func (m *recordsModel) composerView(c ui.C) {
	ui.ModalWith(c, &m.draft.open, "记一笔", ui.ModalSpec{
		Width:       460,
		Description: "当前为 mock 数据：只在本次运行内新增，不写入数据库。",
	}, func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, "类型").FontSize(12).Muted()
			ui.Choice(c, &m.draft.kind, kindLabels, nil)
			ui.Text(c, "金额（元）").FontSize(12).Muted()
			ui.TextField(c, &m.draft.amount, "金额", "0.00")
			ui.Text(c, "分类").FontSize(12).Muted()
			ui.TextField(c, &m.draft.category, "分类", "例如：餐饮美食")
			ui.Text(c, "描述").FontSize(12).Muted()
			ui.TextField(c, &m.draft.description, "描述", "可选")
			ui.Text(c, "标签").FontSize(12).Muted()
			ui.TextField(c, &m.draft.tags, "标签", "用逗号分隔")
			ui.Text(c, "日期").FontSize(12).Muted()
			ui.TextField(c, &m.draft.date, "日期", "YYYY-MM-DD")
			if m.draft.err != "" {
				ui.Text(c, m.draft.err).FontSize(12).Color(c.Theme().Danger)
			}
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

// saveDraft validates the dialog's values and appends one mock record.
func (m *recordsModel) saveDraft() {
	amount, err := domain.YuanToCents(m.draft.amount)
	if err != nil || amount <= 0 {
		m.draft.err = "金额需要是大于 0 的数字"
		return
	}
	category := strings.TrimSpace(m.draft.category)
	if category == "" {
		m.draft.err = "请填写分类"
		return
	}
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(m.draft.date), time.Local)
	if err != nil {
		m.draft.err = "日期格式应为 YYYY-MM-DD"
		return
	}
	at := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.Local)

	m.records = append(m.records, record{
		id:          "mock-new-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		kind:        kindValue(m.draft.kind),
		category:    category,
		description: strings.TrimSpace(m.draft.description),
		tags:        splitTags(m.draft.tags),
		amount:      amount,
		at:          at,
	})
	m.refilter()
	m.draft.open = false
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
