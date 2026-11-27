// Package notes is the 笔记 section: a tree of folders and notes on the left and
// the open note on the right. A folder may hold folders and notes, a note is
// always a leaf, and the root holds folders only. The body of a note is a list
// of blocks — headings h1-h5, text and pictures — painted by its own editor,
// because the framework's text controls paint one font size and cannot claim
// Enter, Backspace or the arrow keys while the platform is editing.
//
// The page owns no SQL and no files: the workspace reaches it as
// repository.NotesRepository through SetDataSource, and pictures, the file
// dialog and the clipboard reach it as Commands from the shell.
package notes

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/imageviewer"
	"transactions/internal/components/layout"
	"transactions/internal/components/semantic"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

// saveDelay is how long the editor waits after the last keystroke before it
// writes, the same 600 ms 日记 uses.
const saveDelay = 600 * time.Millisecond

// treeIndent is one level of the notes tree.
const treeIndent = 16

// leafInset is how far a row without a chevron starts past the tree's own inset:
// a branch gives its chevron 24 pixels (Viva's tree), and a note — or a folder
// that holds nothing and so has none — takes the same room, so the icons of a
// level line up down the column.
const leafInset float32 = 24

// railMinWidth and editorMinWidth are how far the divider between the two
// columns may travel: the tree still fits its title and its four commands, and
// the note keeps a measure that reads.
const (
	railMinWidth   float32 = 240
	editorMinWidth float32 = 420
)

// savePhase is what the header says about the draft.
type savePhase uint8

const (
	saveSaved savePhase = iota
	savePending
	saveFailed
)

// Commands are the shell's picture capabilities. The page never opens a file:
// it asks for a picture's path, imports it into the workspace, loads a bitmap
// to paint, or pastes the clipboard's picture.
type Commands struct {
	// PickImage opens the platform's picture dialog and returns one path.
	PickImage func() (string, error)
	// ImportImage copies source into notes/<noteID>/ and returns the stored
	// path relative to data/assets plus the picture's pixel size.
	ImportImage func(noteID, source string) (string, int, int, error)
	// PasteImage stores the clipboard's picture the way ImportImage does.
	PasteImage func(noteID string) (string, int, int, error)
	// LoadImage decodes a stored picture for painting.
	LoadImage func(relative string, maxEdge int) (gfx.Bitmap, error)
	// LoadView decodes a stored picture's view copy: the same picture, already
	// scaled and encoded at import, so opening a note or its lightbox does not
	// decode a camera original. Optional: LoadImage stands in when it is nil.
	LoadView func(relative string, maxEdge int) (gfx.Bitmap, error)
}

// treeRow is one visible row of the tree: the node it shows and how deep the
// tree has it.
type treeRow struct {
	node  string
	depth int
}

// nodeForm is the 新建文件夹 / 重命名 dialog.
type nodeForm struct {
	open   bool
	mode   string // "folder" (create) or "rename"
	node   string // the node renamed
	parent string // the folder a new folder goes into
	field  ui.Field
	// err is the sentence under the field: what stopped the last 确定. errOn is
	// the field's text when it was set, so the first keystroke clears it.
	err   string
	errOn string
}

// deleteForm is the confirmation for a node and its subtree.
type deleteForm struct {
	open  bool
	node  string
	title string
}

// Model is the 笔记 page's state.
type Model struct {
	repo repository.NotesRepository
	cmds Commands

	// nodes and children are the whole tree, noteCount its notes; rows are the
	// visible part of it, rebuilt when the tree, a fold or the filter changed,
	// and tree is the Viva tree that shows them.
	nodes     map[string]domain.NoteNode
	noteCount int
	children  map[string][]string
	rows      []treeRow
	rowsDirty bool
	expanded  map[string]bool
	tree      ui.TreeState
	// rowKeys name a node's row for the tree, so a row keeps its focus and its
	// identity while rows open and close above it. nextRowKey hands them out.
	rowKeys    map[string]int
	nextRowKey int
	// rowMenu is the actions menu of one row, opened by its "…" button.
	rowMenu rowMenuState
	// node is the selected tree row (a folder or a note), selected the open
	// note. They differ while a folder is selected.
	node     string
	selected string

	// filter is the title search: filterText is what it holds and filterID its
	// identity. searchOpen says whether the toolbar's 搜索笔记 has it showing,
	// and findFocus asks the paint that shows it to put the keyboard in it.
	filter     ui.Field
	filterText string
	filterID   gfx.ID
	searchOpen bool
	findFocus  bool

	// split is the divider between the tree and the note: Pos is the tree's
	// width, kept in pixels so a wider window gives the extra room to the note.
	// railLoaded is true once the stored width was read, and railDragged the
	// last paint's drag state, so the width is written when the drag ends.
	split       ui.SplitterState
	railLoaded  bool
	railDragged bool
	// railHidden hides the tree column: the note then has the whole window,
	// which is what the band's panel button is for.
	railHidden bool

	// title is the note's title field, titleText its last read value and
	// titleID its identity in the band.
	title     ui.Field
	titleText string
	titleID   gfx.ID

	// The body editor. The text and its undo history live in doc; blocks
	// mirrors doc's lines one for one, and lines is doc's last split.
	doc        *text.Editor
	blocks     []domain.NoteBlock
	lines      []string
	linesDirty bool
	bodyKey    gfx.ID
	bodyDraw   func(*ui.DrawContext)
	bodyInput  ui.Handlers
	bodyLayout bodyLayout
	// wraps caches each line's row breaks from the last layout, keyed by block, so
	// typing in one block re-measures that block and nothing else. It is dropped
	// when the column's width changes, the only other thing the breaks depend on.
	wraps map[int]lineWrap
	// scroll is the body's offset, as the scroll area last had it; the caret moves
	// it before the area places the blocks. caretShown is the caret the body last
	// brought into view, and caretViewport the height it brought it into: a caret
	// that did not move leaves the scroll where the user put it.
	scroll        float32
	caretShown    int
	caretViewport float32
	// selectedImage is the picture block the user clicked, or -1.
	selectedImage int

	history  []snapshot
	histAt   time.Duration
	histOpen bool

	// save state
	phase    savePhase
	dirty    bool
	editedAt time.Duration

	// drag and drop between blocks. column is the body's layout: the text
	// control and the grip of the block it shows.
	blockDrag  ui.DragState
	hoverBlock int
	dragBlock  int
	// gripHover is the block whose grip the pointer is on, or -1: the handle
	// marks its block (see blockWash).
	gripHover int
	dropAfter bool
	blockMenu blockMenuState
	// typeMenu is the band's block-type menu, which changes the block at the caret
	// rather than the block a grip belongs to.
	typeMenu typeMenuState
	resize   resizeState
	column   blockColumn
	// area is the body's scroll area as the last paint declared it: the wheel and
	// a drag on its bar reach it between paints.
	area *ui.ScrollState
	// gripBlock is the block whose grip the last paint declared, or -1: the press
	// that starts a drag is on it, so a drag takes its block from here. gripID is
	// that grip's identity, which the wheel over the gutter names.
	gripBlock int
	gripID    gfx.ID

	// pictures: the open note's decoded blocks and the lightbox's one picture
	imageEffect    *ui.EffectState[map[string]gfx.Bitmap, noteImageKey]
	noteImages     map[string]gfx.Bitmap
	lightboxEffect *ui.EffectState[gfx.Bitmap, string]
	lightbox       lightbox
	viewer         *imageviewer.Viewer

	// dialogs
	form   nodeForm
	remove deleteForm

	toaster      ui.Toaster
	errorMessage string

	// focusTitle asks the next paint to put the keyboard in the title, which a
	// note just created does.
	focusTitle bool

	// measure and measureTheme are the last paint's canvas and theme, kept only
	// for the text measurements the movements and the caret hit test need
	// between paints (there is no other way to measure a proportional face).
	// Nothing else is painted through them.
	now             time.Duration
	undoneAt        time.Duration
	canvas          *gfx.Canvas
	theme           *ui.Theme
	bodyPane        gfx.Rect
	lineStartsCache []int
	slash           slashState
	pickPending     bool
	focusBody       bool

	// imageRev changes whenever a picture block is added, so the effect that
	// decodes the note's pictures runs again.
	imageRev int
	imageKey noteImageKey

	// defaultFolder is set once the first entry to an empty tree made the
	// default folder, so a failure is not retried every entry.
	defaultFolder bool
	// loaded is true after the tree was read at least once.
	loaded bool
}

