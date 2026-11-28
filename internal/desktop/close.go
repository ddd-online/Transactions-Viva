package desktop

import (
	"math"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/config"
)

// requestClose is the window's close button. It saves the frame first, then
// applies the configured behavior: quit, hide to the status area, or ask. A
// workspace that is not open yet closes straight away, as the application
// does — there is nothing to keep running.
func (m *Model) requestClose(allow func()) {
	m.saveWindowBounds()
	if m.development {
		// The reload worker has no way to ask its host for a frame outside the
		// host's paint requests, so a prompt raised here would wait for an
		// unrelated repaint. Development closes immediately; the release build
		// applies the configured behavior below.
		allow()
		return
	}
	if m.config.WorkspaceDir == "" {
		allow()
		return
	}
	switch m.config.CloseBehavior {
	case config.CloseQuit:
		allow()
	case config.CloseTray:
		if !m.hideToTray() {
			allow()
		}
	default:
		m.closeAsk = true
		m.closeAllow = allow
		m.invalidate()
	}
}

// hideToTray hides the window when a status item exists to bring it back.
func (m *Model) hideToTray() bool {
	if !m.trayOK || m.window == nil || m.window.Closed() {
		return false
	}
	m.window.Hide()
	return true
}

// saveWindowBounds stores the content area for the next launch. It runs while
// the window closes, so a failed write has nowhere to be reported.
func (m *Model) saveWindowBounds() {
	if m.window == nil || m.window.Closed() {
		return
	}
	frame, ok := m.window.Frame()
	if !ok || frame.Width <= 0 || frame.Height <= 0 {
		return
	}
	m.config.Width = int(math.Round(float64(frame.Width)))
	m.config.Height = int(math.Round(float64(frame.Height)))
	x, y := int(math.Round(float64(frame.X))), int(math.Round(float64(frame.Y)))
	m.config.X, m.config.Y = &x, &y
	_ = m.persist()
}

// closeModal is the 每次询问 prompt the close button raises. Escaping it or
// pressing 取消 cancels the close; the pending allow is dropped.
func (m *Model) closeModal(c ui.C) {
	if !m.closeAsk {
		return
	}
	ui.Modal(c, &m.closeAsk, "关闭窗口", ui.ModalSpec{Width: 440}, func() {
		if m.trayOK {
			typography.CaptionParagraph(c, "可在「设置 → 通用设置 → 关闭行为」里固定这个选择。", 2)
		} else {
			typography.CaptionParagraph(c, "当前系统没有托盘，关闭会直接退出。可在「设置 → 通用设置 → 关闭行为」里固定这个选择。", 3)
		}
	}, func() {
		if ui.Button(c, "取消").Secondary().Clicked() {
			m.closeAsk, m.closeAllow = false, nil
		}
		if m.trayOK && ui.Button(c, "最小化到托盘").Secondary().Clicked() {
			m.closeAsk, m.closeAllow = false, nil
			m.hideToTray()
		}
		if ui.Button(c, "退出应用").Destructive().Clicked() {
			if allow := m.closeAllow; allow != nil {
				m.closeAsk, m.closeAllow = false, nil
				allow()
			}
		}
	})
	if !m.closeAsk {
		m.closeAllow = nil
	}
}
