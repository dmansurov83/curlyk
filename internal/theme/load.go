package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// themesDir is the directory scanned for custom `*.theme` schemes, relative to
// the working directory. It is a package var so tests can redirect it.
var themesDir = BuiltinDir

// SetThemesDir overrides the custom-theme directory (used by tests).
func SetThemesDir(dir string) { themesDir = dir }

// ThemesDir returns the current custom-theme directory.
func ThemesDir() string { return themesDir }

// LoadFile reads a custom scheme from a YAML file at path. The file may define
// only a subset of fields: every omitted field is inherited from Default, so a
// user only declares what they want to change. Unknown fields are ignored.
func LoadFile(path string) (Scheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scheme{}, err
	}
	base := Default()
	var overrides map[string]any
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return Scheme{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	// Re-marshal to a typed Scheme so only known fields are accepted and the
	// default fallback is automatic.
	typed, err := yaml.Marshal(overrides)
	if err != nil {
		return Scheme{}, err
	}
	merged := base
	if err := yaml.Unmarshal(typed, &merged); err != nil {
		return Scheme{}, err
	}
	return merged, nil
}

// ListFiles scans the themes directory (relative to the working directory) and
// returns the base names (without extension) of every `*.theme` file, sorted.
// A missing themes directory yields an empty list, not an error.
func ListFiles() []string {
	return ListFilesInDir(themesDir)
}

// ListFilesInDir scans a specific directory for `*.theme` files (used by tests
// and to keep the working directory independent). Returns sorted names without
// the extension.
func ListFilesInDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ThemeExt) {
			out = append(out, strings.TrimSuffix(e.Name(), ThemeExt))
		}
	}
	slices.Sort(out)
	return out
}

// themeDir returns the absolute path of the theme directory.
func themeDir() string {
	abs, err := filepath.Abs(themesDir)
	if err != nil {
		return themesDir
	}
	return abs
}

// ThemeFilePath returns the absolute path where a custom scheme named name
// would live: `themes/<name>.theme`.
func ThemeFilePath(name string) string {
	return filepath.Join(themeDir(), name+ThemeExt)
}