// noteImageKey says which note's pictures to decode, and that the set changed.
type noteImageKey struct {
	note string
	rev  int
}

// New returns the page with nothing loaded; desktop calls SetDataSource once a
// workspace is open.
func New() *Model {
	m := &Model{
		nodes:         map[string]domain.NoteNode{},
		children:      map[string][]string{},
		expanded:      map[string]bool{},
		rowKeys:       map[string]int{},
		noteImages:    map[string]gfx.Bitmap{},
		selectedImage: -1,
		caretShown:    -1,
		hoverBlock:    -1,
		gripHover:     -1,
		dragBlock:     -1,
		blockMenu:     blockMenuState{block: -1},
		typeMenu:      typeMenuState{block: -1},
		resize:        resizeState{block: -1},
		doc:           text.NewEditor(""),
		viewer:        imageviewer.New(),
		split:         ui.SplitterState{Pos: layout.RailWidth},
	}
	m.bindTree()
	m.bodyDraw = m.drawBody
	m.column = blockColumn{m: m, block: -1}
	m.gripBlock = -1
	m.bodyInput.Key = m.bodyKeyEvent
	m.bodyInput.Pointer = m.bodyPointer
	m.bodyInput.Text = m.bodyText
	m.imageEffect = ui.NewEffectKeyed(m.loadNoteImages)
	m.lightboxEffect = ui.NewEffectKeyed(m.loadLightboxImage)
	return m
}

// SetDataSource connects the page to a workspace. nil clears everything, as on
// closing the workspace.
func (m *Model) SetDataSource(repo repository.NotesRepository) {
	m.repo = repo
	m.loaded = false
	m.defaultFolder = false
	m.nodes = map[string]domain.NoteNode{}
	m.children = map[string][]string{}
	m.rows = nil
	m.expanded = map[string]bool{}
	m.rowKeys = map[string]int{}
	m.nextRowKey = 0
	m.searchOpen, m.findFocus = false, false
	m.filterText = ""
	m.filter.SetText("")
	m.tree = ui.TreeState{}
	m.rowMenu = rowMenuState{}
	m.bindTree()
	m.node, m.selected = "", ""
	m.titleText = ""
	m.focusTitle = false
	m.phase, m.dirty = saveSaved, false
	m.errorMessage = ""
	// The tree's width belongs to the workspace that was open, so another one
	// starts at the default until its own width is read.
	m.railLoaded, m.railDragged = false, false
	m.railHidden = false
	m.split.Pos = layout.RailWidth
	m.history = nil
	m.noteImages = map[string]gfx.Bitmap{}
	if repo == nil {
		m.blocks = nil
		m.lines = nil
		m.doc = text.NewEditor("")
		m.bodyLayout = bodyLayout{}
	}
}

// SetCommands supplies the shell's picture capabilities.
func (m *Model) SetCommands(cmds Commands) { m.cmds = cmds }

// Reload rereads the tree, keeping the open note when it still exists.
func (m *Model) Reload() {
	if m.repo == nil {
		return
	}
	if !m.railLoaded {
		// The tree's width and its visibility are read once, so a reload never
		// undoes what the user just did to it.
		m.railLoaded = true
		if width, err := m.repo.NoteRailWidth(context.Background()); err == nil && width > 0 {
			m.split.Pos = width
		}
		if hidden, err := m.repo.NoteRailHidden(context.Background()); err == nil {
			m.railHidden = hidden
		}
	}
	m.Read()
	if len(m.nodes) == 0 && !m.defaultFolder {
		// The first entry into an empty workspace makes the default folder, so
		// 新建笔记 always has somewhere to go.
		m.defaultFolder = true
		if folder, err := m.repo.CreateNoteNode(context.Background(), "",
			domain.NoteKindFolder, "我的笔记"); err == nil {
			m.node = folder.ID
			m.Read()
			m.rebuildRows()
			return
		}
	}
	if m.selected != "" {
		if node, ok := m.nodes[m.selected]; ok && node.Kind == domain.NoteKindNote {
			m.imageRev++
			m.imageKey = noteImageKey{note: m.selected, rev: m.imageRev}
			m.rebuildRows()
			return
		}
	}
	m.selected = ""
	m.titleText = ""
	m.focusTitle = false
	m.blocks, m.lines = nil, nil
	m.doc = text.NewEditor("")
	m.history = nil
	m.rebuildRows()
	if last, err := m.repo.LastNoteID(context.Background()); err == nil && last != "" {
		if node, ok := m.nodes[last]; ok && node.Kind == domain.NoteKindNote {
			m.node, m.selected = "", ""
			m.openNote(last)
		}
	}
}

