package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка запуска:", err)
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
