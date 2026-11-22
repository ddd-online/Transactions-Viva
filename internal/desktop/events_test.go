package desktop

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gen2brain/h265/heic"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"

	"transactions/internal/assets"
	"transactions/internal/config"
	"transactions/internal/domain"
	"transactions/internal/store/storetest"
)

// testImage writes a small solid PNG and returns its path.
func testImage(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := range 20 {
		for x := range 40 {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 120, B: 60, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEventsAddEditColorAndDelete(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "事件")
	if !shows(driver, "暂无事件") {
		t.Fatal("an empty year should say it has no events")
	}
	if !shows(driver, "选择左侧事件查看详情") {
		t.Fatal("the detail pane has no empty state")
	}
	if !shows(driver, "选择事件查看关联交易") {
		t.Fatal("the linked pane has no empty state")
	}

	today := time.Now().Format("2006-01-02")
	click(t, driver, "新增事件")
	if !inDialog(driver, "新增") {
		t.Fatal("新增事件 did not open its dialog")
	}
	clickLast(t, driver, "新增")
	if !hasControl(driver, today) {
		t.Fatalf("the new event (%s) is not in the list", today)
	}
	if !shows(driver, "暂无描述") {
		t.Fatal("a new event has no body and should say 暂无描述")
	}

	click(t, driver, "编辑描述")
	click(t, driver, "事件描述")
	if err := driver.Type("抽奖与聚餐"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "保存")

	event, err := model.workspace.KeyEvent(context.Background(), today)
	if err != nil {
		t.Fatalf("the event was not written: %v", err)
	}
	if event.Content != "抽奖与聚餐" || event.Title != "抽奖与聚餐" {
		t.Fatalf("written event = %+v", event)
	}
	if !hasControl(driver, "抽奖与聚餐") {
		t.Fatal("the card did not take the body's first line as its title")
	}

	click(t, driver, "砖红")
	event, err = model.workspace.KeyEvent(context.Background(), today)
	if err != nil || event.Color != "#D9705A" {
		t.Fatalf("event colour = %q, err=%v", event.Color, err)
	}

	click(t, driver, "删除事件「抽奖与聚餐」")
	if !shows(driver, "事件里的图片会一并删除。") {
		t.Fatal("the delete confirmation does not warn about the pictures")
	}
	clickLast(t, driver, "删除")
	if !shows(driver, "暂无事件") {
		t.Fatal("the deleted event is still in the list")
	}
	if _, err := model.workspace.KeyEvent(context.Background(), today); err == nil {
		t.Fatal("the event row survived its deletion")
	}
}

func TestEventLinkedPanelSpansLedgers(t *testing.T) {
	driver, model := open(t)
	today := time.Now().Format("2006-01-02")
	ctx := context.Background()
	ledgers, err := model.workspace.Ledgers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var travelID string
	for _, ledger := range ledgers {
		if ledger.Name == "旅行账本" {
			travelID = ledger.ID
		}
	}
	if travelID == "" {
		t.Fatal("the fixture has no travel ledger")
	}
	if _, err := model.workspace.CreateRecord(ctx, domain.TransactionRecord{
		TransactionID: "event-travel-1", LedgerID: travelID,
		AmountCents: 128000, TransactionType: "expense", Category: "交通出行",
		Description: "去程机票", KeyEventDate: today,
		TransactionAt: time.Now().Unix(),
	}, []string{"旅行"}); err != nil {
		t.Fatal(err)
	}

	click(t, driver, "事件")
	click(t, driver, "新增事件")
	clickLast(t, driver, "新增")

	for _, want := range []string{"午餐 · 公司楼下", "去程机票", "家庭账本", "旅行账本", "-¥32.00"} {
		if !shows(driver, want) {
			t.Fatalf("the linked rail does not show %q", want)
		}
	}

	click(t, driver, "收起关联交易")
	if model.config.KeyEventLinkedOpen {
		t.Fatal("collapsing the rail did not reach the configuration")
	}
	if shows(driver, "午餐 · 公司楼下") {
		t.Fatal("the linked rail still paints after it was collapsed")
	}
	click(t, driver, "展开关联交易")
	if !model.config.KeyEventLinkedOpen {
		t.Fatal("expanding the rail did not reach the configuration")
	}
	if !shows(driver, "午餐 · 公司楼下") {
		t.Fatal("the linked rail did not come back")
	}
}

func TestEventImagesAddOpenSaveAndDelete(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "事件")
	click(t, driver, "新增事件")
	clickLast(t, driver, "新增")
	today := time.Now().Format("2006-01-02")

	source := testImage(t)
	if err := driver.QueueOpenFiles([]string{source}, nil); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "添加图片")
	label := "查看第 1 张图片，共 1 张"
	if !hasControl(driver, label) {
		t.Fatal("the imported picture is not in the gallery")
	}

	images, err := model.workspace.KeyEventImages(context.Background(), today)
	if err != nil || len(images) != 1 {
		t.Fatalf("stored images = %+v, err=%v", images, err)
	}
	directory := model.workspace.Directory()
	display := filepath.Join(directory, "data", "assets", filepath.FromSlash(images[0].FilePath))
	if _, err := os.Stat(display); err != nil {
		t.Fatalf("the display file is missing: %v", err)
	}

	click(t, driver, "查看大图")
	if !shows(driver, "1 / 1") {
		t.Fatal("the lightbox did not open on the only picture")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if shows(driver, "1 / 1") {
		t.Fatal("Escape did not close the lightbox")
	}

	destination := filepath.Join(t.TempDir(), "saved.png")
	if err := driver.QueueSaveFile(destination, nil); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "保存图片")
	want, err := os.ReadFile(display)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("saved copy differs, err=%v", err)
	}

	if hasControl(driver, "删除第 1 张图片") {
		t.Fatal("the thumbnail still carries its own delete button")
	}
	if !hasControl(driver, "删除图片") {
		t.Fatal("the stage has no delete button")
	}
	click(t, driver, "删除图片")
	if !shows(driver, "删除这张图片？") {
		t.Fatal("deleting a picture did not ask for confirmation")
	}
	clickLast(t, driver, "删除")
	if hasControl(driver, label) {
		t.Fatal("the deleted picture is still in the gallery")
	}
	if images, err := model.workspace.KeyEventImages(context.Background(), today); err != nil || len(images) != 0 {
		t.Fatalf("image rows after delete = %+v, err=%v", images, err)
	}
	if _, err := os.Stat(display); !os.IsNotExist(err) {
		t.Fatalf("the display file outlived its row: %v", err)
	}
}

