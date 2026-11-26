package notes

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"

	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
)

// The body editor. The platform's text service owns input; this file owns the
// block model, the layout that gives headings their size, the painting and the
// block semantics (Enter splits, Backspace merges, pictures hold no caret).
//
// The text lives in doc as the blocks joined by '\n'; blocks mirrors the lines
// one for one, so a structural edit is one text edit plus one block-list edit.
// Undo is a stack of whole-document snapshots, so it restores both together.

const (
	// bodyWidthShare is how much of the pane's width the body's text column
	// takes: the column is 80% of the pane at every width, centred in it, so the
	// text keeps its margins instead of running wide on a large window.
	bodyWidthShare float32 = 0.8
	// bodyTopInset is the gap between the band and the first block.
	bodyTopInset float32 = 14
	// bodyBottomSpace is the empty strip under the last block that a click
	// turns into a new block.
	bodyBottomSpace float32 = 140
	// blockGap separates two blocks: it is what a paragraph gets from the one
	// beside it, so prose reads as paragraphs and not as one run of lines.
	blockGap float32 = 8
	// imageGap separates a picture from its neighbours.
	imageGap float32 = 10
	// dividerHeight is the band a rule takes: the line sits in the middle of it,
	// with the air a section break needs above and below.
	dividerHeight float32 = 24
	// historyLimit is how many undo snapshots one note keeps while it is open.
	historyLimit = 200
	// historyGroup is how long consecutive keystrokes share one snapshot.
	historyGroup = time.Second
)

// bodyRow is one painted row of one block.
type bodyRow struct {
	block      int
	start, end int     // byte offsets inside the block's line
	y          float32 // top, from the start of the body content
	height     float32
}

// bodyLayout is the laid-out body at one width.
type bodyLayout struct {
	width  float32
	value  string
	rows   []bodyRow
	tops   []float32 // top of each block, from the start of the content
	height float32   // of the content, before the bottom space
	column float32   // x of the text column inside the pane
}

// snapshot is one undo step: the text, the block attributes beside it and the
// selection to restore.
type snapshot struct {
	text   string
	blocks []domain.NoteBlock
	sel    text.Range
}

// bodyCommand is one command of a block menu: what it turns the block into (or
// that it deletes it), with the icon and the chord both menus show for it. The
// chord is declared where the action is (see blockShortcuts), so the hint on the
// row is the shortcut that works, not a label of one.
type bodyCommand struct {
	label string
	kind  string
	level int
	icon  gfx.Icon
	// modifiers zero means the command has no chord of its own; hint is then what
	// the row shows at its trailing edge — the marks that are the command, as the
	// "---" that makes a divider is, or nothing at all.
	modifiers gfx.Modifiers
	key       string
	hint      string
}

// bodyCommands are the command menu's entries, in order: the block kinds a line can
// become, then the picture.
var bodyCommands = []bodyCommand{
	{label: "正文", kind: domain.NoteBlockText, icon: icons.Pilcrow,
		modifiers: ui.Primary | ui.Alt, key: "0"},
	{label: "标题 1", kind: domain.NoteBlockHeading, level: 1, icon: icons.Heading1,
		modifiers: ui.Primary | ui.Alt, key: "1"},
	{label: "标题 2", kind: domain.NoteBlockHeading, level: 2, icon: icons.Heading2,
		modifiers: ui.Primary | ui.Alt, key: "2"},
	{label: "标题 3", kind: domain.NoteBlockHeading, level: 3, icon: icons.Heading3,
		modifiers: ui.Primary | ui.Alt, key: "3"},
	{label: "标题 4", kind: domain.NoteBlockHeading, level: 4, icon: icons.Heading4,
		modifiers: ui.Primary | ui.Alt, key: "4"},
	{label: "标题 5", kind: domain.NoteBlockHeading, level: 5, icon: icons.Heading5,
		modifiers: ui.Primary | ui.Alt, key: "5"},
	{label: "图片", kind: domain.NoteBlockImage, icon: icons.Image,
		modifiers: ui.Primary | ui.Shift, key: "i"},
	{label: "分隔线", kind: domain.NoteBlockDivider, icon: icons.Minus, hint: "---"},
}

// slashState is the "/" command menu: it is open at a point of the body, for the
// block it will turn into the command the user chooses. The menu itself is a
// Viva context menu, so its keys, its rows and its closing are the framework's.
type slashState struct {
	open  bool
	block int
	at    gfx.Point
	// focus asks the paint that opens the menu to put the keyboard in its first
	// row, so typing and Enter belong to the menu rather than the note.
	focus bool
}

// bodyView declares the body: its column of blocks — the text control and the
// grip of the block it shows — and the command menu that hangs off the text. A menu
// is the framework's, so its panel, rows, hover, clicks and keys are the app's
// everywhere else too.
func (m *Model) bodyView(c ui.C) {
	if m.selected == "" {
		m.area = nil
		return
	}
	// A drag keeps the grip it took hold of: the source of a drag must be
	// declared every paint, or the framework cancels the drag.
	if m.blockDrag.Dragging() {
		if m.dragBlock < 0 {
			m.dragBlock = m.gripBlock
		}
	} else {
		m.dragBlock = -1
	}
	// The note owns its offset: the wheel (Scroll), the caret and a drag on the
	// bar write it, and the area places the blocks at it in this same paint. The
	// caret can only be brought into view once the blocks are laid out, so the
	// body is laid out here first, at the width it had; PlaceWidth lays it out
	// again at the width the pane of this paint gets, and the cache makes that a
	// lookup when the width did not change.
	if area := m.area; area != nil {
		if area.Dragging() {
			m.scroll = area.Offset
		}
		if m.canvas != nil && m.bodyPane.Width > 0 {
			width, _ := bodyColumn(m.bodyPane.Width)
			m.layoutBody(m.canvas, width, m.theme)
			m.followCaret(area)
		}
	}
	m.column.m, m.column.block = m, m.litBlock()
	m.gripID = gfx.ID{}
	m.gripHover = -1
	var body ui.Box
	// The note is a document taller than its pane: the area clips it at the
	// offset the note gives, draws its scrollbar and hands the wheel back to the
	// page. Its name carries the note, so another note opens at its first line
	// rather than where the last one was left.
	c.Key("note-body-" + m.selected)
	area := ui.ScrollAt(c, m.scroll).Grow(1).FillWidth().Scrollbars().ScrollbarLabels("笔记滚动条", "")
	m.area = area.ScrollState()
	area.Children(func() {
		ui.CustomLayout(c, &m.column).FillWidth().Children(func() {
			body = ui.Control(c, "笔记正文", &m.bodyInput).Draw(m.bodyDraw)
			if m.column.block >= 0 {
				m.blockGrip(c, m.column.block)
			}
		})
	})
	m.gripBlock = m.column.block
	m.slashMenu(c, body)
}

