package theme

import (
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
)

// TransparentBg controls whether the terminal canvas background fill is
// skipped. When true the host terminal's native background (including
// transparency) shows through. Toggled via Settings → Terminal BG (key 4).
var TransparentBg atomic.Bool

// Theme holds the reskinnable palette for the whole UI. Text colors use
// lipgloss.AdaptiveColor so they stay readable on light or dark terminals.
type Theme struct {
	// Name is the stable id used in flags and settings (e.g. "default").
	Name string
	// Display is the human-facing label shown in the settings picker.
	Display string
	// Tagline is a short vibe line for the picker.
	Tagline string

	// AppBg is the full-screen terminal canvas color.
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

	// Graph gradient endpoints (concrete colors for per-cell shading).
	GraphDownBottom lipgloss.Color
	GraphDownTop    lipgloss.Color
	GraphUpBottom   lipgloss.Color
	GraphUpTop      lipgloss.Color

	// MenuAccentFill is a faint background for selected menu cards.
	MenuAccentFill lipgloss.AdaptiveColor

	// Concrete fills for modern menu buttons (selected state).
	MenuIdleFill   lipgloss.Color
	MenuSelectDL   lipgloss.Color
	MenuSelectUL   lipgloss.Color
	MenuSelectExit lipgloss.Color
	MenuSelectSet  lipgloss.Color // settings selected

	// Concrete solid accents for chips / pills (avoid AdaptiveColor holes).
	AccentDL  lipgloss.Color
	AccentUL  lipgloss.Color
	AccentLat lipgloss.Color
	AccentHL  lipgloss.Color

	// LogoStops is a 4-stop vertical gradient for the RIPTIDE wordmark.
	LogoStops [4][3]uint8
}

// DefaultTheme is a modern dark dashboard palette on a VS-style #191a1b canvas,
// with a teal (download) / amber (upload) split.
var DefaultTheme = Theme{
	Name:    "default",
	Display: "Default",
	Tagline: "Teal & amber on charcoal",
	AppBg:   lipgloss.Color("#191a1b"),

	Foreground: lipgloss.AdaptiveColor{Light: "#1c2128", Dark: "#e8eaed"},
	Muted:      lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#8b919a"},
	Border:     lipgloss.AdaptiveColor{Light: "#afb8c1", Dark: "#3a3d40"},
	Download:   lipgloss.AdaptiveColor{Light: "#0a7ea4", Dark: "#39d0d8"},
	Upload:     lipgloss.AdaptiveColor{Light: "#bc4c00", Dark: "#ffb454"},
	Latency:    lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#a371f7"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#7ee787"},

	GraphDownBottom: lipgloss.Color("#0b5563"),
	GraphDownTop:    lipgloss.Color("#56e1e8"),
	GraphUpBottom:   lipgloss.Color("#8a3b00"),
	GraphUpTop:      lipgloss.Color("#ffc15e"),

	MenuAccentFill: lipgloss.AdaptiveColor{Light: "#e8ecf2", Dark: "#25282c"},

	MenuIdleFill:   lipgloss.Color("#222426"),
	MenuSelectDL:   lipgloss.Color("#1a2e34"),
	MenuSelectUL:   lipgloss.Color("#2e2418"),
	MenuSelectExit: lipgloss.Color("#1a2a1e"),
	MenuSelectSet:  lipgloss.Color("#241a2e"),

	AccentDL:  lipgloss.Color("#39d0d8"),
	AccentUL:  lipgloss.Color("#ffb454"),
	AccentLat: lipgloss.Color("#a371f7"),
	AccentHL:  lipgloss.Color("#7ee787"),

	LogoStops: [4][3]uint8{
		{0x0e, 0x4d, 0x64},
		{0x08, 0x83, 0x95},
		{0x14, 0xc4, 0xd4},
		{0x9a, 0xf5, 0xf8},
	},
}

