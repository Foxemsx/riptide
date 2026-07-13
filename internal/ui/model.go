package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/atotto/clipboard"
	"github.com/Foxemsx/riptide/internal/db"
	"github.com/Foxemsx/riptide/internal/engine"
	apptheme "github.com/Foxemsx/riptide/internal/theme"
)

// model is the bubbletea sub-model for the one-shot Speed Test card.
type model struct {
	*cardState

	testStart time.Time
	quitting  bool
	result    engine.Result
	gotResult bool

	store      *db.Store
	history    []db.TestRun
	avg        db.Averages
	savePrompt savePromptModel
	autoSaved  bool
	savedFlash string
	copiedFlash string
}

func newTestModel(cs *cardState, store *db.Store) *model {
	m := &model{
		cardState:  cs,
		store:      store,
		savePrompt: newSavePrompt(cs.theme, "speed"),
	}
	m.loadAverages()
	m.testStart = time.Now()
	return m
}

// loadAverages refreshes the saved-run averages from the DB (best-effort).
func (m *model) loadAverages() {
	if m.store == nil {
		return
	}
	if a, err := m.store.Averages("speed"); err == nil {
		m.avg = a
	}
}

func (m *model) Start() tea.Cmd {
	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		engine.Run(m.ctx, m.progress, engine.DefaultConnections, engine.DefaultDuration)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

func (m *model) reset() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	w, h := m.width, m.height
	hist := m.history
	store := m.store
	cs := newCardState(m.theme, m.compact)
	m.cardState = cs
	m.width, m.height = w, h
	m.syncLayout()
	m.testStart = time.Now()
	m.gotResult = false
	m.quitting = false
	m.autoSaved = false
	m.savedFlash = ""
	m.copiedFlash = ""
	m.history = hist
	m.store = store
	m.savePrompt = newSavePrompt(m.theme, "speed")
	m.loadAverages()
	m.savePrompt.width = w
	m.savePrompt.height = h

	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		engine.Run(m.ctx, m.progress, engine.DefaultConnections, engine.DefaultDuration)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Cmd, bool) {
	if m.savePrompt.active {
		if cmd := m.savePrompt.Update(msg); cmd != nil {
			return cmd, false
		}
		// Still active after update (typing) — consume keys.
		if m.savePrompt.active {
			return nil, false
		}
	}

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
		case "y":
			return m.copyResultCmd(), false
		case "s":
			// Named save of current/final numbers.
			dl := m.result.DownloadMbps
			if dl <= 0 {
				dl = m.dlDisplay
			}
			ul := m.result.UploadMbps
			if ul <= 0 {
				ul = m.ulDisplay
			}
			pg := m.result.PingMs
			if pg <= 0 {
				pg = m.pingDisp
			}
			m.savePrompt.width = m.width
			m.savePrompt.height = m.height
			m.savePrompt.open(dl, ul, pg, m.result.DownloadPeak, m.result.UploadPeak, m.serverName)
			return textinputBlink(), false
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncLayout()
		m.savePrompt.width = msg.Width
		m.savePrompt.height = msg.Height
		return nil, false

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd, false

	case phaseMsg:
		m.phase = msg.phase
		if msg.phase == engine.PhaseConnected && m.progress.ServerName != "" {
			m.serverName = m.progress.ServerName
		}
		if msg.phase == engine.PhaseDownload || msg.phase == engine.PhaseUpload {
			m.phaseStart = time.Now()
			m.phaseDur = engine.DefaultDuration
		}
		return listenCmd(m.events), false

	case sampleMsg:
		mbps := engine.BytesPerSecToMbps(msg.sample.Rate)
		switch msg.sample.Phase {
		case engine.PhaseDownload:
			m.dlTarget = mbps
		case engine.PhaseUpload:
			m.ulTarget = mbps
		}
		return listenCmd(m.events), false

	case tickMsg:
		m.advance()
		return m.tickCmd(), false

	case resultMsg:
		m.result = msg.result
		m.gotResult = true
		m.phase = engine.PhaseDone
		if m.progress != nil && m.progress.Err != nil {
			m.err = m.progress.Err
		}
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
		// Auto-save completed runs (once).
		return m.autoSaveCmd(), false

	case errMsg:
		m.err = msg.err
		m.phase = engine.PhaseDone
		if m.cancel != nil {
			m.cancel()
		}
		return nil, false

	case copyDoneMsg:
		m.applyCopyFlash(msg.text)
		return nil, false
	}
	return nil, false
}

