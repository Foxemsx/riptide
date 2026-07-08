package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

// menuModel is the startup screen. It shows a row of selectable boxes (Speed
// Test / Bandwidth Monitor / Exit) that can be navigated with the keyboard or
// the mouse, and emits a menuSelectMsg when the user picks one.
type menuModel struct {
	theme  Theme
	width  int
	height int
	cursor int
	spinner spinner.Model
	items  []menuItem
}

func newMenuModel(theme Theme) *menuModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Highlight)
	return &menuModel{
		theme:  theme,
		cursor: 0,
		spinner: s,
		items: []menuItem{
			{title: "Speed Test", subtitle: "run a one-shot test", icon: "⚡", screen: screenTest},
			{title: "Bandwidth", subtitle: "live monitor, DL + UL", icon: "📊", screen: screenMonitor},
			{title: "Exit", subtitle: "quit speed", icon: "⏻", screen: screenExit},
		},
	}
}

// Init spins the spinner so the header accent animates.
func (m *menuModel) Init() tea.Cmd { return m.spinner.Tick }

func (m *menuModel) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return tea.Quit, false
		case "left", "h", "k":
			m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
			return nil, false
		case "right", "l", "j":
			m.cursor = (m.cursor + 1) % len(m.items)
			return nil, false
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
	case tea.MouseMsg:
		if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
			return nil, false
		}
		// Hit-test the rendered boxes; if the click lands inside one, move
		// there and select it.
		for i, box := range m.boxRects() {
			if msg.X >= box.x && msg.X < box.x+box.w &&
				msg.Y >= box.y && msg.Y < box.y+box.h {
				m.cursor = i
				return m.selectCurrent(), false
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

// boxRects computes the on-screen rectangle of each menu box, mirroring the
// layout in View(). It is used for click detection.
func (m *menuModel) boxRects() []boxRect {
	w := m.width
	if w <= 0 {
		w = 80
	}
	// Match View() layout: a centered stack of header + spacer + boxes row.
	boxW := m.boxWidth(w)
	gap := 2
	totalW := len(m.items)*boxW + (len(m.items)-1)*gap

	stackH := m.stackHeight(boxW) // header + spacer + boxes
	startY := (m.height - stackH) / 2
	if startY < 0 {
		startY = 0
	}
	// Boxes start after header + 1 spacer line.
	boxesY := startY + m.headerHeight() + 1

	startX := (w - totalW) / 2
	if startX < 0 {
		startX = 0
	}
	rects := make([]boxRect, len(m.items))
	for i := range m.items {
		rects[i] = boxRect{
			x: startX + i*(boxW+gap),
			y: boxesY,
			w: boxW,
			h: m.boxHeight(),
		}
	}
	return rects
}

func (m *menuModel) boxWidth(termW int) int {
	// Three boxes + 2 gaps, with comfortable margins.
	maxEach := 24
	each := (termW - 4 - 2*2) / 3
	if each > maxEach {
		each = maxEach
	}
	if each < 16 {
		each = 16
	}
	return each
}

func (m *menuModel) boxHeight() int { return 5 }

func (m *menuModel) headerHeight() int { return 9 } // SPEED logo (7) + tagline (1) + margin

func (m *menuModel) stackHeight(boxW int) int {
	return m.headerHeight() + 1 + m.boxHeight()
}

func (m *menuModel) View() string {
	// Build each box.
	boxes := make([]string, len(m.items))
	for i, it := range m.items {
		boxes[i] = m.renderBox(i, it)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, boxes...)

	hint := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("←/→ or j/k move  ·  enter select  ·  click a box  ·  q quit")

	header := renderHeader("Choose how you'd like to measure your connection")
	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"",
		row,
		"",
		hint,
	)

	ch := m.height
	if ch <= 0 {
		ch = 30
	}
	placed := lipgloss.Place(m.width, ch, lipgloss.Center, lipgloss.Center, stack)

	return placed
}

// renderBox renders a single menu option: a rounded card with an icon, title,
// and subtitle. The selected box gets a bright border, a subtle filled accent
// background, and a leading marker.
func (m *menuModel) renderBox(i int, it menuItem) string {
	selected := i == m.cursor
	w := m.boxWidth(m.width)

	accent := m.theme.Download // default accent
	if it.screen == screenMonitor {
		accent = m.theme.Upload
	} else if it.screen == screenExit {
		accent = m.theme.Highlight
	}

	icon := lipgloss.NewStyle().Foreground(accent).Render(it.icon)
	title := lipgloss.NewStyle().Foreground(m.theme.Foreground).Bold(true).Render(it.title)
	sub := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(it.subtitle)

	var marker string
	if selected {
		marker = lipgloss.NewStyle().Foreground(accent).Bold(true).Render("▶ ")
	} else {
		marker = "  "
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		marker+icon+"  "+title,
		"",
		"  "+sub,
	)

	border := m.theme.Border
	if selected {
		border = accent
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(1, 2).
		Width(w).
		Align(lipgloss.Left)
	box := style.Render(content)

	// A faint glow line under the selected box.
	if selected {
		glow := lipgloss.NewStyle().Foreground(accent).Render(strings.Repeat("━", w))
		box = lipgloss.JoinVertical(lipgloss.Center, box, glow)
	}
	return box
}
