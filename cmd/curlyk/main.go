package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
	"github.com/user/curlyk/internal/theme"
	"github.com/user/curlyk/internal/tui"
)

func main() {
	if err := run(); err != nil {
		i18n.SetLocale(i18n.Resolve(settings.Load().Lang))
		fmt.Fprintln(os.Stderr, i18n.T("err.startup", err))
		os.Exit(1)
	}
}

func run() error {
	// A pure CLI operation: generate a custom theme template from a builtin
	// scheme and exit without launching the TUI.
	if len(os.Args) > 1 && os.Args[1] == "themetemplate" {
		return runThemeTemplate(os.Args[2:])
	}
	p := tea.NewProgram(tui.New(initialArgs()), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// runThemeTemplate handles `curlyk themetemplate <name> [--base <scheme>]`.
// It writes themes/<name>.theme containing the full field set of the selected
// builtin base scheme (default Darkula), so the user edits only what they want.
// Exits before starting the TUI.
func runThemeTemplate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: curlyk themetemplate <name> [--base default|darkula|light]")
	}
	name := args[0]
	baseName := theme.DarkulaName
	overwrite := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--base":
			if i+1 >= len(args) {
				return fmt.Errorf("--base requires a scheme name (default|darkula|light)")
			}
			baseName = args[i+1]
			i++
		case "--overwrite":
			overwrite = true
		default:
			return fmt.Errorf("unknown argument %q (expected --base or --overwrite)", args[i])
		}
	}
	base, ok := theme.ByName(baseName)
	if !ok {
		return fmt.Errorf("unknown base scheme %q (use default|darkula|light)", baseName)
	}
	if name == theme.DefaultName || name == theme.DarkulaName || name == theme.LightName {
		return fmt.Errorf("scheme name %q clashes with a builtin scheme", name)
	}
	out := theme.ThemeFilePath(name)
	if err := theme.WriteTemplate(name, theme.WriteTemplateOptions{Base: base, Overwrite: overwrite}, out); err != nil {
		return err
	}
	fmt.Printf("%s\n", i18n.T("theme.templateWritten", out))
	return nil
}

func initialArgs() tui.Args {
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	return tui.Args{FilePath: path}
}
