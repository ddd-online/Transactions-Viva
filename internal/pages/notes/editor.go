package notes

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"

	"transactions/internal/domain"
)

// The body editor. The platform's text service owns input; this file owns the
// block model, the layout that gives headings their size, the painting and the
// block semantics (Enter splits, Backspace merges, pictures hold no caret).
//
// The text lives in doc as the blocks joined by '\n'; blocks mirrors the lines
// one for one, so a structural edit is one text edit plus one block-list edit.
// Undo is a stack of whole-document snapshots, so it restores both together.

const (
	// readingWidth is the widest the body runs; the text column is centred in
	// the pane.
	readingWidth float32 = 760
	// bodySideInset is the padding beside the text column.
	bodySideInset float32 = 16
	// bodyTopInset is the gap between the title and the first block.
	bodyTopInset float32 = 6
	// bodyBottomSpace is the empty strip under the last block that a click
	// turns into a new block.
	bodyBottomSpace float32 = 140
	// blockGap separates two blocks.
	blockGap float32 = 4
	// headingGap is the extra space above a heading.
	headingGap float32 = 14
	// imageGap separates a picture from its neighbours.
	imageGap float32 = 10
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

// slashItem is one command of the "/" menu.
type slashItem struct {
	label string
	kind  string
	level int
}

// slashItems are the menu's entries, in order.
var slashItems = []slashItem{
	{"正文", domain.NoteBlockText, 0},
	{"标题 1", domain.NoteBlockHeading, 1},
	{"标题 2", domain.NoteBlockHeading, 2},
	{"标题 3", domain.NoteBlockHeading, 3},
	{"标题 4", domain.NoteBlockHeading, 4},
	{"标题 5", domain.NoteBlockHeading, 5},
	{"图片", domain.NoteBlockImage, 0},
}

// slashState is the "/" block menu: it is open at a caret position, filters by
// what the user keeps typing and turns the current block into the chosen one.
type slashState struct {
	open  bool
	query string
	x, y  float32
	items []slashItem
}

// bodyView declares the body control.
func (m *Model) bodyView(c ui.C) {
	if m.selected == "" {
		return
	}
	ui.Control(c, "笔记正文", &m.bodyInput).Grow(1).FillWidth().Draw(m.bodyDraw)
}

// style is the text style of one block.
func (m *Model) style(theme *ui.Theme, block domain.NoteBlock) gfx.TextStyle {
	size := theme.Metrics.FontSizeM
	weight := gfx.WeightRegular
	if block.Kind == domain.NoteBlockHeading {
		switch domain.NoteHeadingLevel(block.Level) {
		case 1:
			size, weight = theme.Metrics.FontSizeL, gfx.WeightBold
		case 2:
			size, weight = theme.Metrics.FontSizeL*0.85, gfx.WeightBold
		case 3:
			size, weight = theme.Metrics.FontSizeL*0.72, gfx.WeightSemibold
		case 4:
			size, weight = theme.Metrics.FontSizeM+1, gfx.WeightSemibold
		default:
			size, weight = theme.Metrics.FontSizeM, gfx.WeightSemibold
		}
	}
	return gfx.TextStyle{Size: size, Weight: weight, Color: theme.Foreground}
}

// lineHeight is the row height of a style.
func lineHeight(size float32) float32 { return size * 1.55 }

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
			top += headingGap
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
	m.clampScroll()
}

// clampScroll keeps the scroll offset inside the content.
func (m *Model) clampScroll() {
	if m.scroll < 0 {
		m.scroll = 0
	}
	if max := m.maxScroll(); m.scroll > max {
		m.scroll = max
	}
}

