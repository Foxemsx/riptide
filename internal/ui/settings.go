package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Foxemsx/riptide/internal/db"
	apptheme "github.com/Foxemsx/riptide/internal/theme"
)

type settingsFocus int

const (
	focusSearch settingsFocus = iota
	focusThemes
	focusReset
	focusUninstall
	focusAbout
)

const (
	aboutGitHubURL = "https://github.com/Foxemsx/riptide"
	aboutBMCURL    = "https://buymeacoffee.com/foxemsx"
)

type settingsModel struct {
	theme   apptheme.Theme
	compact bool
	width   int
	height  int
	store   *db.Store
	version string

	search textinput.Model
	focus  settingsFocus

	themeNames []string
	themeIdx   int
	filtered   []int

	confirmReset bool
	resetCursor  int
	flash        string
	flashOK      bool

	dbPath   string
	runCount int

	aboutHover int // 0 none, 1 github, 2 bmc
}

type themeChangedMsg struct{ name string }
type dbResetMsg struct{}

func newSettingsModel(theme apptheme.Theme, compact bool, store *db.Store, version string) *settingsModel {
	ti := textinput.New()
	ti.Placeholder = "themes, reset, uninstall…"
	ti.CharLimit = 40
	ti.Width = 48
	ti.Prompt = ""
	ti.Focus()

	names := apptheme.Names()
	idx := 0
	for i, n := range names {
		if n == theme.Name {
			idx = i
			break
		}
	}
	if version == "" {
		version = "dev"
	}

	m := &settingsModel{
		theme:      theme,
		compact:    compact,
		store:      store,
		version:    version,
		search:     ti,
		focus:      focusSearch,
		themeNames: names,
		themeIdx:   idx,
	}
	if store != nil {
		m.dbPath = store.Path()
		n, _ := store.CountRuns()
		m.runCount = n
	}
	m.styleSearch()
	m.refilter()
	return m
}

func (m *settingsModel) styleSearch() {
	bg := m.theme.MenuIdleFill
	m.search.TextStyle = lipgloss.NewStyle().Foreground(m.theme.Foreground).Background(bg)
	m.search.PlaceholderStyle = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg)
	m.search.PromptStyle = lipgloss.NewStyle().Background(bg)
	m.search.Cursor.Style = lipgloss.NewStyle().Foreground(m.theme.Download).Background(bg)
	m.search.Cursor.TextStyle = lipgloss.NewStyle().Foreground(m.theme.Foreground).Background(bg)
}

func (m *settingsModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *settingsModel) refilter() {
	q := m.query()
	m.filtered = m.filtered[:0]
	for i, name := range m.themeNames {
		t := apptheme.Get(name)
		hay := strings.ToLower(t.Name + " " + t.Display + " " + t.Tagline + " theme")
		if q == "" || strings.Contains(hay, q) {
			m.filtered = append(m.filtered, i)
		}
	}
	if q != "" && len(m.filtered) > 0 && !m.isSectionQuery() {
		found := false
		for _, fi := range m.filtered {
			if fi == m.themeIdx {
				found = true
				break
			}
		}
		if !found {
			m.themeIdx = m.filtered[0]
		}
	}
}

func (m *settingsModel) query() string {
	return strings.ToLower(strings.TrimSpace(m.search.Value()))
}

func tokenMatch(q string, keys ...string) bool {
	if q == "" {
		return true
	}
	for _, k := range keys {
		k = strings.ToLower(k)
		if strings.Contains(k, q) || strings.Contains(q, k) {
			return true
		}
	}
	return false
}

func (m *settingsModel) isSectionQuery() bool {
	q := m.query()
	if q == "" {
		return false
	}
	return tokenMatch(q, "theme", "themes", "color", "palette", "look",
		"reset", "database", "db", "wipe", "clear", "history", "runs", "data",
		"uninstall", "remove", "install", "linux", "windows", "manual",
		"about", "support", "donate", "coffee", "github", "version", "info")
}

func (m *settingsModel) showThemes() bool {
	q := m.query()
	if q == "" {
		return true
	}
	if tokenMatch(q, "theme", "themes", "color", "palette", "look") {
		return true
	}
	return len(m.filtered) > 0 && !tokenMatch(q, "reset", "database", "db", "wipe", "clear",
		"uninstall", "remove", "install", "linux", "windows", "about", "support",
		"donate", "coffee", "github", "info")
}

