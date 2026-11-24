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

type itemForm struct {
	open bool
	// cardID is the theme the item is saved into; cardTitle is the name the
	// form shows, which may be a theme that does not exist yet.
	cardID    string
	cardTitle string
	themes    []string
	choices   []string
	newTheme  bool
	// manager shows the theme manager in place of the fields; confirmTheme is
	// the theme whose delete button was pressed, waiting for the second step.
	manager      bool
	confirmTheme string
	id           string
	title        string
	start        string
	due          string
	urgency      int
	importance   int
	startPick    datepicker.State
	duePick      datepicker.State
	err          string
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
	m.refreshItemThemes()
}

// refreshItemThemes rebuilds the Select's entries from the board: the themes,
// then the two commands. It runs when the theme manager deletes a theme while
// the form is open.
func (m *Model) refreshItemThemes() {
	form := &m.itemForm
	form.themes = m.cardTitles()
	form.choices = form.choices[:0]
	form.choices = append(form.choices, form.themes...)
	if len(form.themes) > 0 {
		form.choices = append(form.choices, newThemeChoice, manageThemeChoice)
	}
	if form.cardID != "" && m.cardTitle(form.cardID) == "" {
		form.cardID, form.cardTitle = "", ""
	}
}

func (m *Model) itemFormView(c ui.C) {
	form := &m.itemForm
	if !form.open {
		return
	}
	title := "新建事项"
	if form.id != "" {
		title = "编辑事项"
	}
	if form.manager {
		title = "管理主题"
	}
	ui.ModalWith(c, &form.open, title, ui.ModalSpec{Width: 560}, func() {
		if form.manager {
			m.themeManagerBody(c, form)
			return
		}
		m.itemFields(c, form)
	}, func() {
		if form.manager {
			if ui.Button(c, "返回").Outline().Clicked() {
				form.manager, form.confirmTheme = false, ""
			}
			return
		}
		if ui.Button(c, "取消").Outline().Clicked() {
			form.open = false
		}
		action := "创建"
		if form.id != "" {
			action = "保存"
		}
		if ui.Button(c, action).Clicked() {
			m.saveItem()
		}
	})
}

func (m *Model) itemFields(c ui.C, form *itemForm) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Column(c).Gap(5).Children(func() {
			typography.Caption(c, "主题")
			m.themeField(c, form)
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
}

// themeManagerBody is the theme manager: every theme with its item count and a
// delete button, in place of the item fields. A second press confirms, so
// deleting a theme with items does not open a dialog over this dialog.
func (m *Model) themeManagerBody(c ui.C, form *itemForm) {
	typography.Caption(c, "删除主题会连同它的事项与进度一起删除。").Muted()
	if len(m.cards) == 0 {
		typography.Body(c, "还没有主题").Muted()
		return
	}
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		for index := range m.cards {
			card := &m.cards[index]
			c.Key("theme-" + card.ID)
			ui.Row(c).MinHeight(40).FillWidth().PadSides(0, 6, 0, 6).Gap(8).
				Items(ui.Center).Children(func() {
				typography.Body(c, card.Title).Grow(1).Ellipsis()
				if form.confirmTheme == card.ID {
					typography.Caption(c, fmt.Sprintf("%d 项会一起删除", len(card.Items))).Muted()
					if ui.Button(c, "取消").Small().Outline().Clicked() {
						form.confirmTheme = ""
					}
					if ui.Button(c, "删除").Small().Destructive().Clicked() {
						m.deleteTheme(card.ID)
						form.confirmTheme = ""
					}
					return
				}
				typography.Caption(c, fmt.Sprintf("%d 项", len(card.Items))).Mono().Muted()
				if ui.Button(c, "删除").Small().DestructiveOutline().Clicked() {
					form.confirmTheme = card.ID
				}
			})
		}
	})
}

// deleteTheme removes a theme and its items from the manager.
func (m *Model) deleteTheme(cardID string) {
	if m.todos == nil {
		return
	}
	if err := m.todos.DeleteTodoCard(m.context(), cardID); err != nil {
		m.fail(err)
		return
	}
	m.reload()
	m.refreshItemThemes()
	m.succeed("已删除主题")
}

// The Select's last two entries are commands, not themes: one switches the
// field to a text field for a new theme, the other opens the theme manager.
const (
	newThemeChoice    = "新建主题…"
	manageThemeChoice = "管理主题…"
)

// themeField is the theme row: a Select over the themes the board has, or a
// text field when the user is naming a new theme (and always when the board has
// none yet, where a Select would show an empty list).
func (m *Model) themeField(c ui.C, form *itemForm) {
	if form.newTheme || len(form.themes) == 0 {
		ui.Row(c).FillWidth().Gap(6).Children(func() {
			ui.TextField(c, &form.cardTitle, "主题", "例如：本周").Grow(1).AutoFocus()
			if len(form.themes) > 0 && ui.Button(c, "选择已有").Link().Clicked() {
				form.newTheme, form.cardTitle = false, ""
			}
		})
		return
	}
	choice := form.cardTitle
	selectbox.Select(c, &choice, form.choices, nil).Label("主题").Placeholder("选择主题")
	switch {
	case choice == newThemeChoice:
		form.newTheme, form.cardTitle = true, ""
	case choice == manageThemeChoice:
		form.manager = true
	case choice != form.cardTitle:
		form.cardTitle = choice
	}
}

func (m *Model) saveItem() {
	form := &m.itemForm
	if m.todos == nil {
		return
	}
	title := strings.TrimSpace(form.title)
	if title == "" {
		form.err = "请填写事项"
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
	ui.ModalWith(c, &m.deleteCard.open, "删除主题「"+m.deleteCard.title+"」？", ui.ModalSpec{Width: 420}, func() {
		typography.Body(c, "下面的所有事项和进度都会一起删除。")
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteCard.open = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
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
	ui.ModalWith(c, &m.deleteItem.open, "删除事项「"+m.deleteItem.title+"」？", ui.ModalSpec{Width: 420}, func() {
		typography.Body(c, "进度记录也会一起删除。")
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteItem.open = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
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