// headingStep is the type of one heading level: its size and weight, and the
// space the heading takes above itself.
//
// The five levels sit on the application's own steps — 24, 20, 16, 14 and 12 —
// so a note never shows a size the rest of the application does not have. The
// last two share the body and the caption step and are told apart by weight,
// which is how a document's deeper headings stand: a markdown H4 is a bold body
// line, not a size between two steps. Weight is Bold at every level because
// Chinese has no real semibold — 500 and 600 resolve to the regular face — so a
// level is carried by size, then by the space above it.
type headingStep struct {
	size, space float32
	weight      float32
}

// headingSteps is the ladder, one entry per level the block menu offers.
var headingSteps = [5]headingStep{
	{size: typography.SizeMetric, space: 24, weight: gfx.WeightBold},
	{size: typography.SizeTitle, space: 20, weight: gfx.WeightBold},
	{size: typography.SizeSection, space: 16, weight: gfx.WeightBold},
	{size: typography.SizeBody, space: 12, weight: gfx.WeightBold},
	{size: typography.SizeCaption, space: 10, weight: gfx.WeightBold},
}

// headingOf is the step of a block's heading level, clamped to the five levels.
func headingOf(block domain.NoteBlock) headingStep {
	return headingSteps[domain.NoteHeadingLevel(block.Level)-1]
}

// style is the text style of one block.
func (m *Model) style(theme *ui.Theme, block domain.NoteBlock) gfx.TextStyle {
	if block.Kind == domain.NoteBlockHeading {
		step := headingOf(block)
		return gfx.TextStyle{Size: step.size, Weight: step.weight, Color: theme.Foreground}
	}
	return gfx.TextStyle{Size: typography.SizeBody, Weight: gfx.WeightRegular, Color: theme.Foreground}
}

// takesText reports whether a block kind holds words: the two kinds whose line is
// painted text. A picture and a rule own a line of the document but not its
// letters, so the caret never stops on them.
func takesText(kind string) bool {
	return kind == domain.NoteBlockText || kind == domain.NoteBlockHeading
}

// lineHeight is the row height of a line of type at size: Viva's own leading,
// seven pixels over the size, which every box in the application inherits (12 →
// 19, 14 → 21, 16 → 23, 20 → 27, 24 → 31). The body's rows are the one place
// that has to compute it, because they are painted on the canvas instead of laid
// out by a box; every other page leaves its leading to the framework.
func lineHeight(size float32) float32 { return size + 7 }

// layoutBody lays the blocks out at width, rebuilding only when the text or the
// width changed.
func (m *Model) layoutBody(canvas *gfx.Canvas, width float32, theme *ui.Theme) {
	value := m.doc.String()
	if m.bodyLayout.value == value && m.bodyLayout.width == width {
		return
	}
	m.syncLines()
	layout := bodyLayout{width: width, value: value}
	y := bodyTopInset
	for index := range m.blocks {
		block := m.blocks[index]
		top := y + blockGap
		layout.tops = append(layout.tops, top)
		if block.Kind == domain.NoteBlockDivider {
			layout.rows = append(layout.rows, bodyRow{block: index, y: top, height: dividerHeight})
			y = top + dividerHeight + blockGap
			continue
		}
		if block.Kind == domain.NoteBlockImage {
			_, rawHeight := domain.NoteImageSize(block.ImageWidth, block.ImageHeight, block.ImageScale, float64(width))
			height := float32(rawHeight)
			if height <= 0 {
				height = 120
			}
			layout.rows = append(layout.rows, bodyRow{block: index, y: top, height: height})
			y = top + height + imageGap
			continue
		}
		style := m.style(theme, block)
		height := lineHeight(style.Size)
		if block.Kind == domain.NoteBlockHeading {
			top += headingOf(block).space
		}
		line := ""
		if index < len(m.lines) {
			line = m.lines[index]
		}
		from := 0
		for {
			row, next := canvas.WrapRow(line, from, style.Size, style.Weight, width)
			layout.rows = append(layout.rows, bodyRow{
				block: index, start: row.Start, end: row.End, y: top, height: height,
			})
			top += height
			if next < 0 {
				break
			}
			from = next
		}
		y = top + blockGap
	}
	layout.height = y
	m.bodyLayout = layout
}

// bodyColumn is the width the body's text wraps at, and the offset of the text
// column inside a pane of the given width: the column keeps bodyWidthShare of the
// pane, centred in it.
func bodyColumn(paneWidth float32) (width, offset float32) {
	width = paneWidth * bodyWidthShare
	return width, (paneWidth - width) / 2
}

