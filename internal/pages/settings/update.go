package settings

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/typography"
	"transactions/internal/icons"
)

// UpdateRelease is one GitHub release, as the page shows it.
type UpdateRelease struct {
	HasUpdate   bool
	Version     string
	DownloadURL string
	Digest      string
	Body        string
}

// UpdateProgress is one download sample from the shell.
type UpdateProgress struct {
	Percent int
	Speed   string
}

type updateDownloadRequest struct {
	URL     string
	Digest  string
	Attempt int
}

const (
	updateIdle        = "idle"
	updateChecking    = "checking"
	updateAvailable   = "available"
	updateNoUpdate    = "no-update"
	updateDownloading = "downloading"
	updateDownloaded  = "downloaded"
	updateFailed      = "failed"
)

// KeepUpdate declares the update work for as long as the window is painting.
// Viva cancels an effect the paint stops declaring, so a download declared only
// by the 关于软件 section would be aborted by leaving 设置 and start over when
// the section came back. The shell calls this on every paint; View calls it
// too, so the section still runs its work when it is painted on its own.
func (m *Model) KeepUpdate(c ui.C) {
	if m.updateStatus == "" {
		m.updateStatus = updateIdle
	}
	if m.updateCheckToken > 0 {
		ui.Effect(c, m.updateCheckState, m.updateCheckToken)
		if result, err, ok := m.updateCheckState.Take(); ok {
			m.updateCheckedAt = time.Now()
			if err != nil {
				m.updateStatus = updateFailed
				m.updateError = err.Error()
			} else if result.HasUpdate {
				m.updateStatus = updateAvailable
				m.updateRelease = result
				m.updateError = ""
			} else {
				m.updateStatus = updateNoUpdate
				m.updateRelease = result
				m.updateError = ""
			}
		}
	}
	if m.updateDownloadID > 0 {
		ui.Effect(c, m.updateDownloadState, updateDownloadRequest{
			URL: m.updateRelease.DownloadURL, Digest: m.updateRelease.Digest,
			Attempt: m.updateDownloadID,
		})
		if path, err, ok := m.updateDownloadState.Take(); ok {
			if err != nil {
				m.updateStatus = updateFailed
				m.updateError = err.Error()
				m.setUpdateProgress(0, "")
			} else {
				m.updateStatus = updateDownloaded
				m.updatePath = path
				m.updateError = ""
				m.setUpdateProgress(100, "")
			}
		}
	}
}

// update is the 更新 group: one action at a time, release notes behind a
// button, and the download's own progress bar.
func (m *Model) update(c ui.C, spec Spec) {
	theme := c.Theme()
	metrics := theme.Metrics
	m.updateFnMu.Lock()
	m.updateCheckFn = spec.UpdateCheck
	m.updateDownloadFn = spec.UpdateDownload
	m.updateInstallFn = spec.UpdateInstall
	m.updateInvalidate = spec.UpdateInvalidate
	m.updateFnMu.Unlock()
	if m.updateStatus == "" {
		m.updateStatus = updateIdle
	}

	percent, speed := m.updateProgress()
	ui.Column(c).FillWidth().Shrink(0).PadX(metrics.PadSurface).
		PadY(metrics.GapItem * 2).Gap(metrics.GapItem).Children(func() {
		switch m.updateStatus {
		case updateChecking:
			ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
				ui.Spinner(c)
				typography.Body(c, "正在检查更新…")
			})
		case updateNoUpdate:
			m.updateResultRow(c, true, "已是最新版本", m.checkedNote(spec), func() {
				if ui.Button(c, "重新检查").Outline().Clicked() {
					m.startUpdateCheck()
				}
			})
		case updateAvailable:
			m.updateResultRow(c, false, "发现新版本 v"+m.updateRelease.Version,
				"下载后直接安装，工作空间里的数据不受影响。", func() {
					if ui.Button(c, "查看更新内容").Outline().Clicked() {
						m.updateNotesOpen = true
					}
					if ui.Button(c, "下载更新").Clicked() {
						m.startUpdateDownload()
					}
				})
		case updateDownloading:
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
					typography.Body(c, "正在下载 v"+m.updateRelease.Version)
					ui.Spacer(c)
					typography.Caption(c, fmt.Sprintf("%d%%  %s", percent, speed)).Mono()
					if ui.Button(c, "取消下载").Ghost().Small().Clicked() {
						m.cancelUpdateDownload()
					}
				})
				ui.Progress(c, float32(percent)/100).FillWidth().Height(6)
			})
		case updateDownloaded:
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				m.updateResultRow(c, true, "下载完成", "安装程序已就绪，开始更新后应用会退出。", func() {
					if ui.Button(c, "查看更新内容").Outline().Clicked() {
						m.updateNotesOpen = true
					}
					if ui.Button(c, "开始更新").Clicked() {
						m.installUpdate()
					}
				})
				ui.Row(c).FillWidth().Height(6).Radius(3).Background(theme.Success)
			})
		case updateFailed:
			m.updateResultRow(c, false, "更新失败", m.updateError, func() {
				// The retry is the only thing to do on a failed check, so it wears the
				// filled secondary the card's actions wear; the quiet outline stays for
				// the retry beside a finished check that has nothing to offer.
				if ui.Button(c, "重新检查").Secondary().Clicked() {
					m.startUpdateCheck()
				}
			})
		default:
			ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					typography.Body(c, "检查更新")
					typography.Caption(c, "从 GitHub 的发布页获取最新版本。")
				})
				if ui.Button(c, "检查更新").Secondary().Clicked() {
					m.startUpdateCheck()
				}
			})
		}
	})
}

