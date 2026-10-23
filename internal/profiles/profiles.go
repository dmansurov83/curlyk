// Package profiles locates, creates and reads curlyk environment profiles: the
// *.profile files in a directory whose @var declarations are merged into request
// variables when a request runs. It is UI-free, so the TUI sidebar and the
// desktop window share one definition of "what a profile is".
package profiles

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/user/curlyk/internal/httpfile"
)

// Ext is the file-name suffix of a profile file.
const Ext = ".profile"

// DefaultName is the profile file created when none exists yet.
const DefaultName = "default.profile"

// defaultContent is the skeleton written for a new profile.
const defaultContent = "# Default profile. Add @var name = value lines.\n" +
	"@var host = http://localhost\n" +
	"@var token = \n"

// List returns the sorted names of the *.profile files in dir. A missing or
// unreadable directory yields nil (no profiles).
func List(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), Ext) {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// DisplayName is the human-readable label of a profile file: its name without
// the .profile suffix.
func DisplayName(file string) string {
	return strings.TrimSuffix(file, Ext)
}

// Normalize turns a typed profile name into a profile file name, appending the
// .profile suffix when it is missing.
func Normalize(name string) string {
	if strings.HasSuffix(strings.ToLower(name), Ext) {
		return name
	}
	return name + Ext
}

// Load reads dir/name and returns the profile's @var declarations. It returns
// nil when the file cannot be read.
func Load(dir, name string) map[string]string {
	if name == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil
	}
	return httpfile.ParseProfileVars(string(data))
}

// Vars is Load with the profile's file name already resolved.
func Vars(dir, name string) map[string]string { return Load(dir, Normalize(name)) }

// Create writes a new profile file named name (the .profile suffix is added when
// missing) into dir and returns the file name it wrote. An existing file is
// never overwritten: the error from os.WriteFile is passed through.
func Create(dir, name string) (string, error) {
	name = Normalize(name)
	content := "# " + name + "\n@var host = http://localhost\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// EnsureDefault creates DefaultName in dir when there is no profile at all, so
// the profile pickers always have a usable entry. It never overwrites an
// existing file and reports whether it wrote one.
func EnsureDefault(dir string) (bool, error) {
	if len(List(dir)) > 0 {
		return false, nil
	}
	path := filepath.Join(dir, DefaultName)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(defaultContent), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// Pick returns the profile to activate given the names available in a directory
// and the name stored in the settings: the stored name when it exists, else the
// first available profile, else DefaultName. A profile picker is therefore never
// empty.
func Pick(available []string, want string) string {
	if want != "" && slices.Contains(available, want) {
		return want
	}
	if len(available) > 0 {
		return available[0]
	}
	return DefaultName
}