// bodyOffset is the y of the body's painted content in its rectangle: the strip
// above the first block. A row's own y is added to it. The rectangle is the one
// the scroll area placed, so the offset is already in it.
func bodyOffset(pane gfx.Rect) float32 {
	return pane.Y + bodyTopInset
}

// bodyHeight is how tall the body's content is: the strip above the first block,
// the blocks, and the empty strip under the last one that a click turns into a
// new block.
func (m *Model) bodyHeight() float32 {
	return bodyTopInset + m.bodyLayout.height + bodyBottomSpace
}

// drawBody paints the body and declares its text hit.
func (m *Model) drawBody(d *ui.DrawContext) {
	if m.selected == "" || d.Canvas == nil {
		return
	}
	m.bodyKey = d.Node.ID
	m.bodyPane = d.Rect
	m.canvas = d.Canvas
	m.theme = d.Theme
	canvas, theme, rect := d.Canvas, d.Theme, d.Rect
	// The body's column is the page's column: the right column already carries
	// the page inset, so the text starts on the title's left edge.
	width, offset := bodyColumn(rect.Width)
	m.layoutBody(canvas, width, theme)
	column := rect.X + offset
	m.bodyLayout.column = column
	focused := d.State.Focused
	offsetY := bodyOffset(rect)

	canvas.Clip(rect)
	m.paintBlockWash(canvas, theme, column, offsetY, width)
	m.paintImages(canvas, theme, rect, column, offsetY)
	m.paintDividers(canvas, theme, rect, column, offsetY, width)
	m.paintText(canvas, theme, rect, column, offsetY, width)
	m.paintCaret(canvas, theme, column, offsetY, focused)
	canvas.Unclip()

	if !m.bodyKey.IsZero() {
		canvas.HitWithOptions(m.bodyKey, rect, "笔记正文", gfx.HitOptions{
			Role: gfx.RoleTextField, Cursor: gfx.PointerCursorText, Multiline: true,
			Value: m.doc.String(), TextBounds: rect, TextSize: theme.Metrics.FontSizeM,
			TextColor: gfx.Color{}, HideNativeOverlays: true, Placement: m,
		})
	}
	m.blockDrops(d)
}

// paintText paints the text rows, the selection and the input method's
// composition underline.
func (m *Model) paintText(canvas *gfx.Canvas, theme *ui.Theme, rect gfx.Rect,
	column, offsetY, width float32) {
	selection := m.doc.Selection()
	selStart, selEnd := m.doc.ByteOffset(selection.Start), m.doc.ByteOffset(selection.End)
	if selStart > selEnd {
		selStart, selEnd = selEnd, selStart
	}
	outline := theme.Primary.WithAlpha(0.22)
	compositionStart, compositionEnd := 0, 0
	if composition, ok := m.doc.Composition(); ok {
		compositionStart, compositionEnd = m.doc.ByteOffset(composition.Start), m.doc.ByteOffset(composition.End)
	}
	lineStarts := m.lineStarts()
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		y := offsetY + row.y
		if y+row.height < rect.Y || y > rect.Y+rect.Height {
			continue
		}
		block := m.blocks[row.block]
		if !takesText(block.Kind) {
			continue
		}
		line := ""
		if row.block < len(m.lines) {
			line = m.lines[row.block]
		}
		rowText := ""
		if row.start < len(line) {
			rowText = line[row.start:min(row.end, len(line))]
		}
		global := lineStarts[row.block] + row.start
		rowEnd := lineStarts[row.block] + row.end
		style := m.style(theme, block)
		if selEnd > selStart && selEnd > global && selStart < rowEnd {
			start := max(selStart, global) - global
			end := min(selEnd, rowEnd) - global
			for _, span := range canvas.TextSelectionStyle(nil, rowText, style, start, end) {
				canvas.Fill(gfx.Rect{X: column + span.X, Y: y, Width: span.Width, Height: row.height}, outline)
			}
		}
		if rowText != "" {
			canvas.Label(rowText, gfx.Rect{X: column, Y: y, Width: width, Height: row.height}, style)
		}
		if compositionEnd > compositionStart && compositionEnd > global && compositionStart < rowEnd {
			start := max(compositionStart, global) - global
			end := min(compositionEnd, rowEnd) - global
			if end > start {
				x := canvas.TextCaretStyle(rowText, style, start)
				right := canvas.TextCaretStyle(rowText, style, end)
				canvas.Fill(gfx.Rect{X: column + x, Y: y + row.height - 2, Width: right - x, Height: 1}, theme.Foreground)
			}
		}
	}
}

// paintCaret draws the caret when the control has the keyboard and the
// selection is empty.
func (m *Model) paintCaret(canvas *gfx.Canvas, theme *ui.Theme, column, offsetY float32, focused bool) {
	if !focused {
		return
	}
	rect, ok := m.CaretBounds()
	if ok {
		canvas.Fill(rect, theme.Foreground)
	}
	_ = column
	_ = offsetY
}

// paintImages paints every picture block in view, with a placeholder until the
// bitmap arrives.
func (m *Model) paintImages(canvas *gfx.Canvas, theme *ui.Theme, rect gfx.Rect,
	column, offsetY float32) {
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		block := m.blocks[row.block]
		if block.Kind != domain.NoteBlockImage {
			continue
		}
		y := offsetY + row.y
		if y+row.height < rect.Y || y > rect.Y+rect.Height {
			continue
		}
		width := m.bodyLayout.width
		rawWidth, rawHeight := domain.NoteImageSize(block.ImageWidth, block.ImageHeight, block.ImageScale, float64(width))
		w, h := float32(rawWidth), float32(rawHeight)
		if w <= 0 || h <= 0 {
			w, h = width, 120
		}
		target := gfx.Rect{X: column + (width-w)/2, Y: y, Width: w, Height: h}
		bitmap, ok := m.noteImages[block.ImagePath]
		if !ok || bitmap.Width == 0 {
			canvas.Fill(target, theme.Muted)
			continue
		}
		canvas.DrawBitmap(bitmap, target)
		if block.ID != "" && block.ID == m.selectedBlockID() {
			// The frame is the picture's own edge: square, like the bitmap, so
			// the page never rounds a corner the picture has.
			canvas.Border(target, 2, theme.Primary, gfx.Color{})
		}
	}
}

