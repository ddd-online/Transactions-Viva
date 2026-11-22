package events

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// detail is the middle pane: the tone palette, the pictures, the body and the
// actions. With no event selected it is the page's empty state.
func (m *Model) detail(c ui.C) {
	if !m.hasEvent {
		m.emptyDetail(c)
		return
	}
	ui.Column(c).Grow(1).FillHeight().Pad(layout.Inset).
		Gap(layout.BlockGap).Children(func() {
		m.swatches(c)
		ui.Divider(c)
		m.gallery(c)
		m.description(c)
		ui.Divider(c)
		m.actions(c)
	})
}

// emptyDetail says what to do with the centre pane before an event is chosen.
func (m *Model) emptyDetail(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).Center().Gap(layout.BlockGap).Children(func() {
		ui.Icon(c, icons.Events).Size(28, 28).Color(theme.MutedForeground)
		typography.Body(c, "选择左侧事件查看详情").Muted()
	})
}

// swatches is the twenty-tone palette plus 使用默认颜色. Selecting one stores it
// immediately; the tone colours are content, so they are read from the palette,
// not from the theme.
func (m *Model) swatches(c ui.C) {
	theme := c.Theme()
	ui.Flow(c).Gap(layout.Gap).Children(func() {
		for _, tone := range eventTones {
			value, name := tone.value, tone.name
			chosen := m.color == value
			border := toneColor(value, *theme)
			if chosen {
				border = theme.Foreground
			}
			c.Key("tone-" + value)
			box := ui.Row(c).Size(swatchSize, swatchSize).Radius(swatchSize/2).
				Background(toneColor(value, *theme)).Border(2, border).
				Hover(ui.Look{}.Border(theme.Foreground)).
				Focus(ui.Look{}.Ring(2, theme.Ring)).
				HitWith(ui.ID(c), name, gfx.HitOptions{Role: gfx.RoleButton, Selected: chosen}).
				Tooltip(name)
			if box.Clicked() {
				m.changeColor(value)
			}
		}
		c.Key("tone-default")
		chosen := m.color == ""
		border := theme.Input
		if chosen {
			border = theme.Foreground
		}
		clear := ui.Row(c).Size(swatchSize, swatchSize).Radius(swatchSize/2).
			Border(2, border).
			Hover(ui.Look{}.Border(theme.Foreground)).
			Focus(ui.Look{}.Ring(2, theme.Ring)).
			HitWith(ui.ID(c), "使用默认颜色", gfx.HitOptions{Role: gfx.RoleButton, Selected: chosen}).
			Tooltip("使用默认颜色")
		if !chosen {
			clear = clear.Dashed()
		}
		if clear.Clicked() {
			m.changeColor("")
		}
	})
}

// gallery is the stage and the thumbnail column, or the dashed empty state that
// is also the place the user adds the first picture.
func (m *Model) gallery(c ui.C) {
	theme := c.Theme()
	if len(m.images) == 0 {
		ui.Row(c).Grow(1).FillWidth().MinHeight(stageMinHeight).
			Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Background(theme.Muted).Center().Children(func() {
			typography.Body(c, "暂无图片").Muted()
		})
		return
	}
	ui.Row(c).Grow(1).FillWidth().MinHeight(stageMinHeight).
		Gap(layout.BlockGap).Children(func() {
		m.stage(c)
		m.thumbs(c)
	})
}

// stage is the selected picture shown whole, over a blurred copy of itself. The
// old page's grid: contain the photo (never crop it) and let the same photo
// blurred fill the bars at its sides.
func (m *Model) stage(c ui.C) {
	theme := c.Theme()
	image, ok := m.mainImage()
	if !ok {
		return
	}
	ui.Stack(c).Grow(1).MinWidth(0).MinHeight(160).
		Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).Clip().Children(func() {
		slot := ui.Slot(c).Fill().HitWith(ui.ID(c), "查看大图", gfx.HitOptions{Role: gfx.RoleButton})
		slot.Draw(func(d *ui.DrawContext) {
			m.drawStage(d, *image)
		})
		if slot.Clicked() {
			m.openPreview()
		}
		ui.Column(c).Absolute().Right(layout.Gap).Bottom(layout.Gap).Children(func() {
			if ui.Button(c, "保存图片").Secondary().Small().Clicked() {
				m.saveImage()
			}
		})
		if image.MotionPath != "" {
			ui.Row(c).Absolute().Left(layout.Gap).Bottom(layout.Gap).
				PadSides(2, 8, 2, 8).Radius(999).
				Background(theme.Card).Border(1, theme.Border).
				HitWith(ui.ID(c), "实况", gfx.HitOptions{SkipFocus: true}).
				Tooltip("实况照片的动态部分已保存；此版本不播放视频").Children(func() {
				typography.Caption(c, "实况")
			})
		}
	})
}

