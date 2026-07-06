package main

import "strings"

var dots = [8][2]byte{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
	{0x00, 0x00},
	{0x00, 0x00},
	{0x00, 0x00},
	{0x00, 0x00},
}

func sparkline(values []float64, peak float64, width int) string {
	cols, rows := width*2, 4
	cells := make([]byte, width)

	if len(values) > 1 && peak > 0 {
		for px := range cols {
			pos := float64(px) / float64(cols-1) * float64(len(values)-1)
			i := int(pos)
			v := values[i]
			if i+1 < len(values) {
				v += (values[i+1] - values[i]) * (pos - float64(i))
			}
			height := int(v/peak*float64(rows) + 0.5)
			for y := rows - height; y < rows; y++ {
				cells[px/2] |= dots[y][px%2]
			}
		}
	}

	var b strings.Builder
	for _, bits := range cells {
		if bits == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteRune(rune(0x2800 + int(bits)))
		}
	}
	return b.String()
}

func progressBar(ratio float64, width int) string {
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	empty := width - filled

	var b strings.Builder
	for i := 0; i < filled; i++ {
		b.WriteString("█")
	}
	for i := 0; i < empty; i++ {
		b.WriteString("░")
	}
	return b.String()
}