// paintDividers paints every rule in view: one hairline across the text column,
// the line ui.Divider draws in the rest of the application, centred in its band.
func (m *Model) paintDividers(canvas *gfx.Canvas, theme *ui.Theme, rect gfx.Rect,
	column, offsetY, width float32) {
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		if row.block >= len(m.blocks) || m.blocks[row.block].Kind != domain.NoteBlockDivider {
			continue
		}
		y := offsetY + row.y
		if y+row.height < rect.Y || y > rect.Y+rect.Height {
			continue
		}
		// The rule is one pixel tall and sits on a whole pixel: half a pixel
		// would spread it over two rows at half strength.
		top := float32(math.Round(float64(y + row.height/2 - 0.5)))
		canvas.Fill(gfx.Rect{X: column, Y: top, Width: width, Height: 1}, theme.Border)
	}
}

// selectedBlockID is the picture block the user clicked, if any.
func (m *Model) selectedBlockID() string {
	if m.selectedImage < 0 || m.selectedImage >= len(m.blocks) {
		return ""
	}
	return m.blocks[m.selectedImage].ID
}

// followCaret moves the offset so the caret stays in sight, when the caret moved
// since the last paint. A wheel, a scrollbar drag or a block drag moves the area
// without moving the caret, and that offset is the user's: a caret that stayed
// where it was must not pull the view back.
func (m *Model) followCaret(area *ui.ScrollState) {
	caret := m.doc.ByteOffset(m.doc.Caret())
	if area.Viewport <= 0 || (caret == m.caretShown && area.Viewport == m.caretViewport) {
		return
	}
	m.caretShown, m.caretViewport = caret, area.Viewport
	block, inLine := m.position(caret)
	if block < 0 {
		return
	}
	row, ok := m.rowOf(block, inLine)
	if !ok {
		return
	}
	top := bodyTopInset + row.y
	bottom := top + row.height
	if top < m.scroll {
		m.scroll = top
	}
	if bottom > m.scroll+area.Viewport {
		m.scroll = bottom - area.Viewport
	}
	if max := area.Max(); m.scroll > max {
		m.scroll = max
	}
}

// Scroll handles the wheel over the note. The body is a scroll area whose offset
// the note owns, so the area hands the wheel back here; the area's own stepping
// and range are used, and the page keeps the number.
func (m *Model) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	if m.selected == "" || (event.Target != m.bodyKey && event.Target != m.gripID) {
		return gfx.EventResult{}
	}
	area := m.area
	if area == nil {
		return gfx.EventResult{}
	}
	area.Offset = m.scroll
	result := area.Wheel(event)
	m.scroll = area.Offset
	return result
}

// lineStarts is the byte offset of each block's line inside doc.
func (m *Model) lineStarts() []int {
	starts := m.lineStartsCache[:0]
	offset := 0
	for index, line := range m.lines {
		starts = append(starts, offset)
		offset += len(line)
		if index < len(m.lines)-1 {
			offset++
		}
	}
	m.lineStartsCache = starts
	return starts
}

// position turns a byte offset into a block and an offset inside its line.
func (m *Model) position(offset int) (int, int) {
	lineStarts := m.lineStarts()
	block := 0
	for index := range lineStarts {
		if offset >= lineStarts[index] {
			block = index
		}
	}
	inLine := offset
	if block < len(lineStarts) {
		inLine = offset - lineStarts[block]
	}
	line := ""
	if block < len(m.lines) {
		line = m.lines[block]
	}
	return block, max(0, min(inLine, len(line)))
}

// blockStart is the byte offset of a block's first character.
func (m *Model) blockStart(block int) int {
	if block <= 0 {
		return 0
	}
	starts := m.lineStarts()
	if block < len(starts) {
		return starts[block]
	}
	return len(m.doc.String())
}

// rowOf finds the painted row of a block that holds an offset inside its line.
func (m *Model) rowOf(block, inLine int) (bodyRow, bool) {
	var last bodyRow
	found := false
	for _, row := range m.bodyLayout.rows {
		if row.block != block {
			continue
		}
		if row.start <= inLine && inLine <= row.end {
			return row, true
		}
		last, found = row, true
	}
	return last, found
}

// state is the editor's native text state.
func (m *Model) state() window.TextState {
	sel := m.doc.Selection()
	state := window.TextState{
		Value:     m.doc.String(),
		Selection: window.TextRange{Start: sel.Start, End: sel.End},
	}
	if composition, ok := m.doc.Composition(); ok {
		state.Composition = window.TextRange{Start: composition.Start, End: composition.End}
		state.Composing = true
	}
	return state
}

// bodyText gives the platform the editor while the body has the keyboard.
func (m *Model) bodyText(context window.TextInputContext) *window.TextInput {
	if m.selected == "" || context.ReadOnly {
		return nil
	}
	input := &window.TextInput{}
	input.State = m.state()
	input.NavigationKeys = context.NavigationKeys
	input.OnEvent = m.bodyEvent
	return input
}

