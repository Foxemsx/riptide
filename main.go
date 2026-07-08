package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		themeFlag = flag.String("theme", "default", "color theme: default")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of speed:\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  speed\n")
	}
	flag.Parse()

	theme := DefaultTheme
	_ = themeFlag // reserved for future palettes

	m := newAppModel(theme)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "speed: %v\n", err)
		os.Exit(1)
	}
}