// thumbs is the 160 px column of 160×90 pictures, each a button with its delete
// command and, for a live photo, a 实况 badge.
func (m *Model) thumbs(c ui.C) {
	ui.Scroll(c).Width(thumbWidth).FillHeight().Gap(layout.Gap).Children(func() {
		for index := range m.images {
			m.thumb(c, index)
		}
	})
}

// thumb is one picture's slot. The picture is cropped to the slot (cover): this
// column is navigation, the stage is where the whole photo is read.
func (m *Model) thumb(c ui.C, index int) {
	theme := c.Theme()
	image := m.images[index]
	selected := image.ID == m.selectedImageID
	border := gfx.Color{}
	if selected {
		border = theme.Primary
	}
	c.Key(image.ID)
	ui.Row(c).Width(thumbWidth).Height(thumbHeight).Pad(2).Radius(theme.Metrics.RadiusItem()).
		Background(border).Children(func() {
		ui.Stack(c).Fill().Clip().Radius(theme.Metrics.RadiusItem()).Children(func() {
			slot := ui.Slot(c).Fill().HitWith(ui.ID(c), fmt.Sprintf("查看第 %d 张图片，共 %d 张%s",
				index+1, len(m.images), liveSuffix(image)),
				gfx.HitOptions{Role: gfx.RoleButton, Selected: selected})
			slot.Draw(func(d *ui.DrawContext) {
				m.drawThumb(d, image)
			})
			if slot.Clicked() {
				m.selectImage(image.ID)
			}
			remove := ui.ButtonBase(c, fmt.Sprintf("删除第 %d 张图片", index+1)).
				Size(20, 20).Radius(10).
				Background(theme.Card).Border(1, theme.Border).
				Hover(ui.Look{}.Background(theme.Hover)).
				Focus(ui.Look{}.Ring(2, theme.Ring)).
				Tooltip("删除图片").
				Absolute().Top(4).Right(4)
			remove.Children(func() {
				ui.Icon(c, icons.Trash2).Size(11, 11)
			})
			if remove.Clicked() {
				m.askDeleteImage(image.ID, index+1)
			}
			if image.MotionPath != "" {
				ui.Row(c).Absolute().Left(4).Bottom(4).
					PadSides(1, 6, 1, 6).Radius(999).
					Background(theme.Foreground.WithAlpha(0.65)).Children(func() {
					typography.Caption(c, "实况").Color(theme.Background)
				})
			}
		})
	})
}

// liveSuffix names a live photo in the thumbnail's accessible label.
func liveSuffix(image domain.KeyEventImage) string {
	if image.MotionPath != "" {
		return "，实况照片"
	}
	return ""
}

// description is the body: a muted well that scrolls, or the editor while the
// user writes. It is plain text; newlines are kept and Markdown is literal.
func (m *Model) description(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Grow(1).MinHeight(120).FillWidth().
		Radius(theme.Metrics.RadiusSurface).Background(theme.Muted).Children(func() {
		if m.editing {
			ui.TextArea(c, &m.draft, "事件描述", "写下这件事……").Fill().
				Radius(theme.Metrics.RadiusSurface)
			return
		}
		ui.Scroll(c).Grow(1).Pad(layout.BlockGap).Children(func() {
			if strings.TrimSpace(m.event.Content) == "" {
				typography.Body(c, "暂无描述").Muted()
				return
			}
			ui.Wrap(c, m.event.Content).FontSize(typography.SizeBody).
				Color(theme.Foreground).MaxWidth(75 * 8)
		})
	})
}

// actions is the footer: 添加图片 and 编辑描述 at rest, 取消 and 保存 while
// editing.
func (m *Model) actions(c ui.C) {
	ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
		if m.editing {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.cancelEdit()
			}
			ui.Spacer(c)
			if ui.Button(c, "保存").Clicked() {
				m.saveContent()
			}
			return
		}
		if ui.Button(c, "添加图片").Secondary().Icon(icons.Plus).Clicked() {
			m.addImages()
		}
		ui.Spacer(c)
		if ui.Button(c, "编辑描述").Secondary().Icon(icons.Pencil).Clicked() {
			m.startEdit()
		}
	})
}