func (m *settingsModel) showReset() bool {
	return tokenMatch(m.query(), "reset", "database", "db", "wipe", "clear", "delete", "history", "runs", "data")
}

func (m *settingsModel) showUninstall() bool {
	return tokenMatch(m.query(), "uninstall", "remove", "install", "linux", "windows", "manual")
}

func (m *settingsModel) showAbout() bool {
	q := m.query()
	if q == "" {
		return true
	}
	return tokenMatch(q, "about", "support", "donate", "coffee", "github", "version", "info",
		"buymeacoffee", "star")
}

func (m *settingsModel) themeList() []int {
	q := m.query()
	if q == "" || tokenMatch(q, "theme", "themes", "color", "palette", "look") {
		list := make([]int, len(m.themeNames))
		for i := range m.themeNames {
			list[i] = i
		}
		return list
	}
	return m.filtered
}

func (m *settingsModel) jumpFromSearch() tea.Cmd {
	q := m.query()
	m.search.Blur()

	if q != "" && len(m.filtered) > 0 && !tokenMatch(q, "theme", "themes", "reset", "database",
		"uninstall", "remove", "install", "about", "support", "donate", "coffee", "github") {
		m.themeIdx = m.filtered[0]
		m.focus = focusThemes
		return m.applyThemeCmd()
	}
	if m.showAbout() && !m.showThemes() && !m.showReset() && !m.showUninstall() {
		m.focus = focusAbout
		return nil
	}
	if m.showReset() && !m.showThemes() && !m.showUninstall() && !m.showAbout() {
		m.focus = focusReset
		return nil
	}
	if m.showUninstall() && !m.showThemes() && !m.showReset() && !m.showAbout() {
		m.focus = focusUninstall
		return nil
	}
	if m.showThemes() {
		m.focus = focusThemes
		list := m.themeList()
		if len(list) > 0 {
			in := false
			for _, fi := range list {
				if fi == m.themeIdx {
					in = true
					break
				}
			}
			if !in {
				m.themeIdx = list[0]
			}
		}
		return nil
	}
	if m.showReset() {
		m.focus = focusReset
		return nil
	}
	if m.showUninstall() {
		m.focus = focusUninstall
		return nil
	}
	m.search.Focus()
	m.focus = focusSearch
	return textinput.Blink
}

func (m *settingsModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return nil

	case tea.KeyMsg:
		if m.confirmReset {
			return m.updateConfirm(msg)
		}

		key := msg.String()

		if m.focus != focusSearch {
			switch key {
			case "esc", "m":
				return backToMenuCmd()
			case "q":
				return tea.Quit
			case "tab":
				m.focus = m.nextSection(m.focus)
				if m.focus == focusSearch {
					m.search.Focus()
					return textinput.Blink
				}
				return nil
			case "shift+tab":
				m.focus = m.prevSection(m.focus)
				if m.focus == focusSearch {
					m.search.Focus()
					return textinput.Blink
				}
				return nil
			case "down", "j":
				if m.focus == focusThemes {
					m.moveTheme(1)
					return nil
				}
				if m.focus == focusAbout {
					if m.aboutHover < 2 {
						m.aboutHover++
					}
					return nil
				}
				// leave section → next
				m.focus = m.nextSection(m.focus)
				return nil
			case "up", "k":
				if m.focus == focusThemes {
					m.moveTheme(-1)
					return nil
				}
				if m.focus == focusAbout {
					if m.aboutHover > 0 {
						m.aboutHover--
					}
					return nil
				}
				m.focus = m.prevSection(m.focus)
				if m.focus == focusSearch {
					m.search.Focus()
					return textinput.Blink
				}
				return nil
			case "left", "h":
				if m.focus == focusThemes {
					m.moveTheme(-1)
					return nil
				}
				// switch section tabs
				m.focus = m.prevSection(m.focus)
				if m.focus == focusSearch {
					m.focus = m.prevSection(m.focus) // skip search when arrowing sections
				}
				return nil
			case "right", "l":
				if m.focus == focusThemes {
					m.moveTheme(1)
					return nil
				}
				m.focus = m.nextSection(m.focus)
				if m.focus == focusSearch {
					m.focus = m.nextSection(m.focus)
				}
				return nil
			case "enter", " ":
				switch m.focus {
				case focusThemes:
					return m.applyThemeCmd()
				case focusReset:
					m.confirmReset = true
					m.resetCursor = 0
					return nil
				case focusAbout:
					if m.aboutHover < 1 {
						m.aboutHover = 2
					}
					return m.openAboutCmd(m.aboutHover)
				}
			case "1":
				if m.showThemes() {
					m.focus = focusThemes
				}
				return nil
			case "2":
				if m.showReset() {
					m.focus = focusReset
				}
				return nil
			case "3":
				if m.showUninstall() {
					m.focus = focusUninstall
				}
				return nil
			case "4":
				if m.showAbout() {
					m.focus = focusAbout
				}
				return nil
			case "/":
				m.focus = focusSearch
				m.search.Focus()
				return textinput.Blink
			case "g":
				if m.showAbout() {
					m.focus = focusAbout
					m.aboutHover = 1
					return m.openAboutCmd(1)
				}
			case "b":
				if m.showAbout() {
					m.focus = focusAbout
					m.aboutHover = 2
					return m.openAboutCmd(2)
				}
			case "r":
				if m.showReset() {
					m.focus = focusReset
					m.confirmReset = true
					m.resetCursor = 0
					return nil
				}
			}
			return nil
		}

		// Search focused
		switch key {
		case "esc":
			if m.search.Value() != "" {
				m.search.SetValue("")
				m.refilter()
				return nil
			}
			return backToMenuCmd()
		case "ctrl+c":
			return tea.Quit
		case "tab", "down":
			m.search.Blur()
			m.focus = m.nextSection(focusSearch)
			return nil
		case "shift+tab":
			m.search.Blur()
			m.focus = m.prevSection(focusSearch)
			return nil
		case "enter":
			return m.jumpFromSearch()
		case "up":
			return nil
		}

		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.refilter()
		return cmd
	}
	return nil
}

