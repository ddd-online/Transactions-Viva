package settings

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestUpdateFlow(t *testing.T) {
	model := New()
	model.tab = tabAbout
	progressSent := make(chan struct{})
	finishDownload := make(chan struct{})
	spec := Spec{
		UpdateVersion: "0.1.0",
		UpdateCheck: func(context.Context) (UpdateRelease, error) {
			return UpdateRelease{
				HasUpdate: true, Version: "0.2.0", DownloadURL: "https://github.com/x/a.exe",
				Digest: "sha256:abc", Body: "## 修复\n\n- 更快",
			}, nil
		},
		UpdateDownload: func(_ context.Context, rawURL, digest string, progress func(UpdateProgress)) (string, error) {
			if rawURL != "https://github.com/x/a.exe" || digest != "sha256:abc" {
				t.Errorf("download args = %q, %q", rawURL, digest)
			}
			progress(UpdateProgress{Percent: 40, Speed: "1.2 MB/s"})
			close(progressSent)
			<-finishDownload
			progress(UpdateProgress{Percent: 100})
			return "/tmp/Transactions.exe", nil
		},
		UpdateInstall: func(path string) error {
			if path != "/tmp/Transactions.exe" {
				t.Errorf("install path = %q", path)
			}
			return nil
		},
	}
	invalidator := &app.Invalidator{}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { model.View(c, spec) }})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Invalidator: invalidator}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	clickLabel(t, driver, "检查更新")
	waitFor(t, driver, "发现新版本 v0.2.0")

	clickLabel(t, driver, "查看更新内容")
	waitFor(t, driver, "更新内容 · v0.2.0")
	waitFor(t, driver, "更快")
	clickLabel(t, driver, "关闭")

	clickLabel(t, driver, "下载更新")
	select {
	case <-progressSent:
	case <-time.After(2 * time.Second):
		t.Fatal("download never reported progress")
	}
	waitFor(t, driver, "40%")
	close(finishDownload)
	waitForHit(t, driver, "开始更新")
	clickLabel(t, driver, "开始更新")
}

// TestUpdateRecheckAsksAgain: 重新检查 on a finished check asks GitHub again, and
// the row keeps when the answer came back — a check that finds nothing new changes
// no word of the title, so the time is what says it ran.
func TestUpdateRecheckAsksAgain(t *testing.T) {
	model := New()
	model.tab = tabAbout
	asked := make(chan struct{}, 4)
	spec := Spec{
		UpdateVersion: "0.4.1",
		UpdateCheck: func(context.Context) (UpdateRelease, error) {
			asked <- struct{}{}
			return UpdateRelease{}, nil
		},
	}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { model.View(c, spec) }})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	clickLabel(t, driver, "检查更新")
	waitFor(t, driver, "已是最新版本")
	waitFor(t, driver, "最后检查")
	drain(t, asked)

	clickLabel(t, driver, "重新检查")
	select {
	case <-asked:
	case <-time.After(2 * time.Second):
		t.Fatal("重新检查 did not ask GitHub again")
	}
	waitFor(t, driver, "当前版本 0.4.1")
}

// drain empties the channel of answers already received.
func drain(t *testing.T, answers chan struct{}) {
	t.Helper()
	for {
		select {
		case <-answers:
		default:
			return
		}
	}
}

func waitForHit(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := driver.Paint(); err != nil {
			t.Fatal(err)
		}
		for _, hit := range driver.Snapshot().Hits {
			if hit.Label == name && !hit.IsLabel {
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("no control named %q", name)
}

func clickLabel(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q", name)
}

func waitFor(t *testing.T, driver *app.Driver, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := driver.Paint(); err != nil {
			t.Fatal(err)
		}
		for _, label := range driver.Snapshot().Labels {
			if strings.Contains(label.Value, want) {
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("no text containing %q", want)
}
