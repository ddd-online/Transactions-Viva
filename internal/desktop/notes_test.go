package desktop

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
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

// TestNotesFolderButtonMakesAFolder presses the band's bare folder command: it
// opens the 新建文件夹 dialog, and 确定 makes the folder in the open note's tree.
func TestNotesFolderButtonMakesAFolder(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	click(t, driver, "新建文件夹")
	if !shows(driver, "新建文件夹") || !hasControl(driver, "名称") {
		t.Fatal("the folder command should open the 新建文件夹 dialog")
	}
	if err := driver.Type("工作"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "确定")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "工作") {
		t.Fatal("the new folder should appear in the tree")
	}
	nodes, err := model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.Kind == domain.NoteKindFolder && node.Title == "工作" {
			return
		}
	}
	t.Fatalf("no 工作 folder in %+v", nodes)
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

// noteHitRole is the role of the control named name, or zero when the name is
// only painted text.
func noteHitRole(t *testing.T, driver *app.Driver, name string) gfx.Role {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return hit.Role
		}
	}
	return 0
}

// TestNotesTitleIsAField drives the band's title: it is a field from the start,
// Enter in it leaves for the body, and what is typed is the note's title.
func TestNotesTitleIsAField(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if role := noteHitRole(t, driver, "笔记标题"); role != gfx.RoleTextField {
		t.Fatalf("the title has role %v, want a field", role)
	}
	if err := driver.Type("周会"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	// Enter leaves the title for the body, so the next words are the note's.
	if err := driver.Type("第一行"); err != nil {
		t.Fatalf("typing after Enter in the title: %v", err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "周会")
	if len(note.Blocks) == 0 || note.Blocks[0].Text != "第一行" {
		t.Fatalf("blocks = %+v", note.Blocks)
	}

	// The title is edited by clicking it, with no mode to enter first.
	click(t, driver, "笔记标题")
	if err := driver.Key(gfx.KeyEnd, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("纪要"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	if note := noteByTitle(t, model, "周会纪要"); note.Node.Title != "周会纪要" {
		t.Fatalf("stored title = %q", note.Node.Title)
	}
}

// TestNotesRailToggleHidesTheTree hides the tree from the band's panel button
// and brings it back: the note keeps the window while the tree is away, and the
// workspace remembers the choice.
func TestNotesRailToggleHidesTheTree(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("周报"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	narrow := noteControlBounds(t, driver, "笔记正文")

	click(t, driver, "隐藏目录")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if hasControl(driver, "笔记 周报") {
		t.Fatal("隐藏目录 should take the tree away")
	}
	if wide := noteControlBounds(t, driver, "笔记正文"); wide.Width <= narrow.Width {
		t.Fatalf("the note did not take the tree's room: %v then %v", narrow.Width, wide.Width)
	}
	hidden, err := model.workspace.NoteRailHidden(context.Background())
	if err != nil {
		t.Fatalf("NoteRailHidden: %v", err)
	}
	if !hidden {
		t.Fatal("the workspace should remember the hidden tree")
	}

	click(t, driver, "显示目录")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "笔记 周报") {
		t.Fatal("显示目录 should bring the tree back")
	}
}

// TestNotesBackspaceMergesAndUndoRestores drives the block semantics and the// TestNotesBackspaceMergesAndUndoRestores drives the block semantics and the
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

	insertPicture(t, driver)
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

// TestNotesSlashMenuPicksAHeading types "/" in an empty block: the command menu
// opens, the arrows and Enter choose 标题 1, and the note takes the keyboard back
// for the text that follows.
func TestNotesSlashMenuPicksAHeading(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("斜杠"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("/"); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "标题 1") {
		t.Fatal("typing / should open the command menu")
	}
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("小节"); err != nil {
		t.Fatalf("typing after the command menu: %v", err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "斜杠")
	if len(note.Blocks) != 1 || note.Blocks[0].Kind != domain.NoteBlockHeading ||
		note.Blocks[0].Level != 1 || note.Blocks[0].Text != "小节" {
		t.Fatalf("blocks after the command menu = %+v", note.Blocks)
	}
}

// TestNotesCommandMenuOpensOnTheDunhao types "、" in an empty block: Chinese
// prose reaches for 顿号 where a form reaches for "/", so the same menu opens,
// the mark itself is never inserted, and the note keeps its text.
func TestNotesCommandMenuOpensOnTheDunhao(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("顿号"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("、"); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "标题 1") {
		t.Fatal("typing 、 should open the command menu")
	}
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("小节"); err != nil {
		t.Fatalf("typing after the command menu: %v", err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "顿号")
	if len(note.Blocks) != 1 || note.Blocks[0].Kind != domain.NoteBlockHeading ||
		note.Blocks[0].Level != 1 || note.Blocks[0].Text != "小节" {
		t.Fatalf("blocks after the command menu = %+v", note.Blocks)
	}
}

// blockBand finds the wash the body paints behind a block: a rounded rectangle
// that starts left of the block's painted row and reaches past it, one line tall.
// The grip's own box is square and narrow, so it cannot pass for one.
func blockBand(driver *app.Driver, row gfx.Rect) (gfx.Rect, bool) {
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Kind != app.RectanglePrimitive || primitive.Radius <= 0 {
			continue
		}
		bounds := primitive.Bounds
		if bounds.X >= row.X || bounds.Width <= row.Width || bounds.Height != row.Height {
			continue
		}
		if bounds.Y > row.Y || bounds.Y+bounds.Height < row.Y+row.Height {
			continue
		}
		return bounds, true
	}
	return gfx.Rect{}, false
}

// TestNotesBlockBandWaitsForTheGrip: a pointer over a block paints its grip and
// nothing else; only a pointer on the grip itself washes the block, and the wash
// covers that block alone.
func TestNotesBlockBandWaitsForTheGrip(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("底色"); err != nil {
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

	first := noteTextBounds(t, driver, "甲")
	if err := driver.Move(first.X+first.Width/2, first.Y+first.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if band, ok := blockBand(driver, first); ok {
		t.Fatalf("the band %v should wait for the pointer to reach the grip", band)
	}

	hoverBlock(t, driver, "甲")
	band, ok := blockBand(driver, first)
	if !ok {
		t.Fatal("a pointer on the grip should wash the block it drags")
	}
	second := noteTextBounds(t, driver, "乙")
	if band.Y+band.Height > second.Y {
		t.Fatalf("the band %v runs into the block below it %v", band, second)
	}
}

// typeDivider types the three marks that turn an empty block into a rule.
func typeDivider(t *testing.T, driver *app.Driver) {
	t.Helper()
	for range 3 {
		if err := driver.Type("-"); err != nil {
			t.Fatal(err)
		}
	}
}

// dividerRule finds the hairline the body paints for a divider: one logical pixel
// tall and as wide as the text column.
func dividerRule(driver *app.Driver, column float32) (gfx.Rect, bool) {
	for _, primitive := range driver.Snapshot().Primitives {
		bounds := primitive.Bounds
		if primitive.Kind != app.RectanglePrimitive || bounds.Height > 1.5 {
			continue
		}
		if bounds.Width < column*0.9 || bounds.Width > column*1.1 {
			continue
		}
		return bounds, true
	}
	return gfx.Rect{}, false
}

// TestNotesGripStandsClearOfTheBlockWash: the handle and the band behind its block
// are two shapes. The band starts to the right of the grip's box, so nothing of the
// wash runs under the handle and cuts its corner off.
func TestNotesGripStandsClearOfTheBlockWash(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("间隙"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("甲"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	// The pointer comes onto the block first, so its grip is declared, and then
	// onto the grip itself.
	row := noteTextBounds(t, driver, "甲")
	if err := driver.Move(row.X+row.Width/2, row.Y+row.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	hoverBlock(t, driver, "甲")
	band, ok := blockBand(driver, noteTextBounds(t, driver, "甲"))
	if !ok {
		t.Fatal("a pointer on the grip should wash the block it drags")
	}
	grip := noteControlBounds(t, driver, "块操作")
	if band.X < grip.X+grip.Width {
		t.Fatalf("the band %v runs under the grip %v", band, grip)
	}
}

// TestNotesDividerFromMarks types "---" on an empty line: the marks are the
// command, so the block becomes a rule, the text keeps no dashes, a text line
// opens under it, and the next keystroke lands there.
func TestNotesDividerFromMarks(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("横线"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	typeDivider(t, driver)
	if err := driver.Type("下一段"); err != nil {
		t.Fatalf("typing under the rule: %v", err)
	}
	advance(t, driver)
	blocks := noteByTitle(t, model, "横线").Blocks
	if len(blocks) != 2 || blocks[0].Kind != domain.NoteBlockDivider || blocks[0].Text != "" {
		t.Fatalf("blocks after --- = %+v", blocks)
	}
	if blocks[1].Kind != domain.NoteBlockText || blocks[1].Text != "下一段" {
		t.Fatalf("the text under the rule = %+v", blocks[1])
	}
}

// TestNotesDividerFromTheCommandMenu chooses 分隔线 in the "/" menu: the block it
// was opened on becomes a rule, and the note takes the keyboard back for the line
// under it.
func TestNotesDividerFromTheCommandMenu(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("斜线"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("/"); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "分隔线") {
		t.Fatal("the command menu should offer 分隔线")
	}
	click(t, driver, "分隔线")
	if err := driver.Type("下一段"); err != nil {
		t.Fatalf("typing after the command menu: %v", err)
	}
	advance(t, driver)
	blocks := noteByTitle(t, model, "斜线").Blocks
	if len(blocks) != 2 || blocks[0].Kind != domain.NoteBlockDivider {
		t.Fatalf("blocks after the menu = %+v", blocks)
	}
	if blocks[1].Kind != domain.NoteBlockText || blocks[1].Text != "下一段" {
		t.Fatalf("the text under the rule = %+v", blocks[1])
	}
}

// TestNotesDividerInTheMiddleKeepsTheLineUnderIt: a rule made on an empty line
// between two lines takes that line's place. The line under it is the note's own,
// so no empty block follows the rule, and the next word goes to that line.
func TestNotesDividerInTheMiddleKeepsTheLineUnderIt(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("夹线"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("上"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := driver.Key(gfx.KeyEnter, 0); err != nil {
			t.Fatal(err)
		}
	}
	// The caret goes back to the empty block between 上 and the one below it.
	if err := driver.Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("/"); err != nil {
		t.Fatal(err)
	}
	if !shows(driver, "分隔线") {
		t.Fatal("the command menu should offer 分隔线")
	}
	click(t, driver, "分隔线")
	if err := driver.Type("下"); err != nil {
		t.Fatalf("typing under the rule: %v", err)
	}
	advance(t, driver)
	blocks := noteByTitle(t, model, "夹线").Blocks
	if len(blocks) != 3 || blocks[1].Kind != domain.NoteBlockDivider || blocks[1].Text != "" {
		t.Fatalf("blocks around the rule = %+v", blocks)
	}
	if blocks[2].Kind != domain.NoteBlockText || blocks[2].Text != "下" {
		t.Fatalf("the word under the rule landed in %+v", blocks[2])
	}
}

// TestNotesDividerAfterALineKeepsTheNextLine: a rule put under a line of text
// leaves the line under it alone, so the note gains the rule and nothing else.
func TestNotesDividerAfterALineKeepsTheNextLine(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("分界"); err != nil {
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

	// The caret goes to the end of 甲, then the grip's menu inserts the rule
	// there: between the two lines.
	line := noteTextBounds(t, driver, "甲")
	if err := driver.Click(line.X+line.Width-1, line.Y+line.Height/2); err != nil {
		t.Fatal(err)
	}
	hoverBlock(t, driver, "甲")
	click(t, driver, "块操作")
	click(t, driver, "插入分隔线")
	advance(t, driver)

	blocks := noteByTitle(t, model, "分界").Blocks
	if len(blocks) != 3 {
		t.Fatalf("blocks = %+v, want 甲, the rule and 乙", blocks)
	}
	if blocks[0].Text != "甲" || blocks[1].Kind != domain.NoteBlockDivider ||
		blocks[1].Text != "" || blocks[2].Text != "乙" {
		t.Fatalf("blocks = %+v, want 甲, the rule and 乙", blocks)
	}
	if err := driver.Type("下"); err != nil {
		t.Fatalf("typing under the rule: %v", err)
	}
	advance(t, driver)
	blocks = noteByTitle(t, model, "分界").Blocks
	if len(blocks) != 3 || blocks[2].Text != "下乙" {
		t.Fatalf("the word typed under the rule landed in %+v", blocks)
	}
}

// TestNotesDividerTakesTheCaretBelowIt: a rule holds no caret, so a press on it
// puts the caret at the start of the text line under it, and the next word goes
// there rather than into the rule's own line.
func TestNotesDividerTakesTheCaretBelowIt(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("点击"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("上"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	typeDivider(t, driver)
	if err := driver.Type("下"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	line, ok := dividerRule(driver, noteTextBounds(t, driver, "上").Width)
	if !ok {
		t.Fatal("a divider should paint one hairline across the text column")
	}
	if err := driver.Click(line.X+40, line.Y+0.5); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("先"); err != nil {
		t.Fatalf("typing after a press on the rule: %v", err)
	}
	advance(t, driver)
	blocks := noteByTitle(t, model, "点击").Blocks
	if len(blocks) != 3 || blocks[1].Kind != domain.NoteBlockDivider || blocks[1].Text != "" {
		t.Fatalf("blocks around the rule = %+v", blocks)
	}
	if blocks[2].Text != "先下" {
		t.Fatalf("the word typed after the press landed in %q", blocks[2].Text)
	}
}

// TestNotesDividerKeepsTheTextUnderIt: the line of a rule holds no words, so a
// backspace at the start of the text under one may not join the two — that would
// give the rule a line nothing paints.
func TestNotesDividerKeepsTheTextUnderIt(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("退格"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	typeDivider(t, driver)
	if err := driver.Type("保住了"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	text := noteTextBounds(t, driver, "保住了")
	if err := driver.Click(text.X+2, text.Y+text.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyBackspace, 0); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	blocks := noteByTitle(t, model, "退格").Blocks
	if len(blocks) != 2 || blocks[0].Kind != domain.NoteBlockDivider || blocks[0].Text != "" {
		t.Fatalf("blocks after the backspace = %+v", blocks)
	}
	if blocks[1].Kind != domain.NoteBlockText || blocks[1].Text != "保住了" {
		t.Fatalf("the text under the rule = %+v", blocks[1])
	}
}

// writeLongNote writes a note whose body is taller than its pane: title, then
// lines of text, one per block.
func writeLongNote(t *testing.T, driver *app.Driver, title string, lines int) {
	t.Helper()
	createNote(t, driver)
	if err := driver.Type(title); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	for index := range lines {
		if err := driver.Type(fmt.Sprintf("第%d行", index+1)); err != nil {
			t.Fatal(err)
		}
		if err := driver.Key(gfx.KeyEnter, 0); err != nil {
			t.Fatal(err)
		}
	}
	advance(t, driver)
	// The caret ended on the last line, so the view is at the end of the note:
	// scroll it back to the first line for what the test does next.
	last := noteTextBounds(t, driver, fmt.Sprintf("第%d行", lines))
	if err := driver.Scroll(gfx.ScrollEvent{
		X: last.X + 40, Y: last.Y + last.Height/2, DeltaY: -100000,
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
}

// TestNotesWheelScrollsAwayFromTheCaret: the wheel moves the note, and the caret
// — which did not move — must not pull the view back to itself. The caret sits on
// the first line, so a scroll down is away from it.
func TestNotesWheelScrollsAwayFromTheCaret(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	writeLongNote(t, driver, "滚轮", 40)

	first := noteTextBounds(t, driver, "第1行")
	if err := driver.Click(first.X+20, first.Y+first.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	line := noteTextBounds(t, driver, "第10行")
	if err := driver.Scroll(gfx.ScrollEvent{
		X: line.X + 40, Y: line.Y + line.Height/2, DeltaY: 160,
	}); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if after := noteTextBounds(t, driver, "第10行"); after.Y >= line.Y-100 {
		t.Fatalf("the wheel did not move the note: 第10行 at y=%v, then y=%v", line.Y, after.Y)
	}
}

// TestNotesBodyShowsAScrollbar: a note taller than its pane has the scroll area's
// bar, named for the note, instead of scrolling with nothing to show for it.
func TestNotesBodyShowsAScrollbar(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	writeLongNote(t, driver, "滚动条", 40)

	first := noteTextBounds(t, driver, "第1行")
	if err := driver.Move(first.X+40, first.Y+first.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "笔记滚动条") {
		t.Fatalf("the note has no scrollbar over its long body")
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

// TestNotesBlockGripMenuTakesTheArrows opens the grip's menu from a focused
// body and picks a command with the keyboard: the menu's keys must not reach
// the note's text.
func TestNotesBlockGripMenuTakesTheArrows(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("按键"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("一段"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	hoverBlock(t, driver, "一段")
	click(t, driver, "块操作")
	if !shows(driver, "转为标题 2") {
		t.Fatal("the grip's menu did not open")
	}
	for range 3 {
		if err := driver.Key(gfx.KeyDown, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "按键")
	if len(note.Blocks) != 1 || note.Blocks[0].Kind != domain.NoteBlockHeading || note.Blocks[0].Level != 2 {
		t.Fatalf("blocks after the keys = %+v", note.Blocks)
	}
}

// TestNotesBlockGripMenuClosesOnEscape: Escape closes the grip's menu and the
// note takes the keyboard back, so the next keystroke is text again.
func TestNotesBlockGripMenuClosesOnEscape(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("撤退"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("一"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	hoverBlock(t, driver, "一")
	click(t, driver, "块操作")
	if !shows(driver, "转为标题 2") {
		t.Fatal("the grip's menu did not open")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if shows(driver, "转为标题 2") {
		t.Fatal("Escape should close the grip's menu")
	}
	if err := driver.Type("二"); err != nil {
		t.Fatalf("typing after the menu closed: %v", err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "撤退")
	if len(note.Blocks) != 1 || note.Blocks[0].Text != "一二" {
		t.Fatalf("blocks after Escape = %+v", note.Blocks)
	}
}

// TestNotesBlockGripMenuKeepsTheKeyboard chooses a command from the grip's menu
// and then types: the keyboard belongs to the note again, not to the menu's
// rows that just went away.
func TestNotesBlockGripMenuKeepsTheKeyboard(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("键盘"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("一段"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	hoverBlock(t, driver, "一段")
	click(t, driver, "块操作")
	click(t, driver, "转为标题 2")
	advance(t, driver)

	if err := driver.Type("续写"); err != nil {
		t.Fatalf("typing after the block command: %v", err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "键盘")
	if len(note.Blocks) == 0 || !strings.Contains(note.Blocks[0].Text, "续写") {
		t.Fatalf("blocks after the command = %+v", note.Blocks)
	}
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

// TestNotesRenameAndDeleteFromTheTree renames a note through its row's actions
// menu and deletes it with the confirmation.
func TestNotesRenameAndDeleteFromTheTree(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("旧名"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	openRowMenu(t, driver, "笔记 旧名")
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

	openRowMenu(t, driver, "笔记 "+renamed)
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

// TestNotesSplitterResizesTheRail drags the divider between the tree and the
// note: the column widens, the note moves over, and the workspace remembers the
// width the user left it at.
func TestNotesSplitterResizesTheRail(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	before := noteControlBounds(t, driver, "笔记正文")
	split := noteControlBounds(t, driver, "调整笔记目录宽度")
	x, y := split.X+split.Width/2, split.Y+40
	const step = 60
	if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := driver.Move(x+step, y); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Release(x+step, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	after := noteControlBounds(t, driver, "笔记正文")
	if grown := after.X - before.X; grown < step-8 || grown > step+8 {
		t.Fatalf("the note moved %v px, want about %d", grown, step)
	}
	width, err := model.workspace.NoteRailWidth(context.Background())
	if err != nil {
		t.Fatalf("NoteRailWidth: %v", err)
	}
	if width < layout.RailWidth+step-8 || width > layout.RailWidth+step+8 {
		t.Fatalf("stored rail width = %v, want about %v", width, layout.RailWidth+step)
	}
}

// TestNotesToolbarSearchTogglesTheField presses 搜索笔记 instead of the field:
// the field appears under the toolbar with the keyboard in it, and a second
// press hides it and clears what it held.
func TestNotesToolbarSearchTogglesTheField(t *testing.T) {
	driver, model := open(t)
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
	click(t, driver, "搜索笔记")
	if err := driver.Type("周"); err != nil {
		t.Fatalf("typing after 搜索笔记: %v", err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "笔记 周报") || hasControl(driver, "笔记 月报") {
		t.Fatal("搜索笔记 should open the field with the keyboard in it")
	}
	click(t, driver, "搜索笔记")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if hasControl(driver, "过滤笔记") {
		t.Fatal("the second press should hide the field")
	}
	if !hasControl(driver, "笔记 周报") || !hasControl(driver, "笔记 月报") {
		t.Fatal("hiding the field should clear the query")
	}

	// Escape is the other way out, and it clears the query too.
	click(t, driver, "搜索笔记")
	if err := driver.Type("周"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if hasControl(driver, "过滤笔记") {
		t.Fatal("Escape should hide the field")
	}
	if !hasControl(driver, "笔记 月报") {
		t.Fatal("Escape should clear the query")
	}
	_ = model
}

// TestNotesToolbarMakesANote presses 新建笔记: the note is created in the
// selected folder and opens with its title ready to be typed.
func TestNotesToolbarMakesANote(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	click(t, driver, "新建笔记")
	if !hasControl(driver, "笔记正文") {
		t.Fatal("新建笔记 should open the new note's body")
	}
	if err := driver.Type("清单"); err != nil {
		t.Fatalf("typing the new note's title: %v", err)
	}
	advance(t, driver)
	if note := noteByTitle(t, model, "清单"); note.Node.Title != "清单" {
		t.Fatalf("stored title = %q", note.Node.Title)
	}
}

// TestNotesToolbarCollapsesAndExpands presses 收起全部 and then 展开全部: the
// folder's notes leave the tree and come back, and the command renames itself.
func TestNotesToolbarCollapsesAndExpands(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("周报"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	if !hasControl(driver, "笔记 周报") {
		t.Fatal("the note should be in the tree")
	}
	click(t, driver, "收起全部")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if hasControl(driver, "笔记 周报") {
		t.Fatal("收起全部 should close the folder and hide its notes")
	}
	click(t, driver, "展开全部")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "笔记 周报") {
		t.Fatal("展开全部 should open the folder again")
	}
}

// TestNotesFilterShowsTheMatchingNotes filters the tree by title: 搜索笔记 opens
// the field, and what is typed keeps only the matching notes.
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
	click(t, driver, "搜索笔记")
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

// TestNotesFilterFindsFolders: the field searches folder names too. A folder that
// matches stays, opened, with the notes inside it; a note outside it goes.
func TestNotesFilterFindsFolders(t *testing.T) {
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
	click(t, driver, "新建文件夹")
	if err := driver.Type("工作"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "确定")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	dragRow(t, driver, "笔记 周报", "文件夹 工作", 0.5)
	advance(t, driver)
	// Fold the folder, so the search has to open the way to its notes itself.
	for hasControl(driver, "笔记 周报") {
		click(t, driver, "文件夹 工作")
		if err := driver.Paint(); err != nil {
			t.Fatal(err)
		}
	}

	click(t, driver, "搜索笔记")
	if err := driver.Type("工作"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasControl(driver, "文件夹 工作") {
		t.Fatal("the filter should keep the folder whose name matches")
	}
	if !hasControl(driver, "笔记 周报") {
		t.Fatal("a folder that matches should bring the notes inside it")
	}
	if hasControl(driver, "笔记 月报") {
		t.Fatal("the filter should hide the notes outside the matched folder")
	}
}

// openRowMenu presses the actions button of the tree row named name, which the
// row shows under the pointer: the pointer goes onto the row first, then onto
// the button that appears at its right.
func openRowMenu(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	row := noteControlBounds(t, driver, name)
	if err := driver.Move(row.X+row.Width/2, row.Y+row.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "更多操作 "+strings.TrimPrefix(name, "笔记 "))
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

// TestNotesTreeDragIntoAFolder drops a note onto a folder: the tree reports the
// middle of a folder row as a drop into it, and the note moves there.
func TestNotesTreeDragIntoAFolder(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("甲"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	click(t, driver, "新建文件夹")
	if err := driver.Type("工作"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "确定")
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}

	dragRow(t, driver, "笔记 甲", "文件夹 工作", 0.5)
	advance(t, driver)
	nodes, err := model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	folder := ""
	for _, node := range nodes {
		if node.Kind == domain.NoteKindFolder && node.Title == "工作" {
			folder = node.ID
		}
	}
	if folder == "" {
		t.Fatal("the 工作 folder is gone")
	}
	for _, node := range nodes {
		if node.Kind != domain.NoteKindNote {
			continue
		}
		if node.ParentID != folder {
			t.Fatalf("the note stayed under %q, want it inside 工作", node.ParentID)
		}
	}
}

// TestNotesTreeKeyboardOpensANote walks the tree with the keyboard: the rows
// take the arrows, and Enter opens the row the focus reached.
func TestNotesTreeKeyboardOpensANote(t *testing.T) {
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

	click(t, driver, "笔记 甲")
	for _, key := range []gfx.Key{gfx.KeyDown, gfx.KeyEnter} {
		if err := driver.Key(key, 0); err != nil {
			t.Fatal(err)
		}
	}
	click(t, driver, "笔记正文")
	if err := driver.Type("给乙"); err != nil {
		t.Fatalf("typing after the keyboard chose a row: %v", err)
	}
	advance(t, driver)
	note := noteByTitle(t, model, "乙")
	if len(note.Blocks) == 0 || note.Blocks[0].Text != "给乙" {
		t.Fatalf("the keyboard opened %+v instead of 乙", note.Blocks)
	}
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

	// Dragging down past a neighbour lands right below it, not at the end:
	// 乙 goes between 甲 and the note made last.
	createNote(t, driver)
	if err := driver.Type("丙"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	dragRow(t, driver, "笔记 乙", "笔记 丙", 0.75)
	advance(t, driver)
	order = nil
	nodes, err = model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.Kind == domain.NoteKindNote {
			order = append(order, node.Title)
		}
	}
	if len(order) != 3 || order[0] != "甲" || order[1] != "丙" || order[2] != "乙" {
		t.Fatalf("order after dragging down = %v, want [甲 丙 乙]", order)
	}
}

// dragRow drags the row named from onto the row named to, at share down that
// row: under half drops before it, over half after it.
func dragRow(t *testing.T, driver *app.Driver, from, to string, share float32) {
	t.Helper()
	source := noteControlBounds(t, driver, from)
	target := noteControlBounds(t, driver, to)
	x, y := target.X+target.Width/2, target.Y+target.Height*share
	if err := driver.Press(source.X+source.Width/2, source.Y+source.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := driver.Move(x, y); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.Release(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
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
	insertPicture(t, driver)

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
	// The picture is laid out inside nine tenths of the window, its own aspect
	// kept: a wide picture opens wide instead of in a box of a fixed size.
	view := noteControlBounds(t, driver, "笔记图片预览")
	snapshot := driver.Snapshot()
	if view.Width > float32(snapshot.Width)*0.9+1 || view.Height > float32(snapshot.Height)*0.9+1 {
		t.Fatalf("the lightbox view is %vx%v in a %dx%d window", view.Width, view.Height, snapshot.Width, snapshot.Height)
	}
	if ratio := view.Width / view.Height; ratio < 1.9 || ratio > 2.1 {
		t.Fatalf("the lightbox view is %vx%v, want the picture's 2:1", view.Width, view.Height)
	}
}

// createNote makes a note in the selected folder through the column's 新建笔记
// command, which the shell's tree no longer duplicates in a row menu.
func createNote(t *testing.T, driver *app.Driver) {
	t.Helper()
	click(t, driver, "新建笔记")
}

// insertPicture inserts a picture through the body's "/" menu, the only visible
// insert command now that the page band is gone.
func insertPicture(t *testing.T, driver *app.Driver) {
	t.Helper()
	click(t, driver, "笔记正文")
	if err := driver.Type("/"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "图片")
	advance(t, driver)
}

// TestNotesRowMenuOpensOnRightClick is the same command as the row's actions key:
// a secondary press anywhere on the row opens the menu for that row.
func TestNotesRowMenuOpensOnRightClick(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("右键的笔记"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)

	rightClickText(t, driver, "右键的笔记")
	if !shows(driver, "重命名") || !shows(driver, "删除") {
		t.Fatal("the row menu did not open on a right click")
	}
	// The menu belongs to the row that was pressed: its first command acts on it.
	click(t, driver, "重命名")
	if !hasControl(driver, "确定") {
		t.Fatal("the rename dialog did not open")
	}
}

// TestNotesBlockShortcuts is the chord the block menus print on their rows: the
// block the caret is in becomes the kind the row promises, without touching the
// menu. The rows and the shortcut read the same table, so they cannot disagree.
func TestNotesBlockShortcuts(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	// The new note's band takes the keyboard; the body is the control the block
	// commands act on, so the caret goes there first.
	click(t, driver, "笔记正文")
	if err := driver.Type("第一行"); err != nil {
		t.Fatal(err)
	}
	advance(t, driver)
	// A chord with Control is a raw key: it reaches the page's Shortcut and there is
	// no terminal to type into, which the driver reports as ErrNoTerminal.
	if err := driver.Chord(ui.Primary|ui.Alt, "2"); err != nil && !errors.Is(err, app.ErrNoTerminal) {
		t.Fatal(err)
	}
	advance(t, driver)

	nodes, err := model.workspace.NoteTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	noteID := ""
	for _, node := range nodes {
		if node.Kind == domain.NoteKindNote {
			noteID = node.ID
		}
	}
	if noteID == "" {
		t.Fatalf("the tree holds no note: %+v", nodes)
	}
	note, err := model.workspace.Note(context.Background(), noteID)
	if err != nil {
		t.Fatal(err)
	}
	if len(note.Blocks) != 1 {
		t.Fatalf("the note holds %+v", note.Blocks)
	}
	if block := note.Blocks[0]; block.Kind != domain.NoteBlockHeading || block.Level != 2 {
		t.Fatalf("the block is %+v, want a level-2 heading", block)
	}
}
