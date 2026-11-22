package desktop

import (
	"context"
	"errors"
	"path"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"

	"transactions/internal/assets"
	eventservice "transactions/internal/events"
	eventpage "transactions/internal/pages/events"
)

// errNoEventWorkspace is what the event page's commands report while no
// workspace is open. The section is not reachable then, so this only guards a
// stale command.
var errNoEventWorkspace = errors.New("还没有打开工作空间")

// eventCommands is the platform work the 事件 page asks for: the image and save
// pickers, the files under the workspace, and the configuration bit that keeps
// the linked rail's state.
func (m *Model) eventCommands() eventpage.Commands {
	return eventpage.Commands{
		AddImages:    m.addEventImages,
		DeleteEvent:  m.deleteKeyEvent,
		DeleteImage:  m.deleteEventImage,
		SaveImage:    m.saveEventImage,
		LoadImage:    m.loadEventImage,
		LoadView:     m.loadEventView,
		SetLinksOpen: m.setEventLinksOpen,
	}
}

// addEventImages opens the image picker and stores what it returned on the
// day's event. A dismissed picker is a no-op, not an error.
func (m *Model) addEventImages(date string) (eventpage.AddImagesResult, error) {
	var result eventpage.AddImagesResult
	if m.workspace == nil {
		return result, errNoEventWorkspace
	}
	if m.pickImages == nil {
		return result, errors.New("无法打开图片选择器")
	}
	paths, err := m.pickImages()
	if errors.Is(err, app.ErrCanceled) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if len(paths) == 0 {
		return result, nil
	}
	stored, err := eventservice.AddImages(context.Background(), m.workspace,
		m.workspace.Directory(), date, paths)
	// The page has to reload its image list even when one file failed, so ask
	// for the frame in both cases.
	m.invalidate()
	if err != nil {
		return result, err
	}
	result.Added = stored.Added
	result.Problems = stored.Problems
	return result, nil
}

// deleteKeyEvent removes the day's event with its images and files.
func (m *Model) deleteKeyEvent(date string) error {
	if m.workspace == nil {
		return errNoEventWorkspace
	}
	if err := eventservice.DeleteEvent(context.Background(), m.workspace,
		m.workspace.Directory(), date); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

// deleteEventImage removes one picture with its files.
func (m *Model) deleteEventImage(imageID string) error {
	if m.workspace == nil {
		return errNoEventWorkspace
	}
	if err := eventservice.DeleteImage(context.Background(), m.workspace,
		m.workspace.Directory(), imageID); err != nil {
		return err
	}
	m.invalidate()
	return nil
}

// saveEventImage asks where to put a copy of one stored image and writes it.
// The suggested name is the stored file's; a dismissed dialog is a no-op.
func (m *Model) saveEventImage(relative string) error {
	if m.workspace == nil {
		return errNoEventWorkspace
	}
	if m.pickSaveFile == nil {
		return errors.New("无法打开保存对话框")
	}
	name := path.Base(relative)
	extension := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	options := app.SaveFileOptions{Title: "保存图片", SuggestedName: name, DefaultExtension: extension}
	if extension != "" {
		options.Filters = []app.FileFilter{{Name: "图片", Pattern: "*." + extension}}
	}
	destination, err := m.pickSaveFile(options)
	if errors.Is(err, app.ErrCanceled) {
		return nil
	}
	if err != nil {
		return err
	}
	if destination == "" {
		return nil
	}
	return eventservice.SaveImage(m.workspace.Directory(), relative, destination)
}

// loadEventImage decodes one stored image into the bitmap the page paints.
func (m *Model) loadEventImage(relative string) (gfx.Bitmap, error) {
	if m.workspace == nil {
		return gfx.Bitmap{}, errNoEventWorkspace
	}
	return assets.Load(m.workspace.Directory(), relative, assets.MaxDisplayEdge)
}

// loadEventView decodes the stage copy of a picture, smaller than the camera
// original so switching events and pictures does not pay for a full decode.
func (m *Model) loadEventView(relative string) (gfx.Bitmap, error) {
	if m.workspace == nil {
		return gfx.Bitmap{}, errNoEventWorkspace
	}
	return assets.LoadView(m.workspace.Directory(), relative, assets.ViewMaxEdge)
}

// setEventLinksOpen writes the linked rail's state to the configuration.
func (m *Model) setEventLinksOpen(open bool) error {
	previous := m.config.KeyEventLinkedOpen
	m.config.KeyEventLinkedOpen = open
	if err := m.persist(); err != nil {
		m.config.KeyEventLinkedOpen = previous
		return err
	}
	return nil
}
