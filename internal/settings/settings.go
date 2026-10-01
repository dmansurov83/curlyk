package settings

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Settings holds the persistent app configuration (appsettings.yml).
type Settings struct {
	// LastOpenedFile is the path of the last explicitly opened .http file.
	LastOpenedFile string `yaml:"last_opened_file"`
	// SessionFile is where unsaved editor content is persisted on exit.
	SessionFile string `yaml:"session_file"`
	// DefaultFile is the sample content shown when nothing else is available.
	DefaultFile string `yaml:"default_file,omitempty"`
	// CursorRow / CursorCol restore the editor cursor position on open.
	CursorRow int `yaml:"cursor_row"`
	CursorCol int `yaml:"cursor_col"`
	// EditorScroll restores the vertical scroll offset.
	EditorScroll int `yaml:"editor_scroll"`
	// ActivePane restores the active pane (0 = editor, 1 = response).
	ActivePane int `yaml:"active_pane"`
	// FileCursors remembers the cursor position per opened file, keyed by the
	// file's path, so switching between files and across restarts restores it.
	FileCursors map[string]CursorPos `yaml:"file_cursors,omitempty"`
}

// CursorPos is a remembered cursor position for a single file.
type CursorPos struct {
	Row    int `yaml:"row"`
	Col    int `yaml:"col"`
	Scroll int `yaml:"scroll"`
}

// Default returns settings with sensible defaults.
func Default() Settings {
	return Settings{
		SessionFile: "last.session.http",
	}
}

// appSettingsPath is the path to appsettings.yml (overridable for tests).
var appSettingsPath = func() string {
	return "appsettings.yml"
}

// LoadAt reads settings from the given path. Returns defaults if the file is
// missing or malformed.
func LoadAt(path string) Settings {
	s := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Default()
	}
	return s
}

// Load reads appsettings.yml from the working directory. Returns defaults if
// the file is missing or malformed.
func Load() Settings {
	return LoadAt(appSettingsPath())
}

// SaveAt writes the settings to the given path.
func (s Settings) SaveAt(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Save writes the settings to appsettings.yml.
func (s Settings) Save() error {
	return s.SaveAt(appSettingsPath())
}

// SetAppSettingsPath overrides where appsettings.yml lives (tests).
func SetAppSettingsPath(p string) { appSettingsPath = func() string { return p } }
