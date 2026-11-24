package store

import (
	"context"
	"testing"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// testNoteFolder creates one folder under the root for the note tests.
func testNoteFolder(t *testing.T, workspace *Workspace, name string) domain.NoteNode {
	t.Helper()
	folder, err := workspace.CreateNoteNode(context.Background(), "", domain.NoteKindFolder, name)
	if err != nil {
		t.Fatalf("CreateNoteNode(folder): %v", err)
	}
	return folder
}

func TestNotesTreeCreateMoveAndOrder(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	folder := testNoteFolder(t, workspace, "工作")

	first, err := workspace.CreateNoteNode(ctx, folder.ID, domain.NoteKindNote, "周会")
	if err != nil {
		t.Fatalf("CreateNoteNode(note): %v", err)
	}
	second, err := workspace.CreateNoteNode(ctx, folder.ID, domain.NoteKindNote, "")
	if err != nil {
		t.Fatalf("CreateNoteNode(second note): %v", err)
	}
	child, err := workspace.CreateNoteNode(ctx, folder.ID, domain.NoteKindFolder, "归档")
	if err != nil {
		t.Fatalf("CreateNoteNode(child folder): %v", err)
	}

	tree, err := workspace.NoteTree(ctx)
	if err != nil {
		t.Fatalf("NoteTree: %v", err)
	}
	if len(tree) != 4 {
		t.Fatalf("tree has %d nodes, want 4: %+v", len(tree), tree)
	}
	children := map[string]domain.NoteNode{}
	for _, node := range tree {
		children[node.ID] = node
	}
	if children[first.ID].SortOrder != 0 || children[second.ID].SortOrder != 1 || children[child.ID].SortOrder != 2 {
		t.Fatalf("creation order is not 0,1,2: %+v", tree)
	}

	// A note starts with one empty text block, so the editor always has a place.
	note, err := workspace.Note(ctx, first.ID)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if len(note.Blocks) != 1 || note.Blocks[0].Kind != domain.NoteBlockText || note.Blocks[0].Text != "" {
		t.Fatalf("a new note should hold one empty text block: %+v", note.Blocks)
	}

	// Moving to position 0 puts the folder at the top of its siblings.
	if err := workspace.MoveNoteNode(ctx, child.ID, folder.ID, 0); err != nil {
		t.Fatalf("MoveNoteNode: %v", err)
	}
	tree, err = workspace.NoteTree(ctx)
	if err != nil {
		t.Fatalf("NoteTree after move: %v", err)
	}
	for _, node := range tree {
		if node.ID == child.ID && node.SortOrder != 0 {
			t.Fatalf("moved node sort order = %d, want 0", node.SortOrder)
		}
		if node.ID == first.ID && node.SortOrder != 1 {
			t.Fatalf("first note sort order = %d, want 1", node.SortOrder)
		}
	}

	// Reordering writes exactly 0..n-1.
	if err := workspace.ReorderNoteNodes(ctx, folder.ID, []string{first.ID, child.ID, second.ID}); err != nil {
		t.Fatalf("ReorderNoteNodes: %v", err)
	}
	tree, _ = workspace.NoteTree(ctx)
	got := map[string]int{}
	for _, node := range tree {
		got[node.ID] = node.SortOrder
	}
	if got[first.ID] != 0 || got[child.ID] != 1 || got[second.ID] != 2 {
		t.Fatalf("reorder result = %+v", got)
	}

	// A note cannot live at the root; a folder cannot move into its own subtree.
	if _, err := workspace.CreateNoteNode(ctx, "", domain.NoteKindNote, "根级笔记"); err == nil {
		t.Fatal("root-level note was accepted")
	}
	if err := workspace.MoveNoteNode(ctx, folder.ID, child.ID, 0); err == nil {
		t.Fatal("a folder moved into its own subtree")
	}
	if err := workspace.MoveNoteNode(ctx, first.ID, second.ID, 0); err == nil {
		t.Fatal("a note was used as a parent")
	}
}

func TestNotesSaveBlocksDiff(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	folder := testNoteFolder(t, workspace, "工作")
	node, err := workspace.CreateNoteNode(ctx, folder.ID, domain.NoteKindNote, "")
	if err != nil {
		t.Fatalf("CreateNoteNode: %v", err)
	}
	note, err := workspace.Note(ctx, node.ID)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	first := note.Blocks[0]

	saved, err := workspace.SaveNote(ctx, domain.Note{
		Node: domain.NoteNode{ID: node.ID, Title: "第一篇"},
		Blocks: []domain.NoteBlock{
			{ID: first.ID, Kind: domain.NoteBlockHeading, Level: 9, Text: "标题"},
			{Kind: domain.NoteBlockText, Text: "正文"},
			{Kind: domain.NoteBlockImage, ImagePath: "notes/x/a.png", ImageWidth: 400, ImageHeight: 300, ImageScale: 0.5},
		},
	})
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if saved.Node.Title != "第一篇" {
		t.Fatalf("title = %q", saved.Node.Title)
	}
	if len(saved.Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3: %+v", len(saved.Blocks), saved.Blocks)
	}
	if saved.Blocks[0].Level != domain.NoteHeadingMaxLevel {
		t.Fatalf("heading level = %d, want it clamped to 5", saved.Blocks[0].Level)
	}
	if saved.Blocks[1].ID == "" || saved.Blocks[2].ID == "" {
		t.Fatalf("new blocks need ids: %+v", saved.Blocks)
	}
	if saved.Blocks[1].SortOrder != 1 || saved.Blocks[2].SortOrder != 2 {
		t.Fatalf("sort orders = %+v", saved.Blocks)
	}

	// Dropping the image block deletes its row; the rest is updated in place.
	again, err := workspace.SaveNote(ctx, domain.Note{
		Node: domain.NoteNode{ID: node.ID, Title: "第一篇"},
		Blocks: []domain.NoteBlock{
			{ID: saved.Blocks[0].ID, Kind: domain.NoteBlockHeading, Level: 2, Text: "标题"},
			{ID: saved.Blocks[1].ID, Kind: domain.NoteBlockText, Text: "正文改了"},
		},
	})
	if err != nil {
		t.Fatalf("SaveNote again: %v", err)
	}
	if len(again.Blocks) != 2 {
		t.Fatalf("blocks after delete = %d, want 2", len(again.Blocks))
	}
	if again.Blocks[0].ID != saved.Blocks[0].ID || again.Blocks[1].Text != "正文改了" {
		t.Fatalf("second save = %+v", again.Blocks)
	}
	if _, err := workspace.db.Exec("SELECT COUNT(*) FROM note_block WHERE note_id = ?", node.ID); err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	var count int
	if err := workspace.db.QueryRow("SELECT COUNT(*) FROM note_block WHERE note_id = ?", node.ID).Scan(&count); err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if count != 2 {
		t.Fatalf("note_block rows = %d, want 2", count)
	}

	// An empty block list still leaves one text block.
	empty, err := workspace.SaveNote(ctx, domain.Note{Node: domain.NoteNode{ID: node.ID}})
	if err != nil {
		t.Fatalf("SaveNote empty: %v", err)
	}
	if len(empty.Blocks) != 1 || empty.Blocks[0].Kind != domain.NoteBlockText {
		t.Fatalf("empty save = %+v", empty.Blocks)
	}
}

func TestNotesDeleteSubtreeAndExpanded(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	folder := testNoteFolder(t, workspace, "工作")
	child, err := workspace.CreateNoteNode(ctx, folder.ID, domain.NoteKindFolder, "归档")
	if err != nil {
		t.Fatalf("CreateNoteNode(child): %v", err)
	}
	note, err := workspace.CreateNoteNode(ctx, child.ID, domain.NoteKindNote, "记录")
	if err != nil {
		t.Fatalf("CreateNoteNode(note): %v", err)
	}
	if _, err := workspace.SaveNote(ctx, domain.Note{
		Node: domain.NoteNode{ID: note.ID},
		Blocks: []domain.NoteBlock{
			{Kind: domain.NoteBlockImage, ImagePath: "notes/a/a.png", ImageWidth: 1, ImageHeight: 1},
			{Kind: domain.NoteBlockText, Text: "正文"},
		},
	}); err != nil {
		t.Fatalf("SaveNote: %v", err)
	}

	if err := workspace.SetNoteNodeExpanded(ctx, folder.ID, true); err != nil {
		t.Fatalf("SetNoteNodeExpanded: %v", err)
	}
	tree, _ := workspace.NoteTree(ctx)
	found := false
	for _, node := range tree {
		if node.ID == folder.ID && node.Expanded {
			found = true
		}
	}
	if !found {
		t.Fatal("folder did not keep its expanded state")
	}

	deleted, err := workspace.DeleteNoteNode(ctx, folder.ID)
	if err != nil {
		t.Fatalf("DeleteNoteNode: %v", err)
	}
	if len(deleted.NoteIDs) != 1 || deleted.NoteIDs[0] != note.ID {
		t.Fatalf("deleted notes = %+v", deleted.NoteIDs)
	}
	if len(deleted.ImagePaths) != 1 || deleted.ImagePaths[0] != "notes/a/a.png" {
		t.Fatalf("deleted image paths = %+v", deleted.ImagePaths)
	}
	tree, _ = workspace.NoteTree(ctx)
	if len(tree) != 0 {
		t.Fatalf("tree after delete = %+v", tree)
	}
	var blocks int
	if err := workspace.db.QueryRow("SELECT COUNT(*) FROM note_block").Scan(&blocks); err != nil {
		t.Fatalf("count blocks: %v", err)
	}
	if blocks != 0 {
		t.Fatalf("note_block rows after delete = %d, want 0", blocks)
	}

	if _, err := workspace.DeleteNoteNode(ctx, folder.ID); err == nil {
		t.Fatal("deleting a missing node was accepted")
	}
	if _, err := workspace.Note(ctx, note.ID); err == nil {
		t.Fatal("a deleted note was readable")
	}
}

// TestNotesFoldersExpandAtOnce drives the tree's 展开全部 / 收起全部 command: one
// statement opens or closes every folder, and a note's stored state is left as
// it was.
func TestNotesFoldersExpandAtOnce(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	outer := testNoteFolder(t, workspace, "工作")
	inner, err := workspace.CreateNoteNode(ctx, outer.ID, domain.NoteKindFolder, "归档")
	if err != nil {
		t.Fatalf("CreateNoteNode: %v", err)
	}
	if err := workspace.SetNoteFoldersExpanded(ctx, true); err != nil {
		t.Fatalf("SetNoteFoldersExpanded(true): %v", err)
	}
	states := func() map[string]bool {
		tree, err := workspace.NoteTree(ctx)
		if err != nil {
			t.Fatalf("NoteTree: %v", err)
		}
		expanded := map[string]bool{}
		for _, node := range tree {
			if node.Kind == domain.NoteKindFolder {
				expanded[node.ID] = node.Expanded
			}
		}
		return expanded
	}
	if got := states(); !got[outer.ID] || !got[inner.ID] {
		t.Fatalf("after 展开全部 = %+v, want both folders open", got)
	}
	if err := workspace.SetNoteFoldersExpanded(ctx, false); err != nil {
		t.Fatalf("SetNoteFoldersExpanded(false): %v", err)
	}
	if got := states(); got[outer.ID] || got[inner.ID] {
		t.Fatalf("after 收起全部 = %+v, want both folders closed", got)
	}
}

func TestNotesRenameAndLastNote(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	folder := testNoteFolder(t, workspace, "工作")
	note, err := workspace.CreateNoteNode(ctx, folder.ID, domain.NoteKindNote, "")
	if err != nil {
		t.Fatalf("CreateNoteNode: %v", err)
	}

	if err := workspace.RenameNoteNode(ctx, note.ID, "  改名  "); err != nil {
		t.Fatalf("RenameNoteNode: %v", err)
	}
	read, err := workspace.Note(ctx, note.ID)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if read.Node.Title != "改名" {
		t.Fatalf("title = %q, want trimmed", read.Node.Title)
	}
	if err := workspace.RenameNoteNode(ctx, "missing", "x"); err == nil {
		t.Fatal("renaming a missing node was accepted")
	}

	if got, err := workspace.LastNoteID(ctx); err != nil || got != "" {
		t.Fatalf("LastNoteID before any save = %q, %v", got, err)
	}
	if err := workspace.SetLastNoteID(ctx, note.ID); err != nil {
		t.Fatalf("SetLastNoteID: %v", err)
	}
	if got, err := workspace.LastNoteID(ctx); err != nil || got != note.ID {
		t.Fatalf("LastNoteID = %q, %v", got, err)
	}
	// A deleted note is not reported as the last one.
	if _, err := workspace.DeleteNoteNode(ctx, note.ID); err != nil {
		t.Fatalf("DeleteNoteNode: %v", err)
	}
	if got, err := workspace.LastNoteID(ctx); err != nil || got != "" {
		t.Fatalf("LastNoteID after delete = %q, %v", got, err)
	}
}

// TestNotesRailWidthIsRemembered stores the width the user dragged the divider
// to: unset reads as zero, a stored width comes back, and a value that is not a
// width reads as unset rather than as a pane of no width. The tree's visibility
// rides the same app_meta row.
func TestNotesRailWidthIsRemembered(t *testing.T) {
	ctx := context.Background()
	workspace := testWorkspace(t)
	if got, err := workspace.NoteRailWidth(ctx); err != nil || got != 0 {
		t.Fatalf("NoteRailWidth before any drag = %v, %v", got, err)
	}
	if err := workspace.SetNoteRailWidth(ctx, 342.5); err != nil {
		t.Fatalf("SetNoteRailWidth: %v", err)
	}
	if got, err := workspace.NoteRailWidth(ctx); err != nil || got != 342.5 {
		t.Fatalf("NoteRailWidth = %v, %v", got, err)
	}
	if err := workspace.SetNoteRailWidth(ctx, 0); err == nil {
		t.Fatal("a zero width was stored")
	}
	if _, err := workspace.db.ExecContext(ctx,
		"UPDATE app_meta SET value = 'wide' WHERE key = 'note_rail_width'"); err != nil {
		t.Fatalf("corrupt the width: %v", err)
	}
	if got, err := workspace.NoteRailWidth(ctx); err != nil || got != 0 {
		t.Fatalf("NoteRailWidth of a corrupt value = %v, %v", got, err)
	}

	if hidden, err := workspace.NoteRailHidden(ctx); err != nil || hidden {
		t.Fatalf("NoteRailHidden before any hide = %v, %v", hidden, err)
	}
	if err := workspace.SetNoteRailHidden(ctx, true); err != nil {
		t.Fatalf("SetNoteRailHidden(true): %v", err)
	}
	if hidden, err := workspace.NoteRailHidden(ctx); err != nil || !hidden {
		t.Fatalf("NoteRailHidden = %v, %v", hidden, err)
	}
	if err := workspace.SetNoteRailHidden(ctx, false); err != nil {
		t.Fatalf("SetNoteRailHidden(false): %v", err)
	}
	if hidden, err := workspace.NoteRailHidden(ctx); err != nil || hidden {
		t.Fatalf("NoteRailHidden after showing again = %v, %v", hidden, err)
	}
}

// The store implements the page's notes contract.
var _ repository.NotesRepository = (*Workspace)(nil)
