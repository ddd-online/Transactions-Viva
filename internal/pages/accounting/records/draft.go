package records

import (
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"

	"transactions/internal/domain"
	"transactions/internal/pages/accounting/records/composer"
	tpl "transactions/internal/pages/accounting/template"
)

// Key routes the dialog's canvas controls from the shell's Config.OnKey.
func (m *Model) Key(event gfx.KeyEvent) gfx.EventResult { return m.form.Key(event) }

// Scroll routes the dialog's popups' wheel from the shell's Config.OnScroll.
func (m *Model) Scroll(event gfx.ScrollEvent) gfx.EventResult { return m.form.Scroll(event) }

// TagsEditor is the native editor behind the tags multi-select while it has the
// focus, for Config.TextInput.
func (m *Model) TagsEditor(id gfx.ID) *text.Editor { return m.form.Editor(id) }

// openComposer starts a new record on today's date.
func (m *Model) openComposer() {
	m.form.Open(composer.Draft{
		Kind: "支出",
		Date: todayDate(),
	})
}

// openEdit fills the same dialog from an existing row.
func (m *Model) openEdit(id string) {
	rec := m.find(id)
	if rec == nil {
		return
	}
	m.form.Open(composer.Draft{
		EditingID:   rec.id,
		Kind:        kindLabel(rec.kind),
		Amount:      domain.CentsToYuan(rec.amount),
		Category:    rec.category,
		Description: rec.description,
		Tags:        append([]string(nil), rec.tags...),
		Outlier:     rec.outlier,
		EventDate:   rec.keyEventDate,
		Date:        rec.at.Format(dateLayout),
	})
}

// categoryNames is the 分类 Select's choices for a type, in the mock order, plus
// the draft's own category when it belongs to another type: the old form's
// autocomplete kept its text when the type changed, and the Select needs the
// value among its choices to keep showing it.
func (m *Model) categoryNames(kind, current string) []string {
	names := m.categoryList[:0]
	for _, category := range mockCategories {
		if category.kind == kindValue(kind) {
			names = append(names, category.name)
		}
	}
	if current != "" {
		known := false
		for _, name := range names {
			if name == current {
				known = true
				break
			}
		}
		if !known {
			names = append(names, current)
		}
	}
	m.categoryList = names
	return names
}

// tagOptions are the 标签 choices the dialog offers.
func (m *Model) tagOptions() []string { return mockTagOptions[:] }

// templateList are the 模板 choices the dialog offers.
func (m *Model) templateList() []tpl.Template { return m.templates.List() }

// saveDraft validates the dialog's draft and writes one mock record; the error
// it returns is what the dialog shows under its fields.
func (m *Model) saveDraft(draft composer.Draft) error {
	category := strings.TrimSpace(draft.Category)
	if category == "" {
		return errDraft("请填写分类")
	}
	amount, err := domain.YuanToCents(draft.Amount)
	if err != nil {
		return errDraft("金额格式不正确")
	}
	if amount <= 0 {
		return errDraft("金额需要大于 0")
	}
	day, err := time.ParseInLocation(dateLayout, strings.TrimSpace(draft.Date), time.Local)
	if err != nil {
		return errDraft("日期格式应为 YYYY-MM-DD")
	}
	eventDate := strings.TrimSpace(draft.EventDate)
	if eventDate != "" {
		if _, err := time.ParseInLocation(dateLayout, eventDate, time.Local); err != nil {
			return errDraft("关联事件日期格式应为 YYYY-MM-DD")
		}
	}
	at := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, time.Local)
	kind := kindValue(draft.Kind)
	tags := append([]string(nil), draft.Tags...)
	description := strings.TrimSpace(draft.Description)

	notice := "已记一笔"
	if draft.EditingID != "" {
		if rec := m.find(draft.EditingID); rec != nil {
			// Keep the clock time when the date did not change, so editing a
			// field does not reshuffle rows within the same day.
			if rec.at.Format(dateLayout) == day.Format(dateLayout) {
				at = time.Date(day.Year(), day.Month(), day.Day(), rec.at.Hour(), rec.at.Minute(), 0, 0, time.Local)
			}
			rec.kind, rec.category, rec.description = kind, category, description
			rec.tags, rec.amount, rec.at = tags, amount, at
			rec.outlier, rec.keyEventDate = draft.Outlier, eventDate
			notice = "记录已更新"
		}
	} else {
		m.records = append(m.records, record{
			id:           "mock-new-" + time.Now().Format("20060102150405.000000000"),
			ledgerID:     m.ledgerID,
			kind:         kind,
			category:     category,
			description:  description,
			tags:         tags,
			amount:       amount,
			at:           at,
			outlier:      draft.Outlier,
			keyEventDate: eventDate,
		})
	}
	m.refilter()
	m.notice = notice
	return nil
}

// saveTemplate stores the dialog's repeating fields as a template and returns it
// with the id the dialog selects.
func (m *Model) saveTemplate(one tpl.Template) tpl.Template { return m.templates.Add(one) }

// errDraft is a validation failure the dialog shows.
type errDraft string

func (err errDraft) Error() string { return string(err) }
