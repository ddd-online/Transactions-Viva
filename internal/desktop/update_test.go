package desktop

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/app"

	"transactions/internal/pages/settings"
)

func TestInstallUpdateLaunchesThenQuits(t *testing.T) {
	model := New()
	launched := ""
	model.launchUpdate = func(path string) error {
		launched = path
		return nil
	}
	quit := make(chan struct{})
	model.quit = func() { close(quit) }
	if err := model.installUpdate(`C:\Temp\Transactions.exe`); err != nil {
		t.Fatal(err)
	}
	if launched != `C:\Temp\Transactions.exe` {
		t.Fatalf("launched = %q", launched)
	}
	select {
	case <-quit:
	case <-time.After(2 * time.Second):
		t.Fatal("application did not quit after launching the installer")
	}
}

// TestUpdateDownloadSurvivesLeavingSettings drives a download from 关于软件,
// leaves settings for 记账 and comes back: the download is the shell's work, not
// the section's, so a section change neither cancels it nor starts it again.
func TestUpdateDownloadSurvivesLeavingSettings(t *testing.T) {
	driver, model := open(t)
	var downloads atomic.Int32
	progress := make(chan int, 4)
	finished := make(chan struct{})
	cancelled := make(chan struct{}, 4)
	model.checkUpdateFn = func(context.Context) (settings.UpdateRelease, error) {
		return settings.UpdateRelease{
			HasUpdate:   true,
			Version:     "9.9.9",
			DownloadURL: "https://github.com/x/a.exe",
			Digest:      "sha256:abc",
		}, nil
	}
	model.downloadUpdateFn = func(ctx context.Context, rawURL, digest string, report func(settings.UpdateProgress)) (string, error) {
		downloads.Add(1)
		for {
			select {
			case percent := <-progress:
				report(settings.UpdateProgress{Percent: percent, Speed: "1 MB/s"})
			case <-finished:
				return `C:\Temp\Transactions-9.9.9-setup.exe`, nil
			case <-ctx.Done():
				cancelled <- struct{}{}
				return "", ctx.Err()
			}
		}
	}

	click(t, driver, "设置")
	click(t, driver, "关于软件")
	click(t, driver, "检查更新")
	waitShows(t, driver, "发现新版本 v9.9.9")

	click(t, driver, "下载更新")
	progress <- 10
	waitShows(t, driver, "10%")

	// Leaving 设置 must not end the download: an effect the paint stops
	// declaring is cancelled, so the shell declares it on every paint.
	click(t, driver, "记账")
	select {
	case <-cancelled:
		t.Fatal("leaving settings cancelled the running download")
	case <-time.After(200 * time.Millisecond):
	}
	progress <- 55
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if got := downloads.Load(); got != 1 {
		t.Fatalf("the download ran %d times, want one run across the section change", got)
	}

	// The section comes back on the same download, with its progress kept.
	click(t, driver, "设置")
	waitShows(t, driver, "55%")
	finished <- struct{}{}
	waitShows(t, driver, "下载完成")
	if got := downloads.Load(); got != 1 {
		t.Fatalf("the download ran %d times, want one", got)
	}
}

// waitShows paints until a text containing want is on screen.
func waitShows(t *testing.T, driver *app.Driver, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := driver.Paint(); err != nil {
			t.Fatal(err)
		}
		if shows(driver, want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("no text containing %q on screen", want)
}
