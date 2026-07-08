package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// model is the bubbletea sub-model for the one-shot Speed Test card. It embeds
// *cardState for all shared rendering/graph/animation state and adds only the
// test-specific fields (final result + the phase watchdog). It does NOT
// implement tea.Model directly; the app router (app.go) owns Init/Update/View
// routing and calls this model's Start/Update/View methods.
type model struct {
	*cardState

	// Test-specific state.
	testStart time.Time // when the whole test began (hard watchdog)
	quitting bool
	result   Result
	gotResult bool
}

// newTestModel builds a fresh Speed Test card from shared state.
func newTestModel(cs *cardState) *model {
	m := &model{cardState: cs}
	m.testStart = time.Now()
	return m
}

// Start kicks off the background test + channel bridge and returns the telegram
// of commands that keep the UI alive (spinner tick, refresh tick, event listen).
func (m *model) Start() tea.Cmd {
	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		Run(m.ctx, m.progress, defaultConnections, defaultDuration)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

// reset tears down the in-flight test and starts a fresh one, clearing the
// graphs and all live state. Old goroutines wind down via their cancelled
// context, so this is safe to call mid-test or after completion.
func (m *model) reset() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	cs := newCardState(m.theme)
	m.cardState = cs
	m.testStart = time.Now()
	m.gotResult = false
	m.quitting = false

	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		Run(m.ctx, m.progress, defaultConnections, defaultDuration)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

// Update handles events. It returns a tea.Cmd for the router to perform; it
// never calls tea.Quit itself — the router owns quit/back navigation. The
// returned bool is true when the model wants to go back to the menu.
func (m *model) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return tea.Quit, false
		case "esc", "m":
			// Back to the start menu.
			if m.cancel != nil {
				m.cancel()
			}
			return backToMenuCmd(), false
		case "?":
			m.showHelp = !m.showHelp
			return nil, false
		case "r":
			return m.reset(), false
		case "c":
			m.unit = (m.unit + 1) % 4
			return nil, false
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Keep graphs sized to the card's inner width (wider = more history
		// visible, so spikes are easier to read).
		inner := m.innerWidth(msg.Width)
		m.dlGraph.setWidth(inner)
		m.ulGraph.setWidth(inner)
		return nil, false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd, false

	case phaseMsg:
		m.phase = msg.phase
		if msg.phase == PhaseConnected && m.progress.ServerName != "" {
			m.serverName = m.progress.ServerName
		}
		// Start the per-phase timer for download/upload (the timed phases).
		if msg.phase == PhaseDownload || msg.phase == PhaseUpload {
			m.phaseStart = time.Now()
			m.phaseDur = defaultDuration
		}
		return listenCmd(m.events), false

	case sampleMsg:
		mbps := bytesPerSecToMbps(msg.sample.Rate)
		switch msg.sample.Phase {
		case PhaseDownload:
			m.dlTarget = mbps
		case PhaseUpload:
			m.ulTarget = mbps
		}
		return listenCmd(m.events), false

	case tickMsg:
		// Advance animations (lerp + graph growth) toward targets.
		m.advance()
		return m.tickCmd(), false

	case resultMsg:
		m.result = msg.result
		m.gotResult = true
		m.phase = PhaseDone
		if m.progress != nil && m.progress.Err != nil {
			m.err = m.progress.Err
		}
		// Snap displays to final values for a clean summary.
		m.dlTarget = m.result.DownloadMbps
		m.ulTarget = m.result.UploadMbps
		m.pingDisp = m.result.PingMs
		return nil, false

	case errMsg:
		m.err = msg.err
		m.phase = PhaseDone
		if m.cancel != nil {
			m.cancel()
		}
		return nil, false
	}
	return nil, false
}

// advance interpolates displayed values toward targets and pushes the smoothed
// value into the active phase's graph. It also runs a self-contained phase
// watchdog so the UI can never freeze in a single phase even if a network call
// stalls and the engine's events are delayed.
func (m *model) advance() {
	if !m.gotResult {
		m.dlDisplay = lerp(m.dlDisplay, m.dlTarget, animFactor)
		m.ulDisplay = lerp(m.ulDisplay, m.ulTarget, animFactor)
	} else {
		m.dlDisplay = m.dlTarget
		m.ulDisplay = m.ulTarget
	}
	switch m.phase {
	case PhaseDownload:
		if m.dlDisplay > 0 {
			m.dlGraph.push(m.dlDisplay)
		}
	case PhaseUpload:
		if m.ulDisplay > 0 {
			m.ulGraph.push(m.ulDisplay)
		}
	}

	// Watchdog: drive phase transitions on the local timer so we never hang.
	// The engine normally sends phase messages too; this is the fallback.
	if !m.gotResult {
		now := time.Now()
		switch m.phase {
		case PhaseDownload:
			if now.Sub(m.phaseStart) >= m.phaseDur {
				m.phase = PhaseUpload
				m.phaseStart = now
			}
		case PhaseUpload:
			if now.Sub(m.phaseStart) >= m.phaseDur {
				m.phase = PhaseLatency
				m.phaseStart = now
			}
		}
		// Hard ceiling: if the whole test runs absurdly long, force finish.
		if now.Sub(m.testStart) > 35*time.Second {
			m.phase = PhaseDone
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
		}
	}
}

