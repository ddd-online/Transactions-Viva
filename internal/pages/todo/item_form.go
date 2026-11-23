package todo

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

type itemForm struct {
	open bool
	// cardID is the theme the item is saved into; cardTitle is the name the
	// form shows, which may be a theme that does not exist yet.
	cardID     string
	cardTitle  string
	themes     []string
	themesOpen bool
	id         string
	title      string
	start      string
	due        string
	urgency    int
	importance int
	startPick  datepicker.State
	duePick    datepicker.State
	err        string
}

type deleteCard struct {
	open  bool
	id    string
	title string
}

type deleteItem struct {
	open  bool
	id    string
	title string
}

// openItemForm starts a new item (cardID empty) or edits one. The theme choices
// are the cards the board has now; a name the user types is created on save.
func (m *Model) openItemForm(cardID string, item *domain.TodoItem) {
	form := itemForm{
		open: true, cardID: cardID,
		urgency: domain.DefaultTodoLevel, importance: domain.DefaultTodoLevel,
		themes: m.cardTitles(),
	}
	if item != nil {
		form.id = item.ID
		form.title = item.Title
		form.cardID = item.CardID
		form.cardTitle = m.cardTitle(item.CardID)
		form.start, form.due = item.StartDate, item.DueDate
		form.urgency, form.importance = item.Urgency, item.Importance
	}
	m.itemForm = form
}

func (m *Model) itemFormView(c ui.C) {
	if !m.itemForm.open {
		return
	}
	form := &m.itemForm
	title := "新建事项"
	if form.id != "" {
		title = "编辑事项"
	}
	ui.ModalWith(c, &form.open, title, ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Column(c).Gap(5).Children(func() {
				typography.Caption(c, "主题")
				ui.Row(c).FillWidth().Gap(6).Children(func() {
					ui.TextField(c, &form.cardTitle, "主题", "选择已有主题，或输入新的名字").
						Grow(1).AutoFocus()
					picker := ui.Button(c, "选择主题").Ghost().Icon(icons.ChevronDown).IconOnly().
						Tooltip("选择已有主题")
					if picker.Clicked() {
						form.themesOpen = true
					}
					ui.Menu(c, picker, &form.themesOpen, func(menu *ui.MenuItems) {
						if len(form.themes) == 0 {
							menu.Item("还没有主题").Disabled(true)
							return
						}
						for _, title := range form.themes {
							if menu.Item(title).Clicked() {
								form.cardTitle = title
							}
						}
					})
				})
			})
			ui.Column(c).Gap(5).Children(func() {
				typography.Caption(c, "事项")
				ui.TextField(c, &form.title, "事项", "例如：写周报")
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Gap(5).Children(func() {
					typography.Caption(c, "开始时间")
					datepicker.Picker(c, &form.startPick, datepicker.Spec{
						Label: "开始时间", Start: form.start, Grain: datepicker.ByDay,
						Placeholder: "不填", Fill: true, CloseOnPick: true, Viewport: m.bounds,
						OnRange: func(start, _ string) { form.start = start },
					})
				})
				ui.Column(c).Grow(1).Gap(5).Children(func() {
					typography.Caption(c, "截止时间")
					datepicker.Picker(c, &form.duePick, datepicker.Spec{
						Label: "截止时间", Start: form.due, Grain: datepicker.ByDay,
						Placeholder: "不填", Fill: true, CloseOnPick: true, Viewport: m.bounds,
						OnRange: func(start, _ string) { form.due = start },
					})
				})
			})
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Column(c).Grow(1).Gap(5).Children(func() {
					typography.Caption(c, "紧急度")
					selectbox.Select(c, &form.urgency, domain.TodoLevels[:], domain.TodoLevelLabel).
						Label("紧急度")
				})
				ui.Column(c).Grow(1).Gap(5).Children(func() {
					typography.Caption(c, "重要度")
					selectbox.Select(c, &form.importance, domain.TodoLevels[:], domain.TodoLevelLabel).
						Label("重要度")
				})
			})
			if form.err != "" {
				typography.Caption(c, form.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			form.open = false
		}
		if ui.Button(c, "保存").Clicked() {
			m.saveItem()
		}
	})
}

func (m *Model) saveItem() {
	form := &m.itemForm
	if m.todos == nil {
		return
	}
	title := strings.TrimSpace(form.title)
	if title == "" {
		form.err = "请输入事项"
		return
	}
	themeName := strings.TrimSpace(form.cardTitle)
	if themeName == "" {
		form.err = "请选择或输入主题"
		return
	}
	if form.start != "" && form.due != "" && form.start > form.due {
		form.err = "截止日期不能早于开始日期"
		return
	}
	// A theme the user typed but the board does not have yet is created here,
	// so one form both picks an existing theme and starts a new one.
	cardID := form.cardID
	if cardID == "" || m.cardTitle(cardID) != themeName {
		if card, ok := m.cardByTitle(themeName); ok {
			cardID = card.ID
		} else {
			card, err := m.todos.CreateTodoCard(m.context(), themeName)
			if err != nil {
				form.err = err.Error()
				return
			}
			cardID = card.ID
		}
	}
	item := domain.TodoItem{
		ID: form.id, CardID: cardID, Title: title,
		StartDate: form.start, DueDate: form.due,
		Urgency: form.urgency, Importance: form.importance,
	}
	var err error
	if form.id == "" {
		_, err = m.todos.CreateTodoItem(m.context(), item)
	} else {
		_, err = m.todos.UpdateTodoItem(m.context(), item)
	}
	if err != nil {
		form.err = err.Error()
		return
	}
	form.open = false
	m.reload()
}

func (m *Model) deleteCardView(c ui.C) {
	if !m.deleteCard.open {
		return
	}
	ui.ModalWith(c, &m.deleteCard.open, "删除主题？", ui.ModalSpec{Width: 420}, func() {
		typography.Body(c, fmt.Sprintf("「%s」下面的所有事项和进度都会一起删除。", m.deleteCard.title))
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteCard.open = false
		}
		if ui.Button(c, "删除主题").Destructive().Clicked() {
			if err := m.todos.DeleteTodoCard(m.context(), m.deleteCard.id); err != nil {
				m.fail(err)
				return
			}
			m.deleteCard.open = false
			m.reload()
		}
	})
}

func (m *Model) deleteItemView(c ui.C) {
	if !m.deleteItem.open {
		return
	}
	ui.ModalWith(c, &m.deleteItem.open, "删除事项？", ui.ModalSpec{Width: 420}, func() {
		typography.Body(c, fmt.Sprintf("「%s」的进度记录也会一起删除。", m.deleteItem.title))
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteItem.open = false
		}
		if ui.Button(c, "删除事项").Destructive().Clicked() {
			if err := m.todos.DeleteTodoItem(m.context(), m.deleteItem.id); err != nil {
				m.fail(err)
				return
			}
			m.deleteItem.open = false
			m.reload()
		}
	})
}

func (m *Model) setStatus(itemID, status string) {
	if m.todos == nil {
		return
	}
	if err := m.todos.SetTodoItemStatus(m.context(), itemID, status); err != nil {
		m.fail(err)
		return
	}
	m.reload()
	if status == domain.TodoStatusDone {
		m.succeed("已移到历史")
	}
}
