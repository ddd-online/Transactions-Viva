package notes

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/domain"
)

// Block gestures over the body: the hovered block's grip (click opens its
// commands, drag moves it), the picture's resize handle and the block menu.
//
// The grip and the menu items are parts of the body control, declared in the
// same Draw that paints them. The drag state is declared there too, so a press
// on the grip reaches the page's drag routing.

const (
	// partGrip is the block grip's part number inside the body control.
	partGrip = 20
	// partImageGrip is the picture's resize handle.
	partImageGrip = 21
	// partMenuBase is the block menu's first item part number.
	partMenuBase = 40
	// partDropBase is the first block drop target's part number.
	partDropBase = 100
)

// blockMenuState is the block grip's menu: where it opened and which block it
// commands.
type blockMenuState struct {
	open  bool
	block int
	x, y  float32
}

// resizeState is a picture height drag.
type resizeState struct {
	block int
	from  float32
	scale float64
	// height and width are the displayed size the drag started with.
	height float32
}

// blockMenuItems are the block menu's entries, in order.
var blockMenuItems = []slashItem{
	{"转为正文", domain.NoteBlockText, 0},
	{"转为标题 1", domain.NoteBlockHeading, 1},
	{"转为标题 2", domain.NoteBlockHeading, 2},
	{"转为标题 3", domain.NoteBlockHeading, 3},
	{"转为标题 4", domain.NoteBlockHeading, 4},
	{"转为标题 5", domain.NoteBlockHeading, 5},
	{"删除块", "", 0},
}

// blockView paints the grip of the block under the pointer, the picture's resize
// handle and the block menu, and declares their parts and drop targets.
func (m *Model) blockView(d *ui.DrawContext) {
	canvas, theme := d.Canvas, d.Theme
	rect := d.Rect
	width := m.bodyLayout.width
	column := m.bodyLayout.column
	offsetY := rect.Y + bodyTopInset - m.scroll
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		y := offsetY + row.y
		if y+row.height < rect.Y || y > rect.Y+rect.Height {
			continue
		}
		top := y
		if first, ok := m.firstRowOfBlock(row.block); ok && first.y != row.y {
			top = offsetY + first.y
		}
		if row.block >= len(m.blocks) {
			continue
		}
		if row.block != m.hoverBlock && row.block != m.blockMenu.block {
			continue
		}
		handle := gfx.Rect{X: column - 24, Y: top + 2, Width: 18, Height: 18}
		if handle.X < rect.X+2 {
			continue
		}
		canvas.Rounded(handle, 4, theme.Hover)
		for dot := 0; dot < 3; dot++ {
			for columnDot := 0; columnDot < 2; columnDot++ {
				canvas.Fill(gfx.Rect{
					X: handle.X + 5 + float32(columnDot)*5, Y: handle.Y + 5 + float32(dot)*4,
					Width: 2, Height: 2,
				}, theme.MutedForeground)
			}
		}
		canvas.HitWithOptions(m.bodyInput.Parts().At(partGrip), handle, "块操作",
			gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true, Cursor: gfx.PointerCursorHand})
		d.DragSource(&m.blockDrag, m.bodyInput.Parts().At(partGrip), handle)
	}
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
				from, to := m.blockPart(source), m.blockPart(target)
				if from < 0 {
					from = m.dragBlock
				}
				if from < 0 || to < 0 || from == to {
					return false
				}
				if row, ok := m.firstRowOfBlock(to); ok {
					offsetY := m.bodyPane.Y + bodyTopInset - m.scroll
					middle := offsetY + row.y + m.blockHeight(to)/2
					m.dropAfter = m.blockDrag.At().Y > middle
				}
				return true
			},
			func(source, target gfx.ID) { m.dropBlock(source, target) })
	}
	if m.blockMenu.open {
		m.paintBlockMenu(canvas, theme, rect)
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

// paintBlockMenu paints the grip's menu.
func (m *Model) paintBlockMenu(canvas *gfx.Canvas, theme *ui.Theme, rect gfx.Rect) {
	rowHeight := float32(30)
	width := float32(180)
	height := float32(8) + rowHeight*float32(len(blockMenuItems))
	x := min(m.blockMenu.x, rect.X+rect.Width-width-8)
	y := m.blockMenu.y
	if y+height > rect.Y+rect.Height {
		y = m.blockMenu.y - height
	}
	canvas.Rounded(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 8, theme.Popover)
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 8, 1, theme.Border, gfx.Color{})
	for index, item := range blockMenuItems {
		row := gfx.Rect{X: x, Y: y + 4 + float32(index)*rowHeight, Width: width, Height: rowHeight}
		canvas.Label(item.label, row, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.PopoverForeground})
		canvas.HitWithOptions(m.bodyInput.Parts().At(partMenuBase+index), row, item.label,
			gfx.HitOptions{Role: gfx.RoleOption, SkipFocus: true})
	}
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
	canvas.Rounded(grip, 4, theme.Primary)
	canvas.HitWithOptions(m.bodyInput.Parts().At(partImageGrip), grip, "调整图片高度",
		gfx.HitOptions{Role: gfx.RoleSeparator, SkipFocus: true, Cursor: gfx.PointerCursorResizeVertical})
}

// blockAtPoint is the block under a window y.
func (m *Model) blockAtPoint(y float32) int {
	offsetY := m.bodyPane.Y + bodyTopInset - m.scroll
	local := y - offsetY
	block := -1
	for _, row := range m.bodyLayout.rows {
		if local >= row.y && local <= row.y+row.height {
			block = row.block
		}
	}
	return block
}

// blockMouse handles the grip, the block menu and the picture resize. It reports
// whether the press was taken.
func (m *Model) blockMouse(event gfx.PointerEvent) gfx.EventResult {
	if index, ok := event.Target.In(m.bodyInput.Parts()); ok {
		switch {
		case index >= partMenuBase:
			if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonPrimary {
				item := index - partMenuBase
				if item >= 0 && item < len(blockMenuItems) {
					m.chooseBlockMenu(blockMenuItems[item])
				}
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		case index == partGrip:
			if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonPrimary {
				m.dragBlock = m.blockAtPoint(event.Y)
				m.openBlockMenu(event)
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		case index == partImageGrip:
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

// openBlockMenu opens the grip's menu at the press.
func (m *Model) openBlockMenu(event gfx.PointerEvent) {
	block := m.blockAtPoint(event.Y)
	if block < 0 {
		return
	}
	m.blockMenu = blockMenuState{open: true, block: block, x: event.X + 14, y: event.Y + 4}
}

// chooseBlockMenu runs one command of the block menu.
func (m *Model) chooseBlockMenu(item slashItem) {
	block := m.blockMenu.block
	m.blockMenu = blockMenuState{block: -1}
	if block < 0 || block >= len(m.blocks) {
		return
	}
	if item.kind == "" {
		m.deleteBlock(block)
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

// dropBlock moves a dragged block above or below the block it was let go on.
func (m *Model) dropBlock(source, target gfx.ID) {
	m.blockMenu = blockMenuState{block: -1}
	from, to := m.blockPart(source), m.blockPart(target)
	if from < 0 {
		from = m.dragBlock
	}
	m.dragBlock = -1
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
