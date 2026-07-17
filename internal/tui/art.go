package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/reflect/internal/design"
)

// This file holds reflect's ASCII identity. The signature mark is the wordmark with
// a mirrored, fading reflection beneath a waterline — the name made literal. The
// edge-case art (a wide-eyed reaction, an empty-radar seedling) gives the onboarding
// moments a face so they read as alive rather than as an error string.

// glyphs is a 5×5 block font for exactly the letters in "REFLECT". Each row is a
// fixed 5 columns so the wordmark composes column-aligned regardless of letter.
var glyphs = map[rune][5]string{
	'R': {"████ ", "█   █", "████ ", "█  █ ", "█   █"},
	'E': {"█████", "█    ", "███  ", "█    ", "█████"},
	'F': {"█████", "█    ", "███  ", "█    ", "█    "},
	'L': {"█    ", "█    ", "█    ", "█    ", "█████"},
	'C': {"█████", "█    ", "█    ", "█    ", "█████"},
	'T': {"█████", "  █  ", "  █  ", "  █  ", "  █  "},
}

// wordmarkRows renders WORD as five rows of block letters (single-column gutter).
// Unknown runes render as blank cells, so the layout never breaks.
func wordmarkRows(word string) [5]string {
	var rows [5]string
	blank := [5]string{"     ", "     ", "     ", "     ", "     "}
	letters := make([][5]string, 0, len(word))
	for _, r := range strings.ToUpper(word) {
		if g, ok := glyphs[r]; ok {
			letters = append(letters, g)
		} else {
			letters = append(letters, blank)
		}
	}
	for r := 0; r < 5; r++ {
		parts := make([]string, len(letters))
		for i, g := range letters {
			parts[i] = g[r]
		}
		rows[r] = strings.Join(parts, " ")
	}
	return rows
}

// wordmarkArt returns the "reflect" wordmark in amber with a dimmed, vertically
// mirrored reflection below a faint waterline — the brand's signature graphic.
func wordmarkArt() string {
	rows := wordmarkRows("REFLECT")
	amber := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)
	faint := lipgloss.NewStyle().Foreground(design.Faint)
	width := lipgloss.Width(rows[0])

	var b strings.Builder
	for _, r := range rows {
		b.WriteString(amber.Render(r) + "\n")
	}
	b.WriteString(faint.Render(strings.Repeat("╌", width)) + "\n")
	// The reflection: rows reversed, blocks softened to a watery shade.
	for i := len(rows) - 1; i >= 0; i-- {
		soft := strings.ReplaceAll(rows[i], "█", "▒")
		b.WriteString(faint.Render(soft) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// reactionFace is a wide-eyed, open-mouthed reaction — the beat for "wait, no Claude
// on this machine?!" It carries the tone so the message underneath can stay short.
const reactionFace = `      .-""""""-.
    .'          '.
   /   O      O   \
  :                :
  |     .----.     |
  :     |    |     :
   \    '----'    /
    '.          .'
      '-......-'`

// emptyRadar is an unplotted radar with a single center point: "there's not enough
// here to draw your shape yet." On-brand for the not-enough-data moment.
const emptyRadar = `      ·   ·   ·
   ·               ·
  ·                 ·
 ·                   ·
 ·         ◆         ·
 ·                   ·
  ·                 ·
   ·               ·
      ·   ·   ·`

// centeredArt renders a multi-line art block, styled and horizontally centered to w.
func centeredArt(art string, w int, style lipgloss.Style) string {
	var b strings.Builder
	for i, line := range strings.Split(art, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(style.Render(line))
	}
	return lipgloss.PlaceHorizontal(w, lipgloss.Center, b.String())
}
