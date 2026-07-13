package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	apptheme "github.com/Foxemsx/riptide/internal/theme"
	"github.com/Foxemsx/riptide/internal/update"
)

// Layout thresholds for a responsive menu.
const (
	horizontalThreshold = 100 // below this → vertical stack
	gridThreshold       = 88  // wide enough for 2×2 grid
	menuTickInterval    = 100 * time.Millisecond
)

// screenID identifies which destination the menu routes to.
type screenID int

const (
	screenMenu screenID = iota
	screenTest
	screenMonitor
	screenSettings
	screenExit
)

// menuItem is one selectable box in the startup menu.
type menuItem struct {
	title    string
	subtitle string
	screen   screenID
	hotkey   string
	features []string
	badge    string
}

// updateCheckMsg carries the background GitHub release check.
type updateCheckMsg struct{ result update.Result }

// menuModel is the startup screen.
type menuModel struct {
	theme   apptheme.Theme
	compact bool
	width   int
	height  int
	cursor  int
	hovered int
	pulse   float64
	spinner spinner.Model
	items   []menuItem

	version      string
	updateStatus updateStatus
	updateRes    update.Result
	chipHover    bool
}

type updateStatus int

const (
	updateChecking updateStatus = iota
	updateReady
	updateFailed
)

func newMenuModel(theme apptheme.Theme, compact bool, version string) *menuModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Highlight)
	if version == "" {
		version = "dev"
	}
	return &menuModel{
		theme:        theme,
		compact:      compact,
		cursor:       0,
		hovered:      -1,
		spinner:      s,
		version:      version,
		updateStatus: updateChecking,
		updateRes: update.Result{
			Current: version,
			OpenURL: update.RepoURL,
		},
		items: []menuItem{
			{
				title: "Speed Test", subtitle: "one-shot DL · UL · ping", screen: screenTest, hotkey: "1",
				features: []string{"Download + upload + latency", "~10s timed phases", "Save & compare runs"},
				badge:    "ONE-SHOT",
			},
			{
				title: "Bandwidth", subtitle: "live monitor · real traffic", screen: screenMonitor, hotkey: "2",
				features: []string{"Real PC throughput", "Session peaks", "Zero generated traffic"},
				badge:    "LIVE",
			},
			{
				title: "Settings", subtitle: "themes · history · install", screen: screenSettings, hotkey: "3",
				features: []string{"11 color themes", "Searchable settings", "Database & uninstall"},
				badge:    "TUNE",
			},
			{
				title: "Exit", subtitle: "quit riptide cleanly", screen: screenExit, hotkey: "4",
				features: []string{"Cancel any running test", "Clean shutdown", "See you next wave"},
				badge:    "",
			},
		},
	}
}

func (m *menuModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tickCmd(), m.checkUpdateCmd())
}

func (m *menuModel) checkUpdateCmd() tea.Cmd {
	ver := m.version
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		return updateCheckMsg{result: update.Check(ctx, ver)}
	}
}

func (m *menuModel) tickCmd() tea.Cmd {
	return tea.Tick(menuTickInterval, func(time.Time) tea.Msg { return menuTickMsg{} })
}

type menuTickMsg struct{}

func (m *menuModel) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return tea.Quit, false
		case "left", "h":
			m.move(-1)
			return nil, false
		case "right", "l":
			m.move(1)
			return nil, false
		case "up", "k":
			m.moveUp()
			return nil, false
		case "down", "j":
			m.moveDown()
			return nil, false
		case "1", "2", "3", "4":
			for i, it := range m.items {
				if it.hotkey == msg.String() {
					m.cursor = i
					return m.selectCurrent(), false
				}
			}
		case "enter", " ":
			return m.selectCurrent(), false
		case "g":
			return m.openUpdateLink(), false
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return nil, false
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return cmd, false
	case menuTickMsg:
		m.pulse = m.pulse + 0.08
		if m.pulse > 1 {
			m.pulse = 0
		}
		return m.tickCmd(), false
	case updateCheckMsg:
		m.updateRes = msg.result
		if msg.result.Err != nil && msg.result.Latest == "" {
			m.updateStatus = updateFailed
		} else {
			m.updateStatus = updateReady
		}
		if m.updateRes.OpenURL == "" {
			m.updateRes.OpenURL = update.RepoURL
		}
		return nil, false
	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionMotion:
			chip := m.updateChipRect()
			m.chipHover = pointInRect(msg.X, msg.Y, chip)
			hit := -1
			for i, box := range m.boxRects() {
				if msg.X >= box.x && msg.X < box.x+box.w &&
					msg.Y >= box.y && msg.Y < box.y+box.h {
					hit = i
					break
				}
			}
			if hit != m.hovered {
				m.hovered = hit
				return nil, false
			}
		case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
			if pointInRect(msg.X, msg.Y, m.updateChipRect()) {
				return m.openUpdateLink(), false
			}
			for i, box := range m.boxRects() {
				if msg.X >= box.x && msg.X < box.x+box.w &&
					msg.Y >= box.y && msg.Y < box.y+box.h {
					m.cursor = i
					m.hovered = -1
					return m.selectCurrent(), false
				}
			}
		}
	}
	return nil, false
}

