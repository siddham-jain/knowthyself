package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/synch/internal/design"
)

// sparkRunes are the eight vertical block levels used for sparklines.
var sparkRunes = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// sparkline renders values (each 0..10) as a compact block-run trend. Values ≤ 0 are
// treated as gaps (shown as a faint dot) so insufficient sessions don't read as a
// crash to zero.
func sparkline(values []float64, style lipgloss.Style) string {
	if len(values) == 0 {
		return design.Dim.Render("—")
	}
	var b strings.Builder
	for _, v := range values {
		if v <= 0 {
			b.WriteString(design.Dim.Render("·"))
			continue
		}
		idx := int(v / 10.0 * float64(len(sparkRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx > len(sparkRunes)-1 {
			idx = len(sparkRunes) - 1
		}
		b.WriteString(style.Render(string(sparkRunes[idx])))
	}
	return b.String()
}

// trendArrow compares the last value against the mean of the earlier ones and
// returns a colored delta indicator ("↑ +0.8", "↓ -0.3", "→ ~").
func trendArrow(values []float64) string {
	pts := make([]float64, 0, len(values))
	for _, v := range values {
		if v > 0 {
			pts = append(pts, v)
		}
	}
	if len(pts) < 2 {
		return design.Dim.Render("→ new")
	}
	last := pts[len(pts)-1]
	var sum float64
	for _, v := range pts[:len(pts)-1] {
		sum += v
	}
	prev := sum / float64(len(pts)-1)
	delta := last - prev
	switch {
	case delta >= 0.3:
		return lipgloss.NewStyle().Foreground(design.Success).Render(fmt.Sprintf("↑ +%.1f", delta))
	case delta <= -0.3:
		return lipgloss.NewStyle().Foreground(design.Danger).Render(fmt.Sprintf("↓ %.1f", delta))
	default:
		return design.Dim.Render("→ ~")
	}
}
