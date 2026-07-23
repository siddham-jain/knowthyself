package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
)

const (
	markName  = "K N O W  T H Y S E L F"
	markMaxim = "ΓΝΩΘΙ  ΣΕΑΥΤΟΝ"
	markPad   = 5
)

// markWidth is the rendered width of the framed mark, border included.
var markWidth = lipgloss.Width(markName) + 2*markPad + 2

// inscriptionArt is the identity mark: the letterspaced name over the Delphic maxim
// it is named for, framed like a chiselled lintel. Under markWidth columns the frame
// is dropped rather than broken.
func inscriptionArt(width int) string {
	name := lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render(markName)
	if width > 0 && width < markWidth {
		return name
	}
	maxim := lipgloss.NewStyle().Foreground(design.Faint).Render(markMaxim)
	return lipgloss.NewStyle().
		Border(design.Border).
		BorderForeground(design.Accent).
		Padding(0, markPad).
		Render(lipgloss.JoinVertical(lipgloss.Center, "", name, "", maxim, ""))
}

// emptySlab is the same frame with nothing inscribed in it — the mark for "there is
// no record to read yet".
func emptySlab() string {
	blank := strings.Repeat(" ", markWidth-2)
	return lipgloss.NewStyle().
		Border(design.Border).
		BorderForeground(design.Faint).
		Render(strings.Join([]string{blank, blank, blank}, "\n"))
}

// emptyRadar is an unplotted radar with a single centre point: not enough history to
// draw a shape yet.
const emptyRadar = `      ·   ·   ·
   ·               ·
  ·                 ·
 ·                   ·
 ·         ◆         ·
 ·                   ·
  ·                 ·
   ·               ·
      ·   ·   ·`

// styleLines applies a style to each line of a block, so a multi-line piece of art
// keeps its shape instead of being styled as one wrapped string.
func styleLines(art string, style lipgloss.Style) string {
	lines := strings.Split(art, "\n")
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return strings.Join(lines, "\n")
}
