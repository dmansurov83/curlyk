package tui

import (
	"os"
	"strings"

	"github.com/user/curlyk/internal/i18n"
)

// openProfile opens the given profile file in the main editor pane (exactly
// like opening a .http file: @var lines become the editable buffer, saved via
// Ctrl+S/autosave/dirty marker) and simultaneously activates it as the active
// profile whose {{var}} values are substituted on run.
func (m *model) openProfile(name string) {
	m.selectProfile(name)
	m.editProfile(name)
}

// editProfile opens the given profile file in the main editor pane, exactly
// like opening a .http file.
func (m *model) editProfile(name string) {
	data, err := os.ReadFile(name)
	if err != nil {
		m.status = i18n.T("err.openFile", name, err.Error())
		return
	}
	// persist the current buffer before switching away to the profile
	m.saveCurrentBeforeSwitch()
	// remember where the cursor was in the file being left
	m.rememberCursor(m.filePath)
	m.ed.SetText(string(data))
	m.filePath = name
	m.dirty = false
	m.active = paneEdit
	m.status = i18n.T("status.opened", name)
	rememberLastOpened(name)
	m.applyCursor(name)
	m.refreshFilesPanel()
}

// isProfileFile reports whether the given path has the .profile extension.
func isProfileFile(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".profile")
}