// bodyEvent routes one native text event into the block model.
func (m *Model) bodyEvent(event window.TextEvent) (window.TextState, error) {
	if m.selected == "" {
		return window.TextState{}, nil
	}
	if m.doc.CompositionActive() {
		// The input method owns the text until it commits.
		event.Apply(m.doc)
		m.afterEdit()
		return m.state(), nil
	}
	// A picture and a rule hold no words: a keystroke aimed at one goes to the
	// nearest text block first, so their line never keeps a letter.
	m.moveCaretToText()
	switch event.Kind {
	case window.TextPasteNoText:
		m.pastePicture()
		return m.state(), nil
	case window.TextPaste:
		m.pasteText(event)
		return m.state(), nil
	case window.TextInsert:
		if m.dividerMarks(event.Text) {
			return m.state(), nil
		}
		if slashTrigger(event.Text) && m.slashStart() {
			m.openSlash()
			return m.state(), nil
		}
		m.pushHistory()
		m.applyEvent(event, m.selectionStart())
		m.afterEdit()
		return m.state(), nil
	case window.TextSetSelection, window.TextSetMarked, window.TextUnmark, window.TextCancelComposition:
		event.Apply(m.doc)
		m.linesDirty = true
		return m.state(), nil
	case window.TextCommandEvent:
		switch event.Command {
		case window.TextUndo:
			m.undo()
			return m.state(), nil
		case window.TextRedo:
			return m.state(), nil
		case window.TextInsertNewline, window.TextInsertLineBreak:
			m.newHistoryStep()
			m.applyEvent(event, m.selectionStart())
			m.afterEdit()
			return m.state(), nil
		case window.TextDeleteBackward:
			start := m.backspaceStart()
			if start < 0 || start >= len(m.blocks) || !takesText(m.blocks[start].Kind) {
				// A picture's or a rule's line cannot take the words under it: a
				// backspace there would join them into a line nothing paints.
				return m.state(), nil
			}
			m.newHistoryStep()
			m.applyEvent(event, start)
			m.afterEdit()
			return m.state(), nil
		case window.TextDeleteForward:
			m.newHistoryStep()
			m.applyEvent(event, m.selectionStart())
			m.afterEdit()
			return m.state(), nil
		default:
			event.Apply(m.doc)
			m.moveCaretToText()
			m.linesDirty = true
			return m.state(), nil
		}
	}
	event.Apply(m.doc)
	return m.state(), nil
}

// applyEvent applies one text event and keeps the block list in step: extra
// lines add text blocks after startBlock, removed lines drop the blocks the join
// swallowed.
func (m *Model) applyEvent(event window.TextEvent, startBlock int) {
	before := m.lineCount()
	event.Apply(m.doc)
	m.syncLines()
	delta := len(m.lines) - before
	if delta > 0 {
		for count := 0; count < delta; count++ {
			m.insertBlock(startBlock+1+count, domain.NoteBlock{Kind: domain.NoteBlockText})
		}
	}
	if delta < 0 {
		remove := min(-delta, len(m.blocks)-startBlock-1)
		for count := 0; count < remove; count++ {
			m.removeBlock(startBlock + 1)
		}
	}
	m.moveCaretToText()
}

// backspaceStart is the block a backspace joins into: the one holding the
// character before the caret.
func (m *Model) backspaceStart() int {
	selection := m.doc.Selection()
	if selection.Start != selection.End {
		return m.selectionStart()
	}
	offset := m.doc.ByteOffset(m.doc.Caret())
	if offset <= 0 {
		return 0
	}
	block, _ := m.position(offset - 1)
	return block
}

// selectionStart is the block the selection begins in.
func (m *Model) selectionStart() int {
	selection := m.doc.Selection()
	start := min(selection.Start, selection.End)
	block, _ := m.position(m.doc.ByteOffset(start))
	return block
}

// lineCount is the number of lines in doc.
func (m *Model) lineCount() int { return strings.Count(m.doc.String(), "\n") + 1 }

// insertBlock adds a block at an index.
func (m *Model) insertBlock(index int, block domain.NoteBlock) {
	index = max(0, min(index, len(m.blocks)))
	blocks := make([]domain.NoteBlock, 0, len(m.blocks)+1)
	blocks = append(blocks, m.blocks[:index]...)
	blocks = append(blocks, block)
	blocks = append(blocks, m.blocks[index:]...)
	m.blocks = blocks
}

// removeBlock drops one block. Its line was already joined away by the event.
func (m *Model) removeBlock(index int) {
	if index < 0 || index >= len(m.blocks) {
		return
	}
	m.blocks = append(m.blocks[:index], m.blocks[index+1:]...)
}

// pasteText inserts pasted text, splitting the block on its line breaks.
func (m *Model) pasteText(event window.TextEvent) {
	m.newHistoryStep()
	m.applyEvent(event, m.selectionStart())
	m.afterEdit()
}

// afterEdit records the change and moves the caret off a picture.
func (m *Model) afterEdit() {
	m.moveCaretToText()
	m.syncLines()
	m.linesDirty = false
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.markEdited(m.now)
}

// moveCaretToText puts the caret in the nearest block that takes text: a picture
// and a rule own a line of the document but not its letters, so the caret never
// stops on them.
func (m *Model) moveCaretToText() {
	for passes := 0; passes < len(m.blocks)+1; passes++ {
		block, _ := m.position(m.doc.ByteOffset(m.doc.Caret()))
		if block < 0 || block >= len(m.blocks) || takesText(m.blocks[block].Kind) {
			return
		}
		if block+1 < len(m.blocks) {
			m.doc.MoveTo(m.bytesToRune(m.blockStart(block+1)), false)
			continue
		}
		if block > 0 {
			m.doc.MoveTo(m.bytesToRune(m.blockStart(block)-1), false)
			continue
		}
		return
	}
}

// bytesToRune converts a byte offset into a rune index.
func (m *Model) bytesToRune(offset int) int {
	value := m.doc.String()
	return utf8.RuneCountInString(value[:max(0, min(offset, len(value)))])
}

