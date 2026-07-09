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

// menuSelectMsg is emitted by the menu when the user picks a destination.
type menuSelectMsg struct{ screen screenID }

// backToMenuMsg is emitted by a sub-screen to return to the start menu.
type backToMenuMsg struct{}

// pingMsg carries a one-shot latency measurement for the monitor.
type pingMsg struct{ ms float64 }

func menuSelectCmd(s screenID) tea.Cmd { return func() tea.Msg { return menuSelectMsg{s} } }
func backToMenuCmd() tea.Cmd           { return func() tea.Msg { return backToMenuMsg{} } }

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
	graphHeight    = 9
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

// cardState holds every field and method shared between the Speed Test card and
// the Bandwidth Monitor card. Both sub-models embed *cardState so they get the
// same rendering primitives, graphs, theme, and animation state for free.
type cardState struct {
	theme    Theme
	progress *Progress
	ctx      context.Context
	cancel   context.CancelFunc
	events   chan tea.Msg // bridge from the background runner to Update
	spinner  spinner.Model
	width    int
	height   int

	// Live phase state.
	phase      Phase
	phaseStart time.Time     // when the current timed phase began
	phaseDur   time.Duration // duration of the current timed phase
	serverName string

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
	compact  bool

	err error
}

// newCardState builds the shared state for one run: spinner, channels, context,
// and the two history graphs.
func newCardState(theme Theme, compact bool) *cardState {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Highlight)

	p := &Progress{
		Phases:  make(chan Phase, 16),
		Samples: make(chan Sample, 256),
		Result:  make(chan Result, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())

	return &cardState{
		theme:    theme,
		compact:  compact,
		progress: p,
		ctx:      ctx,
		cancel:   cancel,
		events:   make(chan tea.Msg, 64),
		spinner:  s,
		phase:    PhaseInit,
		dlGraph:  newGraph(40, graphHeight, theme.GraphDownBottom, theme.GraphDownTop),
		ulGraph:  newGraph(40, graphHeight, theme.GraphUpBottom, theme.GraphUpTop),
	}
}

// bridgeLaunch starts the background transfer engine plus the channel bridge
// that fans its events into the tea event stream. Shared by the test and the
// monitor (they differ only in which Run* function they pass).
func bridgeLaunch(ctx context.Context, p *Progress, events chan tea.Msg, run func()) {
	go run()
	go runBridge(ctx, p, events)
}

