package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
	"github.com/user/curlyk/internal/theme"
)

// curScheme is the active color scheme. It backs all package-level UI styles,
// which are rebuilt from it via rebuildStyles() whenever the scheme is applied.
var curScheme = theme.Default()

// themeName is the name of the currently active scheme (builtin or custom).
var themeName = theme.DefaultName

// Resolved scalar colors, read by inline lipgloss.NewStyle() calls and
// borderColor(). Rebuilt by rebuildStyles() from curScheme.
var (
	dialogBorderColor string
	borderActiveColor string
	borderIdleColor   string
	statusBgColor     string
	statusFgColor     string
	headerBgColor     string
	headerFgColor     string
	searchBarBgColor  string
	searchBarFgColor  string
	executingColor    string
	copyBtnFgColor    string
	copyBtnBgColor    string
	copyHoverFgColor  string
	copyHoverBgColor  string
	menuSelFgColor    string
	menuSelBgColor    string
	menuHoverFgColor  string
	menuHoverBgColor  string
	menuFgColor       string
	menuBgColor       string
	findFgColor       string
	findBgColor       string
	findCurFgColor    string
	findCurBgColor    string
)

// schemeNames returns the full ordered list of selectable schemes: the builtin
// schemes followed by any custom schemes found in themes/. It is used both by
// the Ctrl+[ / Ctrl+] cycle and by the action-menu picker.
func schemeNames() []string {
	names := []string{theme.DefaultName, theme.DarkulaName, theme.LightName}
	names = append(names, theme.ListFiles()...)
	return names
}

// applyTheme sets the active scheme by name (builtin or custom file). It
// rebuilds the package-level styles so the change takes effect immediately.
// Returns the scheme and whether a known scheme was applied (unknown names fall
// back to default).
func applyTheme(name string) (theme.Scheme, bool) {
	cur := theme.Default()
	ok := true
	if name != "" {
		if s, found := theme.ByName(name); found {
			cur = s
		} else if s, err := theme.LoadFile(theme.ThemeFilePath(name)); err == nil {
			cur = s
		} else {
			ok = false // unknown custom name
		}
	}
	if !ok {
		name = theme.DefaultName
	}
	curScheme = cur
	themeName = name
	rebuildStyles()
	return cur, ok
}

// currentSchemeName returns the name of the active scheme.
func currentSchemeName() string {
	if themeName == "" {
		return theme.DefaultName
	}
	return themeName
}

// cycleTheme steps the active scheme by delta (+1/-1) through schemeNames(),
// wrapping around. It persists the new choice to appsettings.yml and shows a
// confirmation in the status bar, mirroring toggleLanguage.
func (m *model) cycleTheme(delta int) {
	names := schemeNames()
	idx := 0
	cur := currentSchemeName()
	for i, n := range names {
		if n == cur {
			idx = i
			break
		}
	}
	idx = ((idx+delta)%len(names) + len(names)) % len(names)
	next := names[idx]
	applyTheme(next)
	m.status = i18n.T("status.theme", displayThemeName(next))
	s := settings.Load()
	s.Theme = next
	_ = s.Save()
}

// displayThemeName returns the i18n label for a builtin scheme, or the raw name
// for a custom one.
func displayThemeName(name string) string {
	switch name {
	case theme.DefaultName:
		return i18n.T("theme.default")
	case theme.DarkulaName:
		return i18n.T("theme.darkula")
	case theme.LightName:
		return i18n.T("theme.light")
	default:
		return name
	}
}

