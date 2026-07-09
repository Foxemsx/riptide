package main

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Eighth-block ramp: 0 empty … 8 solid. Gives 8× vertical resolution vs plain █.
var barRamp = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// graph is a live vertical area chart (newest on the right). It renders with
// sub-cell bar tips, a base→tip fire gradient, soft age fade, a faint grid,
// and a bright spark on the peak sample — closer to btop/bottom than a flat
// block strip.
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
	if w < 1 {
		w = 1
	}
	g.width = w
	if len(g.data) > g.width {
		g.data = g.data[len(g.data)-g.width:]
	}
}

// clear wipes the history (used when resetting the test).
func (g *graph) clear() {
	g.data = nil
}

// View renders the chart as `height` rows, top row first.
func (g *graph) View() string {
	if g.width <= 0 || g.height <= 0 {
		return ""
	}

	if len(g.data) == 0 {
		return g.renderEmpty()
	}

	// Scale from 0 → max with headroom so the shape reads as real rate,
	// not min-max normalized noise.
	max := 0.0
	peakIdx := 0
	for i, v := range g.data {
		if v > max {
			max = v
			peakIdx = i
		}
	}
	if max < 1e-9 {
		return g.renderEmpty()
	}
	scale := max * 1.12 // ~12% headroom so tips don't clip the ceiling

	// Right-align samples: empty history grows in from the right (live feel).
	offset := g.width - len(g.data)
	if offset < 0 {
		offset = 0
	}

	// Per-column height in eighth-cells (0 .. height*8).
	levels := make([]int, g.width)
	peakCol := -1
	totalEighths := g.height * 8
	for col := 0; col < g.width; col++ {
		di := col - offset
		if di < 0 || di >= len(g.data) {
			continue
		}
		v := g.data[di]
		if v <= 0 {
			continue
		}
		lv := int(math.Round(v / scale * float64(totalEighths)))
		if lv < 1 {
			lv = 1
		}
		if lv > totalEighths {
			lv = totalEighths
		}
		levels[col] = lv
		if di == peakIdx {
			peakCol = col
		}
	}

	// Precompute which rows get a faint grid (25 / 50 / 75 % from bottom).
	gridRow := make([]bool, g.height)
	for _, frac := range []float64{0.25, 0.5, 0.75} {
		fromBottom := int(math.Round(frac * float64(g.height-1)))
		row := g.height - 1 - fromBottom
		if row >= 0 && row < g.height {
			gridRow[row] = true
		}
	}

	// Peak line row (watermark across empty cells at the peak height).
	peakRow := -1
	if peakCol >= 0 && levels[peakCol] > 0 {
		// Topmost row that still contains the peak bar.
		fromBottom := (levels[peakCol] - 1) / 8
		peakRow = g.height - 1 - fromBottom
	}

	gridStyle := lipgloss.NewStyle().Foreground(dimColor(g.bottom, 0.28))
	peakLineStyle := lipgloss.NewStyle().Foreground(dimColor(g.top, 0.40))

	rows := make([]string, g.height)
	for row := 0; row < g.height; row++ {
		// This row covers eighths [rowBase, rowBase+8) measured from the bottom.
		rowBase := (g.height - 1 - row) * 8
		// Vertical fire gradient: deep at chart floor, bright near ceiling.
		t := float64(g.height-row) / float64(g.height)
		t = easeOutQuad(t)
		rowColor := lerpColor(g.bottom, g.top, t)

		var b strings.Builder
		b.Grow(g.width * 24)
		for col := 0; col < g.width; col++ {
			lv := levels[col]
			age := g.ageFactor(col)

			switch {
			case lv >= rowBase+8:
				// Solid body cell.
				c := dimColor(rowColor, age)
				if col == peakCol && row == peakRow {
					c = peakSpark(g.top)
				}
				b.WriteString(lipgloss.NewStyle().Foreground(c).Render("█"))

			case lv > rowBase:
				// Partial tip cell — sub-cell resolution.
				partial := lv - rowBase // 1..7
				if partial > 8 {
					partial = 8
				}
				c := dimColor(rowColor, age)
				if col == peakCol {
					c = peakSpark(g.top)
				} else {
					// Tips are a touch brighter than the body at this row.
					c = lerpColor(c, g.top, 0.35)
					c = dimColor(c, age)
				}
				b.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(barRamp[partial])))

			default:
				// Empty: grid dots, peak watermark, or blank.
				switch {
				case row == peakRow && col%2 == 1:
					b.WriteString(peakLineStyle.Render("·"))
				case gridRow[row] && col%3 == 0:
					b.WriteString(gridStyle.Render("·"))
				default:
					b.WriteByte(' ')
				}
			}
		}
		rows[row] = b.String()
	}
	return strings.Join(rows, "\n")
}

// renderEmpty draws a stable blank chart with a faint grid so layout never jumps.
func (g *graph) renderEmpty() string {
	gridStyle := lipgloss.NewStyle().Foreground(dimColor(g.bottom, 0.22))
	gridRow := make([]bool, g.height)
	for _, frac := range []float64{0.25, 0.5, 0.75} {
		fromBottom := int(math.Round(frac * float64(g.height-1)))
		row := g.height - 1 - fromBottom
		if row >= 0 && row < g.height {
			gridRow[row] = true
		}
	}
	rows := make([]string, g.height)
	for row := 0; row < g.height; row++ {
		if !gridRow[row] {
			rows[row] = strings.Repeat(" ", g.width)
			continue
		}
		var b strings.Builder
		for col := 0; col < g.width; col++ {
			if col%3 == 0 {
				b.WriteString(gridStyle.Render("·"))
			} else {
				b.WriteByte(' ')
			}
		}
		rows[row] = b.String()
	}
	return strings.Join(rows, "\n")
}

// ageFactor dims older (left) columns slightly so the live edge reads brighter.
func (g *graph) ageFactor(col int) float64 {
	if g.width <= 1 {
		return 1
	}
	// 0 at left (oldest) → 1 at right (newest)
	t := float64(col) / float64(g.width-1)
	return 0.58 + 0.42*t
}

// peakSpark returns a hot highlight color for the current peak tip.
func peakSpark(top lipgloss.Color) lipgloss.Color {
	return lerpColor(top, lipgloss.Color("#f0f6fc"), 0.55)
}

// dimColor blends c toward near-black by keeping `amount` of the original
// (amount 1 = full color, 0 = almost black).
func dimColor(c lipgloss.Color, amount float64) lipgloss.Color {
	if amount >= 0.999 {
		return c
	}
	if amount < 0 {
		amount = 0
	}
	// Blend toward the app canvas so empty graph cells match the UI chrome.
	return lerpColor(lipgloss.Color("#191a1b"), c, amount)
}

// easeOutQuad pushes more of the gradient toward the bright tip.
func easeOutQuad(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return 1 - (1-t)*(1-t)
}

// lerpColor blends from a to b by t in [0,1] in RGB space.
func lerpColor(a, b lipgloss.Color, t float64) lipgloss.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ca, errA := colorful.Hex(string(a))
	cb, errB := colorful.Hex(string(b))
	if errA != nil || errB != nil {
		return a
	}
	// Lab blend keeps mid-stops looking natural on dark terminals.
	blended := ca.BlendLab(cb, t)
	return lipgloss.Color(blended.Hex())
}
