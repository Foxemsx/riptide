package main

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// graph renders a vertical bar chart of recent speed samples, newest on the
// right. Each bar is shaded with a per-cell gradient: a deep color at the base
// blending to a brighter tip, so spikes visibly rise above their neighbours and
// read as a "live" signal rather than a flat strip.
type graph struct {
	width  int
	height int
	data   []float64 // most-recent-last
	bottom lipgloss.Color
	top    lipgloss.Color
}

func newGraph(width, height int, bottom, top lipgloss.Color) *graph {
	return &graph{
		width:  width,
		height: height,
		bottom: bottom,
		top:    top,
	}
}

// push appends a value and trims the history to the visible width.
func (g *graph) push(v float64) {
	g.data = append(g.data, v)
	if len(g.data) > g.width {
		g.data = g.data[len(g.data)-g.width:]
	}
}

// setWidth resizes the visible window (used on terminal resize).
func (g *graph) setWidth(w int) {
	g.width = w
	if len(g.data) > g.width {
		g.data = g.data[len(g.data)-g.width:]
	}
}

// clear wipes the history (used when resetting the test).
func (g *graph) clear() {
	g.data = nil
}

// View renders the chart as `height` rows, top row first. Empty cells are
// spaces so the underlying axis line shows through.
func (g *graph) View() string {
	if g.width <= 0 || g.height <= 0 {
		return ""
	}

	// Baseline row of spaces so the card layout never jumps before data
	// arrives.
	if len(g.data) == 0 {
		return strings.Repeat("\n", g.height-1) + strings.Repeat(" ", g.width)
	}

	min := g.data[0]
	max := g.data[0]
	for _, v := range g.data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	// Floor the span so a flat trace doesn't fill the whole graph.
	span := max - min
	if span < 1e-6 {
		span = 1e-6
	}

	// Height of each column in cells, newest sample on the right.
	heights := make([]int, g.width)
	for col := 0; col < g.width; col++ {
		var val float64
		if col < len(g.data) {
			val = g.data[col]
		}
		if val <= 0 {
			heights[col] = 0
			continue
		}
		h := int(math.Round((val - min) / span * float64(g.height-1)))
		if h < 1 {
			h = 1
		}
		if h > g.height {
			h = g.height
		}
		heights[col] = h
	}

	// Build top-down: row 0 is the top (tallest possible bar).
	var rows []string
	for row := 0; row < g.height; row++ {
		levelFromTop := g.height - 1 - row // 0 == tip level
		var b strings.Builder
		for col := 0; col < g.width; col++ {
			h := heights[col]
			if h > 0 && levelFromTop < h {
				// t goes 0 (base) -> 1 (tip); tip is brightest.
				t := float64(h-1-levelFromTop) / float64(h)
				b.WriteString(lipgloss.NewStyle().Foreground(lerpColor(g.bottom, g.top, t)).Render("█"))
			} else {
				b.WriteString(" ")
			}
		}
		rows = append(rows, b.String())
	}
	return strings.Join(rows, "\n")
}

// lerpColor blends from a to b by t in [0,1] in the OKLCH-ish RGB space and
// returns the result as a hex lipgloss.Color.
func lerpColor(a, b lipgloss.Color, t float64) lipgloss.Color {
	ca, errA := colorful.Hex(string(a))
	cb, errB := colorful.Hex(string(b))
	if errA != nil || errB != nil {
		return a
	}
	blended := ca.BlendRgb(cb, t)
	return lipgloss.Color(blended.Hex())
}