// pastePicture inserts the clipboard's picture as a block.
func (m *Model) pastePicture() {
	if m.repo == nil || m.cmds.PasteImage == nil {
		return
	}
	path, width, height, err := m.cmds.PasteImage(m.selected)
	if err != nil || path == "" {
		return
	}
	m.insertPicture(domain.NoteBlock{
		Kind: domain.NoteBlockImage, ImagePath: path, ImageWidth: width,
		ImageHeight: height, ImageScale: domain.NoteImageDefaultScale,
	})
}

// insertPicture puts a picture block at the caret and remembers that the note's
// pictures changed, so the effect that decodes them runs again.
func (m *Model) insertPicture(block domain.NoteBlock) {
	if m.selected == "" {
		return
	}
	m.newHistoryStep()
	m.insertColdBlock(block)
	m.imageRev++
	m.imageKey = noteImageKey{note: m.selected, rev: m.imageRev}
}

// insertDivider puts a rule at the caret: it takes the empty block's place, or a
// line of its own under the text, and a text block follows it.
func (m *Model) insertDivider() {
	if m.selected == "" {
		return
	}
	m.newHistoryStep()
	m.insertColdBlock(domain.NoteBlock{Kind: domain.NoteBlockDivider})
}

// dividerMarks turns the "--" that a third dash completes into a divider: the
// marks are the command, so they leave the text and the block becomes the rule,
// with a text line under it where the next word goes. Undo brings the dashes back.
func (m *Model) dividerMarks(typed string) bool {
	if typed != "-" {
		return false
	}
	block, inLine := m.position(m.doc.ByteOffset(m.doc.Caret()))
	if block < 0 || block >= len(m.blocks) || block >= len(m.lines) ||
		m.blocks[block].Kind != domain.NoteBlockText {
		return false
	}
	line := m.lines[block]
	if line != "--" || inLine != len(line) {
		return false
	}
	m.newHistoryStep()
	m.doc.ReplaceRange(text.Range{
		Start: m.bytesToRune(m.blockStart(block)),
		End:   m.bytesToRune(m.blockStart(block) + len(line)),
	}, "")
	m.syncLines()
	m.insertColdBlock(domain.NoteBlock{Kind: domain.NoteBlockDivider})
	return true
}

// insertColdBlock puts a block that holds no words — a picture or a rule — at the
// caret: it takes the empty block's place, or a line of its own under the text,
// and a text block always follows it, so the caret never stops on it. The caller
// records the history step, so one gesture stays one undo.
func (m *Model) insertColdBlock(block domain.NoteBlock) {
	if m.selected == "" {
		return
	}
	m.syncLines()
	offset := m.doc.ByteOffset(m.doc.Caret())
	current, inLine := m.position(offset)
	if current < 0 || current >= len(m.blocks) {
		return
	}
	line := ""
	if current < len(m.lines) {
		line = m.lines[current]
	}
	empty := inLine == 0 && strings.TrimSpace(line) == "" && takesText(m.blocks[current].Kind)
	// The picture or the rule occupies a line of its own and an empty text block
	// follows it, so the caret and the next keystroke never land on it.
	end := m.blockStart(current) + len(line)
	if empty {
		m.blocks[current] = block
		m.doc.MoveTo(m.bytesToRune(end), false)
		m.doc.Insert("\n")
		m.insertBlock(current+1, domain.NoteBlock{Kind: domain.NoteBlockText})
		m.doc.MoveTo(m.bytesToRune(m.blockStart(current+1)), false)
	} else {
		m.doc.MoveTo(m.bytesToRune(end), false)
		m.doc.Insert("\n\n")
		m.insertBlock(current+1, block)
		m.insertBlock(current+2, domain.NoteBlock{Kind: domain.NoteBlockText})
		m.doc.MoveTo(m.bytesToRune(m.blockStart(current+2)), false)
	}
	m.syncLines()
	m.linesDirty = false
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.markEdited(m.now)
	m.focusBody = true
	m.flush()
}

// pushHistory keeps the state before a keystroke. Consecutive keystrokes share
// one snapshot.
func (m *Model) pushHistory() {
	if m.histOpen && m.now-m.histAt < historyGroup {
		m.histAt = m.now
		return
	}
	m.pushSnapshot()
	m.histOpen = true
}

// pushSnapshot records the state unconditionally.
func (m *Model) pushSnapshot() {
	if len(m.history) >= historyLimit {
		m.history = m.history[1:]
	}
	blocks := make([]domain.NoteBlock, len(m.blocks))
	copy(blocks, m.blocks)
	m.history = append(m.history, snapshot{text: m.doc.String(), blocks: blocks, sel: m.doc.Selection()})
	m.histAt = m.now
}

// newHistoryStep pushes a snapshot and ends the coalescing group, so a
// structural edit is always its own undo step.
func (m *Model) newHistoryStep() {
	m.pushSnapshot()
	m.histOpen = false
}

// undo restores the last snapshot; the whole document, attributes included.
func (m *Model) undo() {
	if len(m.history) == 0 {
		return
	}
	snap := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.histOpen = false
	m.doc = text.NewEditor(snap.text)
	m.doc.SetSelection(snap.sel)
	m.blocks = make([]domain.NoteBlock, len(snap.blocks))
	copy(m.blocks, snap.blocks)
	m.lines = strings.Split(snap.text, "\n")
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.linesDirty = false
	m.markEdited(m.now)
	m.flush()
}

// slashTrigger reports whether the text just typed asks for the block menu: the
// ASCII slash, and the 顿号 "、" that Chinese prose puts between items. Someone
// writing 中文 reaches for 顿号 without leaving the input method, so both open the
// menu; neither mark is inserted, and both count only at the start of an empty
// block (see slashStart).
func slashTrigger(text string) bool { return text == "/" || text == "、" }

