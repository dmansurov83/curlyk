package gui

// windowIsMaximized and windowSetMaximized are the platform hooks for the
// "maximized" window state. Fyne itself exposes only full screen, so on Windows
// they talk to the OS window directly and elsewhere they are a no-op; they are
// variables so tests can drive the window-state logic without a real window.
var (
	windowIsMaximized  = isWindowMaximized
	windowSetMaximized = setWindowMaximized
)

// restoreWindowState reapplies the maximized state the window was closed with.
// It must run after Show: before that the platform window does not exist yet and
// the handle is zero.
func (u *ui) restoreWindowState() {
	if u.win == nil || !u.settings.WindowMaximized {
		return
	}
	windowSetMaximized(u.win, true)
}

// rememberWindowState records the maximized state in the settings, so the next
// start reopens the same way.
func (u *ui) rememberWindowState() {
	if u.win == nil {
		return
	}
	u.settings.WindowMaximized = windowIsMaximized(u.win)
}
