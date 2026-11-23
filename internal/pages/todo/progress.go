package todo

import (
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

type progressPanel struct {
	open      bool
	item      domain.TodoItem
	cardTitle string
	editID    string
	editText  string
	err       string
}

func (m *Model) openProgress(item domain.TodoItem) {
	cardTitle := ""
	for _, card := range m.cards {
		if card.ID == item.CardID {
			cardTitle = card.Title
			break
		}
	}
	m.progress = progressPanel{open: true, item: item, cardTitle: cardTitle}
}

func (p *progressPanel) close() {
	p.open = false
	p.editID = ""
	p.editText = ""
	p.err = ""
}

func (m *Model) progressView(c ui.C) {
	if !m.progress.open {
		return
	}
	panel := &m.progress
	title := "进度记录"
	if panel.item.Title != "" {
		title = "进度 · " + panel.item.Title
	}
	ui.ModalWith(c, &panel.open, title, ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			meta := time.Unix(panel.item.CompletedAt, 0).Format("2006-01-02 15:04")
			typography.Caption(c, panel.cardTitle+" · 完成于 "+meta).Mono()
			if len(panel.item.Progress) == 0 {
				typography.Body(c, "这条事项没有进度记录。").Muted()
				return
			}
			ui.Scroll(c).Height(320).FillWidth().Gap(2).Children(func() {
				deleted := false
				for index := range panel.item.Progress {
					if deleted {
						break
					}
					line := &panel.item.Progress[index]
					c.Key("progress-" + line.ID)
					ui.Column(c).FillWidth().Gap(4).Children(func() {
						ui.Row(c).FillWidth().Gap(8).Items(ui.Center).Children(func() {
							check := ui.Row(c).Size(16, 16).Shrink(0).Radius(8).
								Border(1, c.Theme().Border).Center().
								HitWith(ui.ID(c), "勾选进度", gfx.HitOptions{
									Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
								}).Hover(ui.Look{}.Background(state.Hover(*c.Theme())))
							check.Children(func() {
								if line.Done {
									ui.Icon(c, icons.CircleCheck).Size(14, 14).Color(c.Theme().Success)
								}
							})
							if check.Clicked() {
								if err := m.todos.SetTodoProgressDone(m.context(), line.ID, !line.Done); err != nil {
									panel.err = err.Error()
								} else {
									line.Done = !line.Done
									m.reload()
								}
							}
							if panel.editID == line.ID {
								ui.TextField(c, &panel.editText, "进度内容", "").Grow(1).AutoFocus()
								if ui.Button(c, "保存").Small().Clicked() {
									m.saveProgressEdit(panel, line)
								}
								if ui.Button(c, "取消").Small().Ghost().Clicked() {
									panel.editID, panel.editText = "", ""
								}
							} else {
								text := typography.Body(c, line.Content).Grow(1)
								if line.Done {
									text.Muted()
								}
								typography.Caption(c, time.Unix(line.CreatedAt, 0).Format("01-02 15:04")).Mono()
								if ui.Button(c, "编辑进度").Ghost().Icon(icons.Pencil).IconOnly().
									Tooltip("编辑").Clicked() {
									panel.editID, panel.editText = line.ID, line.Content
								}
							}
							if ui.Button(c, "删除进度").Ghost().Icon(icons.Trash2).IconOnly().
								Tooltip("删除").Clicked() {
								if err := m.todos.DeleteTodoProgress(m.context(), line.ID); err != nil {
									panel.err = err.Error()
								} else {
									panel.item.Progress = append(panel.item.Progress[:index], panel.item.Progress[index+1:]...)
									deleted = true
									m.reload()
								}
							}
						})
						ui.Divider(c)
					})
				}
			})
			if panel.err != "" {
				typography.Caption(c, panel.err).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "关闭").Outline().Clicked() {
			panel.close()
		}
	})
}

func (m *Model) saveProgressEdit(panel *progressPanel, line *domain.TodoProgress) {
	if strings.TrimSpace(panel.editText) == "" {
		panel.err = "请填写进度"
		return
	}
	saved, err := m.todos.UpdateTodoProgress(m.context(), line.ID, panel.editText)
	if err != nil {
		panel.err = err.Error()
		return
	}
	line.Content = saved.Content
	panel.editID, panel.editText, panel.err = "", "", ""
	m.reload()
}
