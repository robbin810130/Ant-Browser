//go:build !windows
// +build !windows

package backend

func raiseBrowserWindowForPIDImpl(pid int) error {
	return nil
}

var raiseBrowserWindowForPID = raiseBrowserWindowForPIDImpl
