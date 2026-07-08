package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tickMsg is emitted every ~100ms to refresh the UI and advance animations.
type tickMsg struct{}

// phaseMsg carries a phase transition from the background test.
type phaseMsg struct{ phase Phase }

// sampleMsg carries one instantaneous speed sample from the background test.
type sampleMsg struct{ sample Sample }

// resultMsg carries the final measurement from the background test.
type resultMsg struct {
	result Result
}

// errMsg carries a fatal error from the background test.
type errMsg struct{ err error }

// lerp smoothly interpolates a displayed value toward its real target so the
// number/bar animates instead of snapping. Factor is per-tick damping.
func lerp(current, target, factor float64) float64 {
	return current + (target-current)*factor
}

const (
	tickInterval    = 100 * time.Millisecond
	animFactor      = 0.18 // higher = snappier, lower = smoother
	cardWidth       = 64
	cardInnerWidth  = cardWidth - 4 // account for border + padding
)

// model is the bubbletea model for the speed-test card.
type model struct {
	theme    Theme
	progress *Progress
	ctx      context.Context
	cancel   context.CancelFunc
	events   chan tea.Msg // bridge from the background runner to Update
	spinner  spinner.Model
	width    int
	height   int
	hasBg    bool

	// Live phase state.
	phase      Phase
	phaseStart time.Time     // when the current timed phase began
	phaseDur   time.Duration // duration of the current timed phase
	testStart  time.Time     // when the whole test began (hard watchdog)
	quitting   bool
	err        error
	result     Result
	gotResult  bool

	// Animation state (interpolated display values).
	dlDisplay float64 // displayed download Mbps
	ulDisplay float64 // displayed upload Mbps
	dlTarget  float64
	ulTarget  float64
	pingDisp  float64

	// Sparklines (recent rate history, Mbps).
	dlSpark *sparkline
	ulSpark *sparkline

	// Discovered server info for the header.
	serverName string
}

func newModel(theme Theme, hasBg bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Highlight)

	p := &Progress{
		Phases:  make(chan Phase, 16),
		Samples: make(chan Sample, 256),
		Result:  make(chan Result, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())

	return model{
		theme:     theme,
		hasBg:     hasBg,
		progress:  p,
		ctx:       ctx,
		cancel:    cancel,
		events:    make(chan tea.Msg, 64),
		spinner:   s,
		phase:     PhaseInit,
		testStart: time.Now(),
		dlSpark:   newSparkline(24, 1, theme.Download),
		ulSpark:   newSparkline(24, 1, theme.Upload),
	}
}

// Init starts the spinner, the refresh tick, and the background test. The
// test and the channel bridge are launched as goroutines; their events arrive
// as tea.Msgs via listenCmd, so this model always observes live progress.
func (m model) Init() tea.Cmd {
	go Run(m.ctx, m.progress, defaultConnections, defaultDuration)
	go runBridge(m.ctx, m.progress, m.events)
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
}

// runBridge fans the background runner's channels into the tea event stream.
// It exits (and closes events) once the final result is delivered.
func runBridge(ctx context.Context, p *Progress, events chan tea.Msg) {
	for {
		select {
		case <-ctx.Done():
			close(events)
			return
		case ph, ok := <-p.Phases:
			if !ok {
				return
			}
			events <- phaseMsg{ph}
		case s, ok := <-p.Samples:
			if !ok {
				return
			}
			events <- sampleMsg{s}
		case r, ok := <-p.Result:
			if !ok {
				return
			}
			events <- resultMsg{r}
			close(events)
			return
		}
	}
}

// listenCmd waits for the next bridged event. Returning a nil msg (after the
// channel is closed) is a no-op for bubbletea.
func listenCmd(events chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}

// Update handles events.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Keep sparklines sized to the card's inner width.
		inner := m.innerWidth(msg.Width)
		if inner > 24 {
			inner = 24
		}
		m.dlSpark.setWidth(inner)
		m.ulSpark.setWidth(inner)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

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
		return m, listenCmd(m.events)

	case sampleMsg:
		mbps := bytesPerSecToMbps(msg.sample.Rate)
		switch msg.sample.Phase {
		case PhaseDownload:
			m.dlTarget = mbps
		case PhaseUpload:
			m.ulTarget = mbps
		}
		return m, listenCmd(m.events)

	case tickMsg:
		// Advance animations (lerp + sparkline growth) toward targets.
		m.advance()
		return m, m.tickCmd()

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
		return m, nil

	case errMsg:
		m.err = msg.err
		m.phase = PhaseDone
		if m.cancel != nil {
			m.cancel()
		}
		return m, nil
	}
	return m, nil
}