// Read rereads the tree without the first-entry default folder logic.
func (m *Model) Read() {
	if m.repo == nil {
		return
	}
	m.loaded = true
	nodes, err := m.repo.NoteTree(context.Background())
	if err != nil {
		m.report(err)
		return
	}
	m.nodes = make(map[string]domain.NoteNode, len(nodes))
	m.children = map[string][]string{}
	for _, node := range nodes {
		m.nodes[node.ID] = node
		m.expanded[node.ID] = node.Expanded
	}
	m.noteCount = 0
	for _, node := range nodes {
		if node.Kind == domain.NoteKindNote {
			m.noteCount++
		}
	}
	for _, node := range nodes {
		m.children[node.ParentID] = append(m.children[node.ParentID], node.ID)
	}
	for parent := range m.children {
		m.sortChildren(parent)
	}
	m.rowsDirty = true
}

// CloseOverlays writes the draft and closes everything floating, as the shell
// leaves the section.
func (m *Model) CloseOverlays() {
	m.flush()
	m.form.open = false
	m.remove.open = false
	m.closeLightbox()
}

// View paints the section.
func (m *Model) View(c ui.C) {
	if m.repo != nil && !m.loaded {
		m.Reload()
	}
	m.now = c.Canvas().Now()
	if m.selected != "" && !m.bodyKey.IsZero() && c.Canvas().Focused() == m.bodyKey {
		if ui.Shortcut(c, ui.Primary, "z") && m.undoneAt != m.now {
			m.undo()
			m.undoneAt = m.now
		}
		m.blockShortcuts(c)
	}
	m.noticeEdits(m.now)
	m.settleDraft(c)
	// The grip's menu is Viva's, which closes itself on a choice, on Escape and on
	// a press outside: its rows go away with the keyboard. The note takes the
	// keyboard back in the paint after the close.
	if m.blockMenu.wasOpen && !m.blockMenu.open {
		m.focusBody = true
	}
	m.blockMenu.wasOpen = m.blockMenu.open
	// The band's menu is Viva's too, and it closes the same ways: a choice, Escape,
	// or a press outside it all give the keyboard back to the note next paint.
	if m.typeMenu.wasOpen && !m.typeMenu.open {
		m.focusBody = true
	}
	m.typeMenu.wasOpen = m.typeMenu.open
	if m.focusBody && !m.bodyKey.IsZero() {
		c.Canvas().RequestFocus(m.bodyKey)
		m.focusBody = false
	}
	if m.rowsDirty {
		m.rebuildRows()
	}
	ui.Effect(c, m.imageEffect, m.imageKey)
	if images, err, ok := m.imageEffect.Take(); ok {
		if err == nil {
			m.noteImages = images
		}
	}
	m.settleSplit()
	ui.Row(c).Grow(1).FillWidth().Children(func() {
		if m.railHidden {
			m.detail(c)
			return
		}
		m.rail(c)
		// The splitter is the column's edge: one hairline that the pointer can
		// drag, instead of a fixed width and a divider.
		ui.Splitter(c, &m.split).SplitterThickness(1).Mins(railMinWidth, editorMinWidth).
			HitWith(ui.ID(c), "调整笔记目录宽度", gfx.HitOptions{
				Role: gfx.RoleSeparator, SkipFocus: true, Cursor: gfx.PointerCursorResizeHorizontal,
			})
		m.detail(c)
	})
	m.formDialog(c)
	m.deleteDialog(c)
	m.lightboxView(c)
	ui.Toasts(c, &m.toaster)
}

// rail is the tree column: the section's title and count with its four
// commands, the title search under them while 搜索笔记 has it open, and the tree.
func (m *Model) rail(c ui.C) {
	text := strings.TrimSpace(m.filter.Text())
	if text != m.filterText {
		m.filterText = text
		m.rowsDirty = true
		m.rebuildRows()
	}
	width := m.split.Pos
	if width <= 0 {
		width = layout.RailWidth
	}
	layout.Rail(c, width, func(c ui.C) {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, "笔记")
			layout.SectionCount(c, strconv.Itoa(m.noteCount))
			ui.Spacer(c)
			m.toolbar(c)
		})
		if m.searchOpen {
			ui.Column(c).FillWidth().PadX(layout.Inset).PadBottom(4).Children(func() {
				c.Key("note-search")
				m.filterID = ui.ID(c)
				// The button that opened it painted before this field in the
				// same pass, so the request lands on the field the pass declares.
				if m.findFocus {
					c.Canvas().RequestFocus(m.filterID)
					m.findFocus = false
				}
				field := ui.TextFieldID(c, m.filterID, &m.filter, "过滤笔记", "搜索文件夹、笔记").FillWidth()
				field.OnKey(func(event gfx.KeyEvent) gfx.EventResult {
					if event.Key != gfx.KeyEscape {
						return gfx.EventResult{}
					}
					m.closeSearch()
					return gfx.EventResult{Handled: true, Repaint: true}
				})
			})
		}
		// The rule under the header stays; the rows begin right below it, so
		// the header and its tree are one block with no gap between them.
		ui.Divider(c)
		if len(m.rows) == 0 && m.filterText != "" {
			emptystate.Notice(c, "没有匹配的笔记或文件夹")
		} else {
			// The tree keeps the page's inset on both sides, so a row's highlight
			// floats inside the rail instead of touching its edges.
			ui.Column(c).Grow(1).FillWidth().PadX(layout.Inset).Children(func() {
				ui.Tree(c, &m.tree, len(m.rows), m.rowLabel, func(index int) {
					m.rowContent(c, index)
				}).Label("笔记列表").Grow(1)
			})
			m.treeChanges()
		}
	})
}

