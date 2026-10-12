package tui

import (
	"fmt"
	"os"
	"testing"
)

// TestMain runs the whole package in a throwaway working directory.
//
// New() reads and writes appsettings.yml, last.session.http, default.profile and
// lists *.http / *.profile relative to the process working directory. Without
// this the tests would share the developer's real working directory: a stale
// appsettings.yml (last_opened_file, cursor_row, active_profile) left behind by
// the app or by an earlier test run would silently change which file every test
// opens, and the tests themselves would litter the repository. Tests that need
// their own directory still call chdir(t, t.TempDir()).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "curlyk-tui-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create the test working directory:", err)
		os.Exit(1)
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, "cannot enter the test working directory:", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()

	os.Chdir(wd)
	os.RemoveAll(dir)
	os.Exit(code)
}
