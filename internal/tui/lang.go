package tui

import (
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
)

// toggleLanguage switches the UI locale between the supported languages at
// runtime, shows a confirmation in the status bar, and persists the choice to
// appsettings.yml so it survives a restart.
func (m *model) toggleLanguage() {
	next := i18n.Toggle()
	i18n.SetLocale(next)
	m.status = i18n.T("status.lang", i18n.LangName(next))

	s := settings.Load()
	s.Lang = next
	_ = s.Save()
}
