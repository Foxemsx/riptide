package main

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Layout thresholds for a "crazy good" responsive menu.
const (
	horizontalThreshold  = 92  // below this → vertical stack of cards
	previewThreshold     = 118 // wide enough for side preview pane
	fullCardMinHeight    = 30  // below this use "tight" (fewer lines)
	previewSideMinHeight = 12  // on wide terms, side preview only if height >= this
	menuTickInterval     = 100 * time.Millisecond
)

// screenID identifies which destination the menu routes to.
type screenID int

const (
	screenMenu screenID = iota
	screenTest
	screenMonitor
	screenExit
)

// menuItem is one selectable box in the startup menu.
type menuItem struct {
	title    string
	subtitle string
	icon     string
	screen   screenID
	hotkey   string   // "1", "2", "3" etc.
	features []string // short bullets shown in rich cards
	badge    string   // e.g. "one-shot", "● LIVE"
}

// menuModel is the startup screen. It shows a row of selectable boxes (Speed
// Test / Bandwidth Monitor / Exit) that can be navigated with the keyboard or
// the mouse, and emits a menuSelectMsg when the user picks one.
type menuModel struct {
	theme   Theme
	compact bool
	width   int
	height  int
	cursor  int
	hovered int     // transient mouse hover (for highlight, not selection)
	pulse   float64 // 0..1 animation phase for selected glow
	spinner spinner.Model
	items   []menuItem
}

func newMenuModel(theme Theme, compact bool) *menuModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Highlight)
	return &menuModel{
		theme:   theme,
		compact: compact,
		cursor:  0,
		hovered: -1,
		spinner: s,
		items: []menuItem{
			{title: "Speed Test", subtitle: "one-shot DL · UL · ping", icon: "", screen: screenTest, hotkey: "1",
				features: []string{"Download + upload + latency", "~10s timed phases", "Parallel connections"},
				badge: "ONE-SHOT"},
			{title: "Bandwidth", subtitle: "live monitor · no test traffic", icon: "", screen: screenMonitor, hotkey: "2",
				features: []string{"Real PC throughput", "Session peaks", "Zero generated traffic"},
				badge: "LIVE"},
			{title: "Exit", subtitle: "quit riptide cleanly", icon: "", screen: screenExit, hotkey: "3",
				features: []string{"Cancel any running test", "Clean shutdown"},
				badge: ""},
		},
	}
}

// Init spins the spinner and starts the menu pulse tick for animated glow.
func (m *menuModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tickCmd())
}

func (m *menuModel) tickCmd() tea.Cmd {
	return tea.Tick(menuTickInterval, func(time.Time) tea.Msg { return menuTickMsg{} })
}

// menuTickMsg drives subtle selection pulse animation.
type menuTickMsg struct{}

func (m *menuModel) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return tea.Quit, false
		case "left", "h", "k":
			m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
			m.hovered = -1
			return nil, false
		case "right", "l", "j":
			m.cursor = (m.cursor + 1) % len(m.items)
			m.hovered = -1
			return nil, false
		case "1", "2", "3":
			// Direct hotkeys (PR1+)
			for i, it := range m.items {
				if it.hotkey == msg.String() {
					m.cursor = i
					return m.selectCurrent(), false
				}
			}
		case "enter", " ":
			return m.selectCurrent(), false
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return nil, false
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd, false
	case menuTickMsg:
		// Advance subtle pulse for selected glow (0..1)
		m.pulse = m.pulse + 0.08
		if m.pulse > 1 {
			m.pulse = 0
		}
		return m.tickCmd(), false
	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionMotion:
			// Hover highlight (does not change cursor/selection)
			hit := -1
			for i, box := range m.boxRects() {
				if msg.X >= box.x && msg.X < box.x+box.w &&
					msg.Y >= box.y && msg.Y < box.y+box.h {
					hit = i
					break
				}
			}
			if hit != m.hovered {
				m.hovered = hit
				return nil, false
			}
		case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
			for i, box := range m.boxRects() {
				if msg.X >= box.x && msg.X < box.x+box.w &&
					msg.Y >= box.y && msg.Y < box.y+box.h {
					m.cursor = i
					m.hovered = -1
					return m.selectCurrent(), false
				}
			}
		}
	}
	return nil, false
}

// selectCurrent emits the right command for the highlighted item.
func (m *menuModel) selectCurrent() tea.Cmd {
	switch m.items[m.cursor].screen {
	case screenTest:
		return menuSelectCmd(screenTest)
	case screenMonitor:
		return menuSelectCmd(screenMonitor)
	default: // Exit
		return tea.Quit
	}
}

// boxRect is a screen rectangle for mouse hit-testing.
type boxRect struct{ x, y, w, h int }

func (m *menuModel) isTight() bool {
	return m.height > 0 && m.height < fullCardMinHeight
}

