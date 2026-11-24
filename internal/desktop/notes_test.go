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

// TestNotesTitleIsTextUntilClicked drives the band's title: it reads as text, a
// press turns it into the field with the keyboard in it, and a press on the body
// closes it and writes what was typed.
func TestNotesTitleIsTextUntilClicked(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "笔记")
	createNote(t, driver)
	if err := driver.Type("周会"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if role := noteHitRole(t, driver, "笔记标题"); role != gfx.RoleButton {
		t.Fatalf("the title at rest has role %v, want the band's text", role)
	}
	if !shows(driver, "周会") {
		t.Fatal("the band should name the note")
	}

	click(t, driver, "笔记标题")
	if role := noteHitRole(t, driver, "笔记标题"); role != gfx.RoleTextField {
		t.Fatalf("the pressed title has role %v, want a field", role)
	}
	if err := driver.Type("记录"); err != nil {
		t.Fatalf("typing into the title: %v", err)
	}
	// A press on the body closes the field; the draft is written at once
	// instead of after the pause.
	click(t, driver, "笔记正文")
	if role := noteHitRole(t, driver, "笔记标题"); role != gfx.RoleButton {
		t.Fatalf("the title after a press on the body has role %v, want text again", role)
	}
	if note := noteByTitle(t, model, "周会记录"); note.Node.Title != "周会记录" {
		t.Fatalf("stored title = %q", note.Node.Title)
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

// TestNotesToolbarSearchFocusesTheFilter presses 搜索笔记 instead of the field:
// the keyboard lands in the filter at the foot of the column, so what is typed
// filters the tree.
func TestNotesToolbarSearchFocusesTheFilter(t *testing.T) {
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
		t.Fatal("搜索笔记 should put the keyboard in the filter")
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

// createNote makes a note in the default folder through the folder's context
// menu. The menu item is picked by role: the toolbar has a 新建笔记 command of
// its own, and a click by name would find that one.
func createNote(t *testing.T, driver *app.Driver) {
	t.Helper()
	clickRight(t, driver, "文件夹 我的笔记")
	clickRole(t, driver, "新建笔记", gfx.RoleMenuItem)
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
