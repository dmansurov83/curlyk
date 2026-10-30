//go:build !windows

package gui

import "fyne.io/fyne/v2"

// isWindowMaximized always reports false off Windows: Fyne exposes only full
// screen there, and reopening a maximized window as a borderless full-screen one
// would surprise the user more than not restoring it.
func isWindowMaximized(fyne.Window) bool { return false }

// setWindowMaximized does nothing off Windows.
func setWindowMaximized(fyne.Window, bool) {}
