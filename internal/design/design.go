// Package design is the single source of truth for synch's visual language: a
// restrained, industrial-instrument aesthetic (mission-control / financial-terminal),
// monochrome with a single amber accent, structure carried by borders and rules
// rather than color or effects. These tokens style the TUI today and are the same
// palette a future landing page consumes, so both surfaces read as one system.
//
// Explicitly avoided (per brief): matrix/hacker green, neon, cyberpunk, RGB-gaming,
// glassmorphism, gradients.
package design

import "github.com/charmbracelet/lipgloss"

// Palette — monochrome graphite/ink/paper neutrals + one industrial accent.
var (
	Ink       = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#e8e6e1"} // primary text
	Muted     = lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#8a8782"} // secondary text
	Faint     = lipgloss.AdaptiveColor{Light: "#9a9a9a", Dark: "#5a5854"} // rules, hints
	Paper     = lipgloss.AdaptiveColor{Light: "#f5f4f1", Dark: "#141413"} // background wells
	Accent    = lipgloss.AdaptiveColor{Light: "#b5560f", Dark: "#e08a3c"} // oxide amber: live values, active axis
	AccentDim = lipgloss.AdaptiveColor{Light: "#d99a5c", Dark: "#8a5a2a"} // muted amber: filled area behind the edge
	Danger    = lipgloss.AdaptiveColor{Light: "#8a2f2f", Dark: "#c85a5a"} // alerts only
	Success   = lipgloss.AdaptiveColor{Light: "#3a6b3a", Dark: "#7aa86f"} // strong readings only
)

// Border is a squared, technical frame (no rounded corners — this is instrumentation).
var Border = lipgloss.Border{
	Top:         "─",
	Bottom:      "─",
	Left:        "│",
	Right:       "│",
	TopLeft:     "┌",
	TopRight:    "┐",
	BottomLeft:  "└",
	BottomRight: "┘",
}

// Common styles.
var (
	Title = lipgloss.NewStyle().Foreground(Ink).Bold(true)

	Label = lipgloss.NewStyle().Foreground(Muted)

	Value = lipgloss.NewStyle().Foreground(Accent).Bold(true)

	Dim = lipgloss.NewStyle().Foreground(Faint)

	Panel = lipgloss.NewStyle().
		Border(Border).
		BorderForeground(Faint).
		Padding(0, 1)

	// Header is the instrument-readout banner style.
	Header = lipgloss.NewStyle().Foreground(Accent).Bold(true)
)

// ScoreColor maps a 0..10 reading to a readout color: amber accent is the default
// "live value"; only strong/weak extremes shift to success/danger so color stays
// meaningful rather than decorative.
func ScoreColor(v float64) lipgloss.TerminalColor {
	switch {
	case v >= 8:
		return Success
	case v < 4:
		return Danger
	default:
		return Accent
	}
}

// Bar renders a fixed-width horizontal meter for a 0..10 value using block runes.
func Bar(v float64, width int) string {
	if width < 1 {
		width = 1
	}
	filled := int((v/10.0)*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	out := make([]rune, width)
	for i := range out {
		if i < filled {
			out[i] = '█'
		} else {
			out[i] = '░'
		}
	}
	return string(out)
}
