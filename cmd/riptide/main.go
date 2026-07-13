package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Foxemsx/riptide/internal/db"
	"github.com/Foxemsx/riptide/internal/theme"
	"github.com/Foxemsx/riptide/internal/ui"
)

var version = "dev"

func main() {
	themeList := strings.Join(theme.Names(), ", ")
	var (
		themeFlag   = flag.String("theme", "", "color theme: "+themeList+" (overrides saved preference)")
		compactFlag = flag.Bool("compact", false, "skip the large logo, show tagline only")
		versionFlag = flag.Bool("v", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of riptide:\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nThemes: %s\n", themeList)
		fmt.Fprintf(os.Stderr, "\nExamples:\n  riptide\n  riptide --compact\n  riptide --theme ocean\n")
	}
	flag.Parse()
	if *versionFlag {
		fmt.Printf("riptide %s\n", version)
		return
	}

	store, err := db.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "riptide: database: %v (continuing without history)\n", err)
		store = nil
	}

	themeName := "default"
	if store != nil {
		themeName = store.GetSetting("theme", "default")
	}
	if *themeFlag != "" {
		themeName = *themeFlag
		if store != nil {
			_ = store.SetSetting("theme", themeName)
		}
	}
	t := theme.Get(themeName)

	lipgloss.SetHasDarkBackground(true)
	fmt.Fprint(os.Stdout, "\x1b]11;"+t.HexBG()+"\a")
	fmt.Fprint(os.Stdout, "\x1b]10;"+t.HexFG()+"\a")
	defer func() {
		fmt.Fprint(os.Stdout, "\x1b]111\a\x1b]110\a")
	}()

	m := ui.NewApp(t, *compactFlag, store, version)
	defer m.Close()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "riptide: %v\n", err)
		os.Exit(1)
	}
}
