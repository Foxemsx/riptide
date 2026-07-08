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

	err error
}

// newCardState builds the shared state for one run: spinner, channels, context,
// and the two history graphs.
func newCardState(theme Theme) *cardState {
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
// It exits (and closes events) once the context is cancelled.
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
	}

	// Compute elapsed / progress for the timed phases.
	elapsed := time.Since(c.phaseStart)
	if elapsed < 0 {
		elapsed = 0
	}
	total := c.phaseDur
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
	timer := lipgloss.NewStyle().Foreground(c.theme.Muted).Render(fmt.Sprintf("%ds / %ds", secs, int(total.Seconds())))
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
// unit on the first line, a vertical gradient graph beneath it, and a faint
// peak line + axis rule under the graph. Everything is left-aligned so the
// chart sits directly under its headline.
func (c *cardState) metricBlock(label string, color lipgloss.AdaptiveColor, value float64, g *graph, peak float64, ph Phase) string {
	numStr, unit := c.formatValue(value)
	labelStyle := lipgloss.NewStyle().Foreground(color).Bold(true)
	numStyle := lipgloss.NewStyle().Foreground(color).Bold(true).Width(7).Align(lipgloss.Right)
	unitStyle := lipgloss.NewStyle().Foreground(c.theme.Muted).Width(5)

	// Dim the metric if its phase hasn't started yet.
	if c.phase < ph && c.phase != PhaseDone {
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
	graphView := g.View()
	if graphView == "" {
		// Before any data: show an empty axis so the layout is stable.
		graphView = strings.Repeat(" ", g.width)
	}
	axis := lipgloss.NewStyle().Foreground(c.theme.Border).Render(strings.Repeat("─", g.width))
	peakStr := ""
	if peak > 0 {
		peakStr = lipgloss.NewStyle().Foreground(c.theme.Muted).Render("peak " + c.formatPeak(peak))
	}
	below := graphView + "\n" + axis
	if peakStr != "" {
		below += "\n" + peakStr
	}

	return head + "\n" + below
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

// renderHeader draws the prominent title: the word "SPEED" as large pixel
// block-art with beveled 3D edges and a layered green gradient, plus the
// provided tagline beneath it. Pure presentation, no model state.
func renderHeader(tagline string) string {
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
	tag := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56d364")).
		Render(tagline)

	return lipgloss.JoinVertical(lipgloss.Center, logo, tag)
}