// maxScroll is the largest scroll offset the content allows.
func (m *Model) maxScroll() float32 {
	overflow := m.bodyLayout.height + bodyBottomSpace - m.bodyPane.Height
	if overflow < 0 {
		return 0
	}
	return overflow
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
	width := rect.Width - 2*bodySideInset
	if width > readingWidth {
		width = readingWidth
	}
	if width < 80 {
		width = 80
	}
	m.layoutBody(canvas, width, theme)
	column := rect.X + (rect.Width-width)/2
	m.bodyLayout.column = column
	focused := d.State.Focused
	m.followCaret(rect)
	offsetY := rect.Y + bodyTopInset - m.scroll

	canvas.Clip(rect)
	m.paintImages(canvas, theme, rect, column, offsetY)
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
	m.paintSlash(canvas, theme, rect)
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
		if block.Kind == domain.NoteBlockImage {
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
			canvas.Rounded(target, theme.Metrics.RadiusItem(), theme.Muted)
			continue
		}
		canvas.DrawBitmap(bitmap, target)
		if block.ID != "" && block.ID == m.selectedBlockID() {
			canvas.RoundedBorder(target, theme.Metrics.RadiusItem(), 2, theme.Primary, gfx.Color{})
		}
	}
}

// selectedBlockID is the picture block the user clicked, if any.
func (m *Model) selectedBlockID() string {
	if m.selectedImage < 0 || m.selectedImage >= len(m.blocks) {
		return ""
	}
	return m.blocks[m.selectedImage].ID
}

