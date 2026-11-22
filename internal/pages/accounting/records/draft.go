package records

import (
	"context"
	"fmt"
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

// categoryNames is the 分类 Select's choices for a type, plus the draft's own
// category when it belongs to another type: the form kept its text when the
// type changed, and the Select needs the value among its choices.
func (m *Model) categoryNames(kind, current string) []string {
	names := m.categoryList[:0]
	storedKind := kindValue(kind)
	for _, category := range m.labels.Categories {
		if category.TransactionType == storedKind {
			names = append(names, category.Name)
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

// tagOptions are the 标签 choices under the draft's category and type. The
// composer calls this again when either changes.
func (m *Model) tagOptions(kind, category string) []string {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil
	}
	key := category + ":" + kindValue(kind)
	values := m.tagList[:0]
	for _, tag := range m.labels.Tags {
		if tag.CategoryTransactionType == key {
			values = append(values, tag.Name)
		}
	}
	m.tagList = values
	return values
}

// templateList are the 模板 choices the dialog offers for the active ledger.
func (m *Model) templateList() []tpl.Template { return m.templates.List(m.ledgerID) }

// saveDraft validates the dialog's draft and writes it through the repository.
// The error it returns is what the dialog shows under its fields.
func (m *Model) saveDraft(draft composer.Draft) error {
	if m.repo == nil {
		return errDraft("工作空间尚未打开")
	}
	category := strings.TrimSpace(draft.Category)
	if category == "" {
		return errDraft("请选择分类")
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
	if draft.EditingID != "" {
		if rec := m.find(draft.EditingID); rec != nil && rec.at.Format(dateLayout) == day.Format(dateLayout) {
			// Keep the clock time when the date did not change, so editing a
			// field does not reshuffle rows within the same day.
			at = time.Date(day.Year(), day.Month(), day.Day(), rec.at.Hour(), rec.at.Minute(), 0, 0, time.Local)
		}
	}
	one := domain.TransactionRecord{
		TransactionID:   draft.EditingID,
		LedgerID:        m.ledgerID,
		AmountCents:     amount,
		TransactionType: kindValue(draft.Kind),
		Category:        category,
		Description:     strings.TrimSpace(draft.Description),
		Flags:           recordFlags(draft.Outlier),
		KeyEventDate:    eventDate,
		TransactionAt:   at.Unix(),
	}
	notice := "记录已保存"
	if draft.EditingID != "" {
		err = m.repo.UpdateRecord(context.Background(), one, draft.Tags)
		notice = "记录已更新"
	} else {
		_, err = m.repo.CreateRecord(context.Background(), one, draft.Tags)
	}
	if err != nil {
		return errDraft("保存失败：" + err.Error())
	}
	m.Reload()
	m.notice = notice
	return nil
}

// saveTemplate stores the dialog's repeating fields as a template in the active
// ledger and returns it with the id the dialog selects.
func (m *Model) saveTemplate(one tpl.Template) (tpl.Template, error) {
	if m.repo == nil {
		return one, fmt.Errorf("工作空间尚未打开")
	}
	one.LedgerID = m.ledgerID
	stored := tpl.ToDomain(one)
	id, err := m.repo.CreateTemplate(context.Background(), stored)
	if err != nil {
		return one, err
	}
	one.ID = id
	templates, err := m.repo.Templates(context.Background(), m.ledgerID)
	if err != nil {
		return one, err
	}
	list := make([]tpl.Template, 0, len(templates))
	for _, template := range templates {
		list = append(list, tpl.FromDomain(template))
	}
	m.templates.Set(m.ledgerID, list)
	return one, nil
}

// recordFlags is the stored JSON shape the application used for a record's
// 离群值 mark.
func recordFlags(outlier bool) string {
	if outlier {
		return `{"outlier":true}`
	}
	return `{"outlier":false}`
}

// errDraft is a validation failure the dialog shows.
type errDraft string

func (err errDraft) Error() string { return string(err) }
