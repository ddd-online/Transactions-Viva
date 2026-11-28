package notes

import (
	"slices"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/breadcrumb"
	"transactions/internal/components/layout"
	"transactions/internal/components/state"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// The band under the title. The title band says which note is open; this one says
// where that note lives, and what kind of block the caret is writing in — the two
// questions the body cannot answer about itself while a document is open.
const (
	// bandControlHeight is the box of the band's commands. It is the 28px icon key
	// the rail's header uses, so the two bands' controls sit on one line even
	// though their bands are in different columns.
	bandControlHeight float32 = 28
	// bandRuleHeight is the hairline that separates the kind of the block from the
	// two blocks a note adds. It is the page bands' own separator height.
	bandRuleHeight float32 = 24
	// pathMinWidth is the least room the band keeps for the path: three short names
	// fit, so a narrow window slides the block commands off the band's centre line
	// instead of cutting the folders a note lives in down to their first glyph.
	pathMinWidth float32 = 200
)

// editorBand is the strip under the title: where the note lives on the left, the
// commands that change the block being written in the middle, and the right half
// left for what comes next.
//
// The three columns weigh the same, so the commands sit on the band's centre line
// while the path fits in its half; the path's column keeps a floor below that, so
// the path is the last thing to give up its names when the window narrows.
func (m *Model) editorBand(c ui.C) {
	ui.Row(c).Height(layout.SectionHeaderHeight).FillWidth().PadX(layout.Inset).
		Items(ui.Center).Children(func() {
		ui.Row(c).Grow(1).Basis(0).MinWidth(pathMinWidth).Items(ui.Center).Children(func() {
			breadcrumb.Path(c, m.breadcrumbNames())
		})
		m.blockCommands(c)
		ui.Row(c).Grow(1).Basis(0)
	})
}

// blockCommands is the band's middle: the kinds the block can take, then the picture
// and the rule a note adds rather than converts. The hairline keeps "what this block
// is" apart from "what goes into the note", the way a page band separates the ledger
// from the page's own filters.
func (m *Model) blockCommands(c ui.C) {
	ui.Row(c).Shrink(0).Gap(2).Items(ui.Center).Children(func() {
		m.typeButton(c, domain.NoteBlockText, 0, icons.Pilcrow, "正文")
		m.typeButton(c, domain.NoteBlockHeading, 1, icons.Heading1, "标题 1")
		m.typeButton(c, domain.NoteBlockHeading, 2, icons.Heading2, "标题 2")
		m.typeButton(c, domain.NoteBlockHeading, 3, icons.Heading3, "标题 3")
		ui.Divider(c).Height(bandRuleHeight).MarginX(4)
		m.iconButton(c, "插入图片", icons.Image, m.pickPicture)
		m.iconButton(c, "插入分隔线", icons.Minus, m.insertDivider)
	})
}

// typeButton is one kind in the band's set: it turns the block the caret is in into
// its kind, and it wears the selected fill while the caret is in that kind, so the
// band answers "what is this line" as well as "make it that".
//
// The set stops at level 3: deeper headings are a rarer hand and stay in the grip's
// menu and the "/" menu, which list all five levels and their chords.
func (m *Model) typeButton(c ui.C, kind string, level int, icon gfx.Icon, label string) {
	theme := c.Theme()
	active := false
	if index := m.caretBlock(); index >= 0 {
		block := m.blocks[index]
		active = block.Kind == kind && (kind != domain.NoteBlockHeading || block.Level == level)
	}
	button := ui.ButtonBase(c, label).Size(bandControlHeight, bandControlHeight).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Pressed(ui.Look{}.Background(state.Pressed(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip(label).Shrink(0)
	if active {
		// The chosen kind keeps its wash under the pointer: the fill is a state, not
		// a hover that can wash it away.
		button = button.Background(theme.Accent).
			Hover(ui.Look{}.Background(theme.Accent.Over(theme.Hover))).
			Pressed(ui.Look{}.Background(theme.Accent.Over(state.Pressed(*theme))))
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(15, 15)
	})
	if button.Clicked() {
		m.setBlockType(kind, level)
	}
}

// setBlockType turns the block the caret is in into a text block or a heading level.
// Its words stay where they are — only the block's shape changes — and the switch is
// the same one undo step the grip menu's 转为 row records.
func (m *Model) setBlockType(kind string, level int) {
	index := m.caretBlock()
	if index < 0 || index >= len(m.blocks) {
		return
	}
	block := m.blocks[index]
	if block.Kind == kind && block.Level == level {
		return
	}
	m.newHistoryStep()
	m.blocks[index].Kind = kind
	m.blocks[index].Level = level
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.focusBody = true
	m.markEdited(m.now)
	m.flush()
}

// caretBlock is the block the caret is in, or -1 when no body holds a caret.
func (m *Model) caretBlock() int {
	if m.doc == nil || len(m.blocks) == 0 {
		return -1
	}
	index, _ := m.position(m.doc.ByteOffset(m.doc.Caret()))
	if index < 0 || index >= len(m.blocks) {
		return -1
	}
	return index
}

// breadcrumbNames is where the note lives: the folders that hold it from the root
// down, then the note itself. The note's name is the draft in the title field and
// not the title last saved, so the band renames as the note is named.
func (m *Model) breadcrumbNames() []string {
	if m.selected == "" {
		return nil
	}
	node, ok := m.nodes[m.selected]
	if !ok {
		return nil
	}
	names := []string{domain.NoteDisplayTitle(m.titleText)}
	// A folder never holds itself: the walk stops at the root, and the depth guard
	// keeps a malformed parent chain from spinning the paint.
	for depth := 0; node.ParentID != "" && depth < len(m.nodes); depth++ {
		parent, ok := m.nodes[node.ParentID]
		if !ok {
			break
		}
		node = parent
		if name := m.rowName(node); name != "" {
			names = append(names, name)
		}
	}
	slices.Reverse(names)
	return names
}
