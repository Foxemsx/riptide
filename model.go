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
	tickInterval   = 100 * time.Millisecond
	animFactor     = 0.18 // higher = snappier, lower = smoother
	cardWidth      = 64
	cardInnerWidth = cardWidth - 4 // account for border + padding
	graphHeight    = 8
)

// unitMode selects how the measured speed is displayed.
type unitMode int

const (
	unitAuto unitMode = iota // Mbps (or Gbps above 1000)
	unitKB
	unitMB
	unitGB
)

// unitLabel returns the short suffix for the current unit mode.
func (u unitMode) label() string {
	switch u {
	case unitKB:
		return "KB/s"
	case unitMB:
		return "MB/s"
	case unitGB:
		return "GB/s"
	default:
		return "Mbps"
	}
}

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

	// Live graph (vertical bar chart) of recent rate history, in Mbps.
	dlGraph *graph
	ulGraph *graph

	// Controls / display toggles.
	showHelp bool
	unit     unitMode

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
		dlGraph:   newGraph(40, graphHeight, theme.GraphDownBottom, theme.GraphDownTop),
		ulGraph:   newGraph(40, graphHeight, theme.GraphUpBottom, theme.GraphUpTop),
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

// launchTest kicks off the background test and channel bridge for the
// current context/progress. Shared by Init and reset.
func launchTest(ctx context.Context, p *Progress, events chan tea.Msg) {
	go Run(ctx, p, defaultConnections, defaultDuration)
	go runBridge(ctx, p, events)
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

// reset tears down the in-flight test and starts a fresh one, clearing the
// graphs and all live state. Old goroutines wind down via their cancelled
// context, so this is safe to call mid-test or after completion.
func (m *model) reset() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Progress{
		Phases:  make(chan Phase, 16),
		Samples: make(chan Sample, 256),
		Result:  make(chan Result, 1),
	}

	m.ctx = ctx
	m.cancel = cancel
	m.progress = p
	m.events = make(chan tea.Msg, 64)
	m.phase = PhaseInit
	m.testStart = time.Now()
	m.phaseStart = time.Time{}
	m.phaseDur = 0
	m.gotResult = false
	m.err = nil
	m.quitting = false
	m.serverName = ""
	m.result = Result{}
	m.dlDisplay, m.dlTarget = 0, 0
	m.ulDisplay, m.ulTarget = 0, 0
	m.pingDisp = 0
	m.dlGraph.clear()
	m.ulGraph.clear()

	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(m.theme.Highlight)

	launchTest(ctx, p, m.events)
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return tickMsg{} },
		listenCmd(m.events),
	)
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
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "r":
			return m, m.reset()
		case "c":
			m.unit = (m.unit + 1) % 4
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Keep graphs sized to the card's inner width (wider = more history
		// visible, so spikes are easier to read).
		inner := m.innerWidth(msg.Width)
		m.dlGraph.setWidth(inner)
		m.ulGraph.setWidth(inner)
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
		// Advance animations (lerp + graph growth) toward targets.
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
// smoothed value into the active phase's graph, so the chart builds up
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

// fmtSpeed formats a value in Mbps for the default (auto) unit: Gbps above
// 1000, otherwise Mbps.
func (m model) fmtSpeed(mbps float64) (string, string) {
	if mbps >= 1000 {
		return fmt.Sprintf("%5.2f", mbps/1000.0), "Gbps"
	}
	return fmt.Sprintf("%5.1f", mbps), "Mbps"
}

// formatValue formats a measured speed (Mbps) according to the current unit
// mode. Returns the numeric string (fixed width) and the unit suffix. The
// graph shape is unaffected — only the labels change.
func (m model) formatValue(mbps float64) (string, string) {
	switch m.unit {
	case unitKB:
		// bytes/sec / 1e3 = KB/s ; bytes/sec = Mbps * 125000
		kb := mbps * 125 // 1 Mbps = 125000 bytes/s = 125 KB/s
		return fmt.Sprintf("%7.1f", kb), "KB/s"
	case unitMB:
		mb := mbps * 0.125 // 1 Mbps = 125000 bytes/s = 0.125 MB/s
		return fmt.Sprintf("%7.2f", mb), "MB/s"
	case unitGB:
		gb := mbps * 0.000125 // 1 Mbps = 125000 bytes/s = 0.000125 GB/s
		return fmt.Sprintf("%7.3f", gb), "GB/s"
	default:
		return m.fmtSpeed(mbps)
	}
}

// formatPeak renders a measured speed (Mbps) under the current unit mode as a
// single "num unit" string for the peak line / summary.
func (m model) formatPeak(mbps float64) string {
	num, unit := m.formatValue(mbps)
	return strings.TrimSpace(num) + " " + unit
}

// --- View ----------------------------------------------------------------

