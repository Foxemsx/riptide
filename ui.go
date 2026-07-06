package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	dlConns      = 5
	ulConns      = 6
	downloadTime = 10 * time.Second
	uploadTime   = 10 * time.Second
	sparkWidth   = 28
	tickInterval = time.Second / 10
)

var (
	accentCyan    = lipgloss.Color("#2EF8BB")
	accentMagenta = lipgloss.Color("#F78BE0")
	accentYellow  = lipgloss.Color("#FFDF5E")
	accentBlue    = lipgloss.Color("#6C9CFF")
	dimColor      = lipgloss.Color("240")
	dimmerColor   = lipgloss.Color("236")
	whiteBold     = lipgloss.Color("15")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentCyan).
			PaddingLeft(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			PaddingLeft(1)

	borderCyan = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentCyan).
			Padding(0, 1)

	borderMagenta = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentMagenta).
			Padding(0, 1)

	borderDone = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(accentYellow).
			Padding(0, 1)

	speedBig = lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteBold).
			Width(8).
			Align(lipgloss.Right)

	unitSmall = lipgloss.NewStyle().
			Foreground(dimColor)

	sparkCyan = lipgloss.NewStyle().
			Foreground(accentCyan)

	sparkMagenta = lipgloss.NewStyle().
			Foreground(accentMagenta)

	peakLabel = lipgloss.NewStyle().
			Foreground(dimmerColor)

	labelDL = lipgloss.NewStyle().
		Bold(true).
		Foreground(accentCyan)

	labelUL = lipgloss.NewStyle().
		Bold(true).
		Foreground(accentMagenta)

	checkStyle = lipgloss.NewStyle().
			Foreground(accentCyan)

	pendingStyle = lipgloss.NewStyle().
			Foreground(dimmerColor)

	summaryKey = lipgloss.NewStyle().
			Foreground(dimColor).
			Bold(true)

	summaryVal = lipgloss.NewStyle().
			Foreground(whiteBold).
			Bold(true).
			Width(14).
			Align(lipgloss.Right)

	barDL    = lipgloss.NewStyle().Foreground(accentCyan)
	barUL    = lipgloss.NewStyle().Foreground(accentMagenta)
	barEmpty = lipgloss.NewStyle().Foreground(dimmerColor)
)

type phase int

const (
	phaseDownload phase = iota
	phaseUpload
	phaseDone
)

type tickMsg time.Time

func tickCmd(t time.Time) tea.Msg {
	return tickMsg(t)
}

type Model struct {
	targets []string

	bytes  *atomic.Int64
	ctx    context.Context
	cancel context.CancelFunc
	start  time.Time
	speed  float64
	speeds []float64
	peak   float64

	phase    phase
	done     bool
	quitting bool

	dlSpeed  float64
	dlPeak   float64
	dlSpeeds []float64
	ulSpeed  float64
	ulPeak   float64
	ulSpeeds []float64

	width int
}

func NewModel(targets []string) Model {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTime)
	return Model{
		targets: targets,
		bytes:   &atomic.Int64{},
		ctx:     ctx,
		cancel:  cancel,
		start:   time.Now(),
		phase:   phaseDownload,
		width:   60,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.Tick(tickInterval, tickCmd), m.measure)
}

func (m Model) measure() tea.Msg {
	for _, url := range m.targets {
		go download(m.ctx, url, m.bytes)
	}
	return nil
}

