//go:build windows

package update

import (
	"os/exec"
	"syscall"
)

// setDetached lets the installer keep running after this process exits.
func setDetached(command *exec.Cmd) {
	const detachedProcess = 0x00000008
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}
