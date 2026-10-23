package gui

import (
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"

	"github.com/user/curlyk/internal/i18n"
)

// autosaveDelay is how long the window waits after the last edit before writing
// the buffer to disk. It is a variable so tests can shorten the debounce.
var autosaveDelay = 5 * time.Second

// scheduleAutosave (re)arms the debounced save after an edit. An unnamed buffer
// is never autosaved: there is no file to write to.
func (u *ui) scheduleAutosave() {
	if u.path == "" {
		return
	}
	u.autosaveDeadline = time.Now().Add(autosaveDelay)
	if u.autosaveTimer != nil {
		u.autosaveTimer.Stop()
	}
	u.autosaveTimer = time.AfterFunc(autosaveDelay, func() {
		// The timer fires on its own goroutine; Fyne widgets and the buffer state
		// may only be touched from the main runtime context.
		fyne.Do(func() { u.applyAutosave(time.Now()) })
	})
}

// stopAutosave cancels a pending save (explicit save, file switch, window close).
func (u *ui) stopAutosave() {
	if u.autosaveTimer != nil {
		u.autosaveTimer.Stop()
		u.autosaveTimer = nil
	}
}

// applyAutosave writes the buffer when the debounce window has elapsed and the
// buffer actually differs from what is on disk. Explicit saves and file loads
// update u.saved, so autosave never resurrects an old buffer.
func (u *ui) applyAutosave(now time.Time) {
	if u.path == "" {
		return
	}
	if now.Before(u.autosaveDeadline) {
		return
	}
	text := u.ed.Text()
	if text == u.saved {
		return
	}
	if err := os.WriteFile(u.path, []byte(text), 0o644); err != nil {
		u.setStatus(i18n.T("err.save", err.Error()))
		return
	}
	u.saved = text
	u.stopAutosave()
	u.updateHeader()
	u.setStatus(i18n.T("status.autosaved", filepath.Base(u.path)))
}