// tickCmd schedules the next refresh.
func (m model) tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// advance interpolates displayed values toward targets and pushes the
// smoothed value into the active phase's sparkline, so the graph builds up
// in sync with the number instead of snapping to raw readings. It also runs
// a self-contained phase watchdog so the UI can never freeze in a single
// phase even if a network call stalls and the engine's events are delayed.
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
			m.dlSpark.push(m.dlDisplay)
		}
	case PhaseUpload:
		if m.ulDisplay > 0 {
			m.ulSpark.push(m.ulDisplay)
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

// --- Layout helpers ------------------------------------------------------

func (m model) innerWidth(total int) int {
	w := cardInnerWidth
	if total > 0 {
		// Never exceed the terminal.
		maxW := total - 6
		if w > maxW {
			w = maxW
		}
	}
	if w < 20 {
		w = 20
	}
	return w
}

func (m model) cardWidthFor() int {
	w := cardWidth
	maxW := m.width - 2
	if w > maxW {
		w = maxW
	}
	if w < 30 {
		w = 30
	}
	return w
}

// --- Formatting ----------------------------------------------------------

func (m model) fmtSpeed(mbps float64) (string, string) {
	if mbps >= 1000 {
		return fmt.Sprintf("%5.2f", mbps/1000.0), "Gbps"
	}
	return fmt.Sprintf("%5.1f", mbps), "Mbps"
}

// --- View ----------------------------------------------------------------

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var body strings.Builder

	// Title block: centered bold title, plus a second line that shows the
	// connected server/region once known.
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.theme.Foreground)
	title := center(titleStyle.Render("⚡ internet speed test"), m.cardWidthFor())

	var sub string
	if m.serverName != "" {
		sub = "connected to " + m.serverName
	} else {
		sub = "measuring your connection"
	}
	subtitle := center(lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render(truncate(sub, m.cardWidthFor()-4)), m.cardWidthFor())

	body.WriteString(title)
	body.WriteString("\n")
	body.WriteString(subtitle)
	body.WriteString("\n\n")

	// Phase status line (spinner for finding servers, check for connected).
	body.WriteString(m.statusLine())
	body.WriteString("\n\n")

	// Download block.
	body.WriteString(m.metricBlock(
		"↓ download", m.theme.Download, m.dlDisplay, m.dlSpark, m.result.DownloadPeak, PhaseDownload,
	))
	body.WriteString("\n\n")

	// Upload block.
	body.WriteString(m.metricBlock(
		"↑ upload", m.theme.Upload, m.ulDisplay, m.ulSpark, m.result.UploadPeak, PhaseUpload,
	))
	body.WriteString("\n\n")

	// Summary / ping line.
	body.WriteString(m.summaryLine())

	// Footer hint.
	hint := lipgloss.NewStyle().
		Foreground(m.theme.Muted).
		Render("press q / esc to quit")
	body.WriteString("\n\n")
	body.WriteString(center(hint, m.cardWidthFor()))

	// Wrap in a bordered, padded card.
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(1, 2).
		Width(m.cardWidthFor()).
		Render(body.String())

	// Center the card both horizontally and vertically in the terminal.
	placed := lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		card,
	)

	if m.hasBg {
		// Paint the whole terminal area with the custom background, then
		// overlay the (transparent) centered card on top of it.
		full := lipgloss.NewStyle().
			Background(m.theme.Background).
			Width(m.width).
			Height(m.height).
			Render(placed)
		return full
	}
	return placed
}

