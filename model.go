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
	w, h := m.width, m.height
	cs := newCardState(m.theme, m.compact)
	m.cardState = cs
	m.width, m.height = w, h
	m.syncLayout()
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
		m.syncLayout()
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
		// Prefer engine averages; if a partial/empty result arrives (cancel
		// mid-run), keep the live displays so the summary is not all zeros.
		if m.result.DownloadMbps > 0 {
			m.dlTarget = m.result.DownloadMbps
			m.dlDisplay = m.result.DownloadMbps
		} else if m.dlDisplay > 0 {
			m.result.DownloadMbps = m.dlDisplay
			if m.result.DownloadPeak < m.dlDisplay {
				m.result.DownloadPeak = m.dlDisplay
			}
		}
		if m.result.UploadMbps > 0 {
			m.ulTarget = m.result.UploadMbps
			m.ulDisplay = m.result.UploadMbps
		} else if m.ulDisplay > 0 {
			m.result.UploadMbps = m.ulDisplay
			if m.result.UploadPeak < m.ulDisplay {
				m.result.UploadPeak = m.ulDisplay
			}
		}
		if m.result.PingMs > 0 {
			m.pingDisp = m.result.PingMs
		}
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
	// Never cancel the engine here — cancelling used to close the event bridge
	// before Result arrived, which left the summary at 0.0 forever.
	if !m.gotResult {
		now := time.Now()
		switch m.phase {
		case PhaseDownload:
			if !m.phaseStart.IsZero() && now.Sub(m.phaseStart) >= m.phaseDur {
				m.phase = PhaseUpload
				m.phaseStart = now
			}
		case PhaseUpload:
			if !m.phaseStart.IsZero() && now.Sub(m.phaseStart) >= m.phaseDur {
				m.phase = PhaseLatency
				m.phaseStart = now
			}
		case PhaseLatency:
			// Latency should finish in a couple of seconds. If it stalls, keep
			// showing the phase but do not invent a zeroed Result.
		}
	}
}

// --- View ----------------------------------------------------------------

// View renders the Speed Test card. When quitting it returns an empty string so
// the router can clear the screen before exiting.
func (m *model) View() string {
	m.syncLayout()

	var body strings.Builder

	// A faint server/region line inside the card once known. The prominent
	// SPEED header now lives above the card (see renderHeader).
	if m.serverName != "" {
		inner := m.cardWidthFor() - 4 // border + padding
		body.WriteString(center(lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render("connected to "+m.serverName), inner))
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
	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("esc"), mt.Render(" menu  ·  "),
		hl.Render("q"), mt.Render(" quit  ·  "),
		hl.Render("r"), mt.Render(" reset  ·  "),
		hl.Render("c"), mt.Render(" units  ·  "),
		hl.Render("t"), mt.Render(" compact  ·  "),
		hl.Render("?"), mt.Render(" help"),
	)
	body.WriteString("\n\n")
	body.WriteString(center(hint, m.cardWidthFor()))
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Background(m.theme.AppBg).
		Padding(1, 2).
		Width(m.cardWidthFor()).
		Render(body.String())

	// Header (SPEED + tagline) sits above the card.
	var header string
	if m.compact {
		header = renderCompactHeader("Wonder how speedy your internet is?")
	} else {
		header = renderHeader("Wonder how speedy your internet is?")
	}
	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"", // spacer
		card,
	)

	// Help overlay (modal) is drawn when toggled.
	if m.showHelp {
		return m.renderHelp()
	}

	return paintScreen(m.theme, m.width, m.height, stack)
}

// summaryLine shows the final download / upload / ping on one line, with ping
// colored by the latency accent.
func (m *model) summaryLine() string {
	if m.phase != PhaseDone {
		if m.phase == PhaseLatency {
			msg := "measuring latency…"
			if m.pingDisp > 0 {
				msg = fmt.Sprintf("ping  %.0f ms", m.pingDisp)
			}
			return center(lipgloss.NewStyle().Foreground(m.theme.Latency).Render(msg), m.cardWidthFor())
		}
		return ""
	}
	if m.err != nil && m.result.DownloadMbps <= 0 && m.result.UploadMbps <= 0 {
		return center(lipgloss.NewStyle().Foreground(m.theme.Upload).Render(m.err.Error()), m.cardWidthFor())
	}
	dlMbps := m.result.DownloadMbps
	if dlMbps <= 0 {
		dlMbps = m.dlDisplay
	}
	ulMbps := m.result.UploadMbps
	if ulMbps <= 0 {
		ulMbps = m.ulDisplay
	}
	pingMs := m.result.PingMs
	if pingMs <= 0 {
		pingMs = m.pingDisp
	}
	dl := lipgloss.NewStyle().Foreground(m.theme.Download).Bold(true).Render(m.formatPeak(dlMbps))
	ul := lipgloss.NewStyle().Foreground(m.theme.Upload).Bold(true).Render(m.formatPeak(ulMbps))
	pg := lipgloss.NewStyle().Foreground(m.theme.Latency).Bold(true).Render(fmt.Sprintf("%.0f ms", pingMs))
	line := lipgloss.JoinHorizontal(lipgloss.Center,
		"↓ "+dl, "    ", "↑ "+ul, "    ", "◷ "+pg,
	)
	return center(line, m.cardWidthFor())
}

// renderHelp renders a centered help modal describing the live controls. It
// replaces the normal card view while shown (toggle with ?).
func (m *model) renderHelp() string {
	return renderHelpPanel(m.theme, "Speed Test — Help", []helpBinding{
		{keys: "esc / m", action: "back to main menu"},
		{keys: "?", action: "close this help"},
		{keys: "q", action: "quit riptide"},
		{keys: "r", action: "restart the speed test"},
		{keys: "c", action: "cycle units  Mbps · KB/s · MB/s · GB/s"},
		{keys: "t", action: "toggle compact logo"},
	}, m.width, m.height)
}