// toggleRail shows or hides the tree column, and remembers which the workspace
// was left in.
func (m *Model) toggleRail() {
	m.railHidden = !m.railHidden
	if m.repo == nil {
		return
	}
	if err := m.repo.SetNoteRailHidden(context.Background(), m.railHidden); err != nil {
		m.report(err)
	}
}

// toggleSearch shows or hides the title search under the toolbar. Hiding it
// clears the query too, so the tree never stays filtered by a field the user
// cannot see.
func (m *Model) toggleSearch() {
	if m.searchOpen {
		m.closeSearch()
		return
	}
	m.searchOpen = true
	m.findFocus = true
}

// closeSearch hides the field like the query it held never existed.
func (m *Model) closeSearch() {
	m.searchOpen = false
	m.findFocus = false
	m.filter.SetText("")
	if m.filterText != "" {
		m.filterText = ""
		m.rowsDirty = true
		m.rebuildRows()
	}
}

// settleSplit writes the tree's width when the user lets go of the divider. The
// width is the one piece of the page's layout that belongs to the user, so it
// goes back into the workspace and returns with it.
func (m *Model) settleSplit() {
	if m.split.Dragging() {
		m.railDragged = true
		return
	}
	if !m.railDragged {
		return
	}
	m.railDragged = false
	if m.repo == nil {
		return
	}
	if err := m.repo.SetNoteRailWidth(context.Background(), m.split.Pos); err != nil {
		m.report(err)
	}
}

// toolbar is the column's four commands, the row 参考应用 puts above its tree:
// search, a note, a folder, and opening or closing every folder. They are bare
// icons with the appearance toggle's hover and press, not outlined buttons.
func (m *Model) toolbar(c ui.C) {
	ui.Row(c).Gap(2).Shrink(0).Children(func() {
		search := m.iconButton(c, "搜索笔记", icons.Search, m.toggleSearch)
		if m.searchOpen {
			// The command holds the field open, so it reads as on.
			search.Background(state.Selected(*c.Theme()))
		}
		m.iconButton(c, "新建笔记", icons.SquarePen, m.createNote)
		m.iconButton(c, "新建文件夹", icons.FolderPlus, m.openCreateFolder)
		label, icon := "展开全部", icons.ListTree
		if m.allExpanded() {
			label, icon = "收起全部", icons.ListX
		}
		m.iconButton(c, label, icon, m.toggleExpandAll)
	})
}

// iconButton is one bare icon command of the page: the toolbar's four and the
// band's panel button.
func (m *Model) iconButton(c ui.C, label string, icon gfx.Icon, action func()) ui.Box {
	theme := c.Theme()
	button := ui.ButtonBase(c, label).Size(28, 28).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(state.Hover(*theme))).
		Pressed(ui.Look{}.Background(state.Pressed(*theme))).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Tooltip(label)
	button.Children(func() {
		ui.Icon(c, icon).Size(15, 15)
	})
	if button.Clicked() {
		action()
	}
	return button
}

// allExpanded reports whether every folder that holds something is open, which
// is the state 收起全部 leaves the tree in.
func (m *Model) allExpanded() bool {
	found := false
	for id, node := range m.nodes {
		if node.Kind != domain.NoteKindFolder || len(m.children[id]) == 0 {
			continue
		}
		if !m.expanded[id] {
			return false
		}
		found = true
	}
	return found
}

// toggleExpandAll opens every folder, or closes them all when they already are
// open. The filter opens folders on its own, so the command does nothing while
// a query is typed.
func (m *Model) toggleExpandAll() {
	if m.repo == nil || m.filterText != "" {
		return
	}
	expanded := !m.allExpanded()
	if err := m.repo.SetNoteFoldersExpanded(context.Background(), expanded); err != nil {
		m.report(err)
		return
	}
	for id, node := range m.nodes {
		if node.Kind == domain.NoteKindFolder {
			m.expanded[id] = expanded
		}
	}
	m.rowsDirty = true
	m.rebuildRows()
}

// rowMenuState is the actions menu one row opened: which node it belongs to and
// whether it is showing.
type rowMenuState struct {
	open bool
	node string
}

// bindTree gives the tree the page's callbacks. Viva's tree owns the rows'
// geometry, the chevron, the selection, the keyboard walk and the drag, so the
// page only describes a row and answers what the user did to it.
func (m *Model) bindTree() {
	m.tree.Key = m.rowKey
	m.tree.Node = m.treeNode
	m.tree.Label = m.rowLabel
	m.tree.RowHeight = layout.TreeRowHeight
	m.tree.Indent = treeIndent
	m.tree.CanDrop = m.canDropRow
}

// rowKey is a node's row identity. The tree keeps a row's focus by this number,
// so a node keeps the one it was given while rows open and close above it.
func (m *Model) rowKey(index int) int {
	if index < 0 || index >= len(m.rows) {
		return -1
	}
	id := m.rows[index].node
	if key, ok := m.rowKeys[id]; ok {
		return key
	}
	m.nextRowKey++
	m.rowKeys[id] = m.nextRowKey
	return m.nextRowKey
}

// treeNode describes row i to the tree: how deep it is, whether it has children
// and shows them, and whether it is the one the page has selected.
func (m *Model) treeNode(index int) ui.TreeNode {
	if index < 0 || index >= len(m.rows) {
		return ui.TreeNode{}
	}
	row := m.rows[index]
	node, ok := m.nodes[row.node]
	if !ok {
		return ui.TreeNode{}
	}
	folder := node.Kind == domain.NoteKindFolder
	return ui.TreeNode{
		Depth:    row.depth,
		Branch:   folder && len(m.children[node.ID]) > 0,
		Expanded: m.expanded[node.ID] || m.filterText != "",
		Selected: node.ID == m.selected || node.ID == m.node,
	}
}

// rowLabel is a row's accessible name, which a folder and a note spell with
// their own word.
func (m *Model) rowLabel(index int) string {
	if index < 0 || index >= len(m.rows) {
		return ""
	}
	node, ok := m.nodes[m.rows[index].node]
	if !ok {
		return ""
	}
	if node.Kind == domain.NoteKindFolder {
		return "文件夹 " + m.rowName(node)
	}
	return "笔记 " + m.rowName(node)
}

// rowName is the name a row shows: a note without a title is 未命名笔记.
func (m *Model) rowName(node domain.NoteNode) string {
	if node.Kind == domain.NoteKindNote {
		return domain.NoteDisplayTitle(node.Title)
	}
	return strings.TrimSpace(node.Title)
}

