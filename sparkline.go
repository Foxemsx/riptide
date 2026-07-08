package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// sparkline renders a slim, single-row history graph of speed over time,
// using varying block heights (▁▂▃▄▅▆▇█) like the original fast.com CLI.
// Newer samples are on the right; the tallest block is the peak seen so far.
type sparkline struct {
	width int
	data  []float64 // most-recent-last
	style lipgloss.Style
}

// sparkBlocks maps a fractional height (0..1) to a block glyph.
var sparkBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func newSparkline(width int, _ int, color lipgloss.AdaptiveColor) *sparkline {
	return &sparkline{
		width: width,
		style: lipgloss.NewStyle().Foreground(color),
	}
}

// push appends a value and trims the history to the visible width.
func (s *sparkline) push(v float64) {
	s.data = append(s.data, v)
	if len(s.data) > s.width {
		s.data = s.data[len(s.data)-s.width:]
	}
}

// setWidth resizes the visible window (used on terminal resize).
func (s *sparkline) setWidth(w int) {
	s.width = w
	if len(s.data) > s.width {
		s.data = s.data[len(s.data)-s.width:]
	}
}

// View renders the single-row sparkline. When there is no data yet it shows
// a faint baseline so the card layout does not jump.
func (s *sparkline) View() string {
	if s.width <= 0 {
		return ""
	}
	if len(s.data) == 0 {
		return s.style.Render(strings.Repeat(" ", s.width))
	}
	min := s.data[0]
	max := s.data[0]
	for _, v := range s.data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span <= 0 {
		mid := sparkBlocks[len(sparkBlocks)/2]
		return s.style.Render(strings.Repeat(mid, len(s.data)) + strings.Repeat(" ", s.width-len(s.data)))
	}

	var b strings.Builder
	for col := 0; col < s.width; col++ {
		var val float64
		if col < len(s.data) {
			val = s.data[col]
		}
		// Map the value to a block level in [0, len(sparkBlocks)-1].
		idx := int(((val - min) / span) * float64(len(sparkBlocks)-1) + 0.5)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		b.WriteString(sparkBlocks[idx])
	}
	return s.style.Render(b.String())
}