// All is every built-in palette, default first.
var All = []Theme{
	DefaultTheme,
	oceanTheme,
	midnightTheme,
	sunsetTheme,
	forestTheme,
	roseTheme,
	nordTheme,
	draculaTheme,
	cyberTheme,
	emberTheme,
	arcticTheme,
	monoTheme,
	signalTheme,
	inkTheme,
}

var byName = func() map[string]Theme {
	m := make(map[string]Theme, len(All))
	for _, t := range All {
		m[strings.ToLower(t.Name)] = t
	}
	return m
}()

// Get returns a theme by name (case-insensitive). Unknown names fall back to DefaultTheme.
func Get(name string) Theme {
	if t, ok := byName[strings.ToLower(strings.TrimSpace(name))]; ok {
		return t
	}
	return DefaultTheme
}

// Names returns every theme id in display order.
func Names() []string {
	out := make([]string, len(All))
	for i, t := range All {
		out[i] = t.Name
	}
	return out
}

// PaintScreen fills the terminal with AppBg and centers content on it so the
// UI never shows the host console's pure-black default.
func PaintScreen(t Theme, width, height int, content string) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	// macOS transparent terminals (Ghostty, iTerm2): ANSI resets embedded in
	// the content clear the background, exposing the desktop through gaps.
	// Wrap every line with AppBg so no cell is left without a background.
	// runtime.GOOS is a compile-time constant — this branch is eliminated on
	// Linux/Windows builds with zero binary impact.
	// Skipped when the user enables transparent mode via Settings.
	if runtime.GOOS == "darwin" && !TransparentBg.Load() {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lines[i] = lipgloss.NewStyle().Background(t.AppBg).Render(line)
		}
		content = strings.Join(lines, "\n")
	}
	var opts []lipgloss.WhitespaceOption
	if !TransparentBg.Load() {
		opts = append(opts, lipgloss.WithWhitespaceBackground(t.AppBg))
	}
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		content,
		opts...,
	)
}

// Hex returns the AppBg as a #rrggbb string for OSC sequences.
func (t Theme) HexBG() string {
	s := string(t.AppBg)
	if s == "" {
		return "#191a1b"
	}
	return s
}

// HexFG returns a concrete foreground for OSC sequences.
func (t Theme) HexFG() string {
	if t.AccentDL != "" {
		// Prefer a soft off-white derived from foreground dark branch.
		return "#e8eaed"
	}
	return "#e8eaed"
}

// --- Built-in palettes ---------------------------------------------------

