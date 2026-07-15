package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/synch/internal/design"
)

// canvas is a braille pixel buffer: each terminal cell packs 2×4 dots (Unicode
// braille, base U+2800), giving smooth lines for the radar at character resolution.
// A parallel layer grid records whether each cell holds grid or data pixels so the
// renderer can color them distinctly (faint grid, amber data).
type canvas struct {
	wDots, hDots int
	wCells, hCells int
	dots  []uint8 // braille dot bits per cell
	layer []uint8 // 0=empty, 1=grid, 2=data (data wins)
}

// braille dot bit for a pixel at (col,row) within a cell: col∈{0,1}, row∈{0,1,2,3}.
var brailleBit = [2][4]uint8{
	{0x01, 0x02, 0x04, 0x40}, // col 0: dots 1,2,3,7
	{0x08, 0x10, 0x20, 0x80}, // col 1: dots 4,5,6,8
}

func newCanvas(wDots, hDots int) *canvas {
	wc := (wDots + 1) / 2
	hc := (hDots + 3) / 4
	return &canvas{
		wDots: wDots, hDots: hDots, wCells: wc, hCells: hc,
		dots:  make([]uint8, wc*hc),
		layer: make([]uint8, wc*hc),
	}
}

// set lights the pixel at (x,y) on the given layer (higher layer wins per cell).
func (c *canvas) set(x, y int, layer uint8) {
	if x < 0 || y < 0 || x >= c.wDots || y >= c.hDots {
		return
	}
	cx, cy := x/2, y/4
	idx := cy*c.wCells + cx
	c.dots[idx] |= brailleBit[x%2][y%4]
	if layer > c.layer[idx] {
		c.layer[idx] = layer
	}
}

// line draws a straight line (Bresenham) between two dot coordinates.
func (c *canvas) line(x0, y0, x1, y1 int, layer uint8) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	err := dx + dy
	for {
		c.set(x0, y0, layer)
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

// render returns the colorized braille block: faint grid cells, amber data cells.
func (c *canvas) render() string {
	gridStyle := lipgloss.NewStyle().Foreground(design.Faint)
	dataStyle := lipgloss.NewStyle().Foreground(design.Accent)
	var b strings.Builder
	for row := 0; row < c.hCells; row++ {
		for col := 0; col < c.wCells; col++ {
			idx := row*c.wCells + col
			r := rune(0x2800 + int(c.dots[idx]))
			if c.dots[idx] == 0 {
				b.WriteRune(' ')
				continue
			}
			if c.layer[idx] == 2 {
				b.WriteString(dataStyle.Render(string(r)))
			} else {
				b.WriteString(gridStyle.Render(string(r)))
			}
		}
		if row < c.hCells-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	default:
		return 0
	}
}