// followCaret scrolls the view so the caret stays visible.
func (m *Model) followCaret(rect gfx.Rect) {
	block, inLine := m.position(m.doc.ByteOffset(m.doc.Caret()))
	if block < 0 {
		return
	}
	row, ok := m.rowOf(block, inLine)
	if !ok {
		return
	}
	top := row.y + bodyTopInset
	bottom := top + row.height
	height := rect.Height - bodyTopInset
	if top < m.scroll {
		m.scroll = top
	}
	if bottom > m.scroll+height {
		m.scroll = bottom - height
	}
	m.clampScroll()
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
	if m.slash.open {
		if m.slashKey(event) {
			return m.state(), nil
		}
	}
	if m.doc.CompositionActive() {
		// The input method owns the text until it commits.
		event.Apply(m.doc)
		m.afterEdit()
		return m.state(), nil
	}
	switch event.Kind {
	case window.TextPasteNoText:
		m.pastePicture()
		return m.state(), nil
	case window.TextPaste:
		m.pasteText(event)
		return m.state(), nil
	case window.TextInsert:
		if event.Text == "/" && m.slashStart() {
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
			m.pushHistory()
			m.closeSlash()
			m.applyEvent(event, m.selectionStart())
			m.afterEdit()
			return m.state(), nil
		case window.TextDeleteBackward:
			m.pushHistory()
			m.applyEvent(event, m.backspaceStart())
			m.afterEdit()
			return m.state(), nil
		case window.TextDeleteForward:
			m.pushHistory()
			m.applyEvent(event, m.selectionStart())
			m.afterEdit()
			return m.state(), nil
		default:
			event.Apply(m.doc)
			m.skipImageCaret()
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
	m.skipImageCaret()
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
	m.pushHistory()
	m.applyEvent(event, m.selectionStart())
	m.afterEdit()
}

// afterEdit records the change and moves the caret off a picture.
func (m *Model) afterEdit() {
	m.skipImageCaret()
	m.syncLines()
	m.linesDirty = false
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.markEdited(m.now)
}

// skipImageCaret moves the caret out of a picture's line: a picture block holds
// no text, so the caret belongs on the nearest text line.
func (m *Model) skipImageCaret() {
	for passes := 0; passes < len(m.blocks)+1; passes++ {
		block, _ := m.position(m.doc.ByteOffset(m.doc.Caret()))
		if block < 0 || block >= len(m.blocks) || m.blocks[block].Kind != domain.NoteBlockImage {
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

// insertPicture puts a picture block after the block the caret is in, or turns
// the empty block it is in into the picture.
func (m *Model) insertPicture(block domain.NoteBlock) {
	if m.selected == "" {
		return
	}
	m.pushHistory()
	m.syncLines()
	offset := m.doc.ByteOffset(m.doc.Caret())
	current, inLine := m.position(offset)
	line := ""
	if current < len(m.lines) {
		line = m.lines[current]
	}
	empty := inLine == 0 && strings.TrimSpace(line) == "" &&
		m.blocks[current].Kind != domain.NoteBlockImage
	if empty {
		m.blocks[current] = block
		m.doc.MoveTo(m.bytesToRune(m.blockStart(current)), false)
	} else {
		at := current + 1
		m.doc.MoveTo(m.bytesToRune(m.blockStart(current)+len(line)), false)
		m.doc.Insert("\n")
		m.insertBlock(at, block)
		m.doc.MoveTo(m.bytesToRune(m.blockStart(at)), false)
	}
	m.syncLines()
	m.linesDirty = false
	m.bodyLayout = bodyLayout{}
	m.selectedImage = -1
	m.markEdited(m.now)
	m.flush()
	m.imageRev++
	m.imageKey = noteImageKey{note: m.selected, rev: m.imageRev}
}

// pushHistory keeps the state before an edit. Consecutive keystrokes share one
// snapshot.
func (m *Model) pushHistory() {
	if m.histOpen && m.now-m.histAt < historyGroup {
		m.histAt = m.now
		return
	}
	if len(m.history) >= historyLimit {
		m.history = m.history[1:]
	}
	blocks := make([]domain.NoteBlock, len(m.blocks))
	copy(blocks, m.blocks)
	m.history = append(m.history, snapshot{text: m.doc.String(), blocks: blocks, sel: m.doc.Selection()})
	m.histOpen, m.histAt = true, m.now
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

// slashStart reports whether a slash typed now should open the block menu: the
// block is empty text and the caret is at its start.
func (m *Model) slashStart() bool {
	offset := m.doc.ByteOffset(m.doc.Caret())
	block, inLine := m.position(offset)
	if block < 0 || block >= len(m.blocks) || m.blocks[block].Kind == domain.NoteBlockImage {
		return false
	}
	return inLine == 0 && strings.TrimSpace(m.lines[block]) == ""
}

// openSlash opens the "/" menu at the caret.
func (m *Model) openSlash() {
	if rect, ok := m.CaretBounds(); ok {
		m.slash.x, m.slash.y = rect.X, rect.Y+rect.Height+2
	}
	m.slash = slashState{open: true, x: m.slash.x, y: m.slash.y, items: slashItems}
}

// closeSlash closes the menu.
func (m *Model) closeSlash() { m.slash = slashState{} }

// slashKey filters the menu with what the user types; Enter takes the first
// match and Escape closes it.
func (m *Model) slashKey(event window.TextEvent) bool {
	switch event.Kind {
	case window.TextInsert:
		m.slash.query += event.Text
	case window.TextPaste:
		m.slash.query += event.Text
	case window.TextCommandEvent:
		switch event.Command {
		case window.TextDeleteBackward:
			if m.slash.query == "" {
				m.closeSlash()
				return true
			}
			runes := []rune(m.slash.query)
			m.slash.query = string(runes[:len(runes)-1])
		case window.TextInsertNewline, window.TextInsertLineBreak:
			items := m.slashMatches()
			if len(items) == 0 {
				m.closeSlash()
				return true
			}
			m.chooseSlash(items[0])
			return true
		}
	case window.TextSetMarked, window.TextUnmark, window.TextCancelComposition:
		return false
	}
	return true
}

// slashMatches is the menu's entries that contain the query.
func (m *Model) slashMatches() []slashItem {
	query := strings.TrimSpace(m.slash.query)
	var items []slashItem
	for _, item := range slashItems {
		if query == "" || strings.Contains(item.label, query) {
			items = append(items, item)
		}
	}
	return items
}

// chooseSlash turns the current block into the chosen kind.
func (m *Model) chooseSlash(item slashItem) {
	m.closeSlash()
	if item.kind == domain.NoteBlockImage {
		m.pickPicture()
		return
	}
	offset := m.doc.ByteOffset(m.doc.Caret())
	block, _ := m.position(offset)
	if block < 0 || block >= len(m.blocks) {
		return
	}
	m.pushHistory()
	if item.kind == domain.NoteBlockImage {
		return
	}
	m.blocks[block].Kind = item.kind
	m.blocks[block].Level = item.level
	m.bodyLayout = bodyLayout{}
	m.markEdited(m.now)
	m.flush()
}

// paintSlash paints the "/" menu at the caret.
func (m *Model) paintSlash(canvas *gfx.Canvas, theme *ui.Theme, rect gfx.Rect) {
	if !m.slash.open {
		return
	}
	items := m.slashMatches()
	rowHeight := float32(30)
	width := float32(180)
	height := float32(8) + rowHeight*float32(len(items))
	x := min(m.slash.x, rect.X+rect.Width-width-8)
	y := m.slash.y
	if y+height > rect.Y+rect.Height {
		y = m.slash.y - height - 8
	}
	if len(items) == 0 {
		height = 30
	}
	canvas.Rounded(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 8, theme.Popover)
	canvas.RoundedBorder(gfx.Rect{X: x, Y: y, Width: width, Height: height}, 8, 1, theme.Border, gfx.Color{})
	for index, item := range items {
		row := gfx.Rect{X: x, Y: y + 4 + float32(index)*rowHeight, Width: width, Height: rowHeight}
		canvas.Label(item.label, row, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.PopoverForeground})
		if !m.bodyKey.IsZero() {
			canvas.HitWithOptions(m.bodyInput.Parts().At(index), row, item.label,
				gfx.HitOptions{Role: gfx.RoleOption})
		}
	}
	if m.slash.query != "" {
		canvas.Label("/"+m.slash.query, gfx.Rect{X: x, Y: y - 18, Width: width, Height: 16},
			gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
	}
}

// bodyPointer handles presses on pictures, on the "/" menu and on the strip
// under the last block; text presses go to the placement.
func (m *Model) bodyPointer(event gfx.PointerEvent) gfx.EventResult {
	if m.selected == "" || event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	if m.slash.open {
		if index, ok := event.Target.In(m.bodyInput.Parts()); ok {
			items := m.slashMatches()
			if index >= 0 && index < len(items) {
				m.chooseSlash(items[index])
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		}
		m.closeSlash()
	}
	if m.bodyKey.IsZero() || event.Target != m.bodyKey {
		return gfx.EventResult{}
	}
	offsetY := m.bodyPane.Y + bodyTopInset - m.scroll
	for index := range m.bodyLayout.rows {
		row := m.bodyLayout.rows[index]
		if m.blocks[row.block].Kind != domain.NoteBlockImage {
			continue
		}
		y := offsetY + row.y
		if event.Y >= y && event.Y <= y+row.height {
			m.selectedImage = row.block
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

// appendBlockAtEnd adds or focuses the empty block under the last block.
func (m *Model) appendBlockAtEnd() {
	m.pushHistory()
	last := len(m.blocks) - 1
	if last >= 0 && m.blocks[last].Kind == domain.NoteBlockText &&
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

// bodyScroll scrolls the body with the wheel.
func (m *Model) bodyScroll(event gfx.ScrollEvent) gfx.EventResult {
	if m.selected == "" || event.Target != m.bodyKey {
		return gfx.EventResult{}
	}
	m.scroll += event.DeltaY
	m.clampScroll()
	return gfx.EventResult{Handled: true, Repaint: true}
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
		if m.blocks[block].Kind != domain.NoteBlockImage {
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
	offsetY := m.bodyPane.Y + bodyTopInset - m.scroll
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
	if m.blocks[row.block].Kind != domain.NoteBlockImage {
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
	y := m.bodyPane.Y + bodyTopInset - m.scroll + row.y
	return gfx.Rect{X: x, Y: y + 2, Width: 2, Height: row.height - 4}, true
}
