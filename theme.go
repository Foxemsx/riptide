package main

import "github.com/charmbracelet/lipgloss"

// Theme holds the reskinnable palette for the whole UI. Text colors use
// lipgloss.AdaptiveColor so they stay readable on light or dark terminals.
type Theme struct {
	// Foreground is the default text color (AdaptiveColor).
	Foreground lipgloss.AdaptiveColor
	// Muted is for labels / units / secondary text.
	Muted lipgloss.AdaptiveColor
	// Border is the card border color.
	Border lipgloss.AdaptiveColor
	// Download accent (down arrow, download number).
	Download lipgloss.AdaptiveColor
	// Upload accent.
	Upload lipgloss.AdaptiveColor
	// Latency accent.
	Latency lipgloss.AdaptiveColor
	// Highlight is used for peak values / summary emphasis.
	Highlight lipgloss.AdaptiveColor

	// Graph gradient endpoints (concrete colors, so the bars can be shaded
	// per-cell: dark at the base, brighter at the tip).
	GraphDownBottom lipgloss.Color // deep end of the download gradient
	GraphDownTop    lipgloss.Color // bright tip of the download gradient
	GraphUpBottom   lipgloss.Color // deep end of the upload gradient
	GraphUpTop      lipgloss.Color // bright tip of the upload gradient
}

// DefaultTheme is a modern dark dashboard palette with a deep slate background
// and a teal (download) / amber (upload) split. Picked to be distinguishable
// yet harmonious: cool tone for the incoming flow, warm tone for the outgoing
// flow, with muted greys for structure.
var DefaultTheme = Theme{
	Foreground: lipgloss.AdaptiveColor{Light: "#1c2128", Dark: "#e6edf3"},
	Muted:      lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#7d8590"},
	Border:     lipgloss.AdaptiveColor{Light: "#afb8c1", Dark: "#30363d"},
	Download:   lipgloss.AdaptiveColor{Light: "#0a7ea4", Dark: "#39d0d8"},
	Upload:     lipgloss.AdaptiveColor{Light: "#bc4c00", Dark: "#ffb454"},
	Latency:    lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#a371f7"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#7ee787"},

	// Download gradient: deep teal -> bright cyan.
	GraphDownBottom: lipgloss.Color("#0b5563"),
	GraphDownTop:    lipgloss.Color("#56e1e8"),
	// Upload gradient: deep amber -> warm gold.
	GraphUpBottom: lipgloss.Color("#8a3b00"),
	GraphUpTop:    lipgloss.Color("#ffc15e"),
}