// TestEventDeleteImageSelectsTheNextPicture covers the stage's delete action:
// the picture after the deleted one becomes the stage, and deleting the last
// picture falls back to the one before it.
func TestEventDeleteImageSelectsTheNextPicture(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "事件")
	click(t, driver, "新增事件")
	clickLast(t, driver, "新增")

	directory := t.TempDir()
	var sources []string
	for index := range 3 {
		data, err := os.ReadFile(testImage(t))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, "photo-"+strconv.Itoa(index+1)+".png")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, path)
	}
	if err := driver.QueueOpenFiles(sources, nil); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "添加图片")
	if !hasControl(driver, "查看第 1 张图片，共 3 张") {
		t.Fatal("the three pictures did not arrive in the gallery")
	}
	click(t, driver, "查看第 1 张图片，共 3 张")
	click(t, driver, "删除图片")
	clickLast(t, driver, "删除")
	if !hasControl(driver, "查看第 1 张图片，共 2 张") {
		t.Fatal("deleting the first picture did not leave the other two")
	}
	if !selectedControl(driver, "查看第 1 张图片，共 2 张") {
		t.Fatal("after deleting the first picture the next one is not selected")
	}

	click(t, driver, "查看第 2 张图片，共 2 张")
	click(t, driver, "删除图片")
	clickLast(t, driver, "删除")
	if !hasControl(driver, "查看第 1 张图片，共 1 张") {
		t.Fatal("deleting the last picture did not leave the earlier one")
	}
	if !selectedControl(driver, "查看第 1 张图片，共 1 张") {
		t.Fatal("deleting the last picture did not fall back to the previous one")
	}
}

func selectedControl(driver *app.Driver, name string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && hit.Selected {
			return true
		}
	}
	return false
}