// slashStart reports whether a slash typed now should open the block menu: the
// block is empty text and the caret is at its start.
func (m *Model) slashStart() bool {
	offset := m.doc.ByteOffset(m.doc.Caret())
	block, inLine := m.position(offset)
	if block < 0 || block >= len(m.blocks) || !takesText(m.blocks[block].Kind) {
		return false
	}
	if block >= len(m.lines) {
		return false
	}
	return inLine == 0 && strings.TrimSpace(m.lines[block]) == ""
}

// openSlash opens the "/" menu under the caret, for the block the caret is in.
func (m *Model) openSlash() {
	offset := m.doc.ByteOffset(m.doc.Caret())
	block, _ := m.position(offset)
	at := gfx.Point{}
	if rect, ok := m.CaretBounds(); ok {
		at = gfx.Point{X: rect.X, Y: rect.Y + rect.Height + 2}
	}
	m.slash = slashState{open: true, block: block, at: at, focus: true}
}

// closeSlash closes the menu.
func (m *Model) closeSlash() { m.slash = slashState{} }

// slashMenu declares the command menu as a Viva context menu at the caret. It is the
// same table the grip opens, raised by typing "/" or "、" at the start of a block.
// It is declared after the body, so the body is the control the menu hangs off
// and the press that chooses a row lands in the menu rather than the text.
func (m *Model) slashMenu(c ui.C, body ui.Box) {
	if !m.slash.open {
		return
	}
	var first gfx.ID
	ui.ContextMenuAt(c, body, &m.slash.open, m.slash.at, func(items *ui.MenuItems) {
		for _, command := range bodyCommands {
			entry := shortcutHint(items.Item(command.label).Icon(command.icon), command)
			if first.IsZero() {
				first = entry.ID()
			}
			if entry.Clicked() {
				m.chooseSlash(command)
			}
		}
	})
	if m.slash.focus && !first.IsZero() {
		c.Canvas().RequestFocus(first)
		m.slash.focus = false
	}
}

