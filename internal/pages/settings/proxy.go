package settings

import (
	"fmt"
	"strings"
	"time"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/config"
)

// ProxyTestResult is one connectivity check from the proxy row.
type ProxyTestResult struct {
	OK      bool
	Message string
}

var proxyModes = [...]config.ProxyMode{
	config.ProxyOff, config.ProxyAuto, config.ProxyManual,
}

func proxyModeLabel(mode config.ProxyMode) string {
	switch mode {
	case config.ProxyOff:
		return "不使用"
	case config.ProxyManual:
		return "手动"
	}
	return "自动探测"
}

// proxy is 网络代理: the mode is written as soon as it is chosen; a manual
// address is edited locally and saved, then tested through the same client the
// stock pages use. The previous test result is dropped when the setting changes.
func (m *Model) proxy(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	if !m.proxyLoaded {
		m.proxyLoaded = true
		m.proxyURL = spec.Proxy.URL
		m.proxyMode = spec.Proxy.Mode
	}
	m.proxyTestFn = spec.OnTestProxy
	if m.proxyTestKey != "" && m.proxyTestDone != m.proxyTestKey {
		ui.Effect(c, m.proxyEffect, m.proxyTestAsk)
		if result, err, ok := m.proxyEffect.Take(); ok {
			m.proxyTestDone = m.proxyTestKey
			m.finishProxyTest(result, err)
		}
	}
	mode := m.proxyMode
	ui.Column(c).FillWidth().Shrink(0).PadX(metrics.PadSurface).
		PadY(metrics.GapItem * 2).Gap(metrics.GapItem).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
			typography.Body(c, "代理")
			if m.proxyState != "" {
				color := theme.Success
				tip := "上次检测：连通"
				if m.proxyState == "fail" {
					color, tip = theme.Danger, "上次检测：失败"
				}
				ui.Row(c).Size(8, 8).Radius(4).Background(color).Tooltip(tip)
			}
			ui.Spacer(c)
			if ui.Choice(c, &mode, proxyModes[:], proxyModeLabel).Changed() {
				m.proxyMode = mode
				m.proxyState = ""
				if mode != config.ProxyManual {
					m.saveProxy(spec, config.ProxySetting{Mode: mode, URL: m.proxyURL})
				}
			}
			if mode != config.ProxyOff {
				test := ui.Button(c, "检测").Secondary()
				if spec.OnTestProxy == nil {
					test = test.Disabled(true)
				}
				if test.Clicked() {
					m.proxyTestAsk = config.ProxySetting{Mode: mode, URL: m.proxyURL}
					m.proxyTestKey = fmt.Sprintf("%s|%s|%d", mode, m.proxyURL, time.Now().UnixNano())
					m.proxyTestDone = ""
					m.proxyState = ""
				}
			}
		})
		typography.CaptionParagraph(c, "行情、涨停池与更新检查走代理；自动探测先读环境变量，再读系统代理。", 2)
		switch mode {
		case config.ProxyAuto:
			typography.Caption(c, "当前："+spec.ProxyResolved).Mono().Ellipsis()
		case config.ProxyManual:
			ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
				field := ui.TextField(c, &m.proxyURL, "代理地址", "http://127.0.0.1:7890").
					Grow(1).MaxWidth(360)
				if field.Submitted() {
					m.saveProxy(spec, config.ProxySetting{Mode: config.ProxyManual, URL: m.proxyURL})
				}
				if ui.Button(c, "保存").Secondary().Clicked() {
					m.saveProxy(spec, config.ProxySetting{Mode: config.ProxyManual, URL: m.proxyURL})
				}
			})
		default:
			typography.Caption(c, "所有联网都直连，不读取环境变量里的代理设置。").Muted()
		}
	})
}

func (m *Model) saveProxy(spec Spec, setting config.ProxySetting) {
	if spec.OnSaveProxy == nil {
		return
	}
	saved, err := spec.OnSaveProxy(setting)
	if err != nil {
		m.toaster.Show(err.Error(), "", ui.ToastError)
		return
	}
	m.proxyURL = saved.URL
	m.proxyMode = saved.Mode
	m.toaster.Show("代理设置已保存", "", ui.ToastSuccess)
}

func (m *Model) finishProxyTest(result ProxyTestResult, err error) {
	if err != nil {
		m.proxyState = "fail"
		m.toaster.Show(err.Error(), "", ui.ToastError)
		return
	}
	if result.OK {
		m.proxyState = "ok"
	} else {
		m.proxyState = "fail"
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = "代理连接失败"
	}
	kind := ui.ToastSuccess
	if !result.OK {
		kind = ui.ToastError
	}
	m.toaster.Show(message, "", kind)
}