func (m *menuModel) headerHeight() int {
	if m.compact {
		return 4
	}
	// ANSI Shadow logo: 6 rows + tagline + gaps.
	return 10
}

// computeLayout is the single source of truth. Used by View + boxRects.
func (m *menuModel) computeLayout() (mode string, boxW, boxH, startY, startX int, gap int) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	gap = 2
	boxW = m.boxWidth(w)
	boxH = 10 // fixed for consistent rich cards (8 inner lines + padding + room for glow)

	mode = "horizontal"
	if w < horizontalThreshold {
		mode = "vertical"
		boxW = min(w-6, 48)
	}

	num := len(m.items)
	totalW := num * boxW
	if mode != "vertical" {
		totalW += (num - 1) * gap
	}

	stackH := m.headerHeight() + 1
	if mode == "vertical" {
		stackH += num*boxH + (num-1)
	} else {
		stackH += boxH
	}

	startY = (h - stackH) / 2
	if startY < 0 {
		startY = 0
	}
	startX = (w - totalW) / 2
	if startX < 0 {
		startX = 0
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// boxRects computes the on-screen rectangle of each menu box, mirroring the
// layout in View(). It is used for click detection.
func (m *menuModel) boxRects() []boxRect {
	mode, boxW, boxH, startY, startX, gap := m.computeLayout()

	rects := make([]boxRect, len(m.items))
	boxesY := startY + m.headerHeight() + 1

	if mode == "vertical" {
		for i := range m.items {
			rects[i] = boxRect{
				x: startX,
				y: boxesY + i*(boxH+1),
				w: boxW,
				h: boxH,
			}
		}
		return rects
	}

	// Horizontal (with or without preview)
	for i := range m.items {
		rects[i] = boxRect{
			x: startX + i*(boxW+gap),
			y: boxesY,
			w: boxW,
			h: boxH,
		}
	}
	return rects
}

func (m *menuModel) boxWidth(termW int) int {
	// Three boxes + 2 gaps, with comfortable margins.
	// 28 leaves enough inner width for feature lines (e.g. "• ~10s fixed duration")
	// so lipgloss does not wrap mid-card and punch holes in the selection fill.
	maxEach := 28
	each := (termW - 4 - 2*2) / 3
	if each > maxEach {
		each = maxEach
	}
	if each < 18 {
		each = 18
	}
	return each
}

func (m *menuModel) View() string {
	mode, boxW, _, _, _, gap := m.computeLayout()

	// Build cards with spacing so they read as separate buttons.
	boxes := make([]string, len(m.items))
	for i, it := range m.items {
		box := m.renderBox(i, it, boxW)
		if mode == "vertical" {
			if i < len(m.items)-1 {
				box = lipgloss.NewStyle().MarginBottom(1).Render(box)
			}
		} else if i < len(m.items)-1 {
			box = lipgloss.NewStyle().MarginRight(gap).Render(box)
		}
		boxes[i] = box
	}

	var cards string
	if mode == "vertical" {
		cards = lipgloss.JoinVertical(lipgloss.Left, boxes...)
	} else {
		cards = lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
	}

	// Footer
	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("←/→"), mt.Render(" or "),
		hl.Render("j/k"), mt.Render(" move  ·  "),
		hl.Render("1/2/3"), mt.Render(" pick  ·  "),
		hl.Render("enter"), mt.Render(" select  ·  "),
		hl.Render("q"), mt.Render(" quit  ·  "),
		hl.Render("t"), mt.Render(" compact"),
	)

	var header string
	if m.compact {
		header = renderCompactHeader("Choose how you'd like to measure your connection")
	} else {
		header = renderHeader("Choose how you'd like to measure your connection")
	}

	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"",
		cards,
		"",
		hint,
	)

	ch := m.height
	if ch <= 0 {
		ch = 30
	}
	return paintScreen(m.theme, m.width, ch, stack)
}

