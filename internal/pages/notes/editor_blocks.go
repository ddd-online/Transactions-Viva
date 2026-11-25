package notes

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// A block is the body's unit: a line of text with its size, or a picture, and a
// grip in the gutter beside it. The grip and the block are one component
// (blockColumn declares the one, blockGrip the other), so the grip is the
// block's own: the menu it opens is that block's commands, which a kind may
// change, and the drag it starts moves that block.
const (
	// partImageGrip is the picture's resize handle inside the body control.
	partImageGrip = 20
	// partDropBase is the first block drop target's part number.
	partDropBase = 100
	// gripSize is the grip's box and gripGap the space between it and the text
	// column. The grip is centred on its block's first line, so the taller line
	// of a heading carries it down; a picture has no line, so its grip holds the
	// picture's top edge instead.
	gripSize float32 = 18
	gripGap  float32 = 6
	// gripRadius is the corner of both handles in the body: the grip's box and
	// the picture's resize bar, so the two read as one control family.
	gripIconWidth, gripIconHeight float32 = 10, 12
	gripRadius                    float32 = 4
)

// blockMenuState is the grip's menu: whether it is open, which block it commands,
// and whether the last paint had it open. The menu is Viva's, anchored to that
// block's grip, and it closes itself on a choice, Escape or a press outside — the
// page reads that close from wasOpen and open.
type blockMenuState struct {
	open    bool
	block   int
	wasOpen bool
}

// resizeState is a picture height drag.
type resizeState struct {
	block int
	from  float32
	scale float64
	// height and width are the displayed size the drag started with.
	height float32
}

// gripCommands are the menu's entries, in order: what a block can be turned into
// and what is done to it. Every kind offers them today; a kind that needs its own
// words writes them in blockMenuItems, which has the block. The chords are the
// same ones the "/" menu shows, so a heading is one chord away from either menu.
var gripCommands = []bodyCommand{
	{label: "转为正文", kind: domain.NoteBlockText, icon: icons.Pilcrow,
		modifiers: ui.Primary | ui.Alt, key: "0"},
	{label: "转为标题 1", kind: domain.NoteBlockHeading, level: 1, icon: icons.Heading1,
		modifiers: ui.Primary | ui.Alt, key: "1"},
	{label: "转为标题 2", kind: domain.NoteBlockHeading, level: 2, icon: icons.Heading2,
		modifiers: ui.Primary | ui.Alt, key: "2"},
	{label: "转为标题 3", kind: domain.NoteBlockHeading, level: 3, icon: icons.Heading3,
		modifiers: ui.Primary | ui.Alt, key: "3"},
	{label: "转为标题 4", kind: domain.NoteBlockHeading, level: 4, icon: icons.Heading4,
		modifiers: ui.Primary | ui.Alt, key: "4"},
	{label: "转为标题 5", kind: domain.NoteBlockHeading, level: 5, icon: icons.Heading5,
		modifiers: ui.Primary | ui.Alt, key: "5"},
	{label: "插入图片", kind: domain.NoteBlockImage, icon: icons.Image,
		modifiers: ui.Primary | ui.Shift, key: "i"},
	{label: "删除块", icon: icons.Trash2},
}

// The three groups of the block menu, in the order it shows them.
const (
	commandGroupTurn = iota
	commandGroupInsert
	commandGroupRemove
)

// group is which group of the menu a command belongs to: the table above carries
// it in what the command does — a kind converts, a picture is inserted, no kind
// removes.
func (command bodyCommand) group() int {
	switch {
	case command.kind == "":
		return commandGroupRemove
	case command.kind == domain.NoteBlockImage:
		return commandGroupInsert
	}
	return commandGroupTurn
}

// blockColumn is the body's column of blocks: the body control, which fills the
// pane, and the grip of the block the pointer is on. It is a custom layout
// because a grip's row is a row of the text: only the layout, which runs after
// the text is wrapped and before anything is painted, knows where a block's first
// line ends up.
type blockColumn struct {
	m *Model
	// block is the block whose grip is declared, or -1 for none.
	block int
}