func (m *model) autoSaveCmd() tea.Cmd {
	if m.autoSaved || m.store == nil {
		return nil
	}
	if m.result.DownloadMbps <= 0 && m.result.UploadMbps <= 0 {
		return nil
	}
	m.autoSaved = true
	m.loadAverages()
	run := db.TestRun{
		Name:         db.AutoName("speed", time.Now()),
		Kind:         "speed",
		DownloadMbps: m.result.DownloadMbps,
		UploadMbps:   m.result.UploadMbps,
		PingMs:       m.result.PingMs,
		DownloadPeak: m.result.DownloadPeak,
		UploadPeak:   m.result.UploadPeak,
		Server:       m.serverName,
		CreatedAt:    time.Now(),
	}
	return func() tea.Msg { return saveRunMsg{run: run} }
}

func textinputBlink() tea.Cmd {
	return func() tea.Msg { return nil }
}

// resultValues returns the most representative ↓/↑/ping to show or copy:
// prefer the finished result, else the live displayed values.
func (m *model) resultValues() (dl, ul, ping float64) {
	if m.gotResult {
		return m.result.DownloadMbps, m.result.UploadMbps, m.result.PingMs
	}
	return m.dlDisplay, m.ulDisplay, m.pingDisp
}

// copyResultCmd copies the result as "↓248 ↑19 12ms" to the system clipboard.
func (m *model) copyResultCmd() tea.Cmd {
	dl, ul, ping := m.resultValues()
	if dl <= 0 && ul <= 0 && ping <= 0 {
		return nil
	}
	text := fmt.Sprintf("↓%.0f ↑%.0f %.0fms", dl, ul, ping)
	t := text
	return func() tea.Msg {
		_ = clipboard.WriteAll(t)
		return copyDoneMsg{text: t}
	}
}

type copyDoneMsg struct{ text string }

func (m *model) applyCopyFlash(text string) {
	m.copiedFlash = "Copied  " + text
}

func (m *model) advance() {
	if !m.gotResult {
		m.dlDisplay = lerp(m.dlDisplay, m.dlTarget, animFactor)
		m.ulDisplay = lerp(m.ulDisplay, m.ulTarget, animFactor)
	} else {
		m.dlDisplay = m.dlTarget
		m.ulDisplay = m.ulTarget
	}
	switch m.phase {
	case engine.PhaseDownload:
		if m.dlDisplay > 0 {
			m.dlGraph.push(m.dlDisplay)
		}
	case engine.PhaseUpload:
		if m.ulDisplay > 0 {
			m.ulGraph.push(m.ulDisplay)
		}
	}

	if !m.gotResult {
		now := time.Now()
		switch m.phase {
		case engine.PhaseDownload:
			if !m.phaseStart.IsZero() && now.Sub(m.phaseStart) >= m.phaseDur {
				m.phase = engine.PhaseUpload
				m.phaseStart = now
			}
		case engine.PhaseUpload:
			if !m.phaseStart.IsZero() && now.Sub(m.phaseStart) >= m.phaseDur {
				m.phase = engine.PhaseLatency
				m.phaseStart = now
			}
		}
	}
}