func (m *settingsModel) updateConfirm(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "n", "q":
		m.confirmReset = false
		return nil
	case "left", "h", "right", "l", "tab":
		m.resetCursor = 1 - m.resetCursor
		return nil
	case "enter", " ":
		if m.resetCursor == 1 {
			return m.doReset()
		}
		m.confirmReset = false
		return nil
	case "y":
		return m.doReset()
	}
	return nil
}

func (m *settingsModel) doReset() tea.Cmd {
	m.confirmReset = false
	if m.store == nil {
		m.flash = "No database open"
		m.flashOK = false
		return nil
	}
	if err := m.store.Reset(false); err != nil {
		m.flash = "Reset failed: " + err.Error()
		m.flashOK = false
		return nil
	}
	m.runCount = 0
	m.flash = "Database reset — all saved test runs cleared"
	m.flashOK = true
	return func() tea.Msg { return dbResetMsg{} }
}

func (m *settingsModel) applyThemeCmd() tea.Cmd {
	if len(m.themeNames) == 0 {
		return nil
	}
	if m.themeIdx < 0 || m.themeIdx >= len(m.themeNames) {
		m.themeIdx = 0
	}
	list := m.themeList()
	if len(list) > 0 {
		in := false
		for _, fi := range list {
			if fi == m.themeIdx {
				in = true
				break
			}
		}
		if !in {
			m.themeIdx = list[0]
		}
	}
	name := m.themeNames[m.themeIdx]
	if m.store != nil {
		_ = m.store.SetSetting("theme", name)
	}
	return func() tea.Msg { return themeChangedMsg{name: name} }
}

func (m *settingsModel) moveTheme(delta int) {
	list := m.themeList()
	if len(list) == 0 {
		return
	}
	pos := 0
	for i, fi := range list {
		if fi == m.themeIdx {
			pos = i
			break
		}
	}
	pos = (pos + delta + len(list)) % len(list)
	m.themeIdx = list[pos]
}

func (m *settingsModel) visibleSections() []settingsFocus {
	out := []settingsFocus{focusSearch}
	if m.showThemes() {
		out = append(out, focusThemes)
	}
	if m.showReset() {
		out = append(out, focusReset)
	}
	if m.showUninstall() {
		out = append(out, focusUninstall)
	}
	if m.showAbout() {
		out = append(out, focusAbout)
	}
	return out
}

func (m *settingsModel) nextSection(cur settingsFocus) settingsFocus {
	secs := m.visibleSections()
	for i, s := range secs {
		if s == cur {
			return secs[(i+1)%len(secs)]
		}
	}
	if len(secs) > 1 {
		return secs[1]
	}
	return focusSearch
}