func pointInRect(x, y int, r boxRect) bool {
	if r.w <= 0 || r.h <= 0 {
		return false
	}
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

func (m *menuModel) openUpdateLink() tea.Cmd {
	url := m.updateRes.OpenURL
	if url == "" {
		url = update.RepoURL
	}
	u := url
	return func() tea.Msg {
		_ = openURL(u)
		return nil
	}
}

func (m *menuModel) move(delta int) {
	m.cursor = (m.cursor + delta + len(m.items)) % len(m.items)
	m.hovered = -1
}

func (m *menuModel) moveUp() {
	mode, _, _, _, _, _ := m.computeLayout()
	if mode == "grid" {
		if m.cursor >= 2 {
			m.cursor -= 2
		} else {
			m.cursor += 2
			if m.cursor >= len(m.items) {
				m.cursor = len(m.items) - 1
			}
		}
	} else {
		m.move(-1)
	}
	m.hovered = -1
}

func (m *menuModel) moveDown() {
	mode, _, _, _, _, _ := m.computeLayout()
	if mode == "grid" {
		if m.cursor < 2 {
			m.cursor += 2
			if m.cursor >= len(m.items) {
				m.cursor = len(m.items) - 1
			}
		} else {
			m.cursor -= 2
		}
	} else {
		m.move(1)
	}
	m.hovered = -1
}

func (m *menuModel) selectCurrent() tea.Cmd {
	switch m.items[m.cursor].screen {
	case screenTest:
		return menuSelectCmd(screenTest)
	case screenMonitor:
		return menuSelectCmd(screenMonitor)
	case screenSettings:
		return menuSelectCmd(screenSettings)
	default:
		return tea.Quit
	}
}

type boxRect struct{ x, y, w, h int }

func (m *menuModel) headerHeight() int {
	if m.compact {
		return 4
	}
	return 10
}

// layoutHeight is the vertical space used for the centered menu (excludes update chip strip).
func (m *menuModel) layoutHeight() int {
	h := m.height
	if h <= 0 {
		h = 30
	}
	w := m.width
	if w <= 0 {
		w = 100
	}
	if w < 56 {
		return h
	}
	// Reserve room for chip + spacer when the chip will be drawn.
	chipH := 5 // border + 3 lines + padding — fixed so layout is stable while checking
	footerH := chipH + 1
	if h-footerH >= 12 {
		return h - footerH
	}
	return h
}

func (m *menuModel) computeLayout() (mode string, boxW, boxH, startY, startX int, gap int) {
	w, h := m.width, m.layoutHeight()
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	gap = 2
	boxH = 14 // room for badge + bottom pad so labels stay inside the fill
	num := len(m.items)

	if num == 4 && w >= gridThreshold {
		mode = "grid"
		boxW = min((w-8-gap)/2, 34)
		if boxW < 22 {
			boxW = 22
		}
		totalW := boxW*2 + gap
		totalH := m.headerHeight() + 1 + boxH*2 + gap + 2
		startY = (h - totalH) / 2
		if startY < 0 {
			startY = 0
		}
		startX = (w - totalW) / 2
		if startX < 0 {
			startX = 0
		}
		return
	}

	boxW = m.boxWidth(w, num)
	mode = "horizontal"
	if w < horizontalThreshold {
		mode = "vertical"
		boxW = min(w-6, 48)
	}

	totalW := num * boxW
	if mode != "vertical" {
		totalW += (num - 1) * gap
	}
	stackH := m.headerHeight() + 1
	if mode == "vertical" {
		stackH += num*boxH + (num - 1)
	} else {
		stackH += boxH
	}
	startY = (h - stackH) / 2
	if startY < 0 {
		startY = 0
	}
	startX = (w - totalW) / 2
	if startX < 0 {
		startX = 0
	}
	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *menuModel) boxRects() []boxRect {
	mode, boxW, boxH, startY, startX, gap := m.computeLayout()
	rects := make([]boxRect, len(m.items))
	boxesY := startY + m.headerHeight() + 1

	switch mode {
	case "vertical":
		for i := range m.items {
			rects[i] = boxRect{x: startX, y: boxesY + i*(boxH+1), w: boxW, h: boxH}
		}
	case "grid":
		for i := range m.items {
			col := i % 2
			row := i / 2
			rects[i] = boxRect{
				x: startX + col*(boxW+gap),
				y: boxesY + row*(boxH+gap),
				w: boxW,
				h: boxH,
			}
		}
	default:
		for i := range m.items {
			rects[i] = boxRect{x: startX + i*(boxW+gap), y: boxesY, w: boxW, h: boxH}
		}
	}
	return rects
}

func (m *menuModel) boxWidth(termW, num int) int {
	if num < 1 {
		num = 4
	}
	maxEach := 28
	each := (termW - 4 - (num-1)*2) / num
	if each > maxEach {
		each = maxEach
	}
	if each < 18 {
		each = 18
	}
	return each
}

func (m *menuModel) View() string {
	mode, boxW, _, _, _, gap := m.computeLayout()

	boxes := make([]string, len(m.items))
	for i, it := range m.items {
		boxes[i] = m.renderBox(i, it, boxW)
	}

	var cards string
	switch mode {
	case "vertical":
		parts := make([]string, len(boxes))
		for i, b := range boxes {
			if i < len(boxes)-1 {
				parts[i] = lipgloss.NewStyle().MarginBottom(1).Render(b)
			} else {
				parts[i] = b
			}
		}
		cards = lipgloss.JoinVertical(lipgloss.Left, parts...)
	case "grid":
		row0 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().MarginRight(gap).Render(boxes[0]),
			boxes[1],
		)
		row1 := lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().MarginRight(gap).Render(boxes[2]),
			boxes[3],
		)
		cards = lipgloss.JoinVertical(lipgloss.Center, row0, lipgloss.NewStyle().Height(1).Render(""), row1)
	default:
		parts := make([]string, len(boxes))
		for i, b := range boxes {
			if i < len(boxes)-1 {
				parts[i] = lipgloss.NewStyle().MarginRight(gap).Render(b)
			} else {
				parts[i] = b
			}
		}
		cards = lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	}

	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("←→↑↓"), mt.Render(" move  ·  "),
		hl.Render("1–4"), mt.Render(" pick  ·  "),
		hl.Render("enter"), mt.Render(" select  ·  "),
		hl.Render("g"), mt.Render(" github  ·  "),
		hl.Render("q"), mt.Render(" quit  ·  "),
		hl.Render("t"), mt.Render(" compact"),
	)

	var header string
	if m.compact {
		header = renderCompactHeader("Choose how you'd like to measure your connection")
	} else {
		header = renderHeader("Choose how you'd like to measure your connection")
	}

	rule := lipgloss.NewStyle().Foreground(m.theme.Border).Render(strings.Repeat("─", 36))

	stack := lipgloss.JoinVertical(lipgloss.Center,
		header,
		rule,
		"",
		cards,
		"",
		hint,
	)

	w, h := m.width, m.height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}

	chip := m.renderUpdateChip()
	if chip == "" || w < 56 {
		return apptheme.PaintScreen(m.theme, w, h, stack)
	}

	bg := m.theme.AppBg
	chipBlock := lipgloss.NewStyle().
		PaddingRight(1).
		PaddingBottom(0).
		Background(bg).
		Render(chip)
	chipH := lipgloss.Height(chipBlock)
	if chipH < 1 {
		chipH = 1
	}
	// Leave a 1-row gap above the chip strip so cards don't collide.
	footerH := chipH + 1
	mainH := h - footerH
	if mainH < 12 {
		return apptheme.PaintScreen(m.theme, w, h, stack)
	}

	main := lipgloss.Place(
		w, mainH,
		lipgloss.Center, lipgloss.Center,
		stack,
		lipgloss.WithWhitespaceBackground(bg),
	)
	// Spacer row + right-aligned chip.
	spacer := lipgloss.NewStyle().Width(w).Height(1).Background(bg).Render("")
	footer := lipgloss.Place(
		w, chipH,
		lipgloss.Right, lipgloss.Bottom,
		chipBlock,
		lipgloss.WithWhitespaceBackground(bg),
	)
	return lipgloss.JoinVertical(lipgloss.Left, main, spacer, footer)
}

