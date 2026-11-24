package desktop

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/gfx"

	"transactions/internal/domain"
)

// noteByTitle finds the only note whose title is title and reads it back.
func noteByTitle(t *testing.T, model *Model, title string) domain.Note {
	t.Helper()
	ctx := context.Background()
	nodes, err := model.workspace.NoteTree(ctx)
	if err != nil {
		t.Fatalf("NoteTree: %v", err)
	}
	for _, node := range nodes {
		if node.Kind != domain.NoteKindNote || node.Title != title {
			continue
		}
		note, err := model.workspace.Note(ctx, node.ID)
		if err != nil {
			t.Fatalf("Note: %v", err)
		}
		return note
	}
	t.Fatalf("no note titled %q in the workspace", title)
	return domain.Note{}
}

// TestNotesFirstVisitCreatesTheFolder drives the empty workspace into 笔记: the
// first visit makes the default folder, then 新建笔记 creates and opens a note.
func TestNotesFirstVisitCreatesTheFolder(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	if !shows(driver, "我的笔记") {
		t.Fatal("the first visit should create the default folder")
	}
	click(t, driver, "新建笔记")
	if !hasControl(driver, "笔记正文") {
		t.Fatal("a new note should open its body editor")
	}
}

// TestNotesTypesTitleAndBlocks writes a title and two text blocks, waits for the
// autosave and reads them back from the workspace.
func TestNotesTypesTitleAndBlocks(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	click(t, driver, "新建笔记")
	if err := driver.Type("周会"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("第一行"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("第二行"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	note := noteByTitle(t, model, "周会")
	if len(note.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2: %+v", len(note.Blocks), note.Blocks)
	}
	if note.Blocks[0].Text != "第一行" || note.Blocks[1].Text != "第二行" {
		t.Fatalf("blocks = %+v", note.Blocks)
	}
	if note.Blocks[0].Kind != domain.NoteBlockText {
		t.Fatalf("first block kind = %q", note.Blocks[0].Kind)
	}
}

// TestNotesBackspaceMergesAndUndoRestores drives the block semantics and the
// snapshot undo: Enter splits, Backspace at the start merges, Ctrl+Z restores.
func TestNotesBackspaceMergesAndUndoRestores(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	click(t, driver, "新建笔记")
	if err := driver.Type("会"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("一二"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("三四"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "会")
	if len(note.Blocks) != 2 {
		t.Fatalf("after Enter blocks = %d, want 2", len(note.Blocks))
	}

	// Backspace at the second block's start joins it into the first.
	if err := driver.Key(gfx.KeyHome, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	note = noteByTitle(t, model, "会")
	if len(note.Blocks) != 1 || note.Blocks[0].Text != "一二三四" {
		t.Fatalf("after merge = %+v", note.Blocks)
	}

	// Ctrl+Z restores both the text and the block list.
	if err := driver.Chord(gfx.ModifierControl, "z"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	note = noteByTitle(t, model, "会")
	if len(note.Blocks) != 2 || note.Blocks[0].Text != "一二" || note.Blocks[1].Text != "三四" {
		t.Fatalf("after undo = %+v", note.Blocks)
	}
}

// TestNotesSlashMenuTurnsABlockIntoAHeading opens the "/" menu and takes 标题 1.
func TestNotesSlashMenuTurnsABlockIntoAHeading(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	click(t, driver, "新建笔记")
	if err := driver.Type("大纲"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("/"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "标题 1")
	if err := driver.Type("开会"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "大纲")
	if len(note.Blocks) != 1 {
		t.Fatalf("blocks = %+v", note.Blocks)
	}
	if note.Blocks[0].Kind != domain.NoteBlockHeading || note.Blocks[0].Level != 1 || note.Blocks[0].Text != "开会" {
		t.Fatalf("heading block = %+v", note.Blocks[0])
	}
}

// TestNotesImageBlockIsStored inserts one picture through the shell's picker and
// checks that the block keeps the stored path and the picture's size.
func TestNotesImageBlockIsStored(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	click(t, driver, "新建笔记")
	if err := driver.Type("配图"); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(t.TempDir(), "dot.png")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			canvas.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	if err := png.Encode(file, canvas); err != nil {
		t.Fatal(err)
	}
	file.Close()
	model.pickImages = func() ([]string, error) { return []string{source}, nil }

	click(t, driver, "图片")
	advance(t, driver)
	note := noteByTitle(t, model, "配图")
	found := false
	for _, block := range note.Blocks {
		if block.Kind != domain.NoteBlockImage {
			continue
		}
		found = true
		if block.ImageWidth != 40 || block.ImageHeight != 30 {
			t.Fatalf("image size = %dx%d", block.ImageWidth, block.ImageHeight)
		}
		if block.ImagePath == "" {
			t.Fatal("image block has no path")
		}
		full, err := filepath.Abs(filepath.Join(model.workspace.Directory(), "data", "assets", filepath.FromSlash(block.ImagePath)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("stored image missing: %v", err)
		}
	}
	if !found {
		t.Fatalf("no image block in %+v", note.Blocks)
	}
}
