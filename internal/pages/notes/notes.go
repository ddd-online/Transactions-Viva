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
	"transactions/internal/components/layout"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
)

// saveDelay is how long the editor waits after the last keystroke before it
// writes, the same 600 ms 日记 uses.
const saveDelay = 600 * time.Millisecond

// titleFreshPaints is how many paints a freshly opened title field has to take
// the keyboard: the platform answers its AutoFocus after the first one.
const titleFreshPaints = 2

// treeIndent is one level of the notes tree, and treeRowPad the inset before
// the first level.
const (
	treeIndent = 16
	treeRowPad = 8
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
}

// treeRow is one visible row of the tree.
type treeRow struct {
	node   string
	depth  int
	branch bool
}

// nodeForm is the 新建文件夹 / 重命名 dialog.
type nodeForm struct {
	open   bool
	mode   string // "folder" (create) or "rename"
	node   string // the node renamed
	parent string // the folder a new folder goes into
	field  ui.Field
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
	// visible part of it, rebuilt when the tree, a fold or the filter changed.
	nodes     map[string]domain.NoteNode
	noteCount int
	children  map[string][]string
	rows      []treeRow
	rowsDirty bool
	expanded  map[string]bool
	// node is the selected tree row (a folder or a note), selected the open
	// note. They differ while a folder is selected.
	node     string
	selected string

	filter     ui.Field
	filterText string

	// title is the note's title. The band paints titleText as text and puts
	// the field in place only while the title is being edited; titleID is the
	// field's identity while it is painted.
	title        ui.Field
	titleText    string
	titleID      gfx.ID
	titleEditing bool

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
	scroll     float32
	// selectedImage is the picture block the user clicked, or -1.
	selectedImage int

	history  []snapshot
	histAt   time.Duration
	histOpen bool

	// save state
	phase    savePhase
	dirty    bool
	editedAt time.Duration

	// drag and drop between tree rows, and between blocks
	drag       ui.DragState
	rowHit     map[gfx.ID]string
	rowRect    map[string]gfx.Rect
	treeWhere  int
	treeFrom   string
	treeTo     string
	blockDrag  ui.DragState
	hoverBlock int
	dragBlock  int
	dropAfter  bool
	blockMenu  blockMenuState
	resize     resizeState

	// pictures: the open note's decoded blocks and the lightbox's one picture
	imageEffect    *ui.EffectState[map[string]gfx.Bitmap, noteImageKey]
	noteImages     map[string]gfx.Bitmap
	lightboxEffect *ui.EffectState[gfx.Bitmap, string]
	lightbox       lightboxState

	// dialogs
	form   nodeForm
	remove deleteForm

	toaster      ui.Toaster
	errorMessage string

	// titleFresh counts the paints the title's field gets to take the keyboard
	// before a paint without it closes the field again.
	titleFresh int

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
	// decodes the note's pictures runs again; lightboxRev does the same for the
	// lightbox, whose effect keeps its last key.
	imageRev    int
	imageKey    noteImageKey
	lightboxRev int

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
		rowHit:        map[gfx.ID]string{},
		rowRect:       map[string]gfx.Rect{},
		noteImages:    map[string]gfx.Bitmap{},
		selectedImage: -1,
		hoverBlock:    -1,
		dragBlock:     -1,
		blockMenu:     blockMenuState{block: -1},
		resize:        resizeState{block: -1},
		doc:           text.NewEditor(""),
		lightbox:      lightboxState{},
	}
	m.bodyDraw = m.drawBody
	m.bodyInput.Key = m.bodyKeyEvent
	m.bodyInput.Pointer = m.bodyPointer
	m.bodyInput.Scroll = m.bodyScroll
	m.bodyInput.Text = m.bodyText
	m.imageEffect = ui.NewEffectKeyed(m.loadNoteImages)
	m.lightboxEffect = ui.NewEffectKeyed(m.loadLightboxImage)
	m.lightbox.handlers.Role = gfx.RoleGroup
	m.lightbox.handlers.Pointer = m.lightboxPointer
	m.lightbox.handlers.Scroll = m.lightboxScroll
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
	m.node, m.selected = "", ""
	m.titleText = ""
	m.titleEditing, m.titleFresh = false, 0
	m.phase, m.dirty = saveSaved, false
	m.errorMessage = ""
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
	m.titleEditing, m.titleFresh = false, 0
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
	if m.selected != "" && !m.bodyKey.IsZero() && c.Canvas().Focused() == m.bodyKey &&
		ui.Shortcut(c, ui.Primary, "z") && m.undoneAt != m.now {
		m.undo()
		m.undoneAt = m.now
	}
	m.noticeEdits(m.now)
	m.settleDraft(c)
	if m.rowsDirty {
		m.rebuildRows()
	}
	if m.focusBody && !m.bodyKey.IsZero() {
		c.Canvas().RequestFocus(m.bodyKey)
		m.focusBody = false
	}
	ui.Effect(c, m.imageEffect, m.imageKey)
	if images, err, ok := m.imageEffect.Take(); ok {
		if err == nil {
			m.noteImages = images
		}
	}
	if m.lightbox.open && m.lightbox.path != "" {
		ui.Effect(c, m.lightboxEffect, m.lightboxKey())
		if bitmap, _, ok := m.lightboxEffect.Take(); ok {
			m.lightbox.bitmap = bitmap
			m.lightbox.loaded = true
		}
	}
	ui.Row(c).Grow(1).FillWidth().Children(func() {
		m.rail(c)
		ui.Divider(c)
		m.detail(c)
	})
	m.formDialog(c)
	m.deleteDialog(c)
	m.lightboxView(c)
	ui.Toasts(c, &m.toaster)
}