var oceanTheme = Theme{
	Name: "ocean", Display: "Ocean", Tagline: "Deep sea · cyan foam",
	AppBg: lipgloss.Color("#0b1420"),
	Foreground: lipgloss.AdaptiveColor{Light: "#0c1a28", Dark: "#e0f2fe"},
	Muted:      lipgloss.AdaptiveColor{Light: "#4a6a80", Dark: "#7aa0b8"},
	Border:     lipgloss.AdaptiveColor{Light: "#90b0c4", Dark: "#1e3a4f"},
	Download:   lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"},
	Upload:     lipgloss.AdaptiveColor{Light: "#0d9488", Dark: "#2dd4bf"},
	Latency:    lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#67e8f9"},
	GraphDownBottom: lipgloss.Color("#0c4a6e"), GraphDownTop: lipgloss.Color("#7dd3fc"),
	GraphUpBottom:   lipgloss.Color("#115e59"), GraphUpTop:    lipgloss.Color("#5eead4"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#e0f2fe", Dark: "#122536"},
	MenuIdleFill: lipgloss.Color("#112031"), MenuSelectDL: lipgloss.Color("#0f2d44"),
	MenuSelectUL: lipgloss.Color("#0f2f2c"), MenuSelectExit: lipgloss.Color("#0f2838"),
	MenuSelectSet: lipgloss.Color("#152a48"),
	AccentDL: lipgloss.Color("#38bdf8"), AccentUL: lipgloss.Color("#2dd4bf"),
	AccentLat: lipgloss.Color("#60a5fa"), AccentHL: lipgloss.Color("#67e8f9"),

	LogoStops: [4][3]uint8{
		{0x0c, 0x4a, 0x6e},
		{0x31, 0x77, 0x9c},
		{0x7d, 0xd3, 0xfc},
		{0xb1, 0xe4, 0xfd},
	},
}

var midnightTheme = Theme{
	Name: "midnight", Display: "Midnight", Tagline: "Electric blue · violet night",
	AppBg: lipgloss.Color("#0a0a12"),
	Foreground: lipgloss.AdaptiveColor{Light: "#1a1a28", Dark: "#ececf4"},
	Muted:      lipgloss.AdaptiveColor{Light: "#5a5a78", Dark: "#8b8ba8"},
	Border:     lipgloss.AdaptiveColor{Light: "#a0a0c0", Dark: "#2a2a40"},
	Download:   lipgloss.AdaptiveColor{Light: "#4f46e5", Dark: "#818cf8"},
	Upload:     lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#c084fc"},
	Latency:    lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#38bdf8"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#6366f1", Dark: "#a5b4fc"},
	GraphDownBottom: lipgloss.Color("#312e81"), GraphDownTop: lipgloss.Color("#a5b4fc"),
	GraphUpBottom:   lipgloss.Color("#4c1d95"), GraphUpTop:    lipgloss.Color("#e9d5ff"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#eef0ff", Dark: "#14142a"},
	MenuIdleFill: lipgloss.Color("#12121e"), MenuSelectDL: lipgloss.Color("#1a1a38"),
	MenuSelectUL: lipgloss.Color("#221538"), MenuSelectExit: lipgloss.Color("#161828"),
	MenuSelectSet: lipgloss.Color("#1a1830"),
	AccentDL: lipgloss.Color("#818cf8"), AccentUL: lipgloss.Color("#c084fc"),
	AccentLat: lipgloss.Color("#38bdf8"), AccentHL: lipgloss.Color("#a5b4fc"),

	LogoStops: [4][3]uint8{
		{0x31, 0x2e, 0x81},
		{0x57, 0x5a, 0xa9},
		{0xa5, 0xb4, 0xfc},
		{0xc9, 0xd2, 0xfd},
	},
}

var sunsetTheme = Theme{
	Name: "sunset", Display: "Sunset", Tagline: "Coral dusk · warm gold",
	AppBg: lipgloss.Color("#1a1210"),
	Foreground: lipgloss.AdaptiveColor{Light: "#2a1810", Dark: "#fef3e8"},
	Muted:      lipgloss.AdaptiveColor{Light: "#8a6050", Dark: "#b89888"},
	Border:     lipgloss.AdaptiveColor{Light: "#c0a090", Dark: "#3d2a24"},
	Download:   lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#fb923c"},
	Upload:     lipgloss.AdaptiveColor{Light: "#e11d48", Dark: "#fb7185"},
	Latency:    lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#fbbf24"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#c2410c", Dark: "#fdba74"},
	GraphDownBottom: lipgloss.Color("#9a3412"), GraphDownTop: lipgloss.Color("#fdba74"),
	GraphUpBottom:   lipgloss.Color("#9f1239"), GraphUpTop:    lipgloss.Color("#fda4af"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#fff1e8", Dark: "#261a16"},
	MenuIdleFill: lipgloss.Color("#221816"), MenuSelectDL: lipgloss.Color("#352018"),
	MenuSelectUL: lipgloss.Color("#351820"), MenuSelectExit: lipgloss.Color("#2a2018"),
	MenuSelectSet: lipgloss.Color("#302018"),
	AccentDL: lipgloss.Color("#fb923c"), AccentUL: lipgloss.Color("#fb7185"),
	AccentLat: lipgloss.Color("#fbbf24"), AccentHL: lipgloss.Color("#fdba74"),

	LogoStops: [4][3]uint8{
		{0x9a, 0x34, 0x12},
		{0xba, 0x60, 0x32},
		{0xfd, 0xba, 0x74},
		{0xfd, 0xd5, 0xab},
	},
}

var forestTheme = Theme{
	Name: "forest", Display: "Forest", Tagline: "Moss · gold canopy",
	AppBg: lipgloss.Color("#0f1612"),
	Foreground: lipgloss.AdaptiveColor{Light: "#14201a", Dark: "#e8f5e9"},
	Muted:      lipgloss.AdaptiveColor{Light: "#4a6a55", Dark: "#8aaa90"},
	Border:     lipgloss.AdaptiveColor{Light: "#90b0a0", Dark: "#2a3a30"},
	Download:   lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#4ade80"},
	Upload:     lipgloss.AdaptiveColor{Light: "#a16207", Dark: "#facc15"},
	Latency:    lipgloss.AdaptiveColor{Light: "#0f766e", Dark: "#2dd4bf"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#86efac"},
	GraphDownBottom: lipgloss.Color("#14532d"), GraphDownTop: lipgloss.Color("#86efac"),
	GraphUpBottom:   lipgloss.Color("#713f12"), GraphUpTop:    lipgloss.Color("#fde047"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#e8f5e9", Dark: "#16241c"},
	MenuIdleFill: lipgloss.Color("#15201a"), MenuSelectDL: lipgloss.Color("#1a3024"),
	MenuSelectUL: lipgloss.Color("#2a2818"), MenuSelectExit: lipgloss.Color("#1a2820"),
	MenuSelectSet: lipgloss.Color("#1a2824"),
	AccentDL: lipgloss.Color("#4ade80"), AccentUL: lipgloss.Color("#facc15"),
	AccentLat: lipgloss.Color("#2dd4bf"), AccentHL: lipgloss.Color("#86efac"),

	LogoStops: [4][3]uint8{
		{0x14, 0x53, 0x2d},
		{0x39, 0x86, 0x56},
		{0x86, 0xef, 0xac},
		{0xb6, 0xf5, 0xcd},
	},
}

var roseTheme = Theme{
	Name: "rose", Display: "Rose", Tagline: "Blush · soft magenta",
	AppBg: lipgloss.Color("#161014"),
	Foreground: lipgloss.AdaptiveColor{Light: "#281820", Dark: "#fce7f3"},
	Muted:      lipgloss.AdaptiveColor{Light: "#8a6078", Dark: "#b898a8"},
	Border:     lipgloss.AdaptiveColor{Light: "#c0a0b0", Dark: "#3a2834"},
	Download:   lipgloss.AdaptiveColor{Light: "#db2777", Dark: "#f472b6"},
	Upload:     lipgloss.AdaptiveColor{Light: "#c026d3", Dark: "#e879f9"},
	Latency:    lipgloss.AdaptiveColor{Light: "#9333ea", Dark: "#c084fc"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#be185d", Dark: "#f9a8d4"},
	GraphDownBottom: lipgloss.Color("#9d174d"), GraphDownTop: lipgloss.Color("#f9a8d4"),
	GraphUpBottom:   lipgloss.Color("#86198f"), GraphUpTop:    lipgloss.Color("#f0abfc"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#fce7f3", Dark: "#241820"},
	MenuIdleFill: lipgloss.Color("#1e161c"), MenuSelectDL: lipgloss.Color("#301828"),
	MenuSelectUL: lipgloss.Color("#2c1830"), MenuSelectExit: lipgloss.Color("#281820"),
	MenuSelectSet: lipgloss.Color("#2a1830"),
	AccentDL: lipgloss.Color("#f472b6"), AccentUL: lipgloss.Color("#e879f9"),
	AccentLat: lipgloss.Color("#c084fc"), AccentHL: lipgloss.Color("#f9a8d4"),

	LogoStops: [4][3]uint8{
		{0x9d, 0x17, 0x4d},
		{0xbb, 0x46, 0x79},
		{0xf9, 0xa8, 0xd4},
		{0xfb, 0xca, 0xe5},
	},
}

var nordTheme = Theme{
	Name: "nord", Display: "Nord", Tagline: "Frost · polar aurora",
	AppBg: lipgloss.Color("#2e3440"),
	Foreground: lipgloss.AdaptiveColor{Light: "#2e3440", Dark: "#eceff4"},
	Muted:      lipgloss.AdaptiveColor{Light: "#4c566a", Dark: "#a3adc2"},
	Border:     lipgloss.AdaptiveColor{Light: "#d8dee9", Dark: "#3b4252"},
	Download:   lipgloss.AdaptiveColor{Light: "#5e81ac", Dark: "#88c0d0"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b48ead", Dark: "#b48ead"},
	Latency:    lipgloss.AdaptiveColor{Light: "#81a1c1", Dark: "#81a1c1"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#a3be8c", Dark: "#a3be8c"},
	GraphDownBottom: lipgloss.Color("#3b6a8a"), GraphDownTop: lipgloss.Color("#8fbcbb"),
	GraphUpBottom:   lipgloss.Color("#6b4f6a"), GraphUpTop:    lipgloss.Color("#d8b4d0"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#eceff4", Dark: "#343b49"},
	MenuIdleFill: lipgloss.Color("#343b49"), MenuSelectDL: lipgloss.Color("#3a4a58"),
	MenuSelectUL: lipgloss.Color("#443a4c"), MenuSelectExit: lipgloss.Color("#3a453c"),
	MenuSelectSet: lipgloss.Color("#3a4250"),
	AccentDL: lipgloss.Color("#88c0d0"), AccentUL: lipgloss.Color("#b48ead"),
	AccentLat: lipgloss.Color("#81a1c1"), AccentHL: lipgloss.Color("#a3be8c"),

	LogoStops: [4][3]uint8{
		{0x3b, 0x6a, 0x8a},
		{0x56, 0x85, 0x9a},
		{0x8f, 0xbc, 0xbb},
		{0xbb, 0xd6, 0xd6},
	},
}

var draculaTheme = Theme{
	Name: "dracula", Display: "Dracula", Tagline: "Purple night · neon pink",
	AppBg: lipgloss.Color("#282a36"),
	Foreground: lipgloss.AdaptiveColor{Light: "#282a36", Dark: "#f8f8f2"},
	Muted:      lipgloss.AdaptiveColor{Light: "#6272a4", Dark: "#9aa5ce"},
	Border:     lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#44475a"},
	Download:   lipgloss.AdaptiveColor{Light: "#8be9fd", Dark: "#8be9fd"},
	Upload:     lipgloss.AdaptiveColor{Light: "#ff79c6", Dark: "#ff79c6"},
	Latency:    lipgloss.AdaptiveColor{Light: "#bd93f9", Dark: "#bd93f9"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#50fa7b", Dark: "#50fa7b"},
	GraphDownBottom: lipgloss.Color("#2a6a78"), GraphDownTop: lipgloss.Color("#8be9fd"),
	GraphUpBottom:   lipgloss.Color("#8a3a68"), GraphUpTop:    lipgloss.Color("#ff79c6"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#f0e6ff", Dark: "#343746"},
	MenuIdleFill: lipgloss.Color("#2f3240"), MenuSelectDL: lipgloss.Color("#2a3a48"),
	MenuSelectUL: lipgloss.Color("#3a2a40"), MenuSelectExit: lipgloss.Color("#2a3a30"),
	MenuSelectSet: lipgloss.Color("#322a48"),
	AccentDL: lipgloss.Color("#8be9fd"), AccentUL: lipgloss.Color("#ff79c6"),
	AccentLat: lipgloss.Color("#bd93f9"), AccentHL: lipgloss.Color("#50fa7b"),

	LogoStops: [4][3]uint8{
		{0x2a, 0x6a, 0x78},
		{0x4a, 0x93, 0xa3},
		{0x8b, 0xe9, 0xfd},
		{0xb9, 0xf1, 0xfd},
	},
}

var cyberTheme = Theme{
	Name: "cyber", Display: "Cyber", Tagline: "Neon green · hot magenta",
	AppBg: lipgloss.Color("#0a0f0a"),
	Foreground: lipgloss.AdaptiveColor{Light: "#0a120a", Dark: "#e8ffe8"},
	Muted:      lipgloss.AdaptiveColor{Light: "#4a6a4a", Dark: "#7aaa7a"},
	Border:     lipgloss.AdaptiveColor{Light: "#80c080", Dark: "#1a2e1a"},
	Download:   lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#39ff14"},
	Upload:     lipgloss.AdaptiveColor{Light: "#c026d3", Dark: "#ff00aa"},
	Latency:    lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#00f0ff"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#65a30d", Dark: "#b8ff3c"},
	GraphDownBottom: lipgloss.Color("#0a4a12"), GraphDownTop: lipgloss.Color("#39ff14"),
	GraphUpBottom:   lipgloss.Color("#6a0040"), GraphUpTop:    lipgloss.Color("#ff00aa"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#e8ffe8", Dark: "#0e1a0e"},
	MenuIdleFill: lipgloss.Color("#0e160e"), MenuSelectDL: lipgloss.Color("#0a2410"),
	MenuSelectUL: lipgloss.Color("#1e0a18"), MenuSelectExit: lipgloss.Color("#101a10"),
	MenuSelectSet: lipgloss.Color("#0a1a1e"),
	AccentDL: lipgloss.Color("#39ff14"), AccentUL: lipgloss.Color("#ff00aa"),
	AccentLat: lipgloss.Color("#00f0ff"), AccentHL: lipgloss.Color("#b8ff3c"),

	LogoStops: [4][3]uint8{
		{0x0a, 0x4a, 0x12},
		{0x19, 0x85, 0x12},
		{0x39, 0xff, 0x14},
		{0x88, 0xff, 0x72},
	},
}

var emberTheme = Theme{
	Name: "ember", Display: "Ember", Tagline: "Charcoal fire · molten gold",
	AppBg: lipgloss.Color("#140c0a"),
	Foreground: lipgloss.AdaptiveColor{Light: "#241410", Dark: "#fef0e6"},
	Muted:      lipgloss.AdaptiveColor{Light: "#8a6050", Dark: "#b09080"},
	Border:     lipgloss.AdaptiveColor{Light: "#c09070", Dark: "#3a2420"},
	Download:   lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"},
	Upload:     lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#fbbf24"},
	Latency:    lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#fb923c"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#fcd34d"},
	GraphDownBottom: lipgloss.Color("#7f1d1d"), GraphDownTop: lipgloss.Color("#fca5a5"),
	GraphUpBottom:   lipgloss.Color("#78350f"), GraphUpTop:    lipgloss.Color("#fde68a"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#fff0e8", Dark: "#1e1410"},
	MenuIdleFill: lipgloss.Color("#1a1210"), MenuSelectDL: lipgloss.Color("#2e1414"),
	MenuSelectUL: lipgloss.Color("#2e2010"), MenuSelectExit: lipgloss.Color("#241810"),
	MenuSelectSet: lipgloss.Color("#2a1810"),
	AccentDL: lipgloss.Color("#f87171"), AccentUL: lipgloss.Color("#fbbf24"),
	AccentLat: lipgloss.Color("#fb923c"), AccentHL: lipgloss.Color("#fcd34d"),

	LogoStops: [4][3]uint8{
		{0x7f, 0x1d, 0x1d},
		{0xa8, 0x49, 0x49},
		{0xfc, 0xa5, 0xa5},
		{0xfd, 0xc9, 0xc9},
	},
}

var arcticTheme = Theme{
	Name: "arctic", Display: "Arctic", Tagline: "Ice blue · clean slate",
	AppBg: lipgloss.Color("#0e1418"),
	Foreground: lipgloss.AdaptiveColor{Light: "#1a2228", Dark: "#f0f7fa"},
	Muted:      lipgloss.AdaptiveColor{Light: "#5a7080", Dark: "#8aa0b0"},
	Border:     lipgloss.AdaptiveColor{Light: "#a0b8c8", Dark: "#2a3840"},
	Download:   lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#7dd3fc"},
	Upload:     lipgloss.AdaptiveColor{Light: "#475569", Dark: "#cbd5e1"},
	Latency:    lipgloss.AdaptiveColor{Light: "#0ea5e9", Dark: "#38bdf8"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#bae6fd"},
	GraphDownBottom: lipgloss.Color("#0c4a6e"), GraphDownTop: lipgloss.Color("#bae6fd"),
	GraphUpBottom:   lipgloss.Color("#334155"), GraphUpTop:    lipgloss.Color("#e2e8f0"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#f0f7fa", Dark: "#162028"},
	MenuIdleFill: lipgloss.Color("#141c22"), MenuSelectDL: lipgloss.Color("#1a2c38"),
	MenuSelectUL: lipgloss.Color("#1e2830"), MenuSelectExit: lipgloss.Color("#182428"),
	MenuSelectSet: lipgloss.Color("#1a2834"),
	AccentDL: lipgloss.Color("#7dd3fc"), AccentUL: lipgloss.Color("#cbd5e1"),
	AccentLat: lipgloss.Color("#38bdf8"), AccentHL: lipgloss.Color("#bae6fd"),

	LogoStops: [4][3]uint8{
		{0x0c, 0x4a, 0x6e},
		{0x45, 0x7d, 0x9d},
		{0xba, 0xe6, 0xfd},
		{0xd5, 0xf0, 0xfd},
	},
}

// --- Black-background themes -----------------------------------------------

var monoTheme = Theme{
	Name: "mono", Display: "Mono", Tagline: "True black · arctic white",
	AppBg: lipgloss.Color("#000000"),
	Foreground: lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#f0f0f0"},
	Muted:      lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"},
	Border:     lipgloss.AdaptiveColor{Light: "#888888", Dark: "#333333"},
	Download:   lipgloss.AdaptiveColor{Light: "#333333", Dark: "#e0e0e0"},
	Upload:     lipgloss.AdaptiveColor{Light: "#444444", Dark: "#cccccc"},
	Latency:    lipgloss.AdaptiveColor{Light: "#555555", Dark: "#bbbbbb"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#ffffff"},
	GraphDownBottom: lipgloss.Color("#1a1a1a"), GraphDownTop: lipgloss.Color("#e0e0e0"),
	GraphUpBottom:   lipgloss.Color("#222222"), GraphUpTop:    lipgloss.Color("#cccccc"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#f0f0f0", Dark: "#111111"},
	MenuIdleFill: lipgloss.Color("#0a0a0a"), MenuSelectDL: lipgloss.Color("#1a1a1a"),
	MenuSelectUL: lipgloss.Color("#181818"), MenuSelectExit: lipgloss.Color("#141414"),
	MenuSelectSet: lipgloss.Color("#1a1a1a"),
	AccentDL: lipgloss.Color("#e0e0e0"), AccentUL: lipgloss.Color("#cccccc"),
	AccentLat: lipgloss.Color("#bbbbbb"), AccentHL: lipgloss.Color("#ffffff"),
	LogoStops: [4][3]uint8{
		{0x1a, 0x1a, 0x1a},
		{0x88, 0x88, 0x88},
		{0xcc, 0xcc, 0xcc},
		{0xfa, 0xfa, 0xfa},
	},
}

var signalTheme = Theme{
	Name: "signal", Display: "Signal", Tagline: "True black · rose red",
	AppBg: lipgloss.Color("#000000"),
	Foreground: lipgloss.AdaptiveColor{Light: "#1a0a0a", Dark: "#fef0f0"},
	Muted:      lipgloss.AdaptiveColor{Light: "#665050", Dark: "#998888"},
	Border:     lipgloss.AdaptiveColor{Light: "#886060", Dark: "#331a1a"},
	Download:   lipgloss.AdaptiveColor{Light: "#cc3333", Dark: "#ff4444"},
	Upload:     lipgloss.AdaptiveColor{Light: "#aa2233", Dark: "#ff6677"},
	Latency:    lipgloss.AdaptiveColor{Light: "#cc4444", Dark: "#ff8888"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#991122", Dark: "#ff5566"},
	GraphDownBottom: lipgloss.Color("#330a0a"), GraphDownTop: lipgloss.Color("#ff4444"),
	GraphUpBottom:   lipgloss.Color("#2a0a10"), GraphUpTop:    lipgloss.Color("#ff6677"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#fef0f0", Dark: "#110808"},
	MenuIdleFill: lipgloss.Color("#0a0000"), MenuSelectDL: lipgloss.Color("#1a0808"),
	MenuSelectUL: lipgloss.Color("#180808"), MenuSelectExit: lipgloss.Color("#140808"),
	MenuSelectSet: lipgloss.Color("#1a080a"),
	AccentDL: lipgloss.Color("#ff4444"), AccentUL: lipgloss.Color("#ff6677"),
	AccentLat: lipgloss.Color("#ff8888"), AccentHL: lipgloss.Color("#ff5566"),
	LogoStops: [4][3]uint8{
		{0x33, 0x0a, 0x0a},
		{0x99, 0x22, 0x33},
		{0xff, 0x44, 0x44},
		{0xff, 0xaa, 0xaa},
	},
}

var inkTheme = Theme{
	Name: "ink", Display: "Ink", Tagline: "True black · cold blue",
	AppBg: lipgloss.Color("#000000"),
	Foreground: lipgloss.AdaptiveColor{Light: "#0a0a1a", Dark: "#f0f0ff"},
	Muted:      lipgloss.AdaptiveColor{Light: "#505066", Dark: "#8888aa"},
	Border:     lipgloss.AdaptiveColor{Light: "#606088", Dark: "#1a1a33"},
	Download:   lipgloss.AdaptiveColor{Light: "#3355cc", Dark: "#4488ff"},
	Upload:     lipgloss.AdaptiveColor{Light: "#2244aa", Dark: "#6699ff"},
	Latency:    lipgloss.AdaptiveColor{Light: "#4466cc", Dark: "#88aaff"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#112299", Dark: "#5599ff"},
	GraphDownBottom: lipgloss.Color("#0a0a33"), GraphDownTop: lipgloss.Color("#4488ff"),
	GraphUpBottom:   lipgloss.Color("#0a102a"), GraphUpTop:    lipgloss.Color("#6699ff"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#f0f0ff", Dark: "#080811"},
	MenuIdleFill: lipgloss.Color("#00000a"), MenuSelectDL: lipgloss.Color("#08081a"),
	MenuSelectUL: lipgloss.Color("#080818"), MenuSelectExit: lipgloss.Color("#080814"),
	MenuSelectSet: lipgloss.Color("#0a0a1a"),
	AccentDL: lipgloss.Color("#4488ff"), AccentUL: lipgloss.Color("#6699ff"),
	AccentLat: lipgloss.Color("#88aaff"), AccentHL: lipgloss.Color("#5599ff"),
	LogoStops: [4][3]uint8{
		{0x0a, 0x0a, 0x33},
		{0x22, 0x44, 0xaa},
		{0x44, 0x88, 0xff},
		{0xaa, 0xcc, 0xff},
	},
}
