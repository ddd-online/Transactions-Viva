package todo

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
)

type cardForm struct {
	open  bool
	title string
	err   string
}

type itemForm struct {
	open       bool
	cardID     string
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

func (m *Model) openCardForm() {
	m.cardForm = cardForm{open: true}
}

func (m *Model) openItemForm(cardID string, item *domain.TodoItem) {
	if item == nil {
		m.itemForm = itemForm{
			open: true, cardID: cardID, urgency: domain.DefaultTodoLevel, importance: domain.DefaultTodoLevel,
		}
		return
	}
	m.itemForm = itemForm{
		open: true, cardID: item.CardID, id: item.ID, title: item.Title,
		start: item.StartDate, due: item.DueDate,
		urgency: item.Urgency, importance: item.Importance,
	}
}

func (m *Model) cardFormView(c ui.C) {
	if !m.cardForm.open {
		return
	}
	ui.ModalWith(c, &m.cardForm.open, "新建主题", ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			typography.Caption(c, "主题")
			ui.TextField(c, &m.cardForm.title, "主题", "例如：本周").AutoFocus()
			if m.cardForm.err != "" {
				typography.Caption(c, m.cardForm.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.cardForm.open = false
		}
		if ui.Button(c, "创建").Clicked() {
			m.saveCard()
		}
	})
}

func (m *Model) saveCard() {
	if strings.TrimSpace(m.cardForm.title) == "" {
		m.cardForm.err = "请输入卡片主题"
		return
	}
	if m.todos == nil {
		return
	}
	if _, err := m.todos.CreateTodoCard(m.context(), m.cardForm.title); err != nil {
		m.cardForm.err = err.Error()
		return
	}
	m.cardForm = cardForm{}
	m.reload()
}

func (m *Model) itemFormView(c ui.C) {
	if !m.itemForm.open {
		return
	}
	form := &m.itemForm
	title := "添加事项"
	if form.id != "" {
		title = "编辑事项"
	}
	ui.ModalWith(c, &form.open, title, ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(12).Children(func() {
			ui.Column(c).Gap(5).Children(func() {
				typography.Caption(c, "事项")
				ui.TextField(c, &form.title, "事项", "例如：写周报").AutoFocus()
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
	title := strings.TrimSpace(form.title)
	if title == "" {
		form.err = "请输入事项"
		return
	}
	if form.start != "" && form.due != "" && form.start > form.due {
		form.err = "截止日期不能早于开始日期"
		return
	}
	item := domain.TodoItem{
		ID: form.id, CardID: form.cardID, Title: title,
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