// rail is the tree column: the section's title and count with the create
// command, the tree, and the title filter pinned at the foot where it belongs
// to the list it filters.
func (m *Model) rail(c ui.C) {
	text := strings.TrimSpace(m.filter.Text())
	if text != m.filterText {
		m.filterText = text
		m.rowsDirty = true
		m.rebuildRows()
	}
	layout.Rail(c, layout.RailWidth, func(c ui.C) {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, "笔记")
			layout.SectionCount(c, strconv.Itoa(m.noteCount))
			ui.Spacer(c)
			if ui.Button(c, "新建文件夹").Outline().Icon(icons.FolderPlus).IconOnly().
				Tooltip("新建文件夹").Clicked() {
				m.openCreateFolder()
			}
		})
		ui.Divider(c)
		if len(m.rows) == 0 && m.filterText != "" {
			emptystate.Notice(c, "没有匹配的笔记")
		} else {
			// The rows are copied because a click may rebuild the tree under us.
			rows := append([]treeRow(nil), m.rows...)
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().PadY(4).Children(func() {
					for _, row := range rows {
						m.rowView(c, row)
					}
				})
			})
		}
		ui.Divider(c)
		ui.Column(c).FillWidth().PadX(layout.Inset).PadY(8).Children(func() {
			ui.TextField(c, &m.filter, "过滤笔记", "过滤标题").FillWidth()
		})
	})
}

// rowView is one row of the tree: the chevron of a folder, the icon, the name,
// its commands on a right click, and its place in a drag.
func (m *Model) rowView(c ui.C, row treeRow) {
	node, ok := m.nodes[row.node]
	if !ok {
		return
	}
	theme := c.Theme()
	selected := row.node == m.node || row.node == m.selected
	name := strings.TrimSpace(node.Title)
	if node.Kind == domain.NoteKindNote {
		name = domain.NoteDisplayTitle(node.Title)
	}
	c.Key("note-row-" + row.node)
	id := ui.ID(c)
	m.rowHit[id] = row.node
	label := "文件夹 " + name
	if node.Kind == domain.NoteKindNote {
		label = "笔记 " + name
	}
	box := ui.Row(c).FillWidth().MinHeight(layout.RowHeight).MaxHeight(layout.RowHeight).
		PadLeft(treeRowPad+float32(row.depth)*treeIndent).PadRight(treeRowPad).
		Gap(4).Items(ui.Center).
		HitWith(id, label, gfx.HitOptions{Role: gfx.RoleOption, Selected: selected, Cursor: gfx.PointerCursorHand}).
		Draw(func(d *ui.DrawContext) { m.rowRect[row.node] = d.Rect })
	if selected {
		box = box.Background(state.Selected(*theme))
	} else {
		box = box.Hover(ui.Look{}.Background(state.Hover(*theme)))
	}
	box = box.DragSource(&m.drag).DropTarget(&m.drag, m.acceptDrop, m.dropRow)
	ui.ContextMenu(c, box, func(menu *ui.MenuItems) {
		if node.Kind == domain.NoteKindFolder {
			if menu.Item("新建笔记").Clicked() {
				m.node = node.ID
				m.createNote()
			}
			if menu.Item("新建文件夹").Clicked() {
				m.openCreateFolderAt(node.ID)
			}
			menu.Separator()
		}
		if menu.Item("重命名").Clicked() {
			m.openRename(node.ID)
		}
		if menu.Item("删除").Destructive().Clicked() {
			m.askDelete(node.ID)
		}
	})
	if box.Clicked() {
		m.selectNode(row.node)
	}
	box.Children(func() {
		if row.branch && node.Kind == domain.NoteKindFolder {
			open := m.expanded[node.ID] || m.filterText != ""
			icon := icons.ChevronRight
			if open {
				icon = icons.ChevronDown
			}
			c.Key("note-chevron-" + row.node)
			toggler := ui.Row(c).Size(16, 16).Center().
				HitWith(ui.ID(c), "展开或收起 "+name, gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand})
			if toggler.Clicked() {
				m.toggleFolder(node.ID)
			}
			toggler.Children(func() {
				ui.Icon(c, icon).Size(14, 14).Color(theme.MutedForeground)
			})
		} else {
			// A leaf keeps the chevron's room so names line up down a column.
			ui.Row(c).Width(16)
		}
		icon := icons.Note
		if node.Kind == domain.NoteKindFolder {
			icon = icons.Folder
		}
		ui.Icon(c, icon).Size(15, 15).Color(theme.MutedForeground)
		row := ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			typography.Body(c, name).MaxLines(1).Ellipsis().Shrink(1).MinWidth(0)
		})
		_ = row
	})
}

