package main

import "github.com/charmbracelet/lipgloss"

// Theme holds the reskinnable palette for the whole UI. Text colors use
// lipgloss.AdaptiveColor so they stay readable on light or dark terminals.
type Theme struct {
	// AppBg is the full-screen terminal canvas color (VS Code / modern CMD grey).
	AppBg lipgloss.Color

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

	// MenuAccentFill is a very faint background used for selected menu cards
	// to give a subtle "filled" modern card feel without being loud.
	MenuAccentFill lipgloss.AdaptiveColor

	// Concrete fills for modern menu buttons (selected state). Adaptive colors
	// alone can leave holes under nested styles; these are solid hex fills.
	MenuIdleFill   lipgloss.Color // unselected panel
	MenuSelectDL   lipgloss.Color // speed-test selected
	MenuSelectUL   lipgloss.Color // bandwidth selected
	MenuSelectExit lipgloss.Color // exit selected
}

// DefaultTheme is a modern dark dashboard palette on a VS-style #191a1b canvas,
// with a teal (download) / amber (upload) split.
var DefaultTheme = Theme{
	// Full-screen canvas — charcoal grey, not pure black.
	AppBg: lipgloss.Color("#191a1b"),

	Foreground: lipgloss.AdaptiveColor{Light: "#1c2128", Dark: "#e8eaed"},
	Muted:      lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#8b919a"},
	Border:     lipgloss.AdaptiveColor{Light: "#afb8c1", Dark: "#3a3d40"},
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

	// Selected card background (slightly lifted from AppBg).
	MenuAccentFill: lipgloss.AdaptiveColor{Light: "#e8ecf2", Dark: "#25282c"},

	// Menu button surfaces — slightly above AppBg so panels read as cards.
	MenuIdleFill:   lipgloss.Color("#222426"),
	MenuSelectDL:   lipgloss.Color("#1a2e34"), // teal glass on charcoal
	MenuSelectUL:   lipgloss.Color("#2e2418"), // amber glass on charcoal
	MenuSelectExit: lipgloss.Color("#1a2a1e"), // green glass on charcoal
}

// paintScreen fills the terminal with AppBg and centers content on it so the
// UI never shows the host console's pure-black default.
func paintScreen(theme Theme, width, height int, content string) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		content,
		lipgloss.WithWhitespaceBackground(theme.AppBg),
	)
}