func (m Model) measureUpload() tea.Msg {
	for i := 0; i < ulConns; i++ {
		go upload(m.ctx, "", m.bytes)
	}
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			m.cancel()
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tickMsg:
		elapsed := time.Since(m.start)
		m.speed = mbps(m.bytes.Load(), elapsed)
		m.speeds = append(m.speeds, m.speed)
		if m.speed > m.peak {
			m.peak = m.speed
		}

		switch m.phase {
		case phaseDownload:
			if elapsed >= downloadTime {
				m.dlSpeed = m.speed
				m.dlPeak = m.peak
				m.dlSpeeds = make([]float64, len(m.speeds))
				copy(m.dlSpeeds, m.speeds)

				m.cancel()
				m.phase = phaseUpload
				m.bytes = &atomic.Int64{}
				m.speed = 0
				m.speeds = nil
				m.peak = 0
				m.start = time.Now()

				ctx, cancel := context.WithTimeout(context.Background(), uploadTime)
				m.ctx = ctx
				m.cancel = cancel

				return m, tea.Batch(tea.Tick(tickInterval, tickCmd), m.measureUpload)
			}
			return m, tea.Tick(tickInterval, tickCmd)

		case phaseUpload:
			if elapsed >= uploadTime {
				m.ulSpeed = m.speed
				m.ulPeak = m.peak
				m.ulSpeeds = make([]float64, len(m.speeds))
				copy(m.ulSpeeds, m.speeds)
				m.cancel()
				m.phase = phaseDone
				m.done = true
				return m, tea.Quit
			}
			return m, tea.Tick(tickInterval, tickCmd)
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(titleStyle.Render("⚡ speed"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("internet speed test"))
	s.WriteString("\n\n")

	switch m.phase {
	case phaseDownload:
		s.WriteString(m.viewDownload(true))
		s.WriteString("\n\n")
		s.WriteString(m.viewUploadEmpty())

	case phaseUpload:
		s.WriteString(m.viewDownloadDone())
		s.WriteString("\n\n")
		s.WriteString(m.viewUpload(true))

	case phaseDone:
		s.WriteString(m.viewDownloadDone())
		s.WriteString("\n\n")
		s.WriteString(m.viewUploadDone())
		s.WriteString("\n\n")
		s.WriteString(m.viewSummary())
	}

	s.WriteString("\n")
	return s.String()
}

func (m Model) viewDownload(active bool) string {
	var b strings.Builder

	header := labelDL.Render(" ↓ Download")
	if active {
		header += "  " + checkStyle.Render("● testing")
	} else {
		header += "  " + pendingStyle.Render("○ pending")
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	speed, unit := scale(m.speed)
	b.WriteString(speedBig.Render(fmt.Sprintf("%.1f", speed)))
	b.WriteString(unitSmall.Render(" " + unit))
	b.WriteString("  ")
	b.WriteString(sparkCyan.Render(sparkline(m.speeds, m.peak, sparkWidth)))

	if m.peak > 0 {
		peak, peakUnit := scale(m.peak)
		label := fmt.Sprintf("%.0f", peak)
		if peakUnit != unit {
			label += " " + peakUnit
		}
		b.WriteString("\n")
		b.WriteString(peakLabel.Render("                    peak " + label))
	}

	if active {
		ratio := float64(len(m.speeds)) / float64(downloadTime/tickInterval)
		b.WriteString("\n")
		b.WriteString(barDL.Render(progressBar(ratio, 30)))
		b.WriteString(peakLabel.Render(fmt.Sprintf("  %ds", int(time.Since(m.start).Seconds()))))
	}

	box := borderCyan.Render(b.String())
	return box
}

func (m Model) viewDownloadDone() string {
	var b strings.Builder

	header := labelDL.Render(" ↓ Download") + "  " + checkStyle.Render("✓ done")
	b.WriteString(header)
	b.WriteString("\n\n")

	speed, unit := scale(m.dlSpeed)
	b.WriteString(speedBig.Render(fmt.Sprintf("%.1f", speed)))
	b.WriteString(unitSmall.Render(" " + unit))
	b.WriteString("  ")
	b.WriteString(sparkCyan.Render(sparkline(m.dlSpeeds, m.dlPeak, sparkWidth)))

	if m.dlPeak > 0 {
		peak, peakUnit := scale(m.dlPeak)
		label := fmt.Sprintf("%.0f", peak)
		if peakUnit != unit {
			label += " " + peakUnit
		}
		b.WriteString("\n")
		b.WriteString(peakLabel.Render("                    peak " + label))
	}

	b.WriteString("\n")
	b.WriteString(barDL.Render(progressBar(1.0, 30)))
	b.WriteString(peakLabel.Render("  10s"))

	box := borderCyan.Render(b.String())
	return box
}

func (m Model) viewUploadEmpty() string {
	var b strings.Builder

	header := labelUL.Render(" ↑ Upload") + "  " + pendingStyle.Render("○ pending")
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(speedBig.Render("--.-"))
	b.WriteString(unitSmall.Render(" Mbps"))
	b.WriteString("  ")
	b.WriteString(sparkMagenta.Render(strings.Repeat(" ", sparkWidth)))

	b.WriteString("\n")
	b.WriteString(barEmpty.Render(progressBar(0, 30)))
	b.WriteString(peakLabel.Render("  0s"))

	box := borderMagenta.Render(b.String())
	return box
}

func (m Model) viewUpload(active bool) string {
	var b strings.Builder

	header := labelUL.Render(" ↑ Upload")
	if active {
		header += "  " + checkStyle.Render("● testing")
	} else {
		header += "  " + pendingStyle.Render("○ pending")
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	speed, unit := scale(m.speed)
	b.WriteString(speedBig.Render(fmt.Sprintf("%.1f", speed)))
	b.WriteString(unitSmall.Render(" " + unit))
	b.WriteString("  ")
	b.WriteString(sparkMagenta.Render(sparkline(m.speeds, m.peak, sparkWidth)))

	if m.peak > 0 {
		peak, peakUnit := scale(m.peak)
		label := fmt.Sprintf("%.0f", peak)
		if peakUnit != unit {
			label += " " + peakUnit
		}
		b.WriteString("\n")
		b.WriteString(peakLabel.Render("                    peak " + label))
	}

	if active {
		ratio := float64(len(m.speeds)) / float64(uploadTime/tickInterval)
		b.WriteString("\n")
		b.WriteString(barUL.Render(progressBar(ratio, 30)))
		b.WriteString(peakLabel.Render(fmt.Sprintf("  %ds", int(time.Since(m.start).Seconds()))))
	}

	box := borderMagenta.Render(b.String())
	return box
}

func (m Model) viewUploadDone() string {
	var b strings.Builder

	header := labelUL.Render(" ↑ Upload") + "  " + checkStyle.Render("✓ done")
	b.WriteString(header)
	b.WriteString("\n\n")

	speed, unit := scale(m.ulSpeed)
	b.WriteString(speedBig.Render(fmt.Sprintf("%.1f", speed)))
	b.WriteString(unitSmall.Render(" " + unit))
	b.WriteString("  ")
	b.WriteString(sparkMagenta.Render(sparkline(m.ulSpeeds, m.ulPeak, sparkWidth)))

	if m.ulPeak > 0 {
		peak, peakUnit := scale(m.ulPeak)
		label := fmt.Sprintf("%.0f", peak)
		if peakUnit != unit {
			label += " " + peakUnit
		}
		b.WriteString("\n")
		b.WriteString(peakLabel.Render("                    peak " + label))
	}

	b.WriteString("\n")
	b.WriteString(barUL.Render(progressBar(1.0, 30)))
	b.WriteString(peakLabel.Render("  10s"))

	box := borderMagenta.Render(b.String())
	return box
}

func (m Model) viewSummary() string {
	var b strings.Builder

	b.WriteString("  Results\n\n")

	dlSpeed, dlUnit := scale(m.dlSpeed)
	ulSpeed, ulUnit := scale(m.ulSpeed)

	b.WriteString("  ")
	b.WriteString(summaryKey.Render("Download"))
	b.WriteString(summaryVal.Render(fmt.Sprintf("%.1f %s", dlSpeed, dlUnit)))
	b.WriteString("\n")

	b.WriteString("  ")
	b.WriteString(summaryKey.Render("Upload"))
	b.WriteString(summaryVal.Render(fmt.Sprintf("%.1f %s", ulSpeed, ulUnit)))
	b.WriteString("\n")

	if m.dlPeak > 0 {
		peak, peakUnit := scale(m.dlPeak)
		b.WriteString("  ")
		b.WriteString(summaryKey.Render("Peak DL"))
		b.WriteString(summaryVal.Render(fmt.Sprintf("%.1f %s", peak, peakUnit)))
		b.WriteString("\n")
	}
	if m.ulPeak > 0 {
		peak, peakUnit := scale(m.ulPeak)
		b.WriteString("  ")
		b.WriteString(summaryKey.Render("Peak UL"))
		b.WriteString(summaryVal.Render(fmt.Sprintf("%.1f %s", peak, peakUnit)))
	}

	box := borderDone.Render(b.String())
	return box
}