func TestEventImageProblemsAreReported(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "事件")
	click(t, driver, "新增事件")
	clickLast(t, driver, "新增")

	picked := t.TempDir()
	movie := filepath.Join(picked, "IMG_0001.MOV")
	if err := os.WriteFile(movie, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{movie}, nil); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "添加图片")
	if !shows(driver, "实况视频要和同名的照片一起选") {
		t.Fatal("an orphan live video was not reported")
	}
}

func TestEventYearMovesTheList(t *testing.T) {
	driver, _ := open(t)
	click(t, driver, "事件")
	click(t, driver, "新增事件")
	clickLast(t, driver, "新增")
	if !hasControl(driver, time.Now().Format("2006-01-02")) {
		t.Fatal("the new event is not in this year")
	}

	click(t, driver, "上一周期")
	previous := strconv.Itoa(time.Now().Year() - 1)
	if !hasControl(driver, previous) {
		t.Fatal("the year picker did not move back a year")
	}
	if !shows(driver, "暂无事件") {
		t.Fatal("last year has no events but the list is not empty")
	}
	click(t, driver, "下一周期")
	if !hasControl(driver, time.Now().Format("2006-01-02")) {
		t.Fatal("moving forward did not bring this year's event back")
	}
}

func TestEventLinksToggleWritesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.FileName)
	settings := config.Load(path)
	cfg := settings.Config()
	cfg.Appearance = config.AppearanceLight
	model := NewWithConfig(cfg, settings)
	fixture := storetest.Open(t)
	if err := model.adoptWorkspace(fixture.Workspace); err != nil {
		t.Fatalf("adopt workspace: %v", err)
	}
	driver, _ := openModel(t, model)
	click(t, driver, "事件")
	click(t, driver, "收起关联交易")
	if reloaded := config.Load(path).Config(); reloaded.KeyEventLinkedOpen {
		t.Fatal("the collapsed rail was not written to the configuration")
	}
	click(t, driver, "展开关联交易")
	if reloaded := config.Load(path).Config(); !reloaded.KeyEventLinkedOpen {
		t.Fatal("the expanded rail was not written to the configuration")
	}
}

func TestEventLinksToggleFailureKeepsTheRail(t *testing.T) {
	// A configuration store whose path is a directory cannot write; the rail
	// must stay as it was and the page must say why.
	model := NewWithConfig(config.Default(), config.Load(t.TempDir()))
	fixture := storetest.Open(t)
	if err := model.adoptWorkspace(fixture.Workspace); err != nil {
		t.Fatalf("adopt workspace: %v", err)
	}
	driver, _ := openModel(t, model)
	click(t, driver, "事件")
	click(t, driver, "收起关联交易")
	if !shows(driver, "保存关联交易展开状态失败") {
		t.Fatal("a failed configuration write was silent")
	}
	if !shows(driver, "选择事件查看关联交易") {
		t.Fatal("the rail closed even though its state was not saved")
	}
}