// rowContent declares what one row holds: the icon, the name, and the actions
// menu's button where the row is lit. The tree owns the row itself, so the
// chevron, the indent, the selected fill and the drop marks are its work.
func (m *Model) rowContent(c ui.C, index int) {
	if index < 0 || index >= len(m.rows) {
		return
	}
	node, ok := m.nodes[m.rows[index].node]
	if !ok {
		return
	}
	theme := c.Theme()
	// A folder wears the tree's one hue and a note the secondary ink: the colour
	// marks the rows that hold other rows, so a note never reads as a link.
	icon, color := icons.Note, semantic.LeafInk(*theme)
	if node.Kind == domain.NoteKindFolder {
		icon, color = icons.Folder, semantic.FolderInk(*theme)
	}
	// One child of the tree's row, not three: the tree spaces its row's children
	// by GapItem, and the icon belongs to the name, one pixel from it.
	inset := float32(0)
	if !(node.Kind == domain.NoteKindFolder && len(m.children[node.ID]) > 0) {
		inset = leafInset
	}
	ui.Row(c).FillWidth().PadLeft(inset).Gap(1).Items(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(15, 15).Color(color).Shrink(0)
		typography.Body(c, m.rowName(node)).Grow(1).MinWidth(0).MaxLines(1).Ellipsis()

		// A right click on the row is the same command as the actions key: it opens
		// the row's menu, and the key appears for it in the same paint, so the menu
		// has its anchor. Consume keeps the tree from reading the press as a click.
		if rowID := m.tree.RowID(index); !rowID.IsZero() {
			if event, ok := c.Canvas().PointerOn(rowID); ok &&
				event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary {
				m.rowMenu = rowMenuState{open: true, node: node.ID}
				c.Canvas().Consume()
			}
		}
		// The key names its identity before it is declared: while the pointer is on
		// the key itself the row is no longer the hovered target, and without this
		// the row would un-light, the key would vanish under the pointer, the pointer
		// would fall back on the row — the two flicker and the click never lands.
		c.Key("note-row-menu-" + node.ID)
		keyID := ui.ID(c)
		// The actions are quiet until the row is lit: the pointer is on it or on its
		// key, it has the keyboard, it is the row the menu belongs to, or it is the
		// row the editor shows.
		lit := m.rowMenu.open && m.rowMenu.node == node.ID
		if rowID := m.tree.RowID(index); !rowID.IsZero() {
			lit = lit || c.Canvas().Hovered() == rowID || c.Canvas().FocusVisible() == rowID
		}
		if lit = lit || c.Canvas().Hovered() == keyID || node.ID == m.selected; !lit {
			return
		}
		button := ui.ButtonBase(c, "更多操作 "+m.rowName(node)).Size(24, 24).
			HitWith(keyID, "更多操作 "+m.rowName(node), gfx.HitOptions{
				Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
			}).
			Radius(theme.Metrics.RadiusControl).
			Hover(ui.Look{}.Background(state.Hover(*theme))).
			Pressed(ui.Look{}.Background(state.Pressed(*theme))).
			Focus(ui.Look{}.Ring(2, theme.Ring)).
			Tooltip("更多操作").Shrink(0)
		button.Children(func() {
			ui.Icon(c, icons.Ellipsis).Size(15, 15).Color(theme.MutedForeground)
		})
		if button.Clicked() {
			m.rowMenu = rowMenuState{open: true, node: node.ID}
		}
		// Only the row the menu belongs to declares it: a menu per lit row would
		// paint the same items twice, once under the chosen row and once under
		// whichever row happens to be under the pointer.
		if m.rowMenu.open && m.rowMenu.node == node.ID {
			ui.Menu(c, button, &m.rowMenu.open, func(items *ui.MenuItems) {
				m.rowMenuItems(items, node)
			})
		}
	})
}

// rowMenuItems is a row's actions: a folder makes notes and folders, both kinds
// rename and delete.
func (m *Model) rowMenuItems(items *ui.MenuItems, node domain.NoteNode) {
	if node.Kind == domain.NoteKindFolder {
		if items.Item("新建笔记").Clicked() {
			m.node = node.ID
			m.createNote()
		}
		if items.Item("新建文件夹").Clicked() {
			m.openCreateFolderAt(node.ID)
		}
		items.Separator()
	}
	if items.Item("重命名").Clicked() {
		m.openRename(node.ID)
	}
	if items.Item("删除").Destructive().Clicked() {
		m.askDelete(node.ID)
	}
}

// treeChanges applies what the tree reported this paint: a branch opened or
// closed, a row chosen, or a row dropped on another. The rows it paints next
// follow from the state set here, which the click's own repaint shows.
func (m *Model) treeChanges() {
	if index, ok := m.tree.Toggled(); ok {
		if index < len(m.rows) {
			m.toggleFolder(m.rows[index].node)
		}
	}
	if index, ok := m.tree.Chosen(); ok {
		if index < len(m.rows) {
			m.selectNode(m.rows[index].node)
		}
	}
	if move, ok := m.tree.Dropped(); ok {
		m.applyTreeMove(move)
	}
}

// canDropRow reports whether the tree may let row from go at where of row to.
// A folder never lands inside itself or its own subtree, only a folder takes a
// row into it, and a note never lands at the root, which holds folders only.
func (m *Model) canDropRow(from, to int, where ui.TreeDrop) bool {
	if from < 0 || to < 0 || from >= len(m.rows) || to >= len(m.rows) || from == to {
		return false
	}
	source, ok := m.nodes[m.rows[from].node]
	if !ok {
		return false
	}
	target, ok := m.nodes[m.rows[to].node]
	if !ok || m.subtreeContains(source.ID, target.ID) {
		return false
	}
	if where == ui.TreeDropInside {
		return target.Kind == domain.NoteKindFolder
	}
	return target.ParentID != "" || source.Kind == domain.NoteKindFolder
}

// applyTreeMove moves the dropped row where it was let go and opens the folder
// that received it, so a row dropped into a folded folder is seen arriving.
func (m *Model) applyTreeMove(move ui.TreeMove) {
	if move.From < 0 || move.To < 0 || move.From >= len(m.rows) || move.To >= len(m.rows) {
		return
	}
	from, to := m.rows[move.From].node, m.rows[move.To].node
	parent := m.nodes[to].ParentID
	if move.Where == ui.TreeDropInside {
		parent = to
	}
	m.moveRow(from, parent, m.dropPosition(from, to, parent, move.Where))
}

