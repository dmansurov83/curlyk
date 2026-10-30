//go:build windows

package gui

import (
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// ShowWindow commands used to change the maximized state.
const (
	swMaximize = 3
	swRestore  = 9
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procIsZoomed   = user32.NewProc("IsZoomed")
	procShowWindow = user32.NewProc("ShowWindow")
)

// nativeWindowHandle runs fn with the OS window handle. Fyne builds the platform
// window lazily, so the handle is zero until the window has been shown.
func nativeWindowHandle(w fyne.Window, fn func(hwnd uintptr)) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		if win, ok := ctx.(driver.WindowsWindowContext); ok && win.HWND != 0 {
			fn(win.HWND)
		}
	})
}

// isWindowMaximized reports whether the OS window is maximized.
func isWindowMaximized(w fyne.Window) bool {
	max := false
	nativeWindowHandle(w, func(hwnd uintptr) {
		if r, _, _ := procIsZoomed.Call(hwnd); r != 0 {
			max = true
		}
	})
	return max
}

// setWindowMaximized maximizes or restores the OS window.
func setWindowMaximized(w fyne.Window, max bool) {
	cmd := uintptr(swRestore)
	if max {
		cmd = swMaximize
	}
	nativeWindowHandle(w, func(hwnd uintptr) {
		_, _, _ = procShowWindow.Call(hwnd, cmd)
	})
}