// updateChipRect is the absolute terminal rect of the update chip (for mouse).
func (m *menuModel) updateChipRect() boxRect {
	w, h := m.width, m.height
	if w <= 0 || h <= 0 || w < 56 {
		return boxRect{}
	}
	chip := m.renderUpdateChip()
	if chip == "" {
		return boxRect{}
	}
	chipW := lipgloss.Width(chip)
	chipH := lipgloss.Height(chip)
	if chipW <= 0 || chipH <= 0 {
		return boxRect{}
	}
	// Matches View(): 1 spacer + chip at bottom-right with PaddingRight(1).
	return boxRect{
		x: w - chipW - 1,
		y: h - chipH,
		w: chipW,
		h: chipH,
	}
}

func (m *menuModel) renderUpdateChip() string {
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg)
	fg := lipgloss.NewStyle().Foreground(m.theme.Foreground).Background(bg)

	var border lipgloss.TerminalColor = m.theme.Border
	if m.chipHover {
		border = m.theme.AccentHL
	}

	var title, sub, linkLabel string
	var titleStyle lipgloss.Style
	url := m.updateRes.OpenURL
	if url == "" {
		url = update.RepoURL
	}

	switch m.updateStatus {
	case updateChecking:
		titleStyle = lipgloss.NewStyle().
			Foreground(m.theme.Muted).Background(bg).Bold(true)
		spin := m.spinner.View()
		title = spin + " Checking for updates"
		sub = "v" + strings.TrimPrefix(m.version, "v")
		linkLabel = "github.com/Foxemsx/riptide"
	case updateFailed:
		titleStyle = lipgloss.NewStyle().
			Foreground(m.theme.Muted).Background(bg).Bold(true)
		title = "· Offline"
		sub = "v" + strings.TrimPrefix(m.version, "v")
		linkLabel = "github.com/Foxemsx/riptide"
		url = update.RepoURL
	default:
		if m.updateRes.UpdateAvailable {
			titleStyle = lipgloss.NewStyle().
				Foreground(ink).Background(m.theme.AccentUL).Bold(true).Padding(0, 1)
			title = "↑ Update available"
			cur := displayVer(m.updateRes.Current)
			lat := displayVer(m.updateRes.Latest)
			sub = cur + "  →  " + lat
			linkLabel = "Open release · click or g"
			border = m.theme.AccentUL
			if m.chipHover {
				border = m.theme.AccentHL
			}
		} else {
			titleStyle = lipgloss.NewStyle().
				Foreground(ink).Background(m.theme.AccentHL).Bold(true).Padding(0, 1)
			title = "✓ Up to date"
			sub = displayVer(m.updateRes.Current)
			if m.updateRes.Latest != "" {
				sub = displayVer(m.updateRes.Latest)
			}
			linkLabel = "github.com/Foxemsx/riptide"
			border = m.theme.AccentHL
			if m.chipHover {
				border = m.theme.AccentDL
			}
		}
	}

	link := hyperlink(url, muted.Render(linkLabel))
	// Rebuild link with hover emphasis.
	if m.chipHover {
		link = hyperlink(url, lipgloss.NewStyle().
			Foreground(m.theme.Download).Background(bg).Underline(true).
			Render(linkLabel))
	}

	innerW := 30
	line := func(s string) string {
		return lipgloss.NewStyle().Width(innerW).Background(bg).Render(s)
	}

	body := strings.Join([]string{
		line(titleStyle.Render(title)),
		line(fg.Render(sub)),
		line(link),
	}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(0, 1).
		Render(body)
}

