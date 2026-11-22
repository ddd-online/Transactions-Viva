package desktop

import (
	"testing"
	"time"
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