// dropPosition is where the moved row goes among its new siblings, counted
// without itself: the store inserts it there.
func (m *Model) dropPosition(from, to, parent string, where ui.TreeDrop) int {
	if where == ui.TreeDropInside {
		return -1
	}
	position := 0
	for _, id := range m.children[parent] {
		if id == from {
			continue
		}
		if id == to {
			break
		}
		position++
	}
	if where == ui.TreeDropAfter {
		position++
	}
	return position
}

// moveRow moves a node under parent at position and rereads the tree.
func (m *Model) moveRow(from, parent string, position int) {
	if m.repo == nil {
		return
	}
	if err := m.repo.MoveNoteNode(context.Background(), from, parent, position); err != nil {
		m.report(err)
		return
	}
	m.Read()
	if parent != "" {
		m.expanded[parent] = true
		if err := m.repo.SetNoteNodeExpanded(context.Background(), parent, true); err != nil {
			m.report(err)
		}
	}
	m.rebuildRows()
}

// detail is the right column: the note's band, then the block editor. The band
// spans the column the way 日记's day band does, so the page opens on the note
// with its title where a document's title belongs.
func (m *Model) detail(c ui.C) {
	if m.selected == "" {
		// A hidden tree leaves the empty state without its command, so the
		// band comes back for the button that shows the tree again.
		if m.railHidden {
			layout.SectionHeader(c, layout.Inset).Children(func() { m.railButton(c) })
			ui.Divider(c)
		}
		emptystate.Page(c, &icons.Note, "选择一篇笔记",
			"新建一篇，或在目录里选一篇已有的。", func() {
				if ui.Button(c, "新建文件夹").Secondary().Clicked() {
					m.openCreateFolder()
				}
				if ui.Button(c, "新建笔记").Clicked() {
					m.createNote()
				}
			})
		return
	}
	ui.Column(c).Grow(1).FillHeight().FillWidth().Children(func() {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			m.titleBar(c)
		})
		ui.Divider(c)
		m.editorBand(c)
		ui.Column(c).Grow(1).FillWidth().PadX(layout.Inset).Children(func() {
			m.bodyView(c)
		})
	})
}

// titleBar is the band: the panel button that shows or hides the tree, the
// note's title as a field, and at the right the draft's state. The title is
// always the field, as the reference application has it: a title is read and
// typed in the same place.
func (m *Model) titleBar(c ui.C) {
	m.railButton(c)
	m.titleField(c)
	m.saveState(c)
}

// railButton shows or hides the tree column. It lives in the band rather than
// the tree, so it is still there to bring the tree back.
func (m *Model) railButton(c ui.C) {
	label := "隐藏目录"
	if m.railHidden {
		label = "显示目录"
	}
	m.iconButton(c, label, icons.PanelLeft, m.toggleRail)
}

// titleField is the note's title, one field the band always shows. It keeps the
// band's own step, as every page's band does: the document's hierarchy starts in
// the body, where its first heading is larger than the band naming it.
func (m *Model) titleField(c ui.C) {
	c.Key("note-title")
	m.titleID = ui.ID(c)
	field := ui.TextFieldID(c, m.titleID, &m.title, "笔记标题", "未命名笔记").
		FontSize(typography.SizeSection).Weight(gfx.WeightBold).Grow(1).MinWidth(0)
	if m.focusTitle {
		c.Canvas().RequestFocus(m.titleID)
		m.focusTitle = false
	}
	if field.Submitted() {
		// Enter in the title goes on in the body, the way a document's title
		// leads into its text.
		c.Canvas().RequestFocus(m.bodyKey)
	}
}

// saveState is the quiet state at the right of the band.
func (m *Model) saveState(c ui.C) {
	theme := c.Theme()
	label, color := "已保存", theme.MutedForeground
	switch m.phase {
	case savePending:
		label = "未保存"
	case saveFailed:
		label, color = "保存失败", theme.Danger
	}
	// The state never gives way: a long title truncates instead of pushing it
	// out of the band.
	typography.Caption(c, label).Color(color).Shrink(0)
}

// rebuildRows flattens the tree into the visible rows, honouring the folded
// folders and the title filter.
//
// The filter reads every title, folders included: a folder that matches brings
// the notes inside it, because searching a folder's name is how a person opens it
// — a folder standing alone in the results would be a door with nothing behind
// it. A note that matches brings its way in: the folders above it, opened.
func (m *Model) rebuildRows() {
	m.rowsDirty = false
	rows := m.rows[:0]
	visible := map[string]bool{}
	forced := map[string]bool{}
	if m.filterText != "" {
		needle := strings.ToLower(m.filterText)
		// walked keeps a folder's subtree from being marked twice: a folder can be
		// both a match and the way to another one.
		walked := map[string]bool{}
		var mark func(id string)
		mark = func(id string) {
			visible[id] = true
			for parent := m.nodes[id].ParentID; parent != ""; parent = m.nodes[parent].ParentID {
				visible[parent] = true
				forced[parent] = true
			}
			if m.nodes[id].Kind != domain.NoteKindFolder || walked[id] {
				return
			}
			walked[id] = true
			forced[id] = true
			for _, child := range m.children[id] {
				mark(child)
			}
		}
		for id, node := range m.nodes {
			if strings.Contains(strings.ToLower(domain.NoteDisplayTitle(node.Title)), needle) {
				mark(id)
			}
		}
	}
	var walk func(parent string, depth int)
	walk = func(parent string, depth int) {
		for _, id := range m.children[parent] {
			node := m.nodes[id]
			if m.filterText != "" && !visible[id] {
				continue
			}
			rows = append(rows, treeRow{node: id, depth: depth})
			if node.Kind != domain.NoteKindFolder {
				continue
			}
			if m.expanded[id] || forced[id] {
				walk(id, depth+1)
			}
		}
	}
	walk("", 0)
	m.rows = rows
}

