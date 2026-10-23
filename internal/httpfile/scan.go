package httpfile

import (
	"os"
	"slices"
	"strings"
)

// FilesInDir returns the sorted names of the .http files in dir. A missing or
// unreadable directory yields nil. It backs the file list of both front ends, so
// the TUI sidebar and the desktop window's file panel always show the same set.
func FilesInDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".http") {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}
