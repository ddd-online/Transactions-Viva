package settings

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"transactions/internal/config"
)

// TestProxyTestResultPaintsItsStatusDot drives 检测 to its result: the row then
// shows the dot that says how the last check went, and that dot has to carry an
// identity, because its tooltip is what explains it. A dot declared without one
// panicked the paint the moment a result arrived.
func TestProxyTestResultPaintsItsStatusDot(t *testing.T) {
	model := New()
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
}
