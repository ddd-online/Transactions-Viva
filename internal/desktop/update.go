package desktop

import (
	"context"
	"time"

	"transactions/internal/buildinfo"
	"transactions/internal/pages/settings"
	"transactions/internal/proxy"
	"transactions/internal/update"
)

func (m *Model) checkUpdate(ctx context.Context) (settings.UpdateRelease, error) {
	result, err := update.Check(ctx, proxy.Client(m.config.Proxy, update.CheckTimeout), buildinfo.Version)
	if err != nil {
		return settings.UpdateRelease{}, err
	}
	return settings.UpdateRelease{
		HasUpdate:   result.HasUpdate,
		Version:     result.Version,
		DownloadURL: result.DownloadURL,
		Digest:      result.Digest,
		Body:        result.Body,
	}, nil
}

func (m *Model) downloadUpdate(ctx context.Context, rawURL, digest string, progress func(settings.UpdateProgress)) (string, error) {
	return update.Download(ctx, proxy.Client(m.config.Proxy, 0), rawURL, digest, "", false, func(sample update.Progress) {
		if progress != nil {
			progress(settings.UpdateProgress{Percent: sample.Percent, Speed: sample.Speed})
		}
	})
}

// installUpdate starts the downloaded installer, then lets the window close
// gracefully after the child has had a moment to appear.
func (m *Model) installUpdate(path string) error {
	launch := m.launchUpdate
	if launch == nil {
		launch = update.Launch
	}
	if err := launch(path); err != nil {
		return err
	}
	if m.quit != nil {
		time.AfterFunc(600*time.Millisecond, m.quit)
	}
	return nil
}