// detail is the right column: the note's band, then the block editor. The band
// spans the column the way 日记's day band does, so the page opens on the note
// with its title where a document's title belongs.
func (m *Model) detail(c ui.C) {
	if m.selected == "" {
		emptystate.Page(c, &icons.Note, "选择一篇笔记",
			"在左侧文件夹上右键，选择「新建笔记」。", nil)
		return
	}
	ui.Column(c).Grow(1).FillHeight().FillWidth().Children(func() {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			m.titleBar(c)
		})
		ui.Divider(c)
		ui.Column(c).Grow(1).FillWidth().PadX(layout.Inset).Children(func() {
			m.bodyView(c)
		})
	})
}

// titleBar is the band: the note's title and, at the right, the draft's state.
// The title reads as text; a press turns it into the field, Enter or a press
// anywhere else closes it again.
func (m *Model) titleBar(c ui.C) {
	theme := c.Theme()
	if m.titleEditing {
		m.titleField(c)
		m.saveState(c)
		return
	}
	c.Key("note-title")
	name := strings.TrimSpace(m.titleText)
	empty := name == ""
	if empty {
		name = domain.NoteDisplayTitle("")
	}
	titler := ui.Row(c).Grow(1).MinWidth(0).FillHeight().Items(ui.Center).
		HitWith(ui.ID(c), "笔记标题", gfx.HitOptions{
			Role: gfx.RoleButton, Value: name, Cursor: gfx.PointerCursorText,
		})
	if titler.Clicked() {
		m.beginTitle()
	}
	titler.Children(func() {
		label := typography.Section(c, name).MaxLines(1).Ellipsis().Shrink(1).MinWidth(0)
		if empty {
			label = label.Color(theme.MutedForeground)
		}
	})
	m.saveState(c)
}

// titleField is the title while it is being edited: the note's one text field,
// so a title is typed, selected and pasted like any other field. It takes the
// keyboard the first time it is painted and gives it back when the user goes
// elsewhere.
func (m *Model) titleField(c ui.C) {
	c.Key("note-title-field")
	m.titleID = ui.ID(c)
	field := ui.TextFieldID(c, m.titleID, &m.title, "笔记标题", "未命名笔记").
		FontSize(typography.SizeSection).Weight(gfx.WeightBold).Grow(1).MinWidth(0).AutoFocus()
	switch {
	case field.Submitted():
		m.commitTitle()
		c.Canvas().RequestFocus(m.bodyKey)
	case m.titleFresh > 0:
		// The field asked for the keyboard this paint; the platform answers
		// after it.
		m.titleFresh--
	case c.Canvas().Focused() != m.titleID:
		m.commitTitle()
	}
}

// beginTitle turns the title into its field.
func (m *Model) beginTitle() {
	m.titleEditing = true
	m.titleFresh = titleFreshPaints
}

