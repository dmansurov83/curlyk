package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/i18n"
	"github.com/user/curlyk/internal/settings"
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
	p := tea.NewProgram(tui.New(initialArgs()), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func initialArgs() tui.Args {
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	return tui.Args{FilePath: path}
}
