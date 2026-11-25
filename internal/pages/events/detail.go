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

// detail is the middle pane: the selected day's identity and pictures, its body,
// its tone and the actions. With no event selected it is the page's empty state.
func (m *Model) detail(c ui.C) {
	if !m.hasEvent {
		m.emptyDetail(c)
		return
	}
	m.effectDisplay(c)
	// Basis(0) is what keeps the three panes inside the window at its narrowest:
	// the tone row's preferred width would otherwise widen the centre and push
	// the linked rail past the right edge.
	ui.Column(c).Grow(1).Basis(0).Shrink(1).MinWidth(0).FillHeight().Children(func() {
		m.detailHeader(c)
		ui.Divider(c)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).FillHeight().Pad(layout.Inset).
			Gap(layout.BlockGap).Children(func() {
			// The pictures and the body share the height two to one when both
			// have something to show. A side with nothing to show keeps its
			// compact empty state instead of claiming a share; the other side
			// then takes the rest.
			hasPictures := len(m.images) > 0
			hasBody := m.editing || strings.TrimSpace(m.event.Content) != ""
			pictures, body := detailSplit(hasPictures, hasBody)
			m.gallery(c, pictures)
			m.description(c, body)
			if pictures == 0 && body == 0 {
				ui.Spacer(c)
			}
			m.swatches(c)
			ui.Divider(c)
			m.actions(c)
		})
	})
}

// detailSplit is how the detail pane divides the height between the pictures
// and the body: two to one when both have something to show; when one side is
// empty the other takes the rest, and the empty side keeps its compact state.
func detailSplit(hasPictures, hasBody bool) (pictures, body float32) {
	switch {
	case hasPictures && hasBody:
		return 2, 1
	case hasPictures:
		return 1, 0
	case hasBody:
		return 0, 1
	}
	return 0, 0
}

// detailHeader names the selected day the way a rail header names its pane:
// title, date, and how many pictures the day holds. Without it the middle pane
// had no identity of its own; with a long title the left card is the only place
// that could say which day is being read.
func (m *Model) detailHeader(c ui.C) {
	layout.SectionHeader(c, layout.Inset).Shrink(1).MinWidth(0).Children(func() {
		typography.Section(c, displayTitle(m.event)).Grow(1).Shrink(1).MinWidth(0).Ellipsis()
		if strings.TrimSpace(m.event.Title) != "" {
			typography.Caption(c, domain.DiaryDateCn(m.event.Date)).Muted().Shrink(0)
		}
		if count := len(m.images); count > 0 {
			typography.Caption(c, fmt.Sprintf("%d 张图片", count)).Muted().Shrink(0)
		}
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

// swatches is the twenty-tone palette plus 使用默认颜色, kept at the foot with the
// actions as the day's properties. Selecting one stores it immediately; the tone
// colours are content, so they are read from the palette, not from the theme.
// The palette is one focus group: twenty-one separate tab stops at the foot
// would make the keyboard walk every tone before the next control.
func (m *Model) swatches(c ui.C) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Shrink(1).MinWidth(0).Gap(layout.Gap).Items(ui.Start).Children(func() {
		typography.Caption(c, "颜色").Muted().Shrink(0)
		ui.Flow(c).Grow(1).Shrink(1).MinWidth(0).Gap(layout.Gap).
			FocusGroup(ui.FocusHorizontal).Children(func() {
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
	})
}

// gallery is the stage and the thumbnail column, or the compact empty state that
// is also the place the user adds the first picture.
func (m *Model) gallery(c ui.C, grow float32) {
	theme := c.Theme()
	if len(m.images) == 0 {
		ui.Row(c).Height(emptyGalleryHeight).FillWidth().
			Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).
			Background(theme.Muted).Center().Children(func() {
			ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
				typography.Body(c, "暂无图片").Muted()
				if ui.Button(c, "添加图片").Secondary().Small().Icon(icons.Plus).Clicked() {
					c.After(func() { m.addImages() })
				}
			})
		})
		return
	}
	ui.Row(c).Grow(grow).Basis(0).Shrink(1).MinWidth(0).FillWidth().MinHeight(stageMinHeight).
		Gap(layout.BlockGap).Children(func() {
		m.stage(c)
		m.thumbs(c)
	})
}

// stage is the selected picture shown whole, over a blurred copy of itself. The
// page's grid: contain the photo (never crop it) and let the same photo
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
		ui.Row(c).Absolute().Right(layout.Gap).Bottom(layout.Gap).Gap(layout.Gap).Children(func() {
			if ui.Button(c, "删除图片").Secondary().Small().Clicked() {
				m.askDeleteImage(image.ID)
			}
			if ui.Button(c, "保存图片").Secondary().Small().Clicked() {
				c.After(func() { m.saveImage() })
			}
		})
		if image.MotionPath != "" {
			ui.Row(c).Absolute().Left(layout.Gap).Bottom(layout.Gap).
				PadSides(2, 8, 2, 8).Radius(999).
				Background(theme.Card).Border(1, theme.Border).
				HitWith(ui.ID(c), "实况", gfx.HitOptions{SkipFocus: true}).
				Tooltip("实况照片已保存，动态部分暂未支持播放").Children(func() {
				typography.Caption(c, "实况")
			})
		}
	})
}

// thumbs is the 160 px column of 160×90 pictures, each a button with its delete
// command and, for a live photo, a 实况 badge.
func (m *Model) thumbs(c ui.C) {
	// The scroll area takes the gallery's height from the row it sits in and
	// scrolls its column. Do not give it FillHeight: a scroll area with a fill
	// height reports its content as its preferred height, which makes a long
	// thumbnail column stretch the whole gallery (and with it the stage) past
	// the window instead of scrolling.
	ui.Scroll(c).Width(thumbWidth).Gap(layout.Gap).Children(func() {
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
// user writes. It is plain text; newlines are kept and Markdown is literal. When
// it has a share of the pane it takes that share (one third beside pictures) and
// scrolls inside it; with no text it keeps the compact empty height.
func (m *Model) description(c ui.C, grow float32) {
	theme := c.Theme()
	column := ui.Column(c).Shrink(1).MinWidth(0).FillWidth()
	if grow > 0 {
		minimum := bodyEmptyHeight
		if m.editing {
			minimum = bodyEditHeight
		}
		column = column.Grow(grow).Basis(0).MinHeight(minimum)
	} else {
		column = column.Height(bodyEmptyHeight)
	}
	column.Radius(theme.Metrics.RadiusSurface).Background(theme.Muted).Children(func() {
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
		ui.Spacer(c)
		if len(m.images) > 0 {
			if ui.Button(c, "添加图片").Secondary().Icon(icons.Plus).Clicked() {
				c.After(func() { m.addImages() })
			}
		}
		if ui.Button(c, "编辑描述").Secondary().Icon(icons.Pencil).Clicked() {
			m.startEdit()
		}
	})
}