// runBridge fans the background runner's channels into the tea event stream.
// On context cancel it still waits briefly for a final Result so the summary
// is not lost when the user aborts mid-phase.
func runBridge(ctx context.Context, p *Progress, events chan tea.Msg) {
	defer close(events)
	for {
		select {
		case <-ctx.Done():
			// Drain a late Result if the engine is about to emit one.
			select {
			case r, ok := <-p.Result:
				if ok {
					events <- resultMsg{r}
				}
			case <-time.After(800 * time.Millisecond):
			}
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

// tickCmd schedules the next refresh.
func (c *cardState) tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// --- Layout helpers ------------------------------------------------------

func (c *cardState) innerWidth(total int) int {
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

func (c *cardState) cardWidthFor() int {
	w := cardWidth
	maxW := c.width - 2
	if c.width > 0 && w > maxW {
		w = maxW
	}
	if w < 30 {
		w = 30
	}
	return w
}

// syncLayout sizes the history graphs to the card's content width.
// metricBlock draws a 1-column accent rail beside the plot, so the plot itself
// is contentWidth-1. Must be called on enter and on every terminal resize —
// WindowSizeMsg is handled by the app router and does not reach sub-models.
func (c *cardState) syncLayout() {
	// Inner content area inside border + padding.
	content := c.cardWidthFor() - 4
	if content < 12 {
		content = 12
	}
	plotW := content - 1 // leave room for the ▌ rail
	if plotW < 10 {
		plotW = 10
	}
	if c.dlGraph != nil {
		c.dlGraph.setWidth(plotW)
	}
	if c.ulGraph != nil {
		c.ulGraph.setWidth(plotW)
	}
}

// --- Formatting ----------------------------------------------------------

// fmtSpeed formats a value in Mbps for the default (auto) unit: Gbps above
// 1000, otherwise Mbps.
func (c *cardState) fmtSpeed(mbps float64) (string, string) {
	if mbps >= 1000 {
		return fmt.Sprintf("%5.2f", mbps/1000.0), "Gbps"
	}
	return fmt.Sprintf("%5.1f", mbps), "Mbps"
}

// formatValue formats a measured speed (Mbps) according to the current unit
// mode. Returns the numeric string (fixed width) and the unit suffix. The
// graph shape is unaffected — only the labels change.
func (c *cardState) formatValue(mbps float64) (string, string) {
	switch c.unit {
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
		return c.fmtSpeed(mbps)
	}
}

// formatPeak renders a measured speed (Mbps) under the current unit mode as a
// single "num unit" string for the peak line / summary.
func (c *cardState) formatPeak(mbps float64) string {
	num, unit := c.formatValue(mbps)
	return strings.TrimSpace(num) + " " + unit
}

// --- View ----------------------------------------------------------------

// statusLine renders the current phase with a spinner, plus a live timer /
// progress bar for the timed download and upload phases so it's obvious the
// test runs for a fixed duration (not instant).
func (c *cardState) statusLine() string {
	var label string
	var color lipgloss.AdaptiveColor
	switch c.phase {
	case PhaseFinding, PhaseInit:
		return center(c.spinner.View()+"  "+lipgloss.NewStyle().Foreground(c.theme.Muted).Render("finding servers…"), c.cardWidthFor())
	case PhaseConnected:
		who := "server"
		if c.serverName != "" {
			who = c.serverName
		}
		return center(lipgloss.NewStyle().Foreground(c.theme.Highlight).Render("✓ connected to "+who), c.cardWidthFor())
	case PhaseDownload:
		label, color = "measuring download", c.theme.Download
	case PhaseUpload:
		label, color = "measuring upload", c.theme.Upload
	case PhaseLatency:
		label, color = "measuring latency", c.theme.Latency
	case PhaseDone:
		if c.err != nil {
			return center(lipgloss.NewStyle().Foreground(c.theme.Upload).Render("✕ finished with errors"), c.cardWidthFor())
		}
		return center(lipgloss.NewStyle().Foreground(c.theme.Highlight).Render("✓ complete"), c.cardWidthFor())
	default:
		return ""
	}

	// Timed phases get a live countdown + progress bar.
	elapsed := time.Since(c.phaseStart)
	total := c.phaseDur
	if total <= 0 {
		total = defaultDuration
	}
	frac := elapsed.Seconds() / total.Seconds()
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	remain := total - elapsed
	if remain < 0 {
		remain = 0
	}
	labelStyled := lipgloss.NewStyle().Foreground(color).Render(label)
	timer := lipgloss.NewStyle().Foreground(c.theme.Muted).Render(fmt.Sprintf("%4.1fs", remain.Seconds()))
	bar := c.progressBar(frac, color, 16)
	line := lipgloss.JoinHorizontal(lipgloss.Left, labelStyled, "   ", timer, "   ", bar)
	return center(line, c.cardWidthFor())
}

// progressBar draws a compact inline bar for the timed phases.
func (c *cardState) progressBar(frac float64, color lipgloss.AdaptiveColor, width int) string {
	if width < 4 {
		width = 4
	}
	filled := int(frac * float64(width))
	if filled > width {
		filled = width
	}
	fill := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	empty := lipgloss.NewStyle().Foreground(c.theme.Border).Render(strings.Repeat("░", width-filled))
	return fill + empty
}

// metricBlock renders one download or upload metric: a label + big number +
// unit on the first line, a framed high-res graph beneath it, and peak info
// under the axis. Left-aligned so the chart sits under its headline.
func (c *cardState) metricBlock(label string, color lipgloss.AdaptiveColor, value float64, g *graph, peak float64, ph Phase) string {
	numStr, unit := c.formatValue(value)
	labelStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	numStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(7).Align(lipgloss.Right)
	unitStyle := lipgloss.NewStyle().Foreground(c.theme.Muted).Width(5)
	muted := lipgloss.NewStyle().Foreground(c.theme.Muted)
	border := lipgloss.NewStyle().Foreground(c.theme.Border)

	// Dim the metric if its phase hasn't started yet.
	if c.phase < ph && c.phase != PhaseDone {
		labelStyle = labelStyle.Faint(true)
		numStyle = numStyle.Faint(true)
	}

	// Live value on the left; peak scale on the right when known.
	// Chart frame is rail (1) + plot (g.width).
	chartW := g.width + 1
	headLeft := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render(label),
		"  ",
		numStyle.Render(numStr),
		" ",
		unitStyle.Render(unit),
	)
	head := headLeft
	if peak > 0 {
		peakHead := muted.Render("peak " + c.formatPeak(peak))
		pad := chartW - lipgloss.Width(headLeft) - lipgloss.Width(peakHead)
		if pad < 1 {
			pad = 1
		}
		head = headLeft + strings.Repeat(" ", pad) + peakHead
	}

	graphView := g.View()
	if graphView == "" {
		graphView = strings.Repeat(" ", g.width)
	}

	// Left rail in the metric accent — frames the chart like a dashboard panel.
	rail := lipgloss.NewStyle().Foreground(color).Render("▌")
	corner := lipgloss.NewStyle().Foreground(color).Render("└")
	framed := make([]string, 0, g.height+1)
	for _, line := range strings.Split(graphView, "\n") {
		framed = append(framed, rail+line)
	}
	framed = append(framed, corner+border.Render(strings.Repeat("─", g.width)))

	return head + "\n" + strings.Join(framed, "\n")
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

// logoSrc is "RIPTIDE" in FIGlet ANSI Shadow — same technique as flow's logo
// (https://github.com/programmersd21/flow). The 3D/outline look is baked into
// the box-drawing characters; we only color each row with a vertical gradient.
var logoSrc = []string{
	"██████╗ ██╗██████╗ ████████╗██╗██████╗ ███████╗",
	"██╔══██╗██║██╔══██╗╚══██╔══╝██║██╔══██╗██╔════╝",
	"██████╔╝██║██████╔╝   ██║   ██║██║  ██║█████╗  ",
	"██╔══██╗██║██╔═══╝    ██║   ██║██║  ██║██╔══╝  ",
	"██║  ██║██║██║        ██║   ██║██████╔╝███████╗",
	"╚═╝  ╚═╝╚═╝╚═╝        ╚═╝   ╚═╝╚═════╝ ╚══════╝",
}

// logoStops is a 4-stop vertical water gradient (deep ocean → teal → cyan → ice).
var logoStops = [4][3]uint8{
	{0x0e, 0x4d, 0x64}, // deep ocean
	{0x08, 0x83, 0x95}, // teal
	{0x14, 0xc4, 0xd4}, // bright cyan
	{0x9a, 0xf5, 0xf8}, // ice / foam
}

// renderHeader draws the RIPTIDE wordmark the same way flow draws FLOW:
// pre-baked ANSI Shadow art + per-row 4-stop gradient + muted tagline.
func renderHeader(tagline string) string {
	n := len(logoSrc)
	lines := make([]string, n)
	for i, line := range logoSrc {
		rowT := 0.0
		if n > 1 {
			rowT = float64(i) / float64(n-1)
		}
		r, g, b := logoGradient(rowT)
		color := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
		lines[i] = lipgloss.NewStyle().Foreground(color).Bold(true).Render(line)
	}
	logo := lipgloss.JoinVertical(lipgloss.Left, lines...)

	tag := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#94a3b8")).
		Render(tagline)

	return lipgloss.JoinVertical(lipgloss.Center, logo, "", tag)
}

// logoGradient samples the 4-stop logo palette at position t in [0,1]
// (top → bottom), same approach as flow's fourStopLogoGradient.
func logoGradient(t float64) (uint8, uint8, uint8) {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	segment := t * 3.0
	idx := int(segment)
	if idx >= 3 {
		idx = 2
		segment = 3.0
	}
	u := segment - float64(idx)
	a, b := logoStops[idx], logoStops[idx+1]
	return lerpU8(a[0], b[0], u), lerpU8(a[1], b[1], u), lerpU8(a[2], b[2], u)
}

func lerpU8(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5)
}

// renderCompactHeader draws a minimal header: just the tagline text without the
// large pixel-art logo. Used when --compact mode is active or toggled with 't'.
func renderCompactHeader(tagline string) string {
	tag := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56d364")).
		Bold(true).
		Render(tagline)
	return tag
}

// helpBinding is one key → action row for the help modal.
type helpBinding struct {
	keys   string
	action string
}

// renderHelpPanel draws a polished centered help modal shared by speed test
// and bandwidth screens. Every style carries the panel fill so nested SGR
// resets cannot punch holes in the background.
func renderHelpPanel(theme Theme, title string, bindings []helpBinding, width, height int) string {
	const (
		innerW = 42 // content width inside padding
		keyCol = 12
	)
	bg := theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")

	// Base styles always re-apply the panel fill.
	plain := lipgloss.NewStyle().Background(bg)
	fg := func(c lipgloss.TerminalColor, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c).Background(bg)
		if bold {
			s = s.Bold(true)
		}
		return s
	}
	// Full-width line with continuous background.
	line := func(parts ...string) string {
		return lipgloss.NewStyle().
			Width(innerW).
			Background(bg).
			Inline(true).
			Render(strings.Join(parts, ""))
	}

	titleChip := lipgloss.NewStyle().
		Foreground(ink).
		Background(theme.Highlight).
		Bold(true).
		Padding(0, 1).
		Render(title)

	keyChip := func(keys string) string {
		return lipgloss.NewStyle().
			Foreground(ink).
			Background(theme.Download).
			Bold(true).
			Padding(0, 1).
			Render(keys)
	}

	row := func(keys, action string) string {
		chip := keyChip(keys)
		pad := keyCol - lipgloss.Width(chip)
		if pad < 1 {
			pad = 1
		}
		return line(
			chip,
			plain.Render(strings.Repeat(" ", pad)),
			fg(theme.Foreground, false).Render(action),
		)
	}

	// Split navigation (back/quit/help) from the rest of the controls.
	var nav, controls []helpBinding
	for _, b := range bindings {
		k := strings.ToLower(b.keys)
		if strings.Contains(k, "esc") || k == "q" || k == "?" || k == "m" {
			nav = append(nav, b)
		} else {
			controls = append(controls, b)
		}
	}

	var body []string
	body = append(body, line(titleChip))
	body = append(body, line(""))
	body = append(body, line(fg(theme.Download, true).Render("Navigation")))
	body = append(body, line(fg(theme.Border, false).Render(strings.Repeat("─", 28))))
	for _, b := range nav {
		body = append(body, row(b.keys, b.action))
	}
	if len(controls) > 0 {
		body = append(body, line(""))
		body = append(body, line(fg(theme.Download, true).Render("Controls")))
		body = append(body, line(fg(theme.Border, false).Render(strings.Repeat("─", 28))))
		for _, b := range controls {
			body = append(body, row(b.keys, b.action))
		}
	}
	body = append(body, line(""))
	body = append(body, line(fg(theme.Muted, false).Render("Press  ?  or  esc  to close this help")))
	body = append(body, line(fg(theme.Muted, false).Render("Press  esc  or  m  to return to the main menu")))

	content := strings.Join(body, "\n")
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Highlight).
		Background(bg).
		Padding(1, 2).
		// Width includes padding so the fill matches the border box.
		Width(innerW + 4).
		Render(content)

	return paintScreen(theme, width, height, panel)
}