// checkedNote is the quiet line under 已是最新版本: the version this copy is, and
// when the answer was fetched. A re-check that finds nothing new changes no word of
// the title, so the time is what says it ran.
func (m *Model) checkedNote(spec Spec) string {
	note := "当前版本 " + spec.UpdateVersion
	if m.updateCheckedAt.IsZero() {
		return note
	}
	return note + " · 最后检查 " + m.updateCheckedAt.Format("2006-01-02 15:04")
}

// updateResultRow lays out a state with a leading success mark and its actions.
// Green means finished; it is not a second accent colour.
func (m *Model) updateResultRow(c ui.C, success bool, title, note string, actions func()) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Items(ui.Center).Gap(theme.Metrics.GapItem).Children(func() {
		if success {
			ui.Icon(c, icons.CircleCheck).Size(18, 18).Shrink(0).Color(theme.Success)
		}
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			text := typography.Body(c, title)
			if success {
				text.Color(theme.Success)
			} else if title == "更新失败" {
				text.Color(theme.Danger)
			}
			typography.Caption(c, note).MaxLines(2)
		})
		ui.Row(c).Shrink(0).Items(ui.Center).Gap(8).Children(actions)
	})
}

func (m *Model) startUpdateCheck() {
	m.updateStatus = updateChecking
	m.updateError = ""
	m.updateCheckToken++
}

func (m *Model) startUpdateDownload() {
	if m.updateRelease.DownloadURL == "" {
		return
	}
	m.updateStatus = updateDownloading
	m.updateError = ""
	m.updateDownloadID++
	m.setUpdateProgress(0, "")
}

// cancelUpdateDownload stops the in-flight download. The effect is declared
// for updateDownloadID; zeroing it undeclares the effect on the next paint and
// cancels the request.
func (m *Model) cancelUpdateDownload() {
	m.updateStatus = updateAvailable
	m.updateDownloadID = 0
	m.setUpdateProgress(0, "")
}

func (m *Model) installUpdate() {
	if m.updateInstallFn == nil || m.updatePath == "" {
		return
	}
	if err := m.updateInstallFn(m.updatePath); err != nil {
		m.updateError = err.Error()
		m.toaster.Show(err.Error(), "", ui.ToastError)
		return
	}
	m.toaster.Show("正在启动安装程序", "", ui.ToastSuccess)
}

func (m *Model) reportUpdateProgress(progress UpdateProgress) {
	percent := progress.Percent
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	m.updateMu.Lock()
	m.updatePercent = percent
	if progress.Speed != "" {
		m.updateSpeed = progress.Speed
	}
	m.updateMu.Unlock()
	m.updateFnMu.Lock()
	invalidate := m.updateInvalidate
	m.updateFnMu.Unlock()
	if invalidate != nil {
		invalidate()
	}
}

func (m *Model) setUpdateProgress(percent int, speed string) {
	m.updateMu.Lock()
	m.updatePercent = percent
	m.updateSpeed = speed
	m.updateMu.Unlock()
}

func (m *Model) updateProgress() (int, string) {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	return m.updatePercent, m.updateSpeed
}

// updateNotesModal shows the release body. It is a separate modal so the
// download state stays readable behind it.
func (m *Model) updateNotesModal(c ui.C) {
	title := "更新内容"
	if m.updateRelease.Version != "" {
		title += " · v" + m.updateRelease.Version
	}
	ui.ModalWith(c, &m.updateNotesOpen, title, ui.ModalSpec{Width: 640}, func() {
		blocks := releaseBlocks(m.updateRelease.Body)
		if len(blocks) == 0 {
			typography.Body(c, "这次发布没有填写更新内容。").Muted()
			return
		}
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			for _, block := range blocks {
				if block.heading {
					typography.BodyStrong(c, block.text)
				} else {
					ui.Wrap(c, block.text)
				}
			}
		})
	}, func() {
		if ui.Button(c, "关闭").Outline().Clicked() {
			m.updateNotesOpen = false
		}
	})
}

type releaseBlock struct {
	text    string
	heading bool
}

var markdownLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// releaseBlocks turns GitHub-flavoured Markdown into readable plain blocks:
// headings keep their emphasis, links keep their address, and the markers that
// a TextView cannot honour are dropped rather than shown raw.
func releaseBlocks(body string) []releaseBlock {
	var blocks []releaseBlock
	var current strings.Builder
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			blocks = append(blocks, releaseBlock{text: text})
		}
		current.Reset()
	}
	for _, raw := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			text := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if text != "" {
				blocks = append(blocks, releaseBlock{text: cleanReleaseText(text), heading: true})
			}
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = markdownLink.ReplaceAllString(line, "$1（$2）")
		line = cleanReleaseText(line)
		if line == "" {
			continue
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	flush()
	return blocks
}

func cleanReleaseText(text string) string {
	replacer := strings.NewReplacer("**", "", "__", "", "`", "", "~~", "")
	return strings.TrimSpace(replacer.Replace(text))
}
