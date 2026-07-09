package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// monitorModel is the live Bandwidth Monitor card. It embeds *cardState and
// watches the PC's real network traffic continuously (never finishing) until
// cancelled. The engine reads OS interface counters rather than generating
// test traffic. All-time peaks are tracked here from the sample stream, and
// the usual controls (units, pause, reset, help, back) are available.
type monitorModel struct {
	*cardState

	paused    bool
	startTime time.Time

	// All-time peaks tracked from samples (Mbps).
	dlPeak float64
	ulPeak float64

	// Live ping is filled lazily via a one-shot latency check.
	pingDone bool
}

func newMonitorModel(cs *cardState) *monitorModel {
	m := &monitorModel{cardState: cs}
	m.startTime = time.Now()
	return m
}

// Start kicks off the continuous engine + bridge.
func (m *monitorModel) Start() tea.Cmd {
	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		RunMonitor(m.ctx, m.progress, tickInterval)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

// reset restarts the monitor from scratch (graphs + peaks + engine).
func (m *monitorModel) reset() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	w, h := m.width, m.height
	cs := newCardState(m.theme, m.compact)
	m.cardState = cs
	m.width, m.height = w, h
	m.syncLayout()
	m.startTime = time.Now()
	m.dlPeak = 0
	m.ulPeak = 0
	m.pingDone = false
	m.paused = false

	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		RunMonitor(m.ctx, m.progress, tickInterval)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

func (m *monitorModel) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return tea.Quit, false
		case "esc", "m":
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
		case "p":
			m.paused = !m.paused
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
		// The monitor watches real PC traffic and has no remote target, so
		// there is nothing to measure latency against - only show the
		// adapter label.
		if msg.phase == PhaseConnected {
			if m.progress.ServerName != "" {
				m.serverName = m.progress.ServerName
			}
			// Both directions are measured continuously, so treat the card as
			// fully active (phase Upload = highest) to keep neither DL nor UL
			// block dimmed in metricBlock.
			m.phase = PhaseUpload
		}
		return listenCmd(m.events), false

	case sampleMsg:
		if !m.paused {
			mbps := bytesPerSecToMbps(msg.sample.Rate)
			switch msg.sample.Phase {
			case PhaseDownload:
				m.dlTarget = mbps
				if mbps > m.dlPeak {
					m.dlPeak = mbps
				}
			case PhaseUpload:
				m.ulTarget = mbps
				if mbps > m.ulPeak {
					m.ulPeak = mbps
				}
			}
		}
		return listenCmd(m.events), false

	case pingMsg:
		m.pingDisp = msg.ms
		m.pingDone = true
		return nil, false

	case tickMsg:
		m.advance()
		return m.tickCmd(), false
	}
	return nil, false
}

// advance interpolates the displayed values. The monitor never "finishes", so
// there is no phase watchdog and no result snap. When paused we freeze the
// displayed numbers in place (targets stop updating, so the lerp holds them).
func (m *monitorModel) advance() {
	if m.paused {
		// Keep display pinned; do not push new graph rows.
		return
	}
	m.dlDisplay = lerp(m.dlDisplay, m.dlTarget, animFactor)
	m.ulDisplay = lerp(m.ulDisplay, m.ulTarget, animFactor)
	// Push every tick so the timeline scrolls continuously (zeros = idle gap).
	m.dlGraph.push(m.dlDisplay)
	m.ulGraph.push(m.ulDisplay)
}

// View renders the live monitor card. Layout mirrors the Speed Test card but
// shows both DL + UL live (no progress countdown) plus an uptime line and a
// pause indicator.
func (m *monitorModel) View() string {
	m.syncLayout()

	var body strings.Builder

	if m.serverName != "" {
		inner := m.cardWidthFor() - 4 // border + padding
		body.WriteString(center(lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render("watching "+m.serverName), inner))
		body.WriteString("\n\n")
	}

	// Mode + status line (spinner while connecting, then "live").
	modeLabel := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true).Render("● LIVE")
	if m.paused {
		modeLabel = lipgloss.NewStyle().Foreground(m.theme.Muted).Bold(true).Render("Ⅱ PAUSED")
	}
	body.WriteString(center(lipgloss.JoinHorizontal(lipgloss.Left, m.spinner.View()+" ", modeLabel), m.cardWidthFor()))
	body.WriteString("\n\n")

	// Download block.
	body.WriteString(m.metricBlock(
		"↓ download", m.theme.Download, m.dlDisplay, m.dlGraph, m.dlPeak, PhaseDownload,
	))
	body.WriteString("\n\n")

	// Upload block.
	body.WriteString(m.metricBlock(
		"↑ upload", m.theme.Upload, m.ulDisplay, m.ulGraph, m.ulPeak, PhaseUpload,
	))
	body.WriteString("\n\n")

	// Uptime + ping line.
	uptime := time.Since(m.startTime).Round(time.Second)
	pingStr := "-"
	if m.pingDone {
		pingStr = fmt.Sprintf("%.0f ms", m.pingDisp)
	}
	left := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("uptime " + uptime.String())
	right := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(m.unit.label() + " · ping " + pingStr)
	body.WriteString(center(lipgloss.JoinHorizontal(lipgloss.Left, left, "    ", right), m.cardWidthFor()))

	// Footer hint.
	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("esc"), mt.Render(" menu  ·  "),
		hl.Render("c"), mt.Render(" units  ·  "),
		hl.Render("p"), mt.Render(" pause  ·  "),
		hl.Render("r"), mt.Render(" reset  ·  "),
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

	var header string
	if m.compact {
		header = renderCompactHeader("Watching your connection in real time")
	} else {
		header = renderHeader("Watching your connection in real time")
	}
	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"",
		card,
	)

	if m.showHelp {
		return m.renderHelp()
	}

	return paintScreen(m.theme, m.width, m.height, stack)
}

// renderHelp renders the monitor's control help modal.
func (m *monitorModel) renderHelp() string {
	return renderHelpPanel(m.theme, "Bandwidth — Help", []helpBinding{
		{keys: "esc / m", action: "back to main menu"},
		{keys: "?", action: "close this help"},
		{keys: "q", action: "quit riptide"},
		{keys: "p", action: "pause / resume monitoring"},
		{keys: "r", action: "restart the monitor"},
		{keys: "c", action: "cycle units  Mbps · KB/s · MB/s · GB/s"},
		{keys: "t", action: "toggle compact logo"},
	}, m.width, m.height)
}