// sortChildren orders one folder's children by their stored sort order.
func (m *Model) sortChildren(parent string) {
	ids := m.children[parent]
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && m.nodes[ids[j]].SortOrder < m.nodes[ids[j-1]].SortOrder; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

// toggleFolder opens or closes a folder and stores the state.
func (m *Model) toggleFolder(id string) {
	open := !m.expanded[id]
	m.expanded[id] = open
	if m.repo != nil {
		if err := m.repo.SetNoteNodeExpanded(context.Background(), id, open); err != nil {
			m.report(err)
		}
	}
	m.rowsDirty = true
}

// selectNode selects a tree row: a folder folds and unfolds, a note opens.
func (m *Model) selectNode(id string) {
	node, ok := m.nodes[id]
	if !ok {
		return
	}
	m.node = id
	if node.Kind == domain.NoteKindFolder {
		m.toggleFolder(id)
		return
	}
	m.openNote(id)
}

// openNote writes the current draft and loads another note.
func (m *Model) openNote(id string) {
	if id == m.selected {
		return
	}
	m.flush()
	note, err := m.repo.Note(context.Background(), id)
	if err != nil {
		m.report(err)
		return
	}
	m.selected = id
	m.node = id
	m.titleText = note.Node.Title
	m.title.SetText(m.titleText)
	m.focusTitle = false
	m.loadBlocks(note.Blocks)
	m.history = nil
	m.phase, m.dirty = saveSaved, false
	m.bodyKey = gfx.ID{}
	m.bodyLayout = bodyLayout{}
	m.area = nil
	m.gripID = gfx.ID{}
	m.scroll, m.caretShown, m.caretViewport = 0, -1, 0
	m.selectedImage = -1
	m.noteImages = map[string]gfx.Bitmap{}
	m.imageRev++
	m.imageKey = noteImageKey{note: id, rev: m.imageRev}
	if m.repo != nil {
		if err := m.repo.SetLastNoteID(context.Background(), id); err != nil {
			m.report(err)
		}
	}
}

// loadBlocks copies a note's blocks into the editor. The editor keeps the text
// in doc and the block attributes beside it, one block per line.
func (m *Model) loadBlocks(blocks []domain.NoteBlock) {
	if len(blocks) == 0 {
		blocks = []domain.NoteBlock{{Kind: domain.NoteBlockText}}
	}
	m.blocks = make([]domain.NoteBlock, len(blocks))
	copy(m.blocks, blocks)
	lines := make([]string, len(blocks))
	for index, block := range blocks {
		if block.Kind == domain.NoteBlockImage {
			lines[index] = ""
			m.blocks[index].Text = ""
			continue
		}
		lines[index] = block.Text
	}
	m.doc = text.NewEditor(strings.Join(lines, "\n"))
	// A loaded note opens at its first line: the editor starts with the caret at
	// the end, which would open the note at its last one.
	m.doc.MoveTo(0, false)
	m.lines = lines
	m.linesDirty = false
	m.bodyLayout = bodyLayout{}
}

// createNote creates a note in the selected folder, or beside the selected
// note, and opens it with the title field focused.
func (m *Model) createNote() {
	if m.repo == nil {
		return
	}
	parent := m.newParent()
	if parent == "" {
		m.toast("先建一个文件夹，笔记要放在文件夹里")
		return
	}
	m.flush()
	node, err := m.repo.CreateNoteNode(context.Background(), parent, domain.NoteKindNote, "")
	if err != nil {
		m.report(err)
		return
	}
	if !m.expanded[parent] {
		m.expanded[parent] = true
		if err := m.repo.SetNoteNodeExpanded(context.Background(), parent, true); err != nil {
			m.report(err)
		}
	}
	m.selected = ""
	m.Read()
	m.node = node.ID
	m.openNote(node.ID)
	m.focusTitle = true
	m.rebuildRows()
}

// newParent is the folder a new note or folder goes into: the selected folder,
// the selected note's folder, or the first root folder.
func (m *Model) newParent() string {
	if node, ok := m.nodes[m.node]; ok {
		if node.Kind == domain.NoteKindFolder {
			return node.ID
		}
		return node.ParentID
	}
	for _, id := range m.children[""] {
		if m.nodes[id].Kind == domain.NoteKindFolder {
			return id
		}
	}
	return ""
}

// openCreateFolder opens 新建文件夹 on the selected folder.
func (m *Model) openCreateFolder() {
	m.openCreateFolderAt(m.newParent())
}

// openCreateFolderAt opens 新建文件夹 inside parent.
func (m *Model) openCreateFolderAt(parent string) {
	m.form = nodeForm{open: true, mode: "folder", parent: parent}
	m.form.field.SetText("")
}

// openRename opens 重命名 for one node.
func (m *Model) openRename(id string) {
	node, ok := m.nodes[id]
	if !ok {
		return
	}
	m.form = nodeForm{open: true, mode: "rename", node: id}
	m.form.field.SetText(node.Title)
}

// askDelete opens the confirmation for one node.
func (m *Model) askDelete(id string) {
	node, ok := m.nodes[id]
	if !ok {
		return
	}
	title := domain.NoteDisplayTitle(node.Title)
	if node.Kind == domain.NoteKindFolder {
		title = strings.TrimSpace(node.Title)
	}
	m.remove = deleteForm{open: true, node: id, title: title}
}

// formDialog is 新建文件夹 / 重命名.
func (m *Model) formDialog(c ui.C) {
	title := "新建文件夹"
	placeholder := "文件夹名称"
	if m.form.mode == "rename" {
		title = "重命名"
		placeholder = "名称"
	}
	// The error belongs to the text it complained about: typing again removes it.
	if m.form.err != "" && m.form.field.Text() != m.form.errOn {
		m.form.err, m.form.errOn = "", ""
	}
	theme := c.Theme()
	ui.ModalWith(c, &m.form.open, title, ui.ModalSpec{Width: 380}, func() {
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			typography.Caption(c, "名称")
			field := ui.TextField(c, &m.form.field, "名称", placeholder).AutoFocus()
			if m.form.err != "" {
				field.Invalid(true)
				typography.Caption(c, m.form.err).Color(theme.Danger)
			}
		})
	}, func() {
		ui.Row(c).Gap(layout.Gap).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.form.open = false
			}
			if ui.Button(c, "确定").Clicked() {
				m.submitForm()
			}
		})
	})
}

