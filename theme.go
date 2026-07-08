package main

import "github.com/charmbracelet/lipgloss"

// Theme holds the reskinnable palette for the whole UI. Colors are stored as
// lipgloss.Color so they can be applied to both the full-screen background and
// individual text styles. Text colors use lipgloss.AdaptiveColor so they stay
// readable on light or dark backgrounds.
type Theme struct {
	// Background is the full-screen terminal background. May be empty, in
	// which case we fall back to the terminal's own default background.
	Background lipgloss.Color
	// Foreground is the default text color (AdaptiveColor).
	Foreground lipgloss.AdaptiveColor
	// Muted is for labels / units / secondary text.
	Muted lipgloss.AdaptiveColor
	// Border is the card border color.
	Border lipgloss.AdaptiveColor
	// Download accent (down arrow, download sparkline/bars, download number).
	Download lipgloss.AdaptiveColor
	// Upload accent.
	Upload lipgloss.AdaptiveColor
	// Latency accent.
	Latency lipgloss.AdaptiveColor
	// Highlight is used for peak values / summary emphasis.
	Highlight lipgloss.AdaptiveColor
}

// DefaultTheme is a modern dark dashboard palette with a deep slate background
// and a teal (download) / amber (upload) split. Picked to be distinguishable
// yet harmonious: cool tone for the incoming flow, warm tone for the outgoing
// flow, with muted greys for structure.
var DefaultTheme = Theme{
	Background:  lipgloss.Color("#0d1117"),
	Foreground:  lipgloss.AdaptiveColor{Light: "#1c2128", Dark: "#e6edf3"},
	Muted:       lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#7d8590"},
	Border:      lipgloss.AdaptiveColor{Light: "#afb8c1", Dark: "#30363d"},
	Download:    lipgloss.AdaptiveColor{Light: "#0a7ea4", Dark: "#39d0d8"},
	Upload:      lipgloss.AdaptiveColor{Light: "#bc4c00", Dark: "#ffb454"},
	Latency:     lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#a371f7"},
	Highlight:   lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#7ee787"},
}

// NoBgTheme is used when the user does not pass --bg: the card still renders
// but the surrounding terminal keeps its native background.
var NoBgTheme = func() Theme {
	t := DefaultTheme
	t.Background = ""
	return t
}()