func TestEventImageImportsHEIC(t *testing.T) {
	driver, model := open(t)
	click(t, driver, "事件")
	click(t, driver, "新增事件")
	clickLast(t, driver, "新增")
	today := time.Now().Format("2006-01-02")

	picture := image.NewRGBA(image.Rect(0, 0, 32, 20))
	for y := range 20 {
		for x := range 32 {
			picture.SetRGBA(x, y, color.RGBA{R: 120, G: 160, B: 90, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := heic.Encode(&buffer, picture); err != nil {
		t.Fatalf("heic.Encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "IMG_0200.HEIC")
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "添加图片")
	if !hasControl(driver, "查看第 1 张图片，共 1 张") {
		t.Fatal("the HEIC picture is not in the gallery")
	}

	images, err := model.workspace.KeyEventImages(context.Background(), today)
	if err != nil || len(images) != 1 {
		t.Fatalf("stored images = %+v, err=%v", images, err)
	}
	if !strings.HasSuffix(images[0].FilePath, ".jpg") || !strings.HasSuffix(images[0].OriginalPath, ".heic") {
		t.Fatalf("HEIC row paths = %+v", images[0])
	}
	for _, relative := range []string{images[0].FilePath, images[0].ThumbPath, images[0].OriginalPath} {
		full := filepath.Join(model.workspace.Directory(), "data", "assets", filepath.FromSlash(relative))
		if _, err := os.Stat(full); err != nil {
			t.Fatalf("stored file %q: %v", relative, err)
		}
	}
}

// TestEventGalleryWithManyImagesStaysInsideTheWindow guards the layout bug a
// long thumbnail column caused: the scroll area reported its content as its
// preferred height, stretched the gallery, and pushed the stage, the body and
// the footer past the bottom of the window.
func TestEventGalleryWithManyImagesStaysInsideTheWindow(t *testing.T) {
	driver, model := open(t)
	today := time.Now().Format("2006-01-02")
	ctx := context.Background()
	if _, err := model.workspace.SaveKeyEvent(ctx, today, "多图", "", ""); err != nil {
		t.Fatalf("SaveKeyEvent: %v", err)
	}
	directory := model.workspace.Directory()
	for index := range 12 {
		picture := image.NewRGBA(image.Rect(0, 0, 40, 30))
		for y := range 30 {
			for x := range 40 {
				picture.SetRGBA(x, y, color.RGBA{R: uint8(40 + index*12), G: 120, B: 160, A: 255})
			}
		}
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, picture); err != nil {
			t.Fatal(err)
		}
		display := buffer.Bytes()
		decoded, err := assets.Decode(display)
		if err != nil {
			t.Fatal(err)
		}
		thumb, err := assets.Thumbnail(decoded, assets.ThumbnailWidth)
		if err != nil {
			t.Fatal(err)
		}
		name := assets.NewName()
		stored, err := assets.Save(directory, today, name, assets.Files{
			Display: display, DisplayFormat: assets.FormatPNG, Thumb: thumb,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := model.workspace.CreateKeyEventImage(ctx, domain.KeyEventImage{
			ID: name, EventDate: today, FilePath: stored.FilePath, ThumbPath: stored.ThumbPath,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := driver.Resize(1100, 800); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "事件")
	click(t, driver, "多图")

	// The smallest window is the one that exposes a gallery or a footer that
	// believes it is wider than the pane, and the compact palette at the foot.
	for _, size := range []struct{ width, height int }{
		{1100, 800},
		{960, 620},
	} {
		if err := driver.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		settle(t, driver)
		frame := driver.Frame()
		found := 0
		for _, hit := range driver.Snapshot().Hits {
			switch hit.Label {
			case "保存图片", "添加图片", "编辑描述", "使用默认颜色":
				found++
				right := hit.Bounds.X + hit.Bounds.Width
				bottom := hit.Bounds.Y + hit.Bounds.Height
				if bottom > frame.Height+0.5 || right > frame.Width+0.5 {
					t.Fatalf("%q at %vx%v is outside the %vx%v window",
						hit.Label, size.width, size.height, frame.Width, frame.Height)
				}
			}
		}
		if found < 4 {
			t.Fatalf("at %vx%v found only %d of the gallery/footer/palette controls",
				size.width, size.height, found)
		}
	}
}

// TestEventLongBodyKeepsTheFooterAtTheSmallestWindow guards the body's reading
// height: a long, picture-less day must give height back at 960x620 instead of
// pushing the palette and the actions below the window.
func TestEventLongBodyKeepsTheFooterAtTheSmallestWindow(t *testing.T) {
	driver, model := open(t)
	date := time.Now().Format("2006-01-02")
	content := strings.Repeat("很长的一段记录，写满了搬家那天的细节。", 40)
	if _, err := model.workspace.SaveKeyEvent(context.Background(), date, "长文", content, "#7868A0"); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "事件")
	click(t, driver, "长文")
	if err := driver.Resize(960, 620); err != nil {
		t.Fatal(err)
	}
	settle(t, driver)

	frame := driver.Frame()
	found := 0
	for _, hit := range driver.Snapshot().Hits {
		switch hit.Label {
		case "添加图片", "编辑描述", "使用默认颜色":
			found++
			right := hit.Bounds.X + hit.Bounds.Width
			bottom := hit.Bounds.Y + hit.Bounds.Height
			if bottom > frame.Height+0.5 || right > frame.Width+0.5 {
				t.Fatalf("%q at %v,%v is outside the %vx%v window",
					hit.Label, hit.Bounds.X, hit.Bounds.Y, frame.Width, frame.Height)
			}
		}
	}
	if found < 3 {
		t.Fatalf("found only %d of the empty-gallery/footer/palette controls", found)
	}
}
