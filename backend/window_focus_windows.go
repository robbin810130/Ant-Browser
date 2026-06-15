//go:build windows
// +build windows

package backend

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	gwOwner       = 4
	swShow        = 5
	swRestore     = 9
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpShowWindow = 0x0040
)

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procAttachThreadInput     = user32.NewProc("AttachThreadInput")
	procBringWindowToTop      = user32.NewProc("BringWindowToTop")
	procEnumWindows           = user32.NewProc("EnumWindows")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procGetShellWindow        = user32.NewProc("GetShellWindow")
	procGetWindow             = user32.NewProc("GetWindow")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
	procIsIconic              = user32.NewProc("IsIconic")
	procIsWindowVisible       = user32.NewProc("IsWindowVisible")
	procSetActiveWindow       = user32.NewProc("SetActiveWindow")
	procSetFocus              = user32.NewProc("SetFocus")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procShowWindow            = user32.NewProc("ShowWindow")
	procGetCurrentThreadID    = kernel32.NewProc("GetCurrentThreadId")
)

func raiseBrowserWindowForPIDImpl(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("browser pid is empty")
	}
	hwnd, err := findMainWindowForPID(uint32(pid))
	if err != nil {
		return err
	}
	if hwnd == 0 {
		return fmt.Errorf("browser window not found for pid=%d", pid)
	}

	showWindow(hwnd, swShow)
	if isIconic(hwnd) {
		showWindow(hwnd, swRestore)
	}
	_, _, _ = procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpShowWindow)
	_, _, _ = procBringWindowToTop.Call(hwnd)

	detach := attachForegroundInput(hwnd)
	defer detach()

	_, _, _ = procSetActiveWindow.Call(hwnd)
	_, _, _ = procSetFocus.Call(hwnd)
	ok, _, callErr := procSetForegroundWindow.Call(hwnd)
	if ok == 0 {
		if callErr != windows.ERROR_SUCCESS {
			return fmt.Errorf("set foreground window failed: %w", callErr)
		}
		return fmt.Errorf("set foreground window rejected by Windows")
	}
	return nil
}

var raiseBrowserWindowForPID = raiseBrowserWindowForPIDImpl

func findMainWindowForPID(pid uint32) (uintptr, error) {
	var found uintptr
	shell, _, _ := procGetShellWindow.Call()
	callback := windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if hwnd == 0 || hwnd == shell || !isWindowVisible(hwnd) || getWindow(hwnd, gwOwner) != 0 {
			return 1
		}
		var windowPID uint32
		procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID != pid {
			return 1
		}
		found = hwnd
		return 0
	})
	ret, _, err := procEnumWindows.Call(callback, 0)
	if ret == 0 && found == 0 && err != windows.ERROR_SUCCESS {
		return 0, fmt.Errorf("enum windows failed: %w", err)
	}
	return found, nil
}

func attachForegroundInput(hwnd uintptr) func() {
	currentThread, _, _ := procGetCurrentThreadID.Call()
	targetThread, _, _ := procGetWindowThreadProcID.Call(hwnd, 0)
	foreground, _, _ := procGetForegroundWindow.Call()
	var foregroundThread uintptr
	if foreground != 0 {
		foregroundThread, _, _ = procGetWindowThreadProcID.Call(foreground, 0)
	}

	attachedTarget := false
	attachedForeground := false
	if targetThread != 0 && targetThread != currentThread {
		ok, _, _ := procAttachThreadInput.Call(currentThread, targetThread, 1)
		attachedTarget = ok != 0
	}
	if foregroundThread != 0 && foregroundThread != currentThread && foregroundThread != targetThread {
		ok, _, _ := procAttachThreadInput.Call(currentThread, foregroundThread, 1)
		attachedForeground = ok != 0
	}

	return func() {
		if attachedForeground {
			procAttachThreadInput.Call(currentThread, foregroundThread, 0)
		}
		if attachedTarget {
			procAttachThreadInput.Call(currentThread, targetThread, 0)
		}
	}
}

func isWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

func isIconic(hwnd uintptr) bool {
	ret, _, _ := procIsIconic.Call(hwnd)
	return ret != 0
}

func showWindow(hwnd uintptr, cmdShow uintptr) {
	procShowWindow.Call(hwnd, cmdShow)
}

func getWindow(hwnd uintptr, cmd uintptr) uintptr {
	ret, _, _ := procGetWindow.Call(hwnd, cmd)
	return ret
}