func (m *settingsModel) prevSection(cur settingsFocus) settingsFocus {
	secs := m.visibleSections()
	for i, s := range secs {
		if s == cur {
			return secs[(i-1+len(secs))%len(secs)]
		}
	}
	return focusSearch
}

func (m *settingsModel) cardWidth() int {
	cardW := 72
	if m.width > 0 && cardW > m.width-4 {
		cardW = m.width - 4
	}
	if cardW < 48 {
		cardW = 48
	}
	return cardW
}

// themeMaxShow: how many theme rows fit under title+search+tabs+footer.
func (m *settingsModel) themeMaxShow() int {
	h := m.height
	if h <= 0 {
		h = 30
	}
	// compact title(1) + search(~5) + tabs(3) + footer(2) + themes chrome(4)
	maxShow := h - 16
	if maxShow < 5 {
		maxShow = 5
	}
	if maxShow > 12 {
		maxShow = 12
	}
	return maxShow
}

func (m *settingsModel) View() string {
	if m.confirmReset {
		return m.viewConfirm()
	}
	m.styleSearch()

	cardW := m.cardWidth()
	h := m.height
	if h <= 0 {
		h = 30
	}
	w := m.width
	if w <= 0 {
		w = 80
	}

	// Always compact title — full logo makes the page too tall.
	header := renderCompactHeader("Settings  ·  Tune riptide to your taste")

	search := m.viewSearch(cardW)

	// Section tabs — only ONE panel expanded (accordion).
	tabs := m.viewTabs(cardW)
	panel := m.viewActivePanel(cardW)

	var flash string
	if m.flash != "" {
		col := m.theme.Upload
		if m.flashOK {
			col = m.theme.Highlight
		}
		flash = lipgloss.NewStyle().Foreground(col).Bold(true).Render(m.flash)
	}

	hl := lipgloss.NewStyle().Foreground(m.theme.Highlight).Bold(true)
	mt := lipgloss.NewStyle().Foreground(m.theme.Muted)
	hint := lipgloss.JoinHorizontal(lipgloss.Center,
		hl.Render("1–4"), mt.Render(" panels  ·  "),
		hl.Render("tab"), mt.Render(" next  ·  "),
		hl.Render("enter"), mt.Render(" open  ·  "),
		hl.Render("esc"), mt.Render(" menu"),
	)

	parts := []string{header, "", search, "", tabs}
	if flash != "" {
		parts = append(parts, flash)
	}
	if panel != "" {
		parts = append(parts, panel)
	}
	parts = append(parts, "", hint)

	stack := lipgloss.JoinVertical(lipgloss.Center, parts...)
	return apptheme.PaintScreen(m.theme, w, h, stack)
}

// viewTabs draws a single row of section chips. Active section is highlighted.
func (m *settingsModel) viewTabs(w int) string {
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")

	type tab struct {
		focus settingsFocus
		label string
		key   string
		accent lipgloss.Color
		show  bool
	}
	tabs := []tab{
		{focusThemes, "Themes", "1", m.theme.AccentDL, m.showThemes()},
		{focusReset, "Reset DB", "2", m.theme.AccentUL, m.showReset()},
		{focusUninstall, "Uninstall", "3", m.theme.AccentHL, m.showUninstall()},
		{focusAbout, "About", "4", m.theme.AccentLat, m.showAbout()},
	}

	var chips []string
	for _, t := range tabs {
		if !t.show {
			continue
		}
		active := m.focus == t.focus
		var chip string
		if active {
			chip = lipgloss.NewStyle().
				Foreground(ink).Background(t.accent).Bold(true).
				Padding(0, 1).
				Render(t.key + " " + t.label)
		} else {
			chip = lipgloss.NewStyle().
				Foreground(t.accent).Background(bg).Bold(true).
				Padding(0, 1).
				Render(t.key + " " + t.label)
		}
		chips = append(chips, chip)
	}
	if len(chips) == 0 {
		return ""
	}

	row := strings.Join(chips, lipgloss.NewStyle().Background(bg).Render("  "))
	// Full-width bar so it reads as a toolbar
	inner := lipgloss.PlaceHorizontal(w-4, lipgloss.Left, row, lipgloss.WithWhitespaceBackground(bg))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Background(bg).
		Padding(0, 1).
		Width(w).
		Render(inner)
}