func (m *model) View() string {
	if m.savePrompt.active {
		m.savePrompt.width = m.width
		m.savePrompt.height = m.height
		return m.savePrompt.View()
	}

	m.syncLayout()

	var body strings.Builder

	if m.serverName != "" {
		inner := m.cardWidthFor() - 4
		body.WriteString(center(lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render("connected to "+m.serverName), inner))
		body.WriteString("\n\n")
	}

	body.WriteString(m.statusLine())
	body.WriteString("\n\n")

	body.WriteString(m.metricBlock(
		"↓ download", m.theme.Download, m.dlDisplay, m.dlGraph, m.result.DownloadPeak, engine.PhaseDownload,
	))
	body.WriteString("\n\n")

	body.WriteString(m.metricBlock(
		"↑ upload", m.theme.Upload, m.ulDisplay, m.ulGraph, m.result.UploadPeak, engine.PhaseUpload,
	))
	body.WriteString("\n\n")

	body.WriteString(m.summaryLine())

	if m.copiedFlash != "" {
		body.WriteString("\n")
		body.WriteString(center(lipgloss.NewStyle().Foreground(m.theme.Highlight).Render(m.copiedFlash), m.cardWidthFor()))
	} else if m.savedFlash != "" {
		body.WriteString("\n")
		body.WriteString(center(lipgloss.NewStyle().Foreground(m.theme.Highlight).Render("✓ "+m.savedFlash), m.cardWidthFor()))
	}

	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("esc"), mt.Render(" menu  ·  "),
		hl.Render("s"), mt.Render(" save  ·  "),
		hl.Render("y"), mt.Render(" copy  ·  "),
		hl.Render("r"), mt.Render(" reset  ·  "),
		hl.Render("c"), mt.Render(" units  ·  "),
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

	// History beside the speed card when the terminal is wide enough;
	// otherwise stack underneath.
	sideBySide := m.width >= historySideMinWidth
	histW := m.cardWidthFor()
	if sideBySide {
		// Leave a gap; size history to fill remaining space (capped).
		histW = m.width - m.cardWidthFor() - 8
		if histW > 56 {
			histW = 56
		}
		if histW < 36 {
			histW = 36
		}
	}
	hist := historyBlock(m.theme, m.history, histW, m.unit, "")

	avgW := histW
	if !sideBySide {
		// Match the speed card width when stacked.
		avgW = m.cardWidthFor()
	}
	avg := averageBlock(m.theme, m.avg, m.unit, avgW)

	// Quality block is based on your average internet (or the latest run when
	// you have no saved history yet).
	var qDl, qUl, qPing float64
	hasQ := false
	if m.avg.Count > 0 {
		qDl, qUl, qPing = m.avg.DownloadMbps, m.avg.UploadMbps, m.avg.PingMs
		hasQ = true
	} else if m.gotResult {
		qDl, qUl, qPing = m.result.DownloadMbps, m.result.UploadMbps, m.result.PingMs
		hasQ = qDl > 0 || qUl > 0 || qPing > 0
	}
	quality := qualityBlock(m.theme, qDl, qUl, qPing, hasQ, avgW)

	var header string
	if m.compact {
		header = renderCompactHeader("Wonder how speedy your internet is?")
	} else {
		header = renderHeader("Wonder how speedy your internet is?")
	}

	var main string
	if sideBySide {
		right := lipgloss.JoinVertical(lipgloss.Top, hist, "", avg, "", quality)
		main = lipgloss.JoinHorizontal(lipgloss.Top,
			card,
			lipgloss.NewStyle().Width(2).Render(" "),
			right,
		)
	} else {
		main = lipgloss.JoinVertical(lipgloss.Center, card, "", hist, "", avg, "", quality)
	}

	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"",
		main,
	)

	if m.showHelp {
		return m.renderHelp()
	}

	return apptheme.PaintScreen(m.theme, m.width, m.height, stack)
}

func (m *model) summaryLine() string {
	if m.phase != engine.PhaseDone {
		if m.phase == engine.PhaseLatency {
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

func (m *model) renderHelp() string {
	return renderHelpPanel(m.theme, "Speed Test — Help", []helpBinding{
		{keys: "esc / m", action: "back to main menu"},
		{keys: "?", action: "close this help"},
		{keys: "q", action: "quit riptide"},
		{keys: "s", action: "save run with a custom name"},
		{keys: "r", action: "restart the speed test"},
		{keys: "c", action: "cycle units  Mbps · KB/s · MB/s · GB/s"},
		{keys: "y", action: "copy result as  ↓248 ↑19 12ms  to clipboard"},
		{keys: "t", action: "toggle compact logo"},
	}, m.width, m.height)
}
