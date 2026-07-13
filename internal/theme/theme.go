package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
	gruvboxTheme,
	tokyoTheme,
	catppuccinTheme,
	solarizedTheme,
	rosePineTheme,
	monokaiTheme,
	oneDarkTheme,
	githubTheme,
	everforestTheme,
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
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		content,
		lipgloss.WithWhitespaceBackground(t.AppBg),
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
}

// --- Community-favorite palettes ------------------------------------------

var gruvboxTheme = Theme{
	Name: "gruvbox", Display: "Gruvbox", Tagline: "Warm retro · earth tones",
	AppBg: lipgloss.Color("#282828"),
	Foreground: lipgloss.AdaptiveColor{Light: "#282828", Dark: "#ebdbb2"},
	Muted:      lipgloss.AdaptiveColor{Light: "#928374", Dark: "#a89984"},
	Border:     lipgloss.AdaptiveColor{Light: "#d5c4a1", Dark: "#504945"},
	Download:   lipgloss.AdaptiveColor{Light: "#427b58", Dark: "#83a598"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b57614", Dark: "#fabd2f"},
	Latency:    lipgloss.AdaptiveColor{Light: "#8f3f71", Dark: "#d3869b"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#79740e", Dark: "#b8bb26"},
	GraphDownBottom: lipgloss.Color("#427b58"), GraphDownTop: lipgloss.Color("#8ec07c"),
	GraphUpBottom:   lipgloss.Color("#b57614"), GraphUpTop:    lipgloss.Color("#fabd2f"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#ebdbb2", Dark: "#32302f"},
	MenuIdleFill: lipgloss.Color("#32302f"), MenuSelectDL: lipgloss.Color("#3a4a40"),
	MenuSelectUL: lipgloss.Color("#4a3c20"), MenuSelectExit: lipgloss.Color("#3a4030"),
	MenuSelectSet: lipgloss.Color("#3a3448"),
	AccentDL: lipgloss.Color("#83a598"), AccentUL: lipgloss.Color("#fabd2f"),
	AccentLat: lipgloss.Color("#d3869b"), AccentHL: lipgloss.Color("#b8bb26"),
}

var tokyoTheme = Theme{
	Name: "tokyo", Display: "Tokyo Night", Tagline: "Neon anime dusk",
	AppBg: lipgloss.Color("#1a1b26"),
	Foreground: lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#a9b1d6"},
	Muted:      lipgloss.AdaptiveColor{Light: "#565f89", Dark: "#565f89"},
	Border:     lipgloss.AdaptiveColor{Light: "#9aa5ce", Dark: "#414868"},
	Download:   lipgloss.AdaptiveColor{Light: "#3b4261", Dark: "#7aa2f7"},
	Upload:     lipgloss.AdaptiveColor{Light: "#7e2a4d", Dark: "#f7768e"},
	Latency:    lipgloss.AdaptiveColor{Light: "#7a5bc7", Dark: "#bb9af7"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#557a45", Dark: "#9ece6a"},
	GraphDownBottom: lipgloss.Color("#3b4261"), GraphDownTop: lipgloss.Color("#7dcfff"),
	GraphUpBottom:   lipgloss.Color("#7e2a4d"), GraphUpTop:    lipgloss.Color("#f7768e"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#a9b1d6", Dark: "#1f2330"},
	MenuIdleFill: lipgloss.Color("#1f2330"), MenuSelectDL: lipgloss.Color("#26304a"),
	MenuSelectUL: lipgloss.Color("#3a2436"), MenuSelectExit: lipgloss.Color("#26304a"),
	MenuSelectSet: lipgloss.Color("#2c2a44"),
	AccentDL: lipgloss.Color("#7aa2f7"), AccentUL: lipgloss.Color("#f7768e"),
	AccentLat: lipgloss.Color("#bb9af7"), AccentHL: lipgloss.Color("#9ece6a"),
}

var catppuccinTheme = Theme{
	Name: "catppuccin", Display: "Catppuccin", Tagline: "Soft pastel mocha",
	AppBg: lipgloss.Color("#1e1e2e"),
	Foreground: lipgloss.AdaptiveColor{Light: "#1e1e2e", Dark: "#cdd6f4"},
	Muted:      lipgloss.AdaptiveColor{Light: "#6c7086", Dark: "#6c7086"},
	Border:     lipgloss.AdaptiveColor{Light: "#cdd6f4", Dark: "#45475a"},
	Download:   lipgloss.AdaptiveColor{Light: "#4584b6", Dark: "#89b4fa"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b45c6e", Dark: "#f38ba8"},
	Latency:    lipgloss.AdaptiveColor{Light: "#8f73b8", Dark: "#cba6f7"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#5fa06a", Dark: "#a6e3a1"},
	GraphDownBottom: lipgloss.Color("#3b4252"), GraphDownTop: lipgloss.Color("#89dceb"),
	GraphUpBottom:   lipgloss.Color("#563a4a"), GraphUpTop:    lipgloss.Color("#f38ba8"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#cdd6f4", Dark: "#242437"},
	MenuIdleFill: lipgloss.Color("#242437"), MenuSelectDL: lipgloss.Color("#26304a"),
	MenuSelectUL: lipgloss.Color("#3a2436"), MenuSelectExit: lipgloss.Color("#26304a"),
	MenuSelectSet: lipgloss.Color("#2c2a44"),
	AccentDL: lipgloss.Color("#89b4fa"), AccentUL: lipgloss.Color("#f38ba8"),
	AccentLat: lipgloss.Color("#cba6f7"), AccentHL: lipgloss.Color("#a6e3a1"),
}

var solarizedTheme = Theme{
	Name: "solarized", Display: "Solarized", Tagline: "Classic dark precision",
	AppBg: lipgloss.Color("#002b36"),
	Foreground: lipgloss.AdaptiveColor{Light: "#002b36", Dark: "#93a1a1"},
	Muted:      lipgloss.AdaptiveColor{Light: "#586e75", Dark: "#586e75"},
	Border:     lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#073642"},
	Download:   lipgloss.AdaptiveColor{Light: "#2aa198", Dark: "#2aa198"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b58900", Dark: "#b58900"},
	Latency:    lipgloss.AdaptiveColor{Light: "#6c71c4", Dark: "#6c71c4"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#859900", Dark: "#859900"},
	GraphDownBottom: lipgloss.Color("#0b4d52"), GraphDownTop: lipgloss.Color("#2aa198"),
	GraphUpBottom:   lipgloss.Color("#5a4a00"), GraphUpTop:    lipgloss.Color("#b58900"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#93a1a1", Dark: "#073642"},
	MenuIdleFill: lipgloss.Color("#073642"), MenuSelectDL: lipgloss.Color("#0b3a40"),
	MenuSelectUL: lipgloss.Color("#3a3200"), MenuSelectExit: lipgloss.Color("#0b3a40"),
	MenuSelectSet: lipgloss.Color("#2a2440"),
	AccentDL: lipgloss.Color("#2aa198"), AccentUL: lipgloss.Color("#b58900"),
	AccentLat: lipgloss.Color("#6c71c4"), AccentHL: lipgloss.Color("#859900"),
}

var rosePineTheme = Theme{
	Name: "rosepine", Display: "Rosé Pine", Tagline: "Dusty dawn calm",
	AppBg: lipgloss.Color("#191724"),
	Foreground: lipgloss.AdaptiveColor{Light: "#191724", Dark: "#e0def4"},
	Muted:      lipgloss.AdaptiveColor{Light: "#6e6a86", Dark: "#6e6a86"},
	Border:     lipgloss.AdaptiveColor{Light: "#e0def4", Dark: "#403d52"},
	Download:   lipgloss.AdaptiveColor{Light: "#31748f", Dark: "#9ccfd8"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b58a3f", Dark: "#f6c177"},
	Latency:    lipgloss.AdaptiveColor{Light: "#8f6bbd", Dark: "#c4a7e7"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#31748f", Dark: "#31748f"},
	GraphDownBottom: lipgloss.Color("#20454f"), GraphDownTop: lipgloss.Color("#9ccfd8"),
	GraphUpBottom:   lipgloss.Color("#5a4a25"), GraphUpTop:    lipgloss.Color("#f6c177"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#e0def4", Dark: "#1f1d2e"},
	MenuIdleFill: lipgloss.Color("#1f1d2e"), MenuSelectDL: lipgloss.Color("#26394a"),
	MenuSelectUL: lipgloss.Color("#3a2e20"), MenuSelectExit: lipgloss.Color("#26394a"),
	MenuSelectSet: lipgloss.Color("#2e2942"),
	AccentDL: lipgloss.Color("#9ccfd8"), AccentUL: lipgloss.Color("#f6c177"),
	AccentLat: lipgloss.Color("#c4a7e7"), AccentHL: lipgloss.Color("#31748f"),
}

var monokaiTheme = Theme{
	Name: "monokai", Display: "Monokai", Tagline: "Vivid code-editor heat",
	AppBg: lipgloss.Color("#272822"),
	Foreground: lipgloss.AdaptiveColor{Light: "#272822", Dark: "#f8f8f2"},
	Muted:      lipgloss.AdaptiveColor{Light: "#75715e", Dark: "#75715e"},
	Border:     lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#49483e"},
	Download:   lipgloss.AdaptiveColor{Light: "#2a9da8", Dark: "#66d9ef"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b4246a", Dark: "#f92672"},
	Latency:    lipgloss.AdaptiveColor{Light: "#8f6bd6", Dark: "#ae81ff"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#7db52a", Dark: "#a6e22e"},
	GraphDownBottom: lipgloss.Color("#2a9da8"), GraphDownTop: lipgloss.Color("#66d9ef"),
	GraphUpBottom:   lipgloss.Color("#8f2150"), GraphUpTop:    lipgloss.Color("#f92672"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#f8f8f2", Dark: "#2d2e26"},
	MenuIdleFill: lipgloss.Color("#2d2e26"), MenuSelectDL: lipgloss.Color("#264e58"),
	MenuSelectUL: lipgloss.Color("#4a2036"), MenuSelectExit: lipgloss.Color("#264e3a"),
	MenuSelectSet: lipgloss.Color("#3a2a52"),
	AccentDL: lipgloss.Color("#66d9ef"), AccentUL: lipgloss.Color("#f92672"),
	AccentLat: lipgloss.Color("#ae81ff"), AccentHL: lipgloss.Color("#a6e22e"),
}

var oneDarkTheme = Theme{
	Name: "onedark", Display: "One Dark", Tagline: "Editor default comfort",
	AppBg: lipgloss.Color("#282c34"),
	Foreground: lipgloss.AdaptiveColor{Light: "#282c34", Dark: "#abb2bf"},
	Muted:      lipgloss.AdaptiveColor{Light: "#5c6370", Dark: "#5c6370"},
	Border:     lipgloss.AdaptiveColor{Light: "#abb2bf", Dark: "#3e4451"},
	Download:   lipgloss.AdaptiveColor{Light: "#3f7fbf", Dark: "#61afef"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b58a3f", Dark: "#e5c07b"},
	Latency:    lipgloss.AdaptiveColor{Light: "#9f5bc7", Dark: "#c678dd"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#5f9b4a", Dark: "#98c379"},
	GraphDownBottom: lipgloss.Color("#2f5d7a"), GraphDownTop: lipgloss.Color("#61afef"),
	GraphUpBottom:   lipgloss.Color("#6b5a2e"), GraphUpTop:    lipgloss.Color("#e5c07b"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#abb2bf", Dark: "#2c313a"},
	MenuIdleFill: lipgloss.Color("#2c313a"), MenuSelectDL: lipgloss.Color("#2f4a63"),
	MenuSelectUL: lipgloss.Color("#3a3326"), MenuSelectExit: lipgloss.Color("#2f4a3a"),
	MenuSelectSet: lipgloss.Color("#342f4a"),
	AccentDL: lipgloss.Color("#61afef"), AccentUL: lipgloss.Color("#e5c07b"),
	AccentLat: lipgloss.Color("#c678dd"), AccentHL: lipgloss.Color("#98c379"),
}

var githubTheme = Theme{
	Name: "github", Display: "GitHub Dark", Tagline: "Exactly like the site",
	AppBg: lipgloss.Color("#0d1117"),
	Foreground: lipgloss.AdaptiveColor{Light: "#0d1117", Dark: "#c9d1d9"},
	Muted:      lipgloss.AdaptiveColor{Light: "#8b949e", Dark: "#8b949e"},
	Border:     lipgloss.AdaptiveColor{Light: "#c9d1d9", Dark: "#30363d"},
	Download:   lipgloss.AdaptiveColor{Light: "#388bf2", Dark: "#58a6ff"},
	Upload:     lipgloss.AdaptiveColor{Light: "#c9762f", Dark: "#f0883e"},
	Latency:    lipgloss.AdaptiveColor{Light: "#9f73e6", Dark: "#bc8cff"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#2f9b4a", Dark: "#3fb950"},
	GraphDownBottom: lipgloss.Color("#1f3f5f"), GraphDownTop: lipgloss.Color("#58a6ff"),
	GraphUpBottom:   lipgloss.Color("#6b4421"), GraphUpTop:    lipgloss.Color("#f0883e"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#c9d1d9", Dark: "#161b22"},
	MenuIdleFill: lipgloss.Color("#161b22"), MenuSelectDL: lipgloss.Color("#1c3146"),
	MenuSelectUL: lipgloss.Color("#3a2e1c"), MenuSelectExit: lipgloss.Color("#1c3146"),
	MenuSelectSet: lipgloss.Color("#2a2440"),
	AccentDL: lipgloss.Color("#58a6ff"), AccentUL: lipgloss.Color("#f0883e"),
	AccentLat: lipgloss.Color("#bc8cff"), AccentHL: lipgloss.Color("#3fb950"),
}

var everforestTheme = Theme{
	Name: "everforest", Display: "Everforest", Tagline: "Calm muted woodland",
	AppBg: lipgloss.Color("#2d353b"),
	Foreground: lipgloss.AdaptiveColor{Light: "#2d353b", Dark: "#d3c6aa"},
	Muted:      lipgloss.AdaptiveColor{Light: "#7a8478", Dark: "#7a8478"},
	Border:     lipgloss.AdaptiveColor{Light: "#d3c6aa", Dark: "#475258"},
	Download:   lipgloss.AdaptiveColor{Light: "#4f7aa3", Dark: "#84a0c6"},
	Upload:     lipgloss.AdaptiveColor{Light: "#b87a57", Dark: "#e69875"},
	Latency:    lipgloss.AdaptiveColor{Light: "#a5719a", Dark: "#d699b6"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#7c9a5a", Dark: "#a7c080"},
	GraphDownBottom: lipgloss.Color("#41566b"), GraphDownTop: lipgloss.Color("#84a0c6"),
	GraphUpBottom:   lipgloss.Color("#6b4733"), GraphUpTop:    lipgloss.Color("#e69875"),
	MenuAccentFill:  lipgloss.AdaptiveColor{Light: "#d3c6aa", Dark: "#333c43"},
	MenuIdleFill: lipgloss.Color("#333c43"), MenuSelectDL: lipgloss.Color("#3a4d5e"),
	MenuSelectUL: lipgloss.Color("#4a3a2c"), MenuSelectExit: lipgloss.Color("#3a4d40"),
	MenuSelectSet: lipgloss.Color("#3a344a"),
	AccentDL: lipgloss.Color("#84a0c6"), AccentUL: lipgloss.Color("#e69875"),
	AccentLat: lipgloss.Color("#d699b6"), AccentHL: lipgloss.Color("#a7c080"),
}
