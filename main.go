package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	var (
		themeFlag   = flag.String("theme", "default", "color theme: default")
		compactFlag = flag.Bool("compact", false, "skip the large logo, show tagline only")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of riptide:\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  riptide\n  riptide --compact\n")
	}
	flag.Parse()

	theme := DefaultTheme
	_ = themeFlag // reserved for future palettes

	// Force dark adaptive colors and paint the host terminal canvas so classic
	// pure-black consoles match the VS-style #191a1b chrome.
	lipgloss.SetHasDarkBackground(true)
	// OSC 11: set default background (Windows Terminal, modern xterm, etc.).
	fmt.Fprint(os.Stdout, "\x1b]11;#191a1b\a")
	// OSC 10: default foreground for unstyled text.
	fmt.Fprint(os.Stdout, "\x1b]10;#e8eaed\a")
	defer func() {
		// Restore terminal default colors on exit (best-effort).
		fmt.Fprint(os.Stdout, "\x1b]111\a\x1b]110\a")
	}()

	m := newAppModel(theme, *compactFlag)
	p := tea.NewProgram(&m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "riptide: %v\n", err)
		os.Exit(1)
	}
}
