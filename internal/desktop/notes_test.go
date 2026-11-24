package desktop

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
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
	createNote(t, driver)
	if !hasControl(driver, "笔记正文") {
		t.Fatal("a new note should open its body editor")
	}
}

// TestNotesTypesTitleAndBlocks writes a title and two text blocks, waits for the
// autosave and reads them back from the workspace.
func TestNotesTypesTitleAndBlocks(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
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
	createNote(t, driver)
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
	createNote(t, driver)
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
	createNote(t, driver)
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
	model.pickNoteFile = func() (string, error) { return source, nil }

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

	// The picture gets a text block after it, and typing goes there.
	if err := driver.Type("插图说明"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	note = noteByTitle(t, model, "配图")
	last := note.Blocks[len(note.Blocks)-1]
	if last.Kind != domain.NoteBlockText || last.Text != "插图说明" {
		t.Fatalf("the text after the picture = %+v", note.Blocks)
	}
}

// noteTextBounds finds the painted text equal to value, which locates a block or
// a picture on the body.
func noteTextBounds(t *testing.T, driver *app.Driver, value string) gfx.Rect {
	t.Helper()
	for _, label := range driver.Snapshot().Labels {
		if label.Value == value {
			return label.Bounds
		}
	}
	t.Fatalf("no painted text %q on screen", value)
	return gfx.Rect{}
}

// hoverBlock moves the pointer onto a block so its grip is painted.
func hoverBlock(t *testing.T, driver *app.Driver, text string) {
	t.Helper()
	bounds := noteTextBounds(t, driver, text)
	if err := driver.Move(bounds.X-16, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
}

// TestNotesBlockGripConvertsAHeading uses the block grip's menu to turn a text
// block into a heading.
func TestNotesBlockGripConvertsAHeading(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("grip"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("小节"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	hoverBlock(t, driver, "小节")
	click(t, driver, "块操作")
	click(t, driver, "转为标题 2")
	advance(t, driver)

	note := noteByTitle(t, model, "grip")
	if note.Blocks[0].Kind != domain.NoteBlockHeading || note.Blocks[0].Level != 2 {
		t.Fatalf("block = %+v", note.Blocks[0])
	}
}

// TestNotesBlockGripDragReorders drags the grip of the first block under the
// second one.
func TestNotesBlockGripDragReorders(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("drag"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("甲"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("乙"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	hoverBlock(t, driver, "甲")
	var grip gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "块操作" {
			grip = hit.Bounds
		}
	}
	if grip.Width == 0 {
		t.Fatal("the block grip was not painted")
	}
	second := noteTextBounds(t, driver, "乙")
	if err := driver.Press(grip.X+grip.Width/2, grip.Y+grip.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(second.X+40, second.Y+second.Height-3); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(second.X+40, second.Y+second.Height-3); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(second.X+40, second.Y+second.Height-3, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	note := noteByTitle(t, model, "drag")
	if len(note.Blocks) != 2 || note.Blocks[0].Text != "乙" || note.Blocks[1].Text != "甲" {
		t.Fatalf("blocks after drag = %+v", note.Blocks)
	}
}

// TestNotesRenameAndDeleteFromTheTree renames a note through the tree's context
// menu and deletes it with the confirmation.
func TestNotesRenameAndDeleteFromTheTree(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("旧名"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	clickRight(t, driver, "笔记 旧名")
	click(t, driver, "重命名")
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "名称" && hit.Role == gfx.RoleTextField {
			if err := driver.Click(hit.Bounds.X+hit.Bounds.Width-8, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := driver.Type("新名"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "确定")
	advance(t, driver)

	left, err := model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	renamed := ""
	for _, node := range left {
		if node.Kind == domain.NoteKindNote {
			renamed = node.Title
		}
	}
	if !strings.Contains(renamed, "新名") {
		t.Fatalf("the note kept the old title %q", renamed)
	}

	clickRight(t, driver, "笔记 "+renamed)
	click(t, driver, "删除")
	click(t, driver, "删除")
	advance(t, driver)
	left, err = model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range left {
		if node.Kind == domain.NoteKindNote {
			t.Fatalf("note survived deletion: %+v", node)
		}
	}
}

// TestNotesFilterShowsTheMatchingNotes filters the tree by title.
func TestNotesFilterShowsTheMatchingNotes(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("周报"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	createNote(t, driver)
	if err := driver.Type("月报"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	click(t, driver, "过滤笔记")
	if err := driver.Type("周"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "笔记 周报") || hasControl(driver, "笔记 月报") {
		t.Fatal("the filter should keep only 周报")
	}
}

// clickRight presses the secondary button on the control named name.
func clickRight(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			b := hit.Bounds
			if err := driver.ClickButton(b.X+b.Width/2, b.Y+b.Height/2, gfx.PointerButtonSecondary); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}

// noteControlBounds finds one control by its name.
func noteControlBounds(t *testing.T, driver *app.Driver, name string) gfx.Rect {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return hit.Bounds
		}
	}
	t.Fatalf("no control named %q on screen", name)
	return gfx.Rect{}
}

// TestNotesTreeDragReorders drags the note 甲 below 乙 in the same folder.
func TestNotesTreeDragReorders(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("甲"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	createNote(t, driver)
	if err := driver.Type("乙"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	from := noteControlBounds(t, driver, "笔记 甲")
	to := noteControlBounds(t, driver, "笔记 乙")
	if err := driver.Press(from.X+from.Width/2, from.Y+from.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(to.X+to.Width/2, to.Y+to.Height-4); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(to.X+to.Width/2, to.Y+to.Height-4); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(to.X+to.Width/2, to.Y+to.Height-4, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	nodes, err := model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, node := range nodes {
		if node.Kind == domain.NoteKindNote {
			order = append(order, node.Title)
		}
	}
	if len(order) != 2 || order[0] != "乙" || order[1] != "甲" {
		t.Fatalf("order after drag = %v, want [乙 甲]", order)
	}
}

// TestNotesFeatureSwitchHidesTheSection turns 笔记 off in settings and checks the
// navigation loses the section.
func TestNotesFeatureSwitchHidesTheSection(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "设置")
	click(t, driver, "笔记功能开关")
	if !model.config.Features.Notes {
		// The label names the switch; the click turned it off.
	}
	if hasControl(driver, "笔记") {
		t.Fatal("笔记 is still in the navigation after being switched off")
	}
}

// TestNotesImageSelectAndLightbox selects a picture and opens it on the second
// press; the lightbox is a modal titled 笔记图片.
func TestNotesImageSelectAndLightbox(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("看图"); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(t.TempDir(), "wide.png")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 120, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 120; x++ {
			canvas.Set(x, y, color.RGBA{R: 30, G: 120, B: 200, A: 255})
		}
	}
	if err := png.Encode(file, canvas); err != nil {
		t.Fatal(err)
	}
	file.Close()
	model.pickNoteFile = func() (string, error) { return source, nil }
	click(t, driver, "图片")
	advance(t, driver)

	body := noteControlBounds(t, driver, "笔记正文")
	centerX, centerY := body.X+body.Width/2, body.Y+60
	if err := driver.Click(centerX, centerY); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "调整图片高度") {
		t.Fatal("selecting the picture should show its resize handle")
	}
	if err := driver.Click(centerX, centerY); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "笔记图片") {
		t.Fatal("the second press should open the lightbox")
	}
}

// createNote makes a note in the default folder through its context menu: the
// header no longer has a create button.
func createNote(t *testing.T, driver *app.Driver) {
	t.Helper()
	clickRight(t, driver, "文件夹 我的笔记")
	click(t, driver, "新建笔记")
}