// commitTitle closes the field. The text is already the page's — noticeEdits
// reads it every frame — so closing only writes the draft now instead of after
// the pause.
func (m *Model) commitTitle() {
	m.titleEditing = false
	m.titleFresh = 0
	if m.dirty {
		m.flush()
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
func (m *Model) rebuildRows() {
	m.rowsDirty = false
	rows := m.rows[:0]
	visible := map[string]bool{}
	forced := map[string]bool{}
	if m.filterText != "" {
		needle := strings.ToLower(m.filterText)
		for id, node := range m.nodes {
			if node.Kind != domain.NoteKindNote {
				continue
			}
			if !strings.Contains(strings.ToLower(domain.NoteDisplayTitle(node.Title)), needle) {
				continue
			}
			visible[id] = true
			for parent := node.ParentID; parent != ""; parent = m.nodes[parent].ParentID {
				visible[parent] = true
				forced[parent] = true
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
			rows = append(rows, treeRow{node: id, depth: depth, branch: len(m.children[id]) > 0})
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
	m.titleEditing, m.titleFresh = false, 0
	m.loadBlocks(note.Blocks)
	m.history = nil
	m.phase, m.dirty = saveSaved, false
	m.bodyKey = gfx.ID{}
	m.bodyLayout = bodyLayout{}
	m.scroll = 0
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
	m.titleEditing = true
	m.titleFresh = titleFreshPaints
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
	if m.form.mode == "rename" {
		title = "重命名"
	}
	ui.ModalWith(c, &m.form.open, title, ui.ModalSpec{Width: 380}, func() {
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			typography.Caption(c, "名称")
			ui.TextField(c, &m.form.field, "名称", "文件夹名称").AutoFocus()
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
		m.toast("名称不能为空")
		return
	}
	ctx := context.Background()
	if m.form.mode == "rename" {
		if err := m.repo.RenameNoteNode(ctx, m.form.node, name); err != nil {
			m.report(err)
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
		m.report(err)
		return
	}
	m.form.open = false
	m.Read()
	m.node = folder.ID
	m.expanded[folder.ID] = true
	m.rebuildRows()
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
		m.titleEditing, m.titleFresh = false, 0
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

// acceptDrop decides whether a dragged row may land on another.
func (m *Model) acceptDrop(source, target gfx.ID) bool {
	from, to := m.rowHit[source], m.rowHit[target]
	if from == "" || to == "" {
		return false
	}
	where := m.dropWhere(from, to)
	if !m.dropAllowed(from, to, where) {
		return false
	}
	// The drag state is reset before the drop callback runs, so the placement
	// is remembered while the pointer still tells where it is.
	m.treeFrom, m.treeTo, m.treeWhere = from, to, where
	return true
}

// dropWhere is where a drag would land on a target: -1 above it, 0 into a
// folder, 1 below it.
func (m *Model) dropWhere(from, to string) int {
	rect, ok := m.rowRect[to]
	if !ok {
		return 0
	}
	at := m.drag.At().Y
	if m.nodes[to].Kind == domain.NoteKindFolder {
		switch {
		case at < rect.Y+rect.Height*0.3:
			return -1
		case at > rect.Y+rect.Height*0.7:
			return 1
		}
		return 0
	}
	if at > rect.Y+rect.Height/2 {
		return 1
	}
	return -1
}

// dropAllowed reports whether a drop is legal: never into itself or its own
// subtree, never into a note, and never onto the root.
func (m *Model) dropAllowed(from, to string, where int) bool {
	if from == to || m.nodes[from].ID == "" || m.nodes[to].ID == "" {
		return false
	}
	if m.subtreeContains(from, to) {
		return false
	}
	if where == 0 {
		return m.nodes[to].Kind == domain.NoteKindFolder
	}
	parent := m.nodes[to].ParentID
	return parent != "" || m.nodes[from].Kind == domain.NoteKindFolder
}

// dropRow moves a dragged row where it was let go.
func (m *Model) dropRow(source, target gfx.ID) {
	from, to := m.treeFrom, m.treeTo
	if m.repo == nil || from == "" || to == "" {
		from, to = m.rowHit[source], m.rowHit[target]
	}
	m.treeFrom, m.treeTo = "", ""
	where := m.treeWhere
	if from == "" || to == "" || !m.dropAllowed(from, to, where) {
		return
	}
	where = m.treeWhere
	if where == 1 {
		parent := m.nodes[to].ParentID
		position := len(m.children[parent])
		for index, id := range m.children[parent] {
			if id == to {
				position = index + 1
				break
			}
		}
		if err := m.repo.MoveNoteNode(context.Background(), from, parent, position); err != nil {
			m.report(err)
			return
		}
		m.Read()
		if parent != "" {
			m.expanded[parent] = true
		}
		m.rebuildRows()
		return
	}
	parent := m.nodes[to].ParentID
	position := 0
	if where == 0 {
		parent = to
		position = -1
	} else {
		for index, id := range m.children[parent] {
			if id == to {
				position = index
				break
			}
		}
	}
	if err := m.repo.MoveNoteNode(context.Background(), from, parent, position); err != nil {
		m.report(err)
		return
	}
	m.Read()
	if parent != "" {
		m.expanded[parent] = true
	}
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