// chooseSlash turns the block the menu was opened on into the chosen kind.
func (m *Model) chooseSlash(item bodyCommand) {
	block := m.slash.block
	m.closeSlash()
	if item.kind == domain.NoteBlockImage {
		m.pickPicture()
		return
	}
	if item.kind == domain.NoteBlockDivider {
		m.insertDivider()
		return
	}
	if block < 0 || block >= len(m.blocks) {
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

// bodyPointer handles presses on pictures, on the command menu and on the strip
// under the last block; text presses go to the placement.
func (m *Model) bodyPointer(event gfx.PointerEvent) gfx.EventResult {
	if m.selected == "" {
		return gfx.EventResult{}
	}
	if result := m.blockMouse(event); result.Handled {
		return result
	}
	if event.Kind == gfx.PointerMove {
		// The grip of the block under the pointer shows and hides with it, so a
		// move that changes the block is a reason to paint.
		block := m.blockAtPoint(event.Y)
		if block == m.hoverBlock {
			return gfx.EventResult{}
		}
		m.hoverBlock = block
		return gfx.EventResult{Repaint: true}
	}
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	if m.slash.open {
		if index, ok := event.Target.In(m.bodyInput.Parts()); ok && index < len(bodyCommands) {
			// The release's click pass chooses the item; the press keeps the
			// menu open for it.
			return gfx.EventResult{}
		}
		m.closeSlash()
	}
	if m.bodyKey.IsZero() || event.Target != m.bodyKey {
		return gfx.EventResult{}
	}
	offsetY := bodyOffset(m.bodyPane)
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		if row.block >= len(m.blocks) {
			continue
		}
		y := offsetY + row.y
		if event.Y < y || event.Y > y+row.height {
			continue
		}
		switch m.blocks[row.block].Kind {
		case domain.NoteBlockImage:
			if m.selectedImage == row.block {
				block := m.blocks[row.block]
				m.openLightbox(block.ImagePath, block.ImageWidth, block.ImageHeight)
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			m.selectedImage = row.block
			return gfx.EventResult{Handled: true, Repaint: true}
		case domain.NoteBlockDivider:
			// A rule holds no caret: a press on it puts the caret in the text
			// line under it, which is where the next word would go.
			m.selectedImage = -1
			m.caretAfterDivider(row.block)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if event.Y > offsetY+m.bodyLayout.height {
		m.appendBlockAtEnd()
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.selectedImage = -1
	return gfx.EventResult{}
}

// caretAfterDivider puts the caret in the text line under a rule, or opens one
// when the rule is the last block: a press on a rule means the words after it.
func (m *Model) caretAfterDivider(block int) {
	if block+1 < len(m.blocks) {
		m.doc.MoveTo(m.bytesToRune(m.blockStart(block+1)), false)
		m.moveCaretToText()
		return
	}
	m.appendBlockAtEnd()
}

// appendBlockAtEnd adds or focuses the empty block under the last block.
func (m *Model) appendBlockAtEnd() {
	m.newHistoryStep()
	last := len(m.blocks) - 1
	if last >= 0 && last < len(m.lines) && m.blocks[last].Kind == domain.NoteBlockText &&
		strings.TrimSpace(m.lines[last]) == "" {
		m.doc.MoveTo(m.bytesToRune(m.blockStart(last)+len(m.lines[last])), false)
		return
	}
	m.doc.MoveTo(m.bytesToRune(len(m.doc.String())), false)
	m.doc.Insert("\n")
	m.insertBlock(len(m.blocks), domain.NoteBlock{Kind: domain.NoteBlockText})
	m.afterEdit()
	m.flush()
}

// bodyKeyEvent handles the keys the platform leaves to the application: the
// visual movements, Escape and Tab.
func (m *Model) bodyKeyEvent(event gfx.KeyEvent) gfx.EventResult {
	if m.selected == "" || event.Target != m.bodyKey {
		return gfx.EventResult{}
	}
	if m.slash.open {
		if event.Key == gfx.KeyEscape {
			m.closeSlash()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	if m.doc.CompositionActive() {
		return gfx.EventResult{}
	}
	extend := event.Modifiers&gfx.ModifierShift != 0
	switch event.Key {
	case gfx.KeyUp:
		m.moveVertical(-1, extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyDown:
		m.moveVertical(1, extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyHome:
		m.moveLine(false, extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEnd:
		m.moveLine(true, extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyPageUp:
		m.movePage(-1, extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyPageDown:
		m.movePage(1, extend)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// moveVertical moves the caret one painted row up or down, skipping pictures
// and keeping the horizontal position.
func (m *Model) moveVertical(direction int, extend bool) {
	block, inLine := m.position(m.doc.ByteOffset(m.doc.Caret()))
	row, ok := m.rowOf(block, inLine)
	if !ok {
		return
	}
	column := m.caretColumn(row, inLine)
	for passes := 0; passes < len(m.bodyLayout.rows); passes++ {
		var next bodyRow
		block, next, ok = m.neighbourRow(block, row, direction)
		if !ok {
			return
		}
		row = next
		if takesText(m.blocks[block].Kind) {
			break
		}
	}
	m.placeInRow(block, row, column, extend)
}

// neighbourRow is the row above or below one.
func (m *Model) neighbourRow(block int, row bodyRow, direction int) (int, bodyRow, bool) {
	for index := range m.bodyLayout.rows {
		candidate := m.bodyLayout.rows[index]
		if candidate.block == block && candidate.start == row.start && candidate.y == row.y {
			next := index + direction
			if next < 0 || next >= len(m.bodyLayout.rows) {
				return 0, bodyRow{}, false
			}
			target := m.bodyLayout.rows[next]
			return target.block, target, true
		}
	}
	return 0, bodyRow{}, false
}

// caretColumn is the horizontal position of the caret inside its row.
func (m *Model) caretColumn(row bodyRow, inLine int) float32 {
	if m.canvas == nil {
		return 0
	}
	line := m.lines[row.block]
	style := m.style(m.theme, m.blocks[row.block])
	text := line[:max(0, min(inLine, len(line)))]
	return m.canvas.TextCaretStyle(text, style, len(text))
}

// placeInRow puts the caret at the column inside a row.
func (m *Model) placeInRow(block int, row bodyRow, column float32, extend bool) {
	if m.canvas == nil {
		return
	}
	line := m.lines[block]
	style := m.style(m.theme, m.blocks[block])
	rowText := ""
	if row.start < len(line) {
		rowText = line[row.start:min(row.end, len(line))]
	}
	offset := m.canvas.TextOffsetAtStyle(rowText, style, column)
	global := m.blockStart(block) + row.start + offset
	m.doc.MoveTo(m.bytesToRune(global), extend)
}

// moveLine moves the caret to the start or the end of its block.
func (m *Model) moveLine(end bool, extend bool) {
	block, _ := m.position(m.doc.ByteOffset(m.doc.Caret()))
	start := m.blockStart(block)
	if end {
		start += len(m.lines[block])
	}
	m.doc.MoveTo(m.bytesToRune(start), extend)
}

// movePage moves the caret a screen up or down.
func (m *Model) movePage(direction int, extend bool) {
	step := max(1, int((m.bodyPane.Height-bodyTopInset)/lineHeight(m.theme.Metrics.FontSizeM)))
	for count := 0; count < step; count++ {
		m.moveVertical(direction, extend)
	}
}

// OffsetAt maps a point in the window to a byte offset in doc: where a press
// lands.
func (m *Model) OffsetAt(point gfx.Point) (int, bool) {
	if m.selected == "" || len(m.bodyLayout.rows) == 0 || m.canvas == nil {
		return 0, false
	}
	offsetY := bodyOffset(m.bodyPane)
	localY := point.Y - offsetY
	row := m.bodyLayout.rows[0]
	found := false
	for _, candidate := range m.bodyLayout.rows {
		if localY >= candidate.y {
			row, found = candidate, true
		}
	}
	if !found {
		row = m.bodyLayout.rows[0]
	}
	line := m.lines[row.block]
	style := m.style(m.theme, m.blocks[row.block])
	rowText := ""
	if row.start < len(line) {
		rowText = line[row.start:min(row.end, len(line))]
	}
	inLine := row.start
	if takesText(m.blocks[row.block].Kind) {
		inRow := m.canvas.TextOffsetAtStyle(rowText, style, point.X-m.bodyLayout.column)
		inLine = row.start + inRow
	}
	return m.blockStart(row.block) + inLine, true
}

// CaretBounds places the input method's window under the caret.
func (m *Model) CaretBounds() (gfx.Rect, bool) {
	if m.selected == "" || m.canvas == nil {
		return gfx.Rect{}, false
	}
	block, inLine := m.position(m.doc.ByteOffset(m.doc.Caret()))
	row, ok := m.rowOf(block, inLine)
	if !ok {
		return gfx.Rect{}, false
	}
	line := m.lines[block]
	style := m.style(m.theme, m.blocks[block])
	rowText := ""
	if row.start < len(line) {
		rowText = line[row.start:min(row.end, len(line))]
	}
	x := m.bodyLayout.column + m.canvas.TextCaretStyle(rowText, style, inLine-row.start)
	y := bodyOffset(m.bodyPane) + row.y
	return gfx.Rect{X: x, Y: y + 2, Width: 2, Height: row.height - 4}, true
}