func (b *blockColumn) MeasureWidth(*ui.LayoutContext) float32 { return 0 }

// MeasureHeight is how tall the body is: the scroll area's content.
func (b *blockColumn) MeasureHeight(*ui.LayoutContext) float32 { return b.m.bodyHeight() }

func (b *blockColumn) PlaceWidth(ctx *ui.LayoutContext, bounds gfx.Rect) {
	width, offset := bodyColumn(bounds.Width)
	// The text is wrapped here, before the page paints: the grip's row is one of
	// its rows, and the body paints the layout this computes.
	b.m.layoutBody(ctx.Frame.Canvas(), width, &ctx.Frame.Palette)
	b.m.bodyLayout.column = bounds.X + offset
	ctx.SetWidth(0, bounds.Width)
	ctx.SetX(0, bounds.X)
	if ctx.Count() > 1 {
		ctx.SetWidth(1, gripSize)
		ctx.SetX(1, bounds.X+offset-gripGap-gripSize)
	}
}

func (b *blockColumn) PlaceHeight(ctx *ui.LayoutContext, bounds gfx.Rect) {
	b.m.bodyPane = bounds
	ctx.SetHeight(0, b.m.bodyHeight())
	ctx.SetY(0, bounds.Y)
	if ctx.Count() < 2 || b.block < 0 {
		return
	}
	if y, ok := b.m.gripY(b.block); ok {
		ctx.SetHeight(1, gripSize)
		ctx.SetY(1, bodyOffset(bounds)+y)
	}
}