func displayVer(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	if !strings.HasPrefix(v, "v") && v != "dev" {
		return "v" + v
	}
	return v
}

// hyperlink wraps text in an OSC 8 terminal hyperlink (clickable in modern terminals).
func hyperlink(url, text string) string {
	if url == "" {
		return text
	}
	// OSC 8 ; ; url ST  text  OSC 8 ; ; ST
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func (m *menuModel) renderBox(i int, it menuItem, cardWidth int) string {
	selected := i == m.cursor || (m.hovered >= 0 && i == m.hovered)

	accent := m.theme.AccentDL
	fill := m.theme.MenuSelectDL
	switch it.screen {
	case screenMonitor:
		accent = m.theme.AccentUL
		fill = m.theme.MenuSelectUL
	case screenSettings:
		accent = m.theme.AccentLat
		fill = m.theme.MenuSelectSet
	case screenExit:
		accent = m.theme.AccentHL
		fill = m.theme.MenuSelectExit
	}

	var bg lipgloss.TerminalColor
	if selected {
		bg = fill
	} else {
		bg = m.theme.MenuIdleFill
	}

	innerW := cardWidth - 4
	if innerW < 12 {
		innerW = 12
	}

	cell := func(fg lipgloss.TerminalColor, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(fg).Background(bg)
		if bold {
			s = s.Bold(true)
		}
		return s
	}
	space := lipgloss.NewStyle().Background(bg)
	line := func(parts ...string) string {
		joined := strings.Join(parts, "")
		return lipgloss.NewStyle().Width(innerW).Background(bg).Inline(true).Render(joined)
	}

	ink := lipgloss.Color("#0a0e14")
	var chip, titleBlock string
	if selected {
		if it.hotkey != "" {
			chip = lipgloss.NewStyle().Foreground(ink).Background(accent).Bold(true).Padding(0, 1).Render(it.hotkey)
		}
		titleBlock = lipgloss.NewStyle().Foreground(ink).Background(accent).Bold(true).Padding(0, 1).Render(it.title)
	} else {
		if it.hotkey != "" {
			chip = lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true).Padding(0, 1).Render(it.hotkey)
		}
		titleBlock = lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true).Padding(0, 1).Render(it.title)
	}
	titleRow := line(chip, space.Render(" "), titleBlock)

	subFG := m.theme.Muted
	if selected {
		subFG = m.theme.Foreground
	}
	subRow := line(space.Render("  "), cell(subFG, false).Render(it.subtitle))

	divCh := "─"
	if selected {
		divCh = "━"
	}
	div := line(cell(accent, false).Render(strings.Repeat(divCh, min(innerW, 20))))

	featRows := make([]string, 3)
	for j := 0; j < 3; j++ {
		if j < len(it.features) {
			bullet := cell(accent, false).Render("› ")
			if !selected {
				bullet = cell(m.theme.Border, false).Render("· ")
			}
			featRows[j] = line(space.Render(" "), bullet, cell(m.theme.Muted, false).Render(it.features[j]))
		} else {
			featRows[j] = line("")
		}
	}

	var badgeRow string
	if it.badge != "" {
		var badge string
		if selected {
			badge = lipgloss.NewStyle().Foreground(ink).Background(accent).Bold(true).Padding(0, 1).Render(it.badge)
		} else {
			badge = lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true).Render(" " + it.badge + " ")
		}
		badgeRow = line(space.Render(" "), badge)
	} else if selected {
		badgeRow = line(space.Render(" "), cell(accent, true).Render("↵ enter"))
	} else {
		badgeRow = line("")
	}

	topBar := line("")
	if selected {
		topBar = line(cell(accent, false).Render(strings.Repeat("▀", innerW)))
	}

	body := strings.Join([]string{
		topBar,
		titleRow,
		subRow,
		line(""),
		div,
		line(""),
		featRows[0],
		featRows[1],
		featRows[2],
		line(""),
		badgeRow,
		line(""), // keep TUNE / LIVE / etc. inside the selected fill
	}, "\n")

	var borderCol lipgloss.TerminalColor = m.theme.Border
	if selected {
		borderCol = accent
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderCol).
		Background(bg).
		Padding(1, 2).
		Width(cardWidth).
		Align(lipgloss.Left).
		Render(body)

	if selected {
		p := pulseFactor(m.pulse)
		gw := int(float64(cardWidth) * (0.72 + 0.28*p))
		if gw < cardWidth/2 {
			gw = cardWidth / 2
		}
		if gw > cardWidth {
			gw = cardWidth
		}
		bar := lipgloss.NewStyle().Foreground(accent).Bold(true).Render(strings.Repeat("▀", gw))
		pad := (cardWidth - gw) / 2
		if pad < 0 {
			pad = 0
		}
		under := strings.Repeat(" ", pad) + bar
		box = lipgloss.JoinVertical(lipgloss.Left, box, under)
	} else {
		box = lipgloss.JoinVertical(lipgloss.Left, box, strings.Repeat(" ", cardWidth))
	}
	return box
}

func pulseFactor(p float64) float64 {
	frac := p - float64(int(p))
	if frac < 0.5 {
		return 0.6 + frac*0.8
	}
	return 1.0 - (frac-0.5)*0.8
}

func (m *menuModel) applyTheme(t apptheme.Theme) {
	m.theme = t
	m.spinner.Style = lipgloss.NewStyle().Foreground(t.Highlight)
}
