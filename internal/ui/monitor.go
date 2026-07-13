package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/Foxemsx/riptide/internal/engine"
	apptheme "github.com/Foxemsx/riptide/internal/theme"
)

// appsSideMinWidth: terminal must be at least this wide to place the apps
// panel beside the monitor card instead of below it.
const appsSideMinWidth = 116

// appUse tracks one process seen using the network during this session.
type appUse struct {
	name      string
	firstSeen time.Time
	lastSeen  time.Time
	conns     int
	active    bool
}

// monitorModel is the live Bandwidth Monitor card.
type monitorModel struct {
	*cardState

	paused    bool
	startTime time.Time

	dlPeak float64
	ulPeak float64

	pingDone bool

	// Per-app activity (accumulated since the monitor started).
	apps     map[string]*appUse
	showApps bool
	appTick  int
}

func newMonitorModel(cs *cardState) *monitorModel {
	m := &monitorModel{
		cardState: cs,
		apps:      map[string]*appUse{},
	}
	m.startTime = time.Now()
	return m
}

func (m *monitorModel) Start() tea.Cmd {
	bridgeLaunch(m.ctx, m.progress, m.events, func() {
		engine.RunMonitor(m.ctx, m.progress, tickInterval)
	})
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

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
		engine.RunMonitor(m.ctx, m.progress, tickInterval)
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
	case "a":
		m.showApps = !m.showApps
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
		if msg.phase == engine.PhaseConnected {
			if m.progress.ServerName != "" {
				m.serverName = m.progress.ServerName
			}
			m.phase = engine.PhaseUpload
		}
		return listenCmd(m.events), false

	case sampleMsg:
		if !m.paused {
			mbps := engine.BytesPerSecToMbps(msg.sample.Rate)
			switch msg.sample.Phase {
			case engine.PhaseDownload:
				m.dlTarget = mbps
				if mbps > m.dlPeak {
					m.dlPeak = mbps
				}
			case engine.PhaseUpload:
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

func (m *monitorModel) advance() {
	if m.paused {
		return
	}
	m.dlDisplay = lerp(m.dlDisplay, m.dlTarget, animFactor)
	m.ulDisplay = lerp(m.ulDisplay, m.ulTarget, animFactor)
	m.dlGraph.push(m.dlDisplay)
	m.ulGraph.push(m.ulDisplay)

	// Refresh the per-app list roughly once per second (every ~10 ticks).
	m.appTick++
	if m.appTick >= 10 {
		m.appTick = 0
		m.refreshApps()
	}
}

// refreshApps snapshots active processes and merges them into the accumulated
// session list. Apps no longer seen are kept but marked inactive.
func (m *monitorModel) refreshApps() {
	live := engine.ActiveProcs()
	now := time.Now()
	for name, n := range live {
		if a, ok := m.apps[name]; ok {
			a.conns = n
			a.active = true
			a.lastSeen = now
		} else {
			m.apps[name] = &appUse{name: name, firstSeen: now, lastSeen: now, conns: n, active: true}
		}
	}
	for _, a := range m.apps {
		if _, ok := live[a.name]; !ok {
			a.active = false
		}
	}
}

func (m *monitorModel) View() string {
	m.syncLayout()

	var body strings.Builder

	if m.serverName != "" {
		inner := m.cardWidthFor() - 4
		body.WriteString(center(lipgloss.NewStyle().
			Foreground(m.theme.Muted).
			Render("watching "+m.serverName), inner))
		body.WriteString("\n\n")
	}

	modeLabel := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true).Render("● LIVE")
	if m.paused {
		modeLabel = lipgloss.NewStyle().Foreground(m.theme.Muted).Bold(true).Render("Ⅱ PAUSED")
	}
	body.WriteString(center(lipgloss.JoinHorizontal(lipgloss.Left, m.spinner.View()+" ", modeLabel), m.cardWidthFor()))
	body.WriteString("\n\n")

	body.WriteString(m.metricBlock(
		"↓ download", m.theme.Download, m.dlDisplay, m.dlGraph, m.dlPeak, engine.PhaseDownload,
	))
	body.WriteString("\n\n")

	body.WriteString(m.metricBlock(
		"↑ upload", m.theme.Upload, m.ulDisplay, m.ulGraph, m.ulPeak, engine.PhaseUpload,
	))
	body.WriteString("\n\n")

	uptime := time.Since(m.startTime).Round(time.Second)
	pingStr := "-"
	if m.pingDone {
		pingStr = fmt.Sprintf("%.0f ms", m.pingDisp)
	}
	left := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("uptime " + uptime.String())
	right := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(m.unit.label() + " · ping " + pingStr)
	body.WriteString(center(lipgloss.JoinHorizontal(lipgloss.Left, left, "    ", right), m.cardWidthFor()))

	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("esc"), mt.Render(" menu  ·  "),
		hl.Render("c"), mt.Render(" units  ·  "),
		hl.Render("p"), mt.Render(" pause  ·  "),
		hl.Render("r"), mt.Render(" reset  ·  "),
		hl.Render("a"), mt.Render(" apps  ·  "),
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

	stack := lipgloss.JoinVertical(lipgloss.Center, header, "", card)

	if m.showApps {
		sideBySide := m.width >= appsSideMinWidth
		appW := m.cardWidthFor()
		if sideBySide {
			appW = m.width - m.cardWidthFor() - 8
			if appW > 52 {
				appW = 52
			}
			if appW < 34 {
				appW = 34
			}
		}
		appsCard := appsBlock(m.theme, m.apps, appW)
		if sideBySide {
			stack = lipgloss.JoinVertical(lipgloss.Center, header, "",
				lipgloss.JoinHorizontal(lipgloss.Top,
					card,
					lipgloss.NewStyle().Width(2).Render(" "),
					appsCard,
				),
			)
		} else {
			stack = lipgloss.JoinVertical(lipgloss.Center, header, "", card, "", appsCard)
		}
	}

	if m.showHelp {
		return m.renderHelp()
	}

	return apptheme.PaintScreen(m.theme, m.width, m.height, stack)
}

func (m *monitorModel) renderHelp() string {
	return renderHelpPanel(m.theme, "Bandwidth — Help", []helpBinding{
		{keys: "esc / m", action: "back to main menu"},
		{keys: "?", action: "close this help"},
		{keys: "q", action: "quit riptide"},
		{keys: "p", action: "pause / resume monitoring"},
		{keys: "r", action: "restart the monitor"},
		{keys: "c", action: "cycle units  Mbps · KB/s · MB/s · GB/s"},
		{keys: "a", action: "toggle the Apps using bandwidth panel"},
		{keys: "t", action: "toggle compact logo"},
	}, m.width, m.height)
}

// sortedApps returns the accumulated apps: active first (then by name), then
// inactive (by name). Keeps the panel readable and stable.
func sortedApps(apps map[string]*appUse) []*appUse {
	out := make([]*appUse, 0, len(apps))
	for _, a := range apps {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].active != out[j].active {
			return out[i].active
		}
		return out[i].name < out[j].name
	})
	return out
}

