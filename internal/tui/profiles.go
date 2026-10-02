package tui

import (
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/curlyk/internal/httpfile"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
)

// profileSepStyle draws the horizontal separator above the profile section in
// the left sidebar.
var profileSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// profileTitleStyle styles the "Профили" header of the profile section.
var profileTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))

// profileActiveStyle highlights the active profile name in the sidebar.
var profileActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

// profileSelStyle highlights the currently focused (but not active) profile row.
var profileSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

// profileNewSelStyle highlights the "+ Новый профиль" action row when focused.
var profileNewSelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)

// profileFilesInDir lists *.profile filenames in path (sorted). These are the
// available environment profiles whose @var declarations feed request variables.
func profileFilesInDir(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".profile") {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// fileListMaxRows returns how many interior rows the file list area of the left
// sidebar gets, after reserving room for the profile section at the bottom.
func (m *model) fileListMaxRows() int {
	H := m.filesPaneH()
	bottom := m.profileAreaRows()
	maxRows := H - 2 - bottom // 2 rows: search box + blank separator
	if maxRows < 0 {
		maxRows = 0
	}
	return maxRows
}

// filesPaneH returns the number of interior rows of the left files pane.
func (m *model) filesPaneH() int {
	h := m.height - 6
	if h < 0 {
		h = 0
	}
	return h
}

// profileAreaRows returns the number of interior rows the profile section of
// the left sidebar occupies: a separator, a title, one row per profile (or a
// hint when there are none) and a "+ Новый профиль" action row. Always non-zero
// so the file list reserves room for the profile section.
func (m *model) profileAreaRows() int {
	rows := 1 + 1 + 1 // separator + title + New-profile action
	n := 0
	if m.filesPanel != nil {
		n = len(m.filesPanel.profiles)
	}
	if n > 0 {
		rows += n
	} else {
		rows += 1 // "нет *.profile" hint line
	}
	return rows
}

// profileTitleRowAbs returns the absolute screen y of the profile section
// title row (used to map mouse clicks onto profile rows). The value is only
// meaningful when profiles exist.
func (m *model) profileTitleRowAbs() int {
	// filesView rows: search(1) + blank(1) + fileMax file rows + separator(1),
	// then the title. Content starts at headerHeight+1.
	return (headerHeight + 1) + 2 + m.fileListMaxRows() + 1
}

// profileRowAbs returns the absolute screen y of a profile row (0-based index
// into the profile list).
func (m *model) profileRowAbs(idx int) int {
	// title row is profileTitleRowAbs; rows follow beneath it.
	return m.profileTitleRowAbs() + 1 + idx
}

// profileNewRowAbs returns the absolute screen y of the "+ Новый профиль"
// action row (the last row of the profile section).
func (m *model) profileNewRowAbs() int {
	return m.profileTitleRowAbs() + 1 + len(m.filesPanel.profiles)
}

// mouseToProfileNewRow reports whether the absolute screen y is on the
// "+ Новый профиль" action row.
func (m *model) mouseToProfileNewRow(y int) bool {
	if m.filesPanel == nil {
		return false
	}
	return y == m.profileNewRowAbs()
}

// mouseToProfileRow maps an absolute screen y to a profile index, or ok=false
// when the click is not on a profile row.
func (m *model) mouseToProfileRow(y int) (int, bool) {
	if m.filesPanel == nil || len(m.filesPanel.profiles) == 0 {
		return 0, false
	}
	first := m.profileRowAbs(0)
	idx := y - first
	if idx < 0 || idx >= len(m.filesPanel.profiles) {
		return 0, false
	}
	return idx, true
}

// activeProfileVars returns the @var map of the active profile file, or nil if
// no profile is active or its file cannot be read/parsed.
func (m *model) activeProfileVars() map[string]string {
	if m.profile == "" {
		return nil
	}
	data, err := os.ReadFile(m.profile)
	if err != nil {
		return nil
	}
	return httpfile.ParseProfileVars(string(data))
}

// defaultProfileName is the file created when no profile exists yet.
const defaultProfileName = "default.profile"

// ensureDefaultProfile creates a default.profile in the working directory when
// there are no *.profile files at all, so the profile section always has a
// usable entry and profiling works out of the box. It never overwrites an
// existing file.
func ensureDefaultProfile() {
	if len(profileFilesInDir(".")) > 0 {
		return
	}
	if _, err := os.Stat(defaultProfileName); err == nil {
		return
	}
	content := "# Default profile. Add @var name = value lines.\n" +
		"@var host = http://localhost\n" +
		"@var token = \n"
	_ = os.WriteFile(defaultProfileName, []byte(content), 0o644)
}

// beginProfileAs opens an input to type the name of a new profile.
func (m *model) beginProfileAs() {
	ti := textinput.New()
	ti.Placeholder = i18n.T("placeholder.profileName")
	ti.Focus()
	ti.Width = 30
	m.profileAs = &ti
	m.active = paneEdit
}

// createProfile creates a new profile file from the typed name, adds it to the
// panel and activates it.
func (m *model) createProfile(name string) {
	if !strings.HasSuffix(strings.ToLower(name), ".profile") {
		name += ".profile"
	}
	content := "# " + name + "\n@var host = http://localhost\n"
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		m.status = i18n.T("err.save", err.Error())
		return
	}
	if m.filesPanel == nil {
		m.filesPanel = &filesPanel{}
	}
	m.filesPanel.loadProfiles()
	m.selectProfile(name)
	m.status = i18n.T("profile.created", strings.TrimSuffix(name, ".profile"))
}

