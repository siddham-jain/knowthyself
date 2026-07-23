package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
)

// This file renders the states that come before there is a profile to show.

// momentWidth clamps a terminal width to a comfortable reading measure.
func momentWidth(termW int) int { return clampInt(termW, 40, 72) }

// moment composes a centred cold-start screen. art is expected to arrive already
// styled, so each caller controls its own emphasis.
func moment(w io.Writer, termW int, art, headline, body string, details []string) {
	width := momentWidth(termW)
	inner := clampInt(width-2, 30, 64)

	blocks := []string{}
	if termW >= markWidth {
		blocks = append(blocks, inscriptionArt(termW), "")
	}
	blocks = append(blocks,
		art,
		"",
		lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render(headline),
		design.Label.Render(wrap(body, inner)),
	)
	if len(details) > 0 {
		blocks = append(blocks, "", strings.Join(details, "\n"))
	}
	blocks = append(blocks,
		"",
		lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("╌", inner)),
		design.Label.Render("run anytime  ")+
			lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render("knowthyself"),
	)

	frame := lipgloss.JoinVertical(lipgloss.Center, blocks...)
	fmt.Fprintln(w, "\n"+lipgloss.PlaceHorizontal(termW, lipgloss.Center, frame)+"\n")
}

// detail formats a "label → value" hint line.
func detail(label, value string) string {
	return design.Label.Render(label+"  ") + lipgloss.NewStyle().Foreground(design.Ink).Render(value)
}

// RenderNoClaudeScreen is shown when there is no ~/.claude to read: Claude Code isn't
// installed, or its history lives somewhere custom.
func RenderNoClaudeScreen(w io.Writer, termW int, base string) {
	moment(w, termW, emptySlab(),
		"Nothing on record.",
		"knowthyself reads your Claude Code history at "+base+" to show how you actually collaborate. There is nothing there yet — so Claude Code isn't installed, or its history lives somewhere custom.",
		[]string{
			detail("get claude code ", "claude.com/claude-code"),
			detail("custom location?", "set CLAUDE_CONFIG_DIR, then rerun"),
		},
	)
}

// RenderNoDataScreen is shown when ~/.claude exists but there is not enough history
// yet to draw a meaningful profile.
func RenderNoDataScreen(w io.Writer, termW int, sessions int) {
	conv := "no conversations yet"
	if sessions > 0 {
		conv = fmt.Sprintf("%d so far — a few more will do it", sessions)
	}
	moment(w, termW, styleLines(emptyRadar, lipgloss.NewStyle().Foreground(design.Accent)),
		"Too early to read.",
		"Claude Code is here, but there is not enough history yet to plot your collaboration shape. Build something real with it, then come back.",
		[]string{detail("conversations", conv)},
	)
}
