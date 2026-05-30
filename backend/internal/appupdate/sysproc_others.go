//go:build !windows
// +build !windows

package appupdate

import "os/exec"

func hideWindow(cmd *exec.Cmd) {
}