// --- View ----------------------------------------------------------------

// View renders the Speed Test card. When quitting it returns an empty string so
// the router can clear the screen before exiting.
func (m *model) View() string {
	var body strings.Builder

	// A faint server/region line inside the card once known. The prominent
	// SPEED header now lives above the card (see renderHeader).
	if m.serverName != "" {
		body.WriteString(center(lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render(truncate("connected to "+m.serverName, m.cardWidthFor()-4)), m.cardWidthFor()))
		body.WriteString("\n\n")
	}

	// Phase status line (spinner for finding servers, check for connected).
	body.WriteString(m.statusLine())
	body.WriteString("\n\n")

	// Download block.
	body.WriteString(m.metricBlock(
		"↓ download", m.theme.Download, m.dlDisplay, m.dlGraph, m.result.DownloadPeak, PhaseDownload,
	))
	body.WriteString("\n\n")

	// Upload block.
	body.WriteString(m.metricBlock(
		"↑ upload", m.theme.Upload, m.ulDisplay, m.ulGraph, m.result.UploadPeak, PhaseUpload,
	))
	body.WriteString("\n\n")

	// Summary / ping line.
	body.WriteString(m.summaryLine())

	// Footer hint.
	hint := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("esc menu · q quit · r reset · c units (" + m.unit.label() + ") · ? help")
	body.WriteString("\n\n")
	body.WriteString(center(hint, m.cardWidthFor()))
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(1, 2).
		Width(m.cardWidthFor()).
		Render(body.String())

	// Header (SPEED + tagline) sits above the card.
	header := renderHeader("Wonder how speedy your internet is?")
	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"", // spacer
		card,
	)

	// Center the whole stack both horizontally and vertically in the terminal.
	placed := lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		stack,
	)

	// Help overlay (modal) is drawn when toggled.
	if m.showHelp {
		return m.renderHelp()
	}

	return placed
}

// summaryLine shows the final download / upload / ping on one line, with ping
// colored by the latency accent.
func (m *model) summaryLine() string {
	if m.phase != PhaseDone {
		// Live ping placeholder while testing.
		if m.phase == PhaseLatency {
			return center(lipgloss.NewStyle().Foreground(m.theme.Latency).
				Render(fmt.Sprintf("ping  %.0f ms", m.pingDisp)), m.cardWidthFor())
		}
		return ""
	}
	if m.err != nil {
		return center(lipgloss.NewStyle().Foreground(m.theme.Upload).Render(m.err.Error()), m.cardWidthFor())
	}
	dl := lipgloss.NewStyle().Foreground(m.theme.Download).Bold(true).Render(m.formatPeak(m.result.DownloadMbps))
	ul := lipgloss.NewStyle().Foreground(m.theme.Upload).Bold(true).Render(m.formatPeak(m.result.UploadMbps))
	pg := lipgloss.NewStyle().Foreground(m.theme.Latency).Bold(true).Render(fmt.Sprintf("%.0f ms", m.result.PingMs))
	line := lipgloss.JoinHorizontal(lipgloss.Center,
		"↓ "+dl, "    ", "↑ "+ul, "    ", "◷ "+pg,
	)
	return center(line, m.cardWidthFor())
}

// renderHelp renders a centered help modal describing the live controls. It
// replaces the normal card view while shown (toggle with ?).
func (m *model) renderHelp() string {
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	key := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)

	lines := []string{
		key.Render("?") + "  " + muted.Render("toggle this help"),
		key.Render("esc / m") + "  " + muted.Render("back to the menu"),
		key.Render("q") + "  " + muted.Render("quit"),
		key.Render("r") + "  " + muted.Render("restart the test"),
		key.Render("c") + "  " + muted.Render("cycle units (Mbps / KB/s / MB/s / GB/s)"),
	}

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Highlight).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	ch := m.height
	if ch <= 0 {
		ch = 1
	}
	return lipgloss.Place(m.width, ch, lipgloss.Center, lipgloss.Center, panel)
}
