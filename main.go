package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := checkNetwork(); err != nil {
		fmt.Fprintln(os.Stderr, "No internet connection.")
		os.Exit(1)
	}

	targets, err := fetchTargets(dlConns)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			fmt.Fprintln(os.Stderr, "Network error:", err)
			os.Exit(1)
		}
		log.Fatal(err)
	}

	p := tea.NewProgram(NewModel(targets), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
