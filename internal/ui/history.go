package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Foxemsx/riptide/internal/db"
	apptheme "github.com/Foxemsx/riptide/internal/theme"
)

// historyLimit is how many recent runs we always show.
const historyLimit = 10

// historySideMinWidth: terminal must be at least this wide to place history
// beside the speed-test card instead of below it.
const historySideMinWidth = 110

// historyBlock renders a polished "Recent tests" card for the speed-test view.
// Speeds are converted with the same unitMode as the main card (toggle with c).
func historyBlock(theme apptheme.Theme, runs []db.TestRun, width int, unit unitMode, hint string) string {
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
		Background(theme.AccentLat).
		Bold(true).
		Padding(0, 1).
		Render("Recent tests")

	unitChip := lipgloss.NewStyle().
		Foreground(ink).
		Background(theme.AccentDL).
		Bold(true).
		Padding(0, 1).
		Render(unit.label())

	var body []string
	body = append(body, line(
		titleChip,
		plain.Render(" "),
		unitChip,
		plain.Render(" "),
		fg(theme.Muted, false).Render("latest 10"),
	))
	body = append(body, line(fg(theme.Border, false).Render(strings.Repeat("─", min(innerW, 48)))))

	// Column widths adapt to card width.
	nameW := 14
	rateW := 9
	if innerW >= 52 {
		nameW = 16
		rateW = 10
	}
	if innerW < 40 {
		nameW = 10
		rateW = 8
	}
	whenW := 11
	pingW := 6

	if len(runs) == 0 {
		body = append(body, line(fg(theme.Muted, false).Render("No runs yet — finish a test")))
		body = append(body, line(fg(theme.Muted, false).Render("or press s to name one.")))
	} else {
		// Header with unit-aware rate labels
		ulab := unit.short()
		body = append(body, line(
			fg(theme.Muted, true).Render(padRight("when", whenW)),
			fg(theme.Muted, true).Render(padRight("name", nameW)),
			fg(theme.Muted, true).Render(padLeft("↓"+ulab, rateW)),
			fg(theme.Muted, true).Render(padLeft("↑"+ulab, rateW)),
			fg(theme.Muted, true).Render(padLeft("ping", pingW)),
		))
		for i, r := range runs {
			if i >= historyLimit {
				break
			}
			when := padRight(db.FormatWhen(r.CreatedAt), whenW)
			name := padRight(truncate(r.Name, nameW-1), nameW)
			dl := padLeft(fmtSpeedUnit(r.DownloadMbps, unit), rateW)
			ul := padLeft(fmtSpeedUnit(r.UploadMbps, unit), rateW)
			pg := padLeft(fmt.Sprintf("%.0f", r.PingMs), pingW)
			nameStyle := fg(theme.Foreground, i == 0)
			body = append(body, line(
				fg(theme.Muted, false).Render(when),
				nameStyle.Render(name),
				fg(theme.Download, i == 0).Render(dl),
				fg(theme.Upload, i == 0).Render(ul),
				fg(theme.Latency, false).Render(pg),
			))
		}
	}

	if hint != "" {
		body = append(body, line(""))
		body = append(body, line(fg(theme.Muted, false).Render(hint)))
	} else {
		body = append(body, line(""))
		body = append(body, line(fg(theme.Muted, false).Render("c units  ·  s save")))
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

// averageBlock renders a polished "Your usual average" card for the speed-test
// view, summarizing the mean download / upload / ping across all saved runs.
func averageBlock(theme apptheme.Theme, avg db.Averages, unit unitMode, width int) string {
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
		Background(theme.AccentHL).
		Bold(true).
		Padding(0, 1).
		Render("Your usual average")

	var body []string
	body = append(body, line(
		titleChip,
		plain.Render(" "),
		fg(theme.Muted, false).Render("all saved runs"),
	))
	body = append(body, line(fg(theme.Border, false).Render(strings.Repeat("─", min(innerW, 48)))))

	if avg.Count == 0 {
		body = append(body, line(fg(theme.Muted, false).Render("No runs yet — finish a test")))
		body = append(body, line(fg(theme.Muted, false).Render("to see your average.")))
	} else {
		ulab := unit.short()
		dl := fmtSpeedUnit(avg.DownloadMbps, unit)
		ul := fmtSpeedUnit(avg.UploadMbps, unit)
		pg := fmt.Sprintf("%.0f ms", avg.PingMs)
		val := lipgloss.JoinHorizontal(lipgloss.Center,
			fg(theme.Download, true).Render("↓ "+dl),
			plain.Render("   "),
			fg(theme.Upload, true).Render("↑ "+ul),
			plain.Render("   "),
			fg(theme.Latency, true).Render("◷ "+pg),
		)
		body = append(body, line(val))
		body = append(body, line(fg(theme.Muted, false).Render(
			fmt.Sprintf("over %d run%s · in %s", avg.Count, plural(avg.Count), ulab),
		)))
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

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// qualityBad is the accent for a failing verdict (no adaptive red in the theme).
var qualityBad = lipgloss.Color("#ff6b6b")

// qualityBlock grades your connection against common real-world activities and
// shows whether it is Perfect / Good / Fair / Bad for each. Grades use widely
// published bandwidth needs (Netflix 4K ≈ 25 Mbps, 2K ≈ 15 Mbps, 1080p ≈ 8 Mbps,
// HD video calls ≈ 4 Mbps up+down) and ping for gaming (≤20ms great, ≤50ms ok).
func qualityBlock(theme apptheme.Theme, dl, ul, ping float64, hasData bool, width int) string {
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
		Background(theme.AccentLat).
		Bold(true).
		Padding(0, 1).
		Render("Good for")

	var body []string
	body = append(body, line(
		titleChip,
		plain.Render(" "),
		fg(theme.Muted, false).Render("your average connection"),
	))
	body = append(body, line(fg(theme.Border, false).Render(strings.Repeat("─", min(innerW, 48)))))

	if !hasData {
		body = append(body, line(fg(theme.Muted, false).Render("Finish a test to see what")))
		body = append(body, line(fg(theme.Muted, false).Render("your connection handles.")))
		content := strings.Join(body, "\n")
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Background(bg).
			Padding(0, 1).
			Width(width).
			Render(content)
	}

	// Subtitle with the basis values.
	sub := fmt.Sprintf("↓ %.0f · ↑ %.0f · %.0fms", dl, ul, ping)
	body = append(body, line(fg(theme.Muted, false).Render(sub)))
	body = append(body, line(""))

	nameW := innerW - 12
	if nameW < 10 {
		nameW = 10
	}
	for _, a := range []string{"gaming", "4k", "2k", "1080p", "calls"} {
		label, color := gradeActivity(theme, dl, ul, ping, a)
		sym := "✗"
		switch label {
		case "Perfect", "Good":
			sym = "✓"
		case "Fair":
			sym = "~"
		}
		disp := activityLabel(a)
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			fg(color, true).Render(sym+" "),
			fg(theme.Foreground, false).Render(padRight(disp, nameW)),
			fg(color, true).Render(label),
		)
		body = append(body, line(row))
	}

	// Tangible time estimates from your actual speeds.
	body = append(body, line(""))
	body = append(body, line(fg(theme.Muted, true).Render("at your speed")))
	labelW := innerW - 14
	if labelW < 10 {
		labelW = 10
	}
	type est struct {
		dir   string
		label string
		gb    float64
		speed float64
		col   lipgloss.TerminalColor
	}
	for _, e := range []est{
		{"↓", "100 GB game", 100, dl, theme.Download},
		{"↓", "25 GB 4K movie", 25, dl, theme.Download},
		{"↑", "1 GB file", 1, ul, theme.Upload},
		{"↑", "10 GB backup", 10, ul, theme.Upload},
	} {
		if e.speed <= 0 {
			continue
		}
		row := lipgloss.JoinHorizontal(lipgloss.Left,
			fg(e.col, true).Render(e.dir+" "),
			fg(theme.Foreground, false).Render(padRight(e.label, labelW)),
			fg(theme.Muted, false).Render("≈ "+estimateDuration(e.gb, e.speed)),
		)
		body = append(body, line(row))
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

// estimateDuration returns a human transfer time for gb at the given Mbps.
func estimateDuration(gb, mbps float64) string {
	if mbps <= 0 {
		return "—"
	}
	secs := (gb * 1e9) / (mbps * 125000)
	if secs < 1 {
		return "<1s"
	}
	if secs < 60 {
		return fmt.Sprintf("%.0fs", secs)
	}
	if secs < 3600 {
		m := int(secs / 60)
		s := int(secs) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm %ds", m, s)
	}
	if secs < 86400 {
		h := int(secs / 3600)
		m := int(secs/60) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	d := int(secs / 86400)
	h := int(secs/3600) % 24
	if h == 0 {
		return fmt.Sprintf("%dd", d)
	}
	return fmt.Sprintf("%dd %dh", d, h)
}

func activityLabel(a string) string {
	switch a {
	case "gaming":
		return "Gaming"
	case "4k":
		return "4K video"
	case "2k":
		return "2K video"
	case "1080p":
		return "1080p video"
	case "calls":
		return "Video calls"
	}
	return a
}

// gradeActivity returns a verdict word and its accent color for one activity.
func gradeActivity(theme apptheme.Theme, dl, ul, ping float64, kind string) (string, lipgloss.TerminalColor) {
	switch kind {
	case "gaming":
		// Latency dominates; a little bandwidth is enough.
		switch {
		case ping <= 20 && dl >= 10:
			return "Perfect", theme.Highlight
		case ping <= 50 && dl >= 10:
			return "Good", theme.Download
		case ping <= 100:
			return "Fair", theme.Upload
		default:
			return "Bad", qualityBad
		}
	case "4k":
		// Netflix recommends 25 Mbps for UHD 4K.
		switch {
		case dl >= 50:
			return "Perfect", theme.Highlight
		case dl >= 25:
			return "Good", theme.Download
		case dl >= 15:
			return "Fair", theme.Upload
		default:
			return "Bad", qualityBad
		}
	case "2k":
		// ~15 Mbps for 1440p.
		switch {
		case dl >= 30:
			return "Perfect", theme.Highlight
		case dl >= 15:
			return "Good", theme.Download
		case dl >= 8:
			return "Fair", theme.Upload
		default:
			return "Bad", qualityBad
		}
	case "1080p":
		// ~8 Mbps for 1080p.
		switch {
		case dl >= 15:
			return "Perfect", theme.Highlight
		case dl >= 8:
			return "Good", theme.Download
		case dl >= 4:
			return "Fair", theme.Upload
		default:
			return "Bad", qualityBad
		}
	case "calls":
		// ~4 Mbps up+down for HD video calls.
		switch {
		case ul >= 10 && dl >= 10:
			return "Perfect", theme.Highlight
		case ul >= 4 && dl >= 4:
			return "Good", theme.Download
		case ul >= 2 && dl >= 2:
			return "Fair", theme.Upload
		default:
			return "Bad", qualityBad
		}
	}
	return "—", theme.Muted
}

// short is a compact unit label for tight column headers.
func (u unitMode) short() string {
	switch u {
	case unitKB:
		return "KB"
	case unitMB:
		return "MB"
	case unitGB:
		return "GB"
	default:
		return "Mb"
	}
}

// fmtSpeedUnit formats a Mbps value under the active unitMode for history rows.
func fmtSpeedUnit(mbps float64, u unitMode) string {
	if mbps <= 0 {
		return "—"
	}
	switch u {
	case unitKB:
		// 1 Mbps = 125 KB/s
		v := mbps * 125
		if v >= 10000 {
			return fmt.Sprintf("%.0fk", v/1000)
		}
		if v >= 100 {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%.0f", v)
	case unitMB:
		// 1 Mbps = 0.125 MB/s
		v := mbps * 0.125
		if v >= 100 {
			return fmt.Sprintf("%.0f", v)
		}
		if v >= 10 {
			return fmt.Sprintf("%.1f", v)
		}
		return fmt.Sprintf("%.2f", v)
	case unitGB:
		v := mbps * 0.000125
		if v >= 1 {
			return fmt.Sprintf("%.2f", v)
		}
		return fmt.Sprintf("%.3f", v)
	default:
		// Mbps / Gbps auto
		if mbps >= 1000 {
			return fmt.Sprintf("%.1fG", mbps/1000)
		}
		if mbps >= 100 {
			return fmt.Sprintf("%.0f", mbps)
		}
		return fmt.Sprintf("%.1f", mbps)
	}
}

func fmtMbpsShort(mbps float64) string {
	return fmtSpeedUnit(mbps, unitAuto)
}

func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return truncate(s, w)
	}
	return s + strings.Repeat(" ", w-vis)
}

func padLeft(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return truncate(s, w)
	}
	return strings.Repeat(" ", w-vis) + s
}

// --- save name modal -----------------------------------------------------

// savePromptModel is a small centered modal for naming a run before save.
type savePromptModel struct {
	theme                        apptheme.Theme
	width                        int
	height                       int
	input                        textinput.Model
	kind                         string // speed
	dl, ul, ping, dlPeak, ulPeak float64
	server                       string
	active                       bool
}

func newSavePrompt(theme apptheme.Theme, kind string) savePromptModel {
	ti := textinput.New()
	ti.Placeholder = "Name this run…"
	ti.CharLimit = 48
	ti.Width = 40
	ti.Prompt = "› "
	return savePromptModel{theme: theme, input: ti, kind: kind}
}

func (s *savePromptModel) styleInput() {
	bg := s.theme.MenuIdleFill
	s.input.PromptStyle = lipgloss.NewStyle().Foreground(s.theme.AccentHL).Background(bg).Bold(true)
	s.input.TextStyle = lipgloss.NewStyle().Foreground(s.theme.Foreground).Background(bg)
	s.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(s.theme.Muted).Background(bg)
	s.input.Cursor.Style = lipgloss.NewStyle().Foreground(s.theme.Download).Background(bg)
	s.input.Cursor.TextStyle = lipgloss.NewStyle().Foreground(s.theme.Foreground).Background(bg)
}

func (s *savePromptModel) open(dl, ul, ping, dlPeak, ulPeak float64, server string) {
	s.dl, s.ul, s.ping = dl, ul, ping
	s.dlPeak, s.ulPeak = dlPeak, ulPeak
	s.server = server
	s.styleInput()
	s.input.SetValue(db.AutoName(s.kind, time.Now()))
	s.input.CursorEnd()
	s.input.Focus()
	s.active = true
}

func (s *savePromptModel) close() {
	s.active = false
	s.input.Blur()
}

// saveRunMsg is emitted when the user confirms a named save.
type saveRunMsg struct {
	run db.TestRun
}

func (s *savePromptModel) Update(msg tea.Msg) tea.Cmd {
	if !s.active {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			s.close()
			return nil
		case "enter":
			name := strings.TrimSpace(s.input.Value())
			if name == "" {
				name = db.AutoName(s.kind, time.Now())
			}
			run := db.TestRun{
				Name:         name,
				Kind:         s.kind,
				DownloadMbps: s.dl,
				UploadMbps:   s.ul,
				PingMs:       s.ping,
				DownloadPeak: s.dlPeak,
				UploadPeak:   s.ulPeak,
				Server:       s.server,
				CreatedAt:    time.Now(),
			}
			s.close()
			return func() tea.Msg { return saveRunMsg{run: run} }
		}
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return cmd
}

func (s *savePromptModel) View() string {
	if !s.active {
		return ""
	}
	s.styleInput()

	bg := s.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	const innerW = 44

	line := func(parts ...string) string {
		return lipgloss.NewStyle().
			Width(innerW).
			Background(bg).
			Inline(true).
			Render(strings.Join(parts, ""))
	}
	fg := func(c lipgloss.TerminalColor, bold bool) lipgloss.Style {
		st := lipgloss.NewStyle().Foreground(c).Background(bg)
		if bold {
			st = st.Bold(true)
		}
		return st
	}

	titleChip := lipgloss.NewStyle().
		Foreground(ink).Background(s.theme.AccentHL).Bold(true).Padding(0, 1).
		Render("Save test run")

	s.input.Width = innerW - 2
	if s.input.Width < 8 {
		s.input.Width = 8
	}
	inputView := lipgloss.NewStyle().Background(bg).Render(s.input.View())
	inputLine := lipgloss.PlaceHorizontal(
		innerW, lipgloss.Left, inputView,
		lipgloss.WithWhitespaceBackground(bg),
	)

	body := strings.Join([]string{
		line(titleChip),
		line(""),
		line(fg(s.theme.Muted, false).Render("Name it, then enter to store · esc to cancel")),
		line(""),
		line(
			fg(s.theme.Download, true).Render(fmt.Sprintf("↓ %s", fmtMbpsShort(s.dl))),
			fg(s.theme.Muted, false).Render("   "),
			fg(s.theme.Upload, true).Render(fmt.Sprintf("↑ %s", fmtMbpsShort(s.ul))),
			fg(s.theme.Muted, false).Render("   "),
			fg(s.theme.Latency, true).Render(fmt.Sprintf("◷ %.0f ms", s.ping)),
		),
		line(""),
		inputLine,
		line(""),
		line(fg(s.theme.Muted, false).Render("enter save  ·  esc cancel")),
	}, "\n")

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.theme.Highlight).
		Background(bg).
		Padding(1, 2).
		Width(innerW + 4).
		Render(body)

	return apptheme.PaintScreen(s.theme, s.width, s.height, panel)
}