// selectProfile activates a profile by file name. Clicking a profile switches
// to it; selecting the already-active profile is a no-op (one profile is always
// active). The choice is persisted to appsettings.yml.
func (m *model) selectProfile(name string) {
	m.setActiveProfile(name)
}

// setActiveProfile selects a profile by file name, persists it to appsettings.yml
// and updates the status line. The active profile is never empty:  if name is
// empty, the first available profile is used instead.
func (m *model) setActiveProfile(name string) {
	if name == "" {
		// never leave the app with no active profile: fall back to the first
		// configured one, or the default file if none exist.
		profs := m.availableProfiles()
		if len(profs) > 0 {
			name = profs[0]
		} else {
			name = defaultProfileName
		}
	}
	m.profile = name
	s := settings.Load()
	if s.ActiveProfile == name {
		return
	}
	s.ActiveProfile = name
	_ = s.Save()
	m.status = i18n.T("profile.activated", strings.TrimSuffix(name, ".profile"))
}

// availableProfiles returns the profile file names known to the panel, falling
// back to a directory scan when the panel is not populated yet.
func (m *model) availableProfiles() []string {
	if m.filesPanel != nil && len(m.filesPanel.profiles) > 0 {
		return m.filesPanel.profiles
	}
	return profileFilesInDir(".")
}

// profileRowLine renders one row of the profile section: a leading marker, the
// profile display name and a "текущий" badge for the active one. hover picks the
// mouse-hover highlight, like files.
func (m *model) profileRowLine(width, idx int, hover bool) string {
	p := m.filesPanel
	name := p.profiles[idx]
	display := strings.TrimSuffix(name, ".profile")
	active := name == m.profile
	sel := p.onProfiles && !p.profNew && p.profSel == idx

	switch {
	case active && hover:
		// show both the active badge and the hover background so hovering the
		// active profile still gives visible feedback (like files)
		row := fileHoverStyle.Render("▸ " + display + "  " + i18n.T("profile.activeBadge"))
		return padToWidth(row, width-2)
	case active:
		row := profileActiveStyle.Render("▸ " + display + "  " + i18n.T("profile.activeBadge"))
		return padToWidth(row, width-2)
	case sel:
		return padToWidth(profileSelStyle.Render("  "+display), width-2)
	case hover:
		return padToWidth(fileHoverStyle.Render("  "+display), width-2)
	default:
		return padToWidth("  "+display, width-2)
	}
}