func (m model) View() string {
	if m.quitting {
		return ""
	}

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
		Render("q quit · r reset · c units (" + m.unit.label() + ") · ? help")
	body.WriteString("\n\n")
	body.WriteString(center(hint, m.cardWidthFor()))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Padding(1, 2).
		Width(m.cardWidthFor()).
		Render(body.String())

	// Header (SPEED + tagline) sits above the card.
	header := m.renderHeader()
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

	if m.hasBg {
		// Paint the whole terminal area with the custom background, then
		// overlay the (transparent) centered stack on top of it.
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

// metricBlock renders one download or upload metric: a label + big number +
// unit on the first line, a vertical gradient graph beneath it, and a faint
// peak line + axis rule under the graph. Everything is left-aligned so the
// chart sits directly under its headline.
func (m model) metricBlock(label string, color lipgloss.AdaptiveColor, value float64, g *graph, peak float64, ph Phase) string {
	numStr, unit := m.formatValue(value)
	labelStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	numStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(7).Align(lipgloss.Right)
	unitStyle := lipgloss.NewStyle().Foreground(m.theme.Muted).Width(5)

	// Dim the metric if its phase hasn't started yet.
	if m.phase < ph && m.phase != PhaseDone {
		labelStyle = labelStyle.Faint(true)
		numStyle = numStyle.Faint(true)
	}

	head := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render(label),
		"  ",
		numStyle.Render(numStr),
		" ",
		unitStyle.Render(unit),
	)

	// Graph + axis rule + peak, only when the timed phase has been reached.
	var below string
	graphView := g.View()
	if graphView == "" {
		// Before any data: show an empty axis so the layout is stable.
		graphView = strings.Repeat(" ", g.width)
	}
	axis := lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", g.width))
	peakStr := ""
	if peak > 0 {
		peakStr = lipgloss.NewStyle().Foreground(m.theme.Muted).Render("peak " + m.formatPeak(peak))
	}
	below = graphView + "\n" + axis
	if peakStr != "" {
		below += "\n" + peakStr
	}

	return head + "\n" + below
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
	dl := lipgloss.NewStyle().Foreground(m.theme.Download).Bold(true).Render(m.formatPeak(m.result.DownloadMbps))
	ul := lipgloss.NewStyle().Foreground(m.theme.Upload).Bold(true).Render(m.formatPeak(m.result.UploadMbps))
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

// renderHeader draws the prominent title shown ABOVE the card: the word
// "SPEED" rendered as a large pixel block-art logo with beveled 3D edges, a
// layered green gradient (dark forest -> neon mint) and a drop shadow, for a
// clean modern network/performance look. The tagline underneath is preserved
// exactly as the soft-green subtitle.
func (m model) renderHeader() string {
	// 5-wide x 6-tall pixel font for the letters we need (S P E E D).
	// Source uses '#' as the "on" pixel; rendered as '█' below.
	glyphs := map[rune][]string{
		'S': {"#####", "#    ", "#    ", "#####", "    #", "#####"},
		'P': {"#####", "#   #", "#   #", "#####", "#    ", "#    "},
		'E': {"#####", "#    ", "#    ", "#### ", "#    ", "#####"},
		'D': {"#####", "#   #", "#   #", "#   #", "#   #", "#####"},
	}

	// Green gradient ramp: dark forest (left) -> neon mint (right).
	ramp := []lipgloss.Color{
		"#0b3d1e", "#11502a", "#1a7f37", "#2ea043",
		"#3fb950", "#56d364", "#7ee787", "#b9f6ca",
	}
	rampAt := func(i int) lipgloss.Color {
		if i < 0 {
			i = 0
		}
		if i >= len(ramp) {
			i = len(ramp) - 1
		}
		return ramp[i]
	}

	const (
		glyphW = 5
		glyphH = 6
		gap    = 1 // space between letters
		blk    = "█"
	)
	word := "SPEED"
	wordW := len(word)*glyphW + (len(word)-1)*gap // total face columns
	gridW := wordW + 1                            // +1 for the drop shadow offset
	gridH := glyphH + 1                           // +1 for the drop shadow offset

	// grid holds a pre-rendered (colored) cell; "" means empty.
	grid := make([][]string, gridH)
	for r := range grid {
		grid[r] = make([]string, gridW)
	}

	// Drop shadow layer (offset down-right by one cell).
	shadow := lipgloss.NewStyle().Foreground(lipgloss.Color("#04150b")).Render(blk)
	for li, r := range word {
		rows := glyphs[r]
		for gr := 0; gr < glyphH; gr++ {
			for gc := 0; gc < glyphW; gc++ {
				if rows[gr][gc] == '#' {
					grid[gr+1][li*(glyphW+gap)+gc+1] = shadow
				}
			}
		}
	}

	// Face layer with beveled edges + per-column gradient.
	for li, r := range word {
		rows := glyphs[r]
		for gr := 0; gr < glyphH; gr++ {
			for gc := 0; gc < glyphW; gc++ {
				if rows[gr][gc] != '#' {
					continue
				}
				absCol := li*(glyphW+gap) + gc
				baseIdx := (absCol * (len(ramp) - 1)) / wordW

				// Bevel: highlight on top/left edges, shadow on bottom/right.
				up := gr > 0 && rows[gr-1][gc] == '#'
				down := gr < glyphH-1 && rows[gr+1][gc] == '#'
				left := gc > 0 && rows[gr][gc-1] == '#'
				right := gc < glyphW-1 && rows[gr][gc+1] == '#'

				var c lipgloss.Color
				switch {
				case !up || !left: // raised top-left edge -> brighter
					c = rampAt(baseIdx + 2)
				case !down || !right: // recessed bottom-right edge -> darker
					c = rampAt(baseIdx - 2)
				default: // inner face
					c = rampAt(baseIdx)
				}
				grid[gr][absCol] = lipgloss.NewStyle().Foreground(c).Render(blk)
			}
		}
	}

	var rows []string
	for _, row := range grid {
		var b strings.Builder
		for _, cell := range row {
			if cell == "" {
				b.WriteString(" ")
			} else {
				b.WriteString(cell)
			}
		}
		rows = append(rows, b.String())
	}
	logo := lipgloss.JoinVertical(lipgloss.Left, rows...)

	// Subtitle preserved exactly.
	tagline := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56d364")).
		Render("Wonder how speedy your internet is?")

	return lipgloss.JoinVertical(lipgloss.Center, logo, tagline)
}

// renderHelp renders a centered help modal describing the live controls. It
// replaces the normal card view while shown (toggle with ?).
func (m model) renderHelp() string {
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted)
	key := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)

	lines := []string{
		key.Render("?") + "  " + muted.Render("toggle this help"),
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