// renderBox draws one modern menu button. Selected buttons get a solid
// accent-tinted fill on every cell (including padding), a bright border, and a
// full-width underline. Unselected buttons stay quiet slate panels.
func (m *menuModel) renderBox(i int, it menuItem, cardWidth int) string {
	selected := i == m.cursor || (m.hovered >= 0 && i == m.hovered)

	accent := m.theme.Download
	fill := m.theme.MenuSelectDL
	if it.screen == screenMonitor {
		accent = m.theme.Upload
		fill = m.theme.MenuSelectUL
	} else if it.screen == screenExit {
		accent = m.theme.Highlight
		fill = m.theme.MenuSelectExit
	}

	// Surfaces: selected = solid tinted glass; idle = quiet panel.
	var bg lipgloss.TerminalColor
	if selected {
		bg = fill
	} else {
		bg = m.theme.MenuIdleFill
	}

	// Padding is inside the border; content width excludes L/R pad (2+2).
	innerW := cardWidth - 4
	if innerW < 12 {
		innerW = 12
	}

	// Every text style must carry bg so SGR resets never punch holes.
	cell := func(fg lipgloss.TerminalColor, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(fg).Background(bg)
		if bold {
			s = s.Bold(true)
		}
		return s
	}
	space := lipgloss.NewStyle().Background(bg)
	// Full-width line: join runs then pad with bg so the fill is continuous.
	line := func(parts ...string) string {
		joined := strings.Join(parts, "")
		return lipgloss.NewStyle().
			Width(innerW).
			Background(bg).
			Inline(true).
			Render(joined)
	}

	// Title row: hotkey chip + title in a colored block (no emoji).
	ink := lipgloss.Color("#0a0e14")
	var chip, titleBlock string
	if selected {
		// Solid accent pills on the glass fill.
		if it.hotkey != "" {
			chip = lipgloss.NewStyle().
				Foreground(ink).
				Background(accent).
				Bold(true).
				Padding(0, 1).
				Render(it.hotkey)
		}
		titleBlock = lipgloss.NewStyle().
			Foreground(ink).
			Background(accent).
			Bold(true).
			Padding(0, 1).
			Render(it.title)
	} else {
		// Soft accent-on-panel chips when idle.
		if it.hotkey != "" {
			chip = lipgloss.NewStyle().
				Foreground(accent).
				Background(bg).
				Bold(true).
				Padding(0, 1).
				Render(it.hotkey)
		}
		titleBlock = lipgloss.NewStyle().
			Foreground(accent).
			Background(bg).
			Bold(true).
			Padding(0, 1).
			Render(it.title)
	}
	titleRow := line(chip, space.Render(" "), titleBlock)

	// Subtitle
	subFG := m.theme.Muted
	if selected {
		subFG = m.theme.Foreground
	}
	subRow := line(space.Render("  "), cell(subFG, false).Render(it.subtitle))

	// Divider
	divCh := "─"
	if selected {
		divCh = "━"
	}
	div := line(cell(accent, false).Render(strings.Repeat(divCh, min(innerW, 18))))

	// Features (always 3 slots for equal height)
	featRows := make([]string, 3)
	for j := 0; j < 3; j++ {
		if j < len(it.features) {
			bullet := cell(accent, false).Render("› ")
			if !selected {
				bullet = cell(m.theme.Border, false).Render("· ")
			}
			featRows[j] = line(
				space.Render(" "),
				bullet,
				cell(m.theme.Muted, false).Render(it.features[j]),
			)
		} else {
			featRows[j] = line("")
		}
	}

	// Badge / footer row
	var badgeRow string
	if it.badge != "" {
		var badge string
		if selected {
			badge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0a0e14")).
				Background(accent).
				Bold(true).
				Padding(0, 1).
				Render(it.badge)
		} else {
			badge = lipgloss.NewStyle().
				Foreground(accent).
				Background(bg).
				Bold(true).
				Render(" " + it.badge + " ")
		}
		badgeRow = line(space.Render(" "), badge)
	} else if selected {
		badgeRow = line(space.Render(" "), cell(accent, true).Render("↵ enter"))
	} else {
		badgeRow = line("")
	}

	body := strings.Join([]string{
		titleRow,
		subRow,
		line(""), // spacer
		div,
		line(""), // spacer
		featRows[0],
		featRows[1],
		featRows[2],
		line(""), // spacer
		badgeRow,
	}, "\n")

	borderCol := m.theme.Border
	if selected {
		borderCol = accent
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderCol).
		Background(bg).
		Padding(1, 2).
		Width(cardWidth).
		Align(lipgloss.Left)

	box := cardStyle.Render(body)

	// Footer bar: full-width accent underline when selected; blank spacer when
	// not so all three buttons stay the same height.
	if selected {
		p := pulseFactor(m.pulse)
		// Soft pulse on brightness via length, still full-ish width.
		gw := int(float64(cardWidth) * (0.72 + 0.28*p))
		if gw < cardWidth/2 {
			gw = cardWidth / 2
		}
		if gw > cardWidth {
			gw = cardWidth
		}
		bar := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(strings.Repeat("▀", gw))
		// Center the bar under the card.
		pad := (cardWidth - gw) / 2
		if pad < 0 {
			pad = 0
		}
		under := strings.Repeat(" ", pad) + bar
		box = lipgloss.JoinVertical(lipgloss.Left, box, under)
	} else {
		box = lipgloss.JoinVertical(lipgloss.Left, box, strings.Repeat(" ", cardWidth))
	}
	return box
}

// pulseFactor returns a nice 0.6–1.0 multiplier for glow length from the 0..1 pulse state.
func pulseFactor(p float64) float64 {
	// Simple smooth-ish saw using fractional part
	frac := p - float64(int(p))
	// triangle wave between ~0.6 and 1.0
	if frac < 0.5 {
		return 0.6 + frac*0.8
	}
	return 1.0 - (frac-0.5)*0.8
}