// statusLine renders the current phase with a spinner, plus a live timer /
// progress bar for the timed download and upload phases so it's obvious the
// test runs for a fixed duration (not instant).
func (m model) statusLine() string {
	var label string
	var color lipgloss.AdaptiveColor
	switch m.phase {
	case PhaseFinding, PhaseInit:
		return center(m.spinner.View()+"  "+lipgloss.NewStyle().Foreground(m.theme.Muted).Render("finding servers…"), m.cardWidthFor())
	case PhaseConnected:
		who := "server"
		if m.serverName != "" {
			who = m.serverName
		}
		return center(lipgloss.NewStyle().Foreground(m.theme.Highlight).Render("✓ connected to "+who), m.cardWidthFor())
	case PhaseDownload:
		label, color = "measuring download", m.theme.Download
	case PhaseUpload:
		label, color = "measuring upload", m.theme.Upload
	case PhaseLatency:
		label, color = "measuring latency", m.theme.Latency
	case PhaseDone:
		if m.err != nil {
			return center(lipgloss.NewStyle().Foreground(m.theme.Upload).Render("✕ finished with errors"), m.cardWidthFor())
		}
		return center(lipgloss.NewStyle().Foreground(m.theme.Highlight).Render("✓ complete"), m.cardWidthFor())
	}

	// Compute elapsed / progress for the timed phases.
	elapsed := time.Since(m.phaseStart)
	if elapsed < 0 {
		elapsed = 0
	}
	total := m.phaseDur
	if total <= 0 {
		total = defaultDuration
	}
	frac := float64(elapsed) / float64(total)
	if frac > 1 {
		frac = 1
	}
	secs := int(elapsed.Seconds())
	if secs > int(total.Seconds()) {
		secs = int(total.Seconds())
	}

	labelStyled := lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)
	timer := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(fmt.Sprintf("%ds / %ds", secs, int(total.Seconds())))
	bar := m.progressBar(frac, color, 16)
	line := lipgloss.JoinHorizontal(lipgloss.Left, labelStyled, "   ", timer, "   ", bar)
	return center(line, m.cardWidthFor())
}

// progressBar draws a compact inline bar for the timed phases.
func (m model) progressBar(frac float64, color lipgloss.AdaptiveColor, width int) string {
	if width < 4 {
		width = 4
	}
	filled := int(frac * float64(width))
	if filled > width {
		filled = width
	}
	fill := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	empty := lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("░", width-filled))
	return fill + empty
}

// metricBlock renders one download or upload row: label, big number, unit,
// and a live sparkline, all left-aligned to the same edge so the sparkline
// underlines the metric. Columns are fixed-width so nothing shifts.
func (m model) metricBlock(label string, color lipgloss.AdaptiveColor, value float64, spark *sparkline, peak float64, ph Phase) string {
	numStr, unit := m.fmtSpeed(value)
	labelStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	numStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(6).Align(lipgloss.Right)
	unitStyle := lipgloss.NewStyle().Foreground(m.theme.Muted).Width(5)

	// Dim the metric if its phase hasn't started yet.
	if m.phase < ph && m.phase != PhaseDone {
		labelStyle = labelStyle.Faint(true)
		numStyle = numStyle.Faint(true)
	}

	top := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render(label),
		"  ",
		numStyle.Render(numStr),
		" ",
		unitStyle.Render(unit),
	)
	sp := spark.View()
	if sp == "" {
		return top
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, top, "   ", sp)
}

// summaryLine shows the final download / upload / ping on one line, with
// ping colored by the latency accent.
func (m model) summaryLine() string {
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
	dl := lipgloss.NewStyle().Foreground(m.theme.Download).Bold(true).Render(formatMbps(m.result.DownloadMbps))
	ul := lipgloss.NewStyle().Foreground(m.theme.Upload).Bold(true).Render(formatMbps(m.result.UploadMbps))
	pg := lipgloss.NewStyle().Foreground(m.theme.Latency).Bold(true).Render(fmt.Sprintf("%.0f ms", m.result.PingMs))
	line := lipgloss.JoinHorizontal(lipgloss.Center,
		"↓ "+dl, "    ", "↑ "+ul, "    ", "◷ "+pg,
	)
	return center(line, m.cardWidthFor())
}

// center centers a string within width w (single-line).
func center(s string, w int) string {
	if w <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, len(lines))
	for i, l := range lines {
		if lipgloss.Width(l) >= w {
			out[i] = l
			continue
		}
		pad := (w - lipgloss.Width(l)) / 2
		out[i] = strings.Repeat(" ", pad) + l
	}
	return strings.Join(out, "\n")
}

// truncate shortens s to at most w visible columns, appending an ellipsis if
// it was cut. Used so long server names never overflow the card.
func truncate(s string, w int) string {
	if w <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// Greedily drop trailing runes until it fits with an ellipsis.
	r := []rune(s)
	for len(r) > 0 {
		candidate := string(r) + "…"
		if lipgloss.Width(candidate) <= w {
			return candidate
		}
		r = r[:len(r)-1]
	}
	return "…"
}

func formatMbps(mbps float64) string {
	if mbps >= 1000 {
		return fmt.Sprintf("%.2f Gbps", mbps/1000.0)
	}
	return fmt.Sprintf("%.1f Mbps", mbps)
}
