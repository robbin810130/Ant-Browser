//go:build !windows
// +build !windows

package appupdate

import (
	"os/exec"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
}

func detachProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}