// blockGrip is one block's grip: the button in the gutter that opens the block's
// commands and drags the block to another place. It is the drag's source, so a
// press that does not move is a click, and a press that moves moves the block.
func (m *Model) blockGrip(c ui.C, block int) {
	theme := c.Theme()
	c.Key("note-block-grip-" + strconv.Itoa(block))
	m.gripID = ui.ID(c)
	grip := ui.ButtonBase(c, "块操作").Size(gripSize, gripSize).
		Radius(gripRadius).
		Background(theme.Hover).
		Hover(ui.Look{}.Background(state.Selected(*theme))).
		Pressed(ui.Look{}.Background(state.Pressed(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	grip.Children(func() {
		ui.Icon(c, icons.GripVertical).Size(gripIconWidth, gripIconHeight).
			Color(theme.MutedForeground)
	})
	grip.DragSource(&m.blockDrag)
	if grip.Clicked() {
		m.openBlockMenu(block)
	}
	// Under the grip, as the tree's row actions open under their button: the list
	// is the block's commands, and it leaves the block it commands in sight.
	ui.Menu(c, grip, &m.blockMenu.open, func(items *ui.MenuItems) {
		m.blockMenuItems(c, items, block)
	})
}

// litBlock is the block whose grip is shown: the one being dragged, the one whose
// menu is open, or the one under the pointer.
func (m *Model) litBlock() int {
	switch {
	case m.dragBlock >= 0 && m.dragBlock < len(m.blocks):
		return m.dragBlock
	case m.blockMenu.open && m.blockMenu.block >= 0 && m.blockMenu.block < len(m.blocks):
		return m.blockMenu.block
	case m.hoverBlock >= 0 && m.hoverBlock < len(m.blocks):
		return m.hoverBlock
	}
	return -1
}

// blockMenuItems declares one block's commands. Every kind offers gripCommands
// today; a kind that takes no text, or one that can be replaced rather than
// converted, declares its own list here from m.blocks[block].Kind. The menu is
// grouped the way the tree's row menu is: what the block becomes, what is put at
// it, then what is taken away — and the last one is the only destructive row.
func (m *Model) blockMenuItems(c ui.C, items *ui.MenuItems, block int) {
	group := gripCommands[0].group()
	for _, command := range gripCommands {
		if command.group() != group {
			items.Separator()
			group = command.group()
		}
		item := shortcutHint(items.Item(command.label).Icon(command.icon), command)
		if group == commandGroupRemove {
			item = item.Destructive()
		}
		if item.Clicked() {
			m.chooseBlockMenu(c, block, command)
			return
		}
	}
}

// blockShortcuts declares the chords both block menus show, on the block the
// caret is in: a heading is one chord from the keyboard exactly where the menu
// says it is. Every paint declares them all, and Shortcut reports the one this
// paint delivers.
func (m *Model) blockShortcuts(c ui.C) {
	if m.doc == nil {
		return
	}
	for _, command := range gripCommands {
		if command.modifiers == 0 || !ui.Shortcut(c, command.modifiers, command.key) {
			continue
		}
		block, _ := m.position(m.doc.ByteOffset(m.doc.Caret()))
		m.chooseBlockMenu(c, block, command)
		return
	}
}

// shortcutHintWidth is the room a row reserves for its chord. The longest chord
// here is Ctrl+Shift+I, which reads in about seventy of these eighty points.
const shortcutHintWidth float32 = 80

// shortcutHint writes the row's chord the way this application writes one — one
// plain string, "Ctrl+Alt+1", in the muted tone — instead of the framework's row
// of keycaps. 思源's menus are the reference: at a glance the right edge reads as
// a column of names, not as a row of little keys.
func shortcutHint(item ui.MenuItem, command bodyCommand) ui.MenuItem {
	if command.modifiers == 0 {
		return item
	}
	chord, ok := keymap.Parse(command.modifiers, command.key)
	if !ok {
		return item
	}
	label := chord.Label()
	return item.Trailing(shortcutHintWidth, func(surface *gfx.Canvas, bounds gfx.Rect, theme gfx.Theme) {
		surface.LabelAligned(label, bounds, gfx.TextStyle{
			Size: typography.SizeCaption, Color: theme.MutedForeground,
		}, gfx.AlignEnd)
	})
}

// openBlockMenu opens the grip's menu for one block, or closes it when it is that
// block's menu already open.
func (m *Model) openBlockMenu(block int) {
	if m.blockMenu.open && m.blockMenu.block == block {
		m.blockMenu.open = false
		return
	}
	m.blockMenu.open, m.blockMenu.block = true, block
}

// chooseBlockMenu runs one command of the block menu on the block it belongs to.
func (m *Model) chooseBlockMenu(c ui.C, block int, item bodyCommand) {
	m.blockMenu.open = false
	// The chosen row closes the menu, and the keyboard goes with its rows: the
	// paint after this one gives it back to the note (see View). That paint has to
	// be asked for: closing a menu asks for none of its own.
	c.After(func() { m.focusBody = true })
	if block < 0 || block >= len(m.blocks) {
		return
	}
	if item.kind == "" {
		m.deleteBlock(block)
		return
	}
	if item.kind == domain.NoteBlockImage {
		m.pickPicture()
		return
	}
	m.newHistoryStep()
	m.blocks[block].Kind = item.kind
	m.blocks[block].Level = item.level
	m.bodyLayout = bodyLayout{}
	m.focusBody = true
	m.markEdited(m.now)
	m.flush()
}

// deleteBlock removes one block and its line.
func (m *Model) deleteBlock(block int) {
	if block < 0 || block >= len(m.blocks) {
		return
	}
	m.newHistoryStep()
	lines := make([]string, 0, len(m.lines))
	for index, line := range m.lines {
		if index != block {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	blocks := make([]domain.NoteBlock, 0, len(lines))
	for index, one := range m.blocks {
		if index != block {
			blocks = append(blocks, one)
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, domain.NoteBlock{Kind: domain.NoteBlockText})
	}
	m.doc = text.NewEditor(strings.Join(lines, "\n"))
	m.blocks = blocks
	m.lines = lines
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.focusBody = true
	m.markEdited(m.now)
	m.flush()
}

// blockDrops declares the drop target of every visible block and paints the
// selected picture's resize handle. The grip is a button of the body's column, so
// it is not declared here.
func (m *Model) blockDrops(d *ui.DrawContext) {
	canvas, theme := d.Canvas, d.Theme
	rect := d.Rect
	width := m.bodyLayout.width
	column := m.bodyLayout.column
	offsetY := bodyOffset(rect)
	// Every visible block takes a drop, so a block can be dragged above or below
	// another one.
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		if row.block >= len(m.blocks) {
			continue
		}
		if first, ok := m.firstRowOfBlock(row.block); !ok || first.y != row.y {
			continue
		}
		y := offsetY + row.y
		height := m.blockHeight(row.block)
		blockRect := gfx.Rect{X: column, Y: y, Width: width, Height: height}
		if blockRect.Y+blockRect.Height < rect.Y || blockRect.Y > rect.Y+rect.Height {
			continue
		}
		d.DropTarget(&m.blockDrag, m.bodyInput.Parts().At(partDropBase+row.block), blockRect,
			func(source, target gfx.ID) bool {
				from, to := m.dragBlock, m.blockPart(target)
				if from < 0 || to < 0 || from == to {
					return false
				}
				if row, ok := m.firstRowOfBlock(to); ok {
					offsetY := bodyOffset(m.bodyPane)
					middle := offsetY + row.y + m.blockHeight(to)/2
					m.dropAfter = m.blockDrag.At().Y > middle
				}
				return true
			},
			func(source, target gfx.ID) { m.dropBlock(target) })
	}
	if m.selectedImage >= 0 && m.selectedImage < len(m.blocks) {
		m.paintImageGrip(canvas, theme, offsetY)
	}
}

// firstRowOfBlock is the first painted row of a block.
func (m *Model) firstRowOfBlock(block int) (bodyRow, bool) {
	for _, row := range m.bodyLayout.rows {
		if row.block == block {
			return row, true
		}
	}
	return bodyRow{}, false
}

// gripY is where a block's grip sits, from the start of the body content: centred
// on the block's first line, so the grip of a heading comes down with the
// heading's taller line and stays level with its text. The centre is the line
// box's, which is where Viva puts the baseline of the label painted there. A
// picture has no line, so its grip holds the picture's top edge.
func (m *Model) gripY(block int) (float32, bool) {
	row, ok := m.firstRowOfBlock(block)
	if !ok {
		return 0, false
	}
	if block >= 0 && block < len(m.blocks) && m.blocks[block].Kind == domain.NoteBlockImage {
		return row.y, true
	}
	return row.y + (row.height-gripSize)/2, true
}

// blockHeight is the painted height of a block.
func (m *Model) blockHeight(block int) float32 {
	var height float32
	for _, row := range m.bodyLayout.rows {
		if row.block == block {
			height += row.height
		}
	}
	return height
}

// blockPart decodes a block's drop target part back into its index.
func (m *Model) blockPart(id gfx.ID) int {
	index, ok := id.In(m.bodyInput.Parts())
	if !ok || index < partDropBase {
		return -1
	}
	return index - partDropBase
}

// paintImageGrip paints the selected picture's resize handle under it.
func (m *Model) paintImageGrip(canvas *gfx.Canvas, theme *ui.Theme, offsetY float32) {
	row, ok := m.firstRowOfBlock(m.selectedImage)
	if !ok || m.selectedImage >= len(m.blocks) {
		return
	}
	width := m.bodyLayout.width
	_, height := domain.NoteImageSize(m.blocks[m.selectedImage].ImageWidth, m.blocks[m.selectedImage].ImageHeight,
		m.blocks[m.selectedImage].ImageScale, float64(width))
	if height <= 0 {
		return
	}
	grip := gfx.Rect{
		X: m.bodyLayout.column + width/2 - 18, Y: offsetY + row.y + float32(height) - 4,
		Width: 36, Height: 8,
	}
	// The handle is a bar under the picture, rounded like the block's grip: the
	// two handles of the body carry the same corner.
	canvas.Rounded(grip, gripRadius, theme.Primary)
	canvas.HitWithOptions(m.bodyInput.Parts().At(partImageGrip), grip, "调整图片高度",
		gfx.HitOptions{Role: gfx.RoleSeparator, SkipFocus: true, Cursor: gfx.PointerCursorResizeVertical})
}

// blockAtPoint is the block under a window y.
func (m *Model) blockAtPoint(y float32) int {
	offsetY := bodyOffset(m.bodyPane)
	local := y - offsetY
	block := -1
	for _, row := range m.bodyLayout.rows {
		if local >= row.y && local <= row.y+row.height {
			block = row.block
		}
	}
	return block
}

// blockMouse handles the picture's resize. It reports whether the press was taken.
func (m *Model) blockMouse(event gfx.PointerEvent) gfx.EventResult {
	if index, ok := event.Target.In(m.bodyInput.Parts()); ok && index == partImageGrip {
		switch event.Kind {
		case gfx.PointerDown:
			if m.selectedImage >= 0 && m.selectedImage < len(m.blocks) {
				m.resize = resizeState{
					block: m.selectedImage, from: event.Y,
					scale: m.blocks[m.selectedImage].ImageScale,
				}
				return gfx.EventResult{Handled: true}
			}
		case gfx.PointerMove:
		case gfx.PointerUp:
			m.resize = resizeState{block: -1}
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if m.resize.block >= 0 {
		switch event.Kind {
		case gfx.PointerMove:
			block := m.blocks[m.resize.block]
			delta := event.Y - m.resize.from
			width := m.bodyLayout.width
			_, rawHeight := domain.NoteImageSize(block.ImageWidth, block.ImageHeight, m.resize.scale, float64(width))
			if height := float32(rawHeight); height > 0 {
				next := height + delta
				if next < 40 {
					next = 40
				}
				block.ImageScale = domain.NoteImageScale(float64(next) / float64(block.ImageHeight))
				m.blocks[m.resize.block] = block
				m.bodyLayout = bodyLayout{}
			}
			return gfx.EventResult{Handled: true, Repaint: true}
		case gfx.PointerUp:
			m.resize = resizeState{block: -1}
			m.markEdited(m.now)
			m.flush()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	return gfx.EventResult{}
}

// dropBlock moves the dragged block above or below the block it was let go on.
func (m *Model) dropBlock(target gfx.ID) {
	m.blockMenu.open = false
	from, to := m.dragBlock, m.blockPart(target)
	if from < 0 || to < 0 || from == to || from >= len(m.blocks) || to >= len(m.blocks) {
		return
	}
	m.moveBlock(from, to, m.dropAfter)
}

// moveBlock reorders the blocks and rebuilds the text in the same order.
func (m *Model) moveBlock(from, to int, after bool) {
	if from < 0 || from >= len(m.blocks) || to < 0 || to >= len(m.blocks) || from == to {
		return
	}
	m.newHistoryStep()
	order := make([]int, 0, len(m.blocks))
	for index := range m.blocks {
		if index != from {
			order = append(order, index)
		}
	}
	at := 0
	for index, one := range order {
		if one == to {
			at = index
			if after {
				at = index + 1
			}
			break
		}
	}
	order = append(order, 0)
	copy(order[at+1:], order[at:])
	order[at] = from
	blocks := make([]domain.NoteBlock, 0, len(order))
	lines := make([]string, 0, len(order))
	for _, index := range order {
		blocks = append(blocks, m.blocks[index])
		lines = append(lines, m.lines[index])
	}
	m.blocks, m.lines = blocks, lines
	m.doc = text.NewEditor(strings.Join(lines, "\n"))
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.focusBody = true
	m.markEdited(m.now)
	m.flush()
}