// rebuildStyles rebuilds every package-level UI style from curScheme. It is
// called once at startup and whenever the theme changes.
func rebuildStyles() {
	// editor + syntax (highlight.go)
	methodStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.Method)).Bold(true)
	urlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.URL))
	httpVerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.HTTPVer))
	headerNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.HeaderName))
	headerValStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.HeaderValue))
	bodyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.Body)).Faint(true)
	commentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.Comment)).Italic(true)
	varStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.Variable)).Bold(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.Separator)).Bold(true)
	optionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.Option)).Bold(true)
	otherStyle = lipgloss.NewStyle()
	cursorStyle = lipgloss.NewStyle().Background(lipgloss.Color(curScheme.CursorBg)).Foreground(lipgloss.Color(curScheme.CursorFg)).Bold(true)
	selStyle = lipgloss.NewStyle().Background(lipgloss.Color(curScheme.SelectionBg))
	selStyleOnlyBg = lipgloss.NewStyle().Background(lipgloss.Color(curScheme.JSONSelBg))

	// JSON colors (jsoncolor.go)
	jsonKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.JSONKey))
	jsonStringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.JSONString))
	jsonNumberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.JSONNumber))
	jsonBoolStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.JSONBool))
	jsonNullStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.JSONNull))
	jsonPunctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.JSONPunct))

	// editor chrome (render_editor.go)
	runIconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.RunIcon))
	iconBlockStyle = lipgloss.NewStyle().Background(lipgloss.Color(curScheme.IconBlockBg)).Foreground(lipgloss.Color(curScheme.IconBlockFg)).Bold(true)
	numCurStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.NumCurrent)).Bold(true)
	numMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.NumMuted))
	scrollbarThumbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.ScrollThumb))
	scrollbarTrackStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.ScrollTrack))

	// files panel (render.go)
	fileSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.FileSelected)).Bold(true)
	fileHoverStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.FileHoverFg)).Background(lipgloss.Color(curScheme.FileHoverBg))

	// profiles (profiles.go)
	profileSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.ProfileSep))
	profileTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(curScheme.ProfileTitle))
	profileActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.ProfileActive)).Bold(true)
	profileSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.ProfileSel))
	profileNewSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.ProfileNew)).Bold(true)

	// help (help.go)
	helpKeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.HelpKey)).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(curScheme.HelpDesc))
	helpTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(curScheme.HelpTitle))

	// dialog (dialog.go)
	dialogTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(curScheme.DialogTitle))
	dialogBorderColor = curScheme.DialogBorder
	// borderColor() reads the resolved border colors:
	borderActiveColor = curScheme.BorderActive
	borderIdleColor = curScheme.BorderIdle
	// status bar / header / find bar
	statusBgColor = curScheme.StatusBg
	statusFgColor = curScheme.StatusFg
	headerBgColor = curScheme.HeaderBg
	headerFgColor = curScheme.HeaderFg
	searchBarBgColor = curScheme.SearchBarBg
	searchBarFgColor = curScheme.SearchBarFg
	// response executing / copy button
	executingColor = curScheme.Executing
	copyBtnFgColor = curScheme.CopyBtnFg
	copyBtnBgColor = curScheme.CopyBtnBg
	copyHoverFgColor = curScheme.CopyHoverFg
	copyHoverBgColor = curScheme.CopyHoverBg
	// action menu
	menuSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(menuSelFgColor)).Background(lipgloss.Color(menuSelBgColor))
	menuHoverStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(menuHoverFgColor)).Background(lipgloss.Color(menuHoverBgColor))
	menuNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(menuFgColor)).Background(lipgloss.Color(menuBgColor))
	menuSelFgColor = curScheme.MenuSelFg
	menuSelBgColor = curScheme.MenuSelBg
	menuHoverFgColor = curScheme.MenuHoverFg
	menuHoverBgColor = curScheme.MenuHoverBg
	menuFgColor = curScheme.MenuFg
	menuBgColor = curScheme.MenuBg
	// find highlights
	findStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(findFgColor)).Background(lipgloss.Color(findBgColor))
	findCurStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(findCurFgColor)).Background(lipgloss.Color(findCurBgColor))
	findFgColor = curScheme.FindFg
	findBgColor = curScheme.FindBg
	findCurFgColor = curScheme.FindCurFg
	findCurBgColor = curScheme.FindCurBg
}
