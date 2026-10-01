package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/curlyk/httptool/internal/tui"
)

func main() {
	m := tui.New(initialArgs())
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка запуска:", err)
		os.Exit(1)
	}
}

func initialArgs() tui.Args {
	var path string
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	return tui.Args{FilePath: path}
}