// viewActivePanel renders only the focused section body (accordion).
func (m *settingsModel) viewActivePanel(w int) string {
	// When search is focused, show themes as the default panel (or first match).
	switch {
	case m.focus == focusThemes && m.showThemes():
		return m.viewThemes(w)
	case m.focus == focusReset && m.showReset():
		return m.viewReset(w)
	case m.focus == focusUninstall && m.showUninstall():
		return m.viewUninstall(w)
	case m.focus == focusAbout && m.showAbout():
		return m.viewAbout(w)
	case m.focus == focusSearch:
		// Preview first available panel under search
		if m.showThemes() {
			return m.viewThemes(w)
		}
		if m.showReset() {
			return m.viewReset(w)
		}
		if m.showUninstall() {
			return m.viewUninstall(w)
		}
		if m.showAbout() {
			return m.viewAbout(w)
		}
		bg := m.theme.MenuIdleFill
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(m.theme.Border).
			Background(bg).
			Foreground(m.theme.Muted).
			Padding(1, 2).
			Width(w).
			Render("No settings match “" + m.search.Value() + "”  ·  esc to clear")
	}
	return ""
}

func (m *settingsModel) viewSearch(w int) string {
	focused := m.focus == focusSearch
	var border lipgloss.TerminalColor = m.theme.Border
	if focused {
		border = m.theme.AccentLat
	}
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	innerW := w - 4
	if innerW < 20 {
		innerW = 20
	}

	line := func(parts ...string) string {
		return lipgloss.NewStyle().Width(innerW).Background(bg).Inline(true).
			Render(strings.Join(parts, ""))
	}

	chip := lipgloss.NewStyle().
		Foreground(ink).Background(m.theme.AccentLat).Bold(true).Padding(0, 1).
		Render("⌕ Search")
	if !focused {
		chip = lipgloss.NewStyle().
			Foreground(m.theme.AccentLat).Background(bg).Bold(true).Padding(0, 1).
			Render("⌕ Search")
	}

	meta := "filter themes & sections"
	if q := m.query(); q != "" {
		n := 0
		if m.showThemes() {
			n += len(m.themeList())
		}
		if m.showReset() {
			n++
		}
		if m.showUninstall() {
			n++
		}
		meta = fmt.Sprintf("%d match(es)  ·  enter to jump", n)
	}

	m.search.Width = innerW - 2
	if m.search.Width < 8 {
		m.search.Width = 8
	}
	iv := lipgloss.NewStyle().Background(bg).Render(m.search.View())
	inputLine := lipgloss.PlaceHorizontal(
		innerW, lipgloss.Left, iv,
		lipgloss.WithWhitespaceBackground(bg),
	)

	body := strings.Join([]string{
		line(chip, lipgloss.NewStyle().Background(bg).Render("  "),
			lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Render(meta)),
		inputLine,
	}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(0, 1).
		Width(w).
		Render(body)
}

func (m *settingsModel) viewThemes(w int) string {
	focused := m.focus == focusThemes || m.focus == focusSearch
	var border lipgloss.TerminalColor = m.theme.Border
	if m.focus == focusThemes {
		border = m.theme.AccentDL
	}
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")

	title := lipgloss.NewStyle().
		Foreground(ink).Background(m.theme.AccentDL).Bold(true).Padding(0, 1).
		Render("Themes")
	sub := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).
		Render(fmt.Sprintf("  %d · active: %s  ·  ↑↓ browse  enter apply", len(m.themeNames), m.theme.Display))

	var rows []string
	rows = append(rows, title+sub)
	rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Border).Background(bg).Render(strings.Repeat("─", min(w-4, 56))))

	list := m.themeList()
	maxShow := m.themeMaxShow()
	if len(list) < maxShow {
		maxShow = len(list)
	}
	selPos := 0
	for i, fi := range list {
		if fi == m.themeIdx {
			selPos = i
			break
		}
	}
	start := 0
	if len(list) > maxShow {
		start = selPos - maxShow/2
		if start < 0 {
			start = 0
		}
		if start+maxShow > len(list) {
			start = len(list) - maxShow
		}
	}

	if len(list) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Render("  No themes match"))
	} else {
		if start > 0 {
			rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).
				Render(fmt.Sprintf("  ↑ %d more", start)))
		}
		for i := start; i < start+maxShow && i < len(list); i++ {
			fi := list[i]
			t := apptheme.Get(m.themeNames[fi])
			selected := fi == m.themeIdx
			active := t.Name == m.theme.Name

			marker := "  "
			if selected {
				marker = "› "
			}
			name := t.Display
			if active {
				name += "  ✓"
			}

			rowBG := bg
			if selected && focused {
				rowBG = m.theme.MenuSelectSet
			}
			row := lipgloss.NewStyle().Background(rowBG).Width(w - 4).Render(
				lipgloss.NewStyle().Background(rowBG).Render(marker) +
					lipgloss.NewStyle().Foreground(t.AccentDL).Background(rowBG).Bold(selected).Width(14).Render(name) +
					" " +
					lipgloss.NewStyle().Foreground(t.AccentDL).Background(rowBG).Render("●") +
					lipgloss.NewStyle().Foreground(t.AccentUL).Background(rowBG).Render("●") +
					lipgloss.NewStyle().Foreground(t.AccentLat).Background(rowBG).Render("●") +
					lipgloss.NewStyle().Foreground(t.AccentHL).Background(rowBG).Render("●") +
					lipgloss.NewStyle().Foreground(m.theme.Muted).Background(rowBG).Render("  "+t.Tagline),
			)
			rows = append(rows, row)
		}
		end := start + maxShow
		if end < len(list) {
			rows = append(rows, lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).
				Render(fmt.Sprintf("  ↓ %d more", len(list)-end)))
		}
	}

	body := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(1, 1).
		Width(w).
		Render(body)
}

