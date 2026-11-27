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

// typeMenuState is the band's block-type menu: the block it will change, and
// whether the last paint had it open. It is Viva's menu, anchored to the band's
// button, and it is kept apart from the grip's blockMenu so the two never both
// claim one menu.
type typeMenuState struct {
	open    bool
	block   int
	wasOpen bool
}

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

// blockCommands is the band's middle: the kind the block is, then the picture and
// the rule a note adds rather than converts. The hairline keeps "what this block
// is" apart from "what goes into the note", the way a page band separates the
// ledger from the page's own filters.
func (m *Model) blockCommands(c ui.C) {
	ui.Row(c).Shrink(0).Gap(2).Items(ui.Center).Children(func() {
		m.blockTypeButton(c)
		ui.Divider(c).Height(bandRuleHeight).MarginX(4)
		m.iconButton(c, "插入图片", icons.Image, m.pickPicture)
		m.iconButton(c, "插入分隔线", icons.Minus, m.insertDivider)
	})
}

// blockTypeButton is the band's menu: it wears the kind of the block the caret is
// in — ¶ for prose, H1–H5 for a heading, and the insert marks for a picture and a
// rule — and opens the kinds that block can become. The grip's menu offers the same
// list with the same chords; the band is the same command without leaving the text.
func (m *Model) blockTypeButton(c ui.C) {
	theme := c.Theme()
	block := domain.NoteBlock{Kind: domain.NoteBlockText}
	if index := m.caretBlock(); index >= 0 {
		block = m.blocks[index]
	}
	button := ui.ButtonBase(c, "块类型").Height(bandControlHeight).PadX(6).Gap(2).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Pressed(ui.Look{}.Background(state.Pressed(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip("块类型").Shrink(0)
	button.Children(func() {
		ui.Icon(c, blockKindIcon(block)).Size(15, 15)
		// The chevron is the only word the band spends on saying "this opens a list":
		// the face is the value, the mark is the affordance.
		ui.Icon(c, icons.ChevronDown).Size(10, 10).Color(theme.MutedForeground)
	})
	if button.Clicked() {
		m.typeMenu.block = m.caretBlock()
		m.typeMenu.open = !m.typeMenu.open
	}
	ui.Menu(c, button, &m.typeMenu.open, func(items *ui.MenuItems) {
		for _, command := range gripCommands {
			if command.group() != commandGroupTurn {
				continue
			}
			// The chord is in the row, so the menu teaches the shortcut the body
			// already declares: what the band does with the pointer, the keyboard
			// does without leaving the line.
			if shortcutHint(items.Item(command.label).Icon(command.icon), command).Clicked() {
				m.chooseBlockType(command)
				return
			}
		}
	})
}

// chooseBlockType turns the block the menu was opened on into the command's kind.
// Its words stay where they are — only the block's shape changes — and the switch
// is the same one undo step the grip menu's 转为 row records.
func (m *Model) chooseBlockType(command bodyCommand) {
	index := m.typeMenu.block
	m.typeMenu.open = false
	if index < 0 || index >= len(m.blocks) {
		return
	}
	block := m.blocks[index]
	if block.Kind == command.kind && block.Level == command.level {
		return
	}
	m.newHistoryStep()
	m.blocks[index].Kind = command.kind
	m.blocks[index].Level = command.level
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
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

// blockKindIcon is the mark of a block's kind: the paragraph sign for prose, the
// level's own H for a heading, and the marks the insert commands carry for a
// picture and a rule, so the band's face says what the block is.
func blockKindIcon(block domain.NoteBlock) gfx.Icon {
	switch block.Kind {
	case domain.NoteBlockHeading:
		return headingIcon(block.Level)
	case domain.NoteBlockImage:
		return icons.Image
	case domain.NoteBlockDivider:
		return icons.Minus
	}
	return icons.Pilcrow
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
