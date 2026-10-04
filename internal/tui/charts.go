package tui

import "strings"

// brailleBit[dx][dy] is the Unicode braille dot bit for a 2x4 cell.
var brailleBit = [2][4]rune{{0x01, 0x02, 0x04, 0x40}, {0x08, 0x10, 0x20, 0x80}}

// LineChart draws values as a connected line on a braille canvas of width x height cells
// (2 x 4 dots per cell). The y axis spans min..max of values; a flat series draws mid-height.
func LineChart(values []int, width, height int) []string {
	if width < 1 || height < 1 || len(values) == 0 {
		return nil
	}
	W, H := width*2, height*4
	grid := make([][]bool, H)
	for i := range grid {
		grid[i] = make([]bool, W)
	}
	lo, hi := values[0], values[0]
	for _, v := range values {
		lo, hi = min(lo, v), max(hi, v)
	}
	px := func(i int) int {
		if len(values) == 1 {
			return W / 2
		}
		return i * (W - 1) / (len(values) - 1)
	}
	py := func(v int) int {
		if hi == lo {
			return H / 2
		}
		return (H - 1) - (v-lo)*(H-1)/(hi-lo)
	}
	plot := func(x0, y0, x1, y1 int) { // Bresenham
		dx, dy := abs(x1-x0), -abs(y1-y0)
		sx, sy := sign(x1-x0), sign(y1-y0)
		for err := dx + dy; ; {
			grid[y0][x0] = true
			if x0 == x1 && y0 == y1 {
				return
			}
			e2 := 2 * err
			if e2 >= dy {
				err += dy
				x0 += sx
			}
			if e2 <= dx {
				err += dx
				y0 += sy
			}
		}
	}
	for i := range values {
		x, y := px(i), py(values[i])
		if i == 0 {
			grid[y][x] = true
			continue
		}
		plot(px(i-1), py(values[i-1]), x, y)
	}
	out := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		for c := 0; c < width; c++ {
			var bits rune
			for dx := 0; dx < 2; dx++ {
				for dy := 0; dy < 4; dy++ {
					if grid[r*4+dy][c*2+dx] {
						bits |= brailleBit[dx][dy]
					}
				}
			}
			b.WriteRune(0x2800 + bits)
		}
		out[r] = b.String()
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x < 0:
		return -1
	case x > 0:
		return 1
	}
	return 0
}

var levels = []rune(" ▁▂▃▄▅▆▇█")

// Bars draws one column per count, height rows tall, scaled to the largest count.
// Any non-zero count shows at least the thinnest bar.
func Bars(counts []int, height int) []string {
	if height < 1 || len(counts) == 0 {
		return nil
	}
	peak := 0
	for _, c := range counts {
		peak = max(peak, c)
	}
	out := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		for _, c := range counts {
			eighths := 0
			if peak > 0 && c > 0 {
				eighths = max(1, c*height*8/peak)
			}
			lvl := min(8, max(0, eighths-(height-1-r)*8))
			b.WriteRune(levels[lvl])
		}
		out[r] = b.String()
	}
	return out
}

// Axis labels the ends of a width-wide axis, e.g. "800        3500".
func Axis(left, right string, width int) string {
	if len(left)+len(right) >= width {
		return left + " " + right
	}
	return left + strings.Repeat(" ", width-len(left)-len(right)) + right
}