func (m *settingsModel) viewReset(w int) string {
	var border lipgloss.TerminalColor = m.theme.Border
	if m.focus == focusReset {
		border = m.theme.Upload
	}
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")

	title := lipgloss.NewStyle().
		Foreground(ink).Background(m.theme.AccentUL).Bold(true).Padding(0, 1).
		Render("Reset database")

	lines := []string{
		title,
		"",
		lipgloss.NewStyle().Foreground(m.theme.Foreground).Background(bg).Render("Wipe all saved speed-test runs."),
		lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Render("Theme preference is kept. Cannot be undone."),
		"",
		lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Render(fmt.Sprintf("Database  %s", truncate(m.dbPath, w-16))),
		lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Render(fmt.Sprintf("Saved runs  %d", m.runCount)),
		"",
	}
	var btn string
	if m.focus == focusReset {
		btn = lipgloss.NewStyle().Foreground(ink).Background(m.theme.AccentUL).Bold(true).Padding(0, 1).Render("Reset all runs  ↵")
	} else {
		btn = lipgloss.NewStyle().Foreground(m.theme.Upload).Background(bg).Bold(true).Render("  Reset all runs  ")
	}
	lines = append(lines, btn)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(1, 1).
		Width(w).
		Render(strings.Join(lines, "\n"))
}

func (m *settingsModel) viewUninstall(w int) string {
	var border lipgloss.TerminalColor = m.theme.Border
	if m.focus == focusUninstall {
		border = m.theme.AccentHL
	}
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg)
	code := lipgloss.NewStyle().Foreground(m.theme.Download).Background(bg).Bold(true)
	fg := lipgloss.NewStyle().Foreground(m.theme.Foreground).Background(bg)

	title := lipgloss.NewStyle().
		Foreground(ink).Background(m.theme.AccentHL).Bold(true).Padding(0, 1).
		Render("Uninstall")

	// Compact copy so it fits with title + tabs on one screen.
	lines := []string{
		title,
		"",
		fg.Render("Linux / WSL"),
		code.Render("  curl -fsSL …/uninstall.sh | sh"),
		"",
		fg.Render("Manual"),
		code.Render("  rm -f \"$(command -v riptide)\""),
		code.Render("  del %USERPROFILE%\\go\\bin\\riptide.exe"),
		"",
		muted.Render("Does not touch Go, PATH, or riptide.db."),
		muted.Render("Clear history first via Reset DB if you want."),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(1, 1).
		Width(w).
		Render(strings.Join(lines, "\n"))
}

