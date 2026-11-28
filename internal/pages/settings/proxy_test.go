package settings

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/config"
)

// TestProxyTestResultPaintsItsStatusDot drives 检测 to its result: the row then
// shows the dot that says how the last check went, and that dot has to carry an
// identity, because its tooltip is what explains it. A dot declared without one
// panicked the paint the moment a result arrived.
func TestProxyTestResultPaintsItsStatusDot(t *testing.T) {
	model := New()
	// The proxy lives on 关于软件 beside the update row: only the update check and
	// the download go through it.
	model.tab = tabAbout
	var tested int
	spec := Spec{
		Proxy:         config.ProxySetting{Mode: config.ProxyAuto},
		ProxyResolved: "127.0.0.1:7890",
		OnTestProxy: func(config.ProxySetting) (ProxyTestResult, error) {
			tested++
			return ProxyTestResult{OK: true, Message: "代理连通"}, nil
		},
	}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) { model.View(c, spec) }})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1200, Height: 800}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	clickLabel(t, driver, "检测")
	waitForHit(t, driver, "上次检测：连通")
	if tested != 1 {
		t.Fatalf("proxy was tested %d times, want 1", tested)
	}

	// The dot explains itself on hover: the tooltip is the only place the
	// result is written, so it has to open.
	var dot gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "上次检测：连通" {
			dot = hit.Bounds
		}
	}
	if err := driver.Move(dot.X+dot.Width/2, dot.Y+dot.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	for _, label := range driver.Snapshot().Labels {
		if label.Value == "上次检测：连通" {
			return
		}
	}
	t.Fatal("the status dot opened no tooltip")
}