func appsBlock(theme apptheme.Theme, apps map[string]*appUse, width int) string {
	if width < 28 {
		width = 28
	}
	bg := theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")

	plain := lipgloss.NewStyle().Background(bg)
	fg := func(c lipgloss.TerminalColor, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c).Background(bg)
		if bold {
			s = s.Bold(true)
		}
		return s
	}
	innerW := width - 4
	if innerW < 24 {
		innerW = 24
	}
	line := func(parts ...string) string {
		return lipgloss.NewStyle().
			Width(innerW).
			Background(bg).
			Inline(true).
			Render(strings.Join(parts, ""))
	}

	titleChip := lipgloss.NewStyle().
		Foreground(ink).
		Background(theme.AccentDL).
		Bold(true).
		Padding(0, 1).
		Render("Apps using bandwidth")

	var body []string
	body = append(body, line(
		titleChip,
		plain.Render(" "),
		fg(theme.Muted, false).Render("active right now"),
	))
	body = append(body, line(fg(theme.Border, false).Render(strings.Repeat("─", min(innerW, 48)))))

	list := sortedApps(apps)
	if len(list) == 0 {
		body = append(body, line(fg(theme.Muted, false).Render("No apps detected yet —")))
		body = append(body, line(fg(theme.Muted, false).Render("give it a second while watching.")))
	} else {
		const maxShow = 14
		if len(list) > maxShow {
			list = list[:maxShow]
		}
		nameW := innerW - 12
		if nameW < 8 {
			nameW = 8
		}
		for _, a := range list {
			dot := fg(theme.Muted, false).Render("○")
			rowStyle := fg(theme.Muted, false)
			if a.active {
				dot = fg(theme.Download, true).Render("●")
				rowStyle = fg(theme.Foreground, false)
			}
			name := padRight(truncate(a.name, nameW-1), nameW)
			conn := padLeft(fmt.Sprintf("%d conn", a.conns), 7)
			body = append(body, line(
				dot,
				plain.Render(" "),
				rowStyle.Render(name),
				fg(theme.Muted, false).Render(conn),
			))
		}
		if len(sortedApps(apps)) > maxShow {
			body = append(body, line(fg(theme.Muted, false).Render(
				fmt.Sprintf("… and %d more", len(sortedApps(apps))-maxShow))))
		}
	}

	content := strings.Join(body, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Background(bg).
		Padding(0, 1).
		Width(width).
		Render(content)
}
