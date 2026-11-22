//go:build !windows

package update

import (
	"os/exec"
	"syscall"
)

// setDetached puts the installer in its own session so it survives this
// process.
func setDetached(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