func (m *settingsModel) viewConfirm() string {
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	const innerW = 50

	line := func(parts ...string) string {
		return lipgloss.NewStyle().Width(innerW).Background(bg).Inline(true).
			Render(strings.Join(parts, ""))
	}
	fg := func(c lipgloss.TerminalColor, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c).Background(bg)
		if bold {
			s = s.Bold(true)
		}
		return s
	}

	title := lipgloss.NewStyle().
		Foreground(ink).Background(m.theme.AccentUL).Bold(true).Padding(0, 1).
		Render("Confirm reset")

	body := strings.Join([]string{
		line(title),
		line(""),
		line(fg(m.theme.Foreground, true).Render("Delete all saved test runs?")),
		line(""),
		line(fg(m.theme.Muted, false).Render(fmt.Sprintf("This will permanently remove %d run(s)", m.runCount))),
		line(fg(m.theme.Muted, false).Render(truncate(m.dbPath, 48))),
		line(""),
		line(fg(m.theme.Muted, false).Render("Kept: theme  ·  Removed: all history")),
		line(""),
		line(m.confirmButtons()),
		line(""),
		line(fg(m.theme.Muted, false).Render("←/→  ·  y confirm  ·  n / esc cancel")),
	}, "\n")

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Upload).
		Background(bg).
		Padding(1, 2).
		Width(innerW + 4).
		Render(body)

	return apptheme.PaintScreen(m.theme, m.width, m.height, panel)
}

func (m *settingsModel) confirmButtons() string {
	ink := lipgloss.Color("#0a0e14")
	bg := m.theme.MenuIdleFill
	var cancel, confirm string
	if m.resetCursor == 0 {
		cancel = lipgloss.NewStyle().Foreground(ink).Background(m.theme.AccentHL).Bold(true).Padding(0, 1).Render("Cancel")
		confirm = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Padding(0, 1).Render("Yes, reset")
	} else {
		cancel = lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg).Padding(0, 1).Render("Cancel")
		confirm = lipgloss.NewStyle().Foreground(ink).Background(m.theme.AccentUL).Bold(true).Padding(0, 1).Render("Yes, reset")
	}
	return cancel + "  " + confirm
}

func (m *settingsModel) viewAbout(w int) string {
	var border lipgloss.TerminalColor = m.theme.Border
	if m.focus == focusAbout {
		border = m.theme.AccentLat
	}
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	muted := lipgloss.NewStyle().Foreground(m.theme.Muted).Background(bg)
	fg := lipgloss.NewStyle().Foreground(m.theme.Foreground).Background(bg)
	code := lipgloss.NewStyle().Foreground(m.theme.Download).Background(bg)

	title := lipgloss.NewStyle().
		Foreground(ink).Background(m.theme.AccentLat).Bold(true).Padding(0, 1).
		Render("About riptide")

	ver := displayVer(m.version)
	verLine := fg.Render("Version  ") + code.Render(ver)

	ghBtn := m.aboutLinkButton(1, "GitHub  ★ star", aboutGitHubURL, m.theme.AccentDL)
	bmcBtn := m.aboutLinkButton(2, "Buy me a coffee  ☕", aboutBMCURL, m.theme.AccentUL)

	var hint string
	if m.focus == focusAbout {
		hint = muted.Render("enter open support  ·  ↑↓ choose  ·  esc back")
	} else {
		hint = muted.Render("press 4 or tab to focus")
	}

	lines := []string{
		title,
		"",
		verLine,
		fg.Render("Free & open source · MIT licensed"),
		"",
		ghBtn,
		bmcBtn,
		"",
		hint,
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Background(bg).
		Padding(1, 1).
		Width(w).
		Render(strings.Join(lines, "\n"))
}

// aboutLinkButton renders one clickable (OSC 8) support link as a chip.
func (m *settingsModel) aboutLinkButton(slot int, label string, url string, accent lipgloss.Color) string {
	bg := m.theme.MenuIdleFill
	ink := lipgloss.Color("#0a0e14")
	hover := m.focus == focusAbout && m.aboutHover == slot
	var btn string
	if hover {
		btn = lipgloss.NewStyle().Foreground(ink).Background(accent).Bold(true).Padding(0, 1).Render(label)
	} else {
		btn = lipgloss.NewStyle().Foreground(accent).Background(bg).Bold(true).Render("  " + label)
	}
	return btn
}

// openAboutCmd opens the link for the given slot (1 = github, 2 = bmc).
func (m *settingsModel) openAboutCmd(slot int) tea.Cmd {
	url := aboutBMCURL
	if slot == 1 {
		url = aboutGitHubURL
	}
	u := url
	return func() tea.Msg {
		_ = openURL(u)
		return nil
	}
}

func (m *settingsModel) applyTheme(t apptheme.Theme) {
	m.theme = t
	m.styleSearch()
	for i, n := range m.themeNames {
		if n == t.Name {
			m.themeIdx = i
			break
		}
	}
}