// submitForm creates a folder or renames a node.
func (m *Model) submitForm() {
	name := strings.TrimSpace(m.form.field.Text())
	if name == "" {
		m.formFail("名称不能为空")
		return
	}
	ctx := context.Background()
	if m.form.mode == "rename" {
		if err := m.repo.RenameNoteNode(ctx, m.form.node, name); err != nil {
			m.formFail(err.Error())
			return
		}
		if m.form.node == m.selected {
			m.titleText = name
			m.title.SetText(name)
			m.markDirty()
		}
		m.form.open = false
		m.Read()
		return
	}
	folder, err := m.repo.CreateNoteNode(ctx, m.form.parent, domain.NoteKindFolder, name)
	if err != nil {
		m.formFail(err.Error())
		return
	}
	m.form.open = false
	m.Read()
	m.node = folder.ID
	m.expanded[folder.ID] = true
	// The folder opens in view: a new folder inside a folded one would
	// otherwise be created where the user cannot see it.
	if parent := folder.ParentID; parent != "" && !m.expanded[parent] {
		m.expanded[parent] = true
		if err := m.repo.SetNoteNodeExpanded(ctx, parent, true); err != nil {
			m.report(err)
		}
	}
	m.rebuildRows()
}

// formFail puts a sentence under the dialog's field, where the app's other forms
// report what stopped them; the dialog stays open on the text it complained
// about.
func (m *Model) formFail(sentence string) {
	m.form.err, m.form.errOn = sentence, m.form.field.Text()
}

// deleteDialog confirms deleting a node and its subtree.
func (m *Model) deleteDialog(c ui.C) {
	sentence := "删除后无法恢复。"
	if node, ok := m.nodes[m.remove.node]; ok && node.Kind == domain.NoteKindFolder {
		sentence = "文件夹里的笔记和图片会一并删除，无法恢复。"
	}
	ui.ModalWith(c, &m.remove.open, "删除「"+m.remove.title+"」？",
		ui.ModalSpec{Width: 420, Description: sentence}, nil, func() {
			ui.Row(c).Gap(layout.Gap).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Secondary().Clicked() {
					m.remove.open = false
				}
				if ui.Button(c, "删除").Destructive().Clicked() {
					m.confirmDelete()
				}
			})
		})
}

// confirmDelete removes the node and cleans the pictures it referenced.
func (m *Model) confirmDelete() {
	id := m.remove.node
	m.remove.open = false
	if m.repo == nil {
		return
	}
	deleted, err := m.repo.DeleteNoteNode(context.Background(), id)
	if err != nil {
		m.report(err)
		return
	}
	for _, path := range deleted.ImagePaths {
		m.dropImage(path)
	}
	if m.selected != "" && containsString(deleted.NoteIDs, m.selected) {
		m.selected = ""
		m.titleText = ""
		m.focusTitle = false
		m.blocks, m.lines = nil, nil
		m.doc = text.NewEditor("")
		m.history = nil
		m.phase, m.dirty = saveSaved, false
	}
	if m.node == id {
		m.node = ""
	}
	m.Read()
	m.rebuildRows()
}

// subtreeContains reports whether target is inside root's subtree.
func (m *Model) subtreeContains(root, target string) bool {
	for id := target; id != ""; id = m.nodes[id].ParentID {
		if id == root {
			return true
		}
	}
	return false
}

// noticeEdits reads the title and the body and marks the draft dirty when they
// changed since the last frame.
func (m *Model) noticeEdits(now time.Duration) {
	if m.selected == "" || m.repo == nil {
		return
	}
	title := m.title.Text()
	if title != m.titleText {
		m.titleText = title
		if node, ok := m.nodes[m.selected]; ok {
			node.Title = domain.NoteTitle(title)
			m.nodes[m.selected] = node
			m.rowsDirty = true
		}
		m.markEdited(now)
	}
}

// markEdited records a change that waits for the pause.
func (m *Model) markEdited(now time.Duration) {
	m.phase, m.dirty = savePending, true
	m.editedAt = now
}

// markDirty marks a change for the next flush.
func (m *Model) markDirty() {
	m.phase, m.dirty = savePending, true
}

// settleDraft writes the draft when the pause elapsed.
func (m *Model) settleDraft(c ui.C) {
	if !m.dirty {
		return
	}
	wait := saveDelay - (c.Canvas().Now() - m.editedAt)
	if wait > 0 {
		c.Canvas().RequestFrameIn(wait)
		return
	}
	m.flush()
}

// flush writes the open note. A failed write keeps the draft and reports it.
func (m *Model) flush() {
	if m.repo == nil || m.selected == "" || !m.dirty {
		return
	}
	m.syncLines()
	node := m.nodes[m.selected]
	node.Title = m.titleText
	blocks := make([]domain.NoteBlock, len(m.blocks))
	for index, block := range m.blocks {
		block.NoteID = m.selected
		if block.Kind == domain.NoteBlockImage {
			block.Text = ""
		} else if index < len(m.lines) {
			block.Text = m.lines[index]
		}
		blocks[index] = block
	}
	saved, err := m.repo.SaveNote(context.Background(), domain.Note{Node: node, Blocks: blocks})
	if err != nil {
		m.phase = saveFailed
		m.report(err)
		return
	}
	m.phase, m.dirty = saveSaved, false
	m.nodes[m.selected] = saved.Node
	m.blocks = saved.Blocks
	m.lines = make([]string, len(m.blocks))
	for index := range m.blocks {
		if m.blocks[index].Kind == domain.NoteBlockImage {
			m.lines[index] = ""
			continue
		}
		m.lines[index] = m.blocks[index].Text
	}
	m.rowsDirty = true
}

// syncLines splits the editor's text into the block lines. It never pads or
// truncates: the caller that changed the text keeps the block list in step.
func (m *Model) syncLines() {
	m.lines = strings.Split(m.doc.String(), "\n")
}

// report shows a write or read failure and keeps it on the page.
func (m *Model) report(err error) {
	if err == nil {
		return
	}
	m.errorMessage = err.Error()
	m.toaster.Show(err.Error(), "", ui.ToastError)
}

// toast is a short informational message.
func (m *Model) toast(text string) {
	m.errorMessage = ""
	m.toaster.Show(text, "", ui.ToastDefault)
}

func containsString(list []string, value string) bool {
	for _, one := range list {
		if one == value {
			return true
		}
	}
	return false
}
