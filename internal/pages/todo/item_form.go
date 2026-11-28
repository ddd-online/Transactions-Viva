package todo

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/layout"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
)

type itemForm struct {
	open bool
	// cardID is the theme the item is saved into; cardTitle is the name the
	// form shows, which may be a theme that does not exist yet.
	cardID     string
	cardTitle  string
	themes     []string
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
		// A new item starts today: most are written the day they begin, and the
		// field is one press away from another day. An item being edited keeps the
		// start it was saved with.
		start: domain.Today(),
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

// refreshItemThemes rebuilds the theme field's choices from the board.
func (m *Model) refreshItemThemes() {
	form := &m.itemForm
	form.themes = m.cardTitles()
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
	ui.Modal(c, &form.open, title, ui.ModalSpec{Width: 560}, func() {
		m.itemFields(c, form)
	}, func() {
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
			layout.StatusLine(c, form.err, func(c ui.C) {
				typography.Caption(c, form.err).Color(c.Theme().Danger)
			})
		}
	})
}

// Key gives the form's date panels their arrows before the page handles the
// keys. The panel owns no key route of its own: it is painted on the canvas.
func (form *itemForm) Key(event gfx.KeyEvent) gfx.EventResult {
	for _, pick := range []*datepicker.State{&form.startPick, &form.duePick} {
		if pick.Open {
			if result := pick.Key(event); result.Handled {
				return result
			}
		}
	}
	return gfx.EventResult{}
}

// themeField is the theme row: a Combobox over the board's themes that also takes
// a name the board does not have yet — it offers what was typed as a new theme,
// and saving creates it. One field therefore both picks an existing theme and
// starts a new one, and the form needs no separate "new theme" command.
func (m *Model) themeField(c ui.C, form *itemForm) {
	field := selectbox.Combo(c, &form.cardTitle, form.themes, nil, "主题").
		FillWidth().
		Placeholder("选择或输入主题").
		Creatable("新建主题 ").
		Empty("没有匹配的主题，输入新名字后回车新建")
	if created, ok := field.Created(); ok {
		form.cardTitle = created
		// The field shows the chosen choice's name, so the new theme joins the
		// choices here and not at save time; it reaches the board only when the
		// item is saved.
		form.themes = appendUnique(form.themes, created)
		// The field paints the chosen choice's name on the next frame; ask for it,
		// or the field would stay blank until something else repaints the page.
		c.Canvas().RequestFrame()
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
	ui.Modal(c, &m.deleteCard.open, "删除主题「"+m.deleteCard.title+"」？", ui.ModalSpec{Width: 420}, func() {
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
	ui.Modal(c, &m.deleteItem.open, "删除事项「"+m.deleteItem.title+"」？", ui.ModalSpec{Width: 420}, func() {
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

// appendUnique adds name to names unless it is already there.
func appendUnique(names []string, name string) []string {
	for _, existing := range names {
		if existing == name {
			return names
		}
	}
	return append(names, name)
}
