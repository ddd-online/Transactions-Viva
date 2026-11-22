package events

import (
	"fmt"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
)

// addDialog is 新增事件: a day and an optional title. 新增 without a day is
// disabled, as the dialog's confirm was.
func (m *Model) addDialog(c ui.C) {
	ui.ModalWith(c, &m.add.open, "新增事件", ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				typography.Caption(c, "日期")
				datepicker.Picker(c, &m.add.pick, datepicker.Spec{
					Label:       "事件日期",
					Start:       m.add.date,
					Grain:       datepicker.ByDay,
					CloseOnPick: true,
					Viewport:    m.bounds,
					Fill:        true,
					OnRange: func(start, _ string) {
						m.add.date = start
					},
				})
			})
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				typography.Caption(c, "标题")
				ui.TextField(c, &m.add.title, "标题", "事件标题").AutoFocus()
			})
		})
	}, func() {
		ui.Row(c).Gap(layout.Gap).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.add.open = false
			}
			if ui.Button(c, "新增").Disabled(m.add.date == "").Clicked() {
				m.submitAdd()
			}
		})
	})
}

// deleteEventDialog confirms deleting a day's event, including its pictures.
func (m *Model) deleteEventDialog(c ui.C) {
	ui.ModalWith(c, &m.deleteEvent.open, "删除事件「"+m.deleteEvent.title+"」？",
		ui.ModalSpec{Width: 420, Description: "事件里的图片会一并删除。"}, nil, func() {
			ui.Row(c).Gap(layout.Gap).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Secondary().Clicked() {
					m.deleteEvent.open = false
				}
				if ui.Button(c, "删除").Destructive().Clicked() {
					m.confirmDeleteEvent()
				}
			})
		})
}

// deleteImageDialog confirms deleting one picture.
func (m *Model) deleteImageDialog(c ui.C) {
	ui.ModalWith(c, &m.deleteImage.open, "删除这张图片？",
		ui.ModalSpec{Width: 420}, nil, func() {
			ui.Row(c).Gap(layout.Gap).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Secondary().Clicked() {
					m.deleteImage.open = false
				}
				if ui.Button(c, "删除").Destructive().Clicked() {
					m.confirmDeleteImage()
				}
			})
		})
}

// previewDialog is the lightbox: the picture at fit size, a wheel zoom and a
// drag, with 上一张 / 下一张 and the arrow keys to move through the day's list.
// The modal itself closes on Escape and on a press outside it.
func (m *Model) previewDialog(c ui.C) {
	theme := c.Theme()
	ui.ModalWith(c, &m.preview.open, "事件图片", ui.ModalSpec{Width: 900}, func() {
		ui.Control(c, "事件图片预览", &m.preview.handlers).
			FillWidth().Height(520).
			Radius(theme.Metrics.RadiusSurface).Clip().
			Draw(func(d *ui.DrawContext) {
				m.drawPreview(d)
			})
	}, func() {
		ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
			if ui.Button(c, "上一张").Secondary().Clicked() {
				m.stepPreview(-1)
			}
			ui.Spacer(c)
			typography.Caption(c, fmt.Sprintf("%d / %d", m.preview.index+1, len(m.images)))
			ui.Spacer(c)
			if ui.Button(c, "下一张").Secondary().Clicked() {
				m.stepPreview(1)
			}
		})
	})
}
