package settings

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// TestBackupPickerOpensAfterThePaint is the crash behind 选择工作空间… opening
// twice: the picker is a native modal, and a modal loop pumps the window's
// messages, so a click handled in the middle of a paint is delivered again to the
// nested repaint. Every command that opens a system dialog has to run in the frame
// after the click — this test fails if one runs while the page is declaring.
func TestBackupPickerOpensAfterThePaint(t *testing.T) {
	model := New()
	model.tab = tabData
	declaring := false
	insidePaint := false
	opened := 0
	spec := Spec{
		Workspace: t.TempDir(),
		OnPickWorkspace: func() error {
			opened++
			insidePaint = declaring
			return nil
		},
	}
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		declaring = true
		defer func() { declaring = false }()
		model.View(c, spec)
	}})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1100, Height: 1600}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	clickLabel(t, driver, "选择工作空间…")
	if opened != 1 {
		t.Fatalf("the picker opened %d times", opened)
	}
	if insidePaint {
		t.Fatal("the picker opened while the page was declaring: its modal loop would deliver the click twice")
	}
}
