package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
)

// This file renders knowthyself's "cold start" moments — the states before there's a
// profile to show. They are deliberately warm and a little funny: a first-time user
// who hits one of these should feel greeted, not error-messaged.

// momentWidth clamps a terminal width to a comfortable reading measure for these
// full-screen messages.
func momentWidth(termW int) int { return clampInt(termW, 40, 72) }

// moment composes a centered cold-start screen: the wordmark, a piece of art, a
// headline, wrapped body copy, optional detail lines, and a run hint.
func moment(w io.Writer, termW int, art string, artStyle lipgloss.Style, headline string, body string, details []string) {
	width := momentWidth(termW)
	inner := clampInt(width-2, 30, 64)

	blocks := []string{}
	if termW >= 46 {
		blocks = append(blocks, wordmarkArt(), "")
	}
	blocks = append(blocks,
		centeredArt(art, width, artStyle),
		"",
		lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render(headline),
		design.Label.Render(wrap(body, inner)),
	)
	if len(details) > 0 {
		blocks = append(blocks, "")
		blocks = append(blocks, strings.Join(details, "\n"))
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

// detail formats a "label → value" hint line for a cold-start screen.
func detail(label, value string) string {
	return design.Label.Render(label+"  ") + lipgloss.NewStyle().Foreground(design.Ink).Render(value)
}

// RenderNoClaudeScreen is shown when there's no ~/.claude to read: Claude Code isn't
// installed, or its history lives somewhere custom.
func RenderNoClaudeScreen(w io.Writer, termW int, base string) {
	amber := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)
	moment(w, termW, reactionFace, amber,
		"Wait — no Claude on this machine?!",
		"knowthyself reads your Claude Code history at "+base+" to show you how you actually collaborate. There's nothing there yet — so Claude Code isn't installed, or its history lives somewhere custom.",
		[]string{
			design.Dim.Render("Shipping code with no AI pair in this economy? Bold. Let's fix that."),
			"",
			detail("get claude code ", "claude.com/claude-code"),
			detail("custom location?", "set CLAUDE_CONFIG_DIR, then rerun"),
		},
	)
}

// RenderNoDataScreen is shown when ~/.claude exists but there aren't enough
// conversations yet to draw a meaningful profile.
func RenderNoDataScreen(w io.Writer, termW int, sessions int) {
	muted := lipgloss.NewStyle().Foreground(design.Accent)
	conv := "no conversations yet"
	if sessions > 0 {
		conv = fmt.Sprintf("%d so far — need a few more", sessions)
	}
	moment(w, termW, emptyRadar, muted,
		"You're early.",
		"Claude Code is here, but there isn't enough history yet for me to plot your collaboration shape. Go build something real with Claude — then come back and meet yourself.",
		[]string{
			detail("conversations", conv),
		},
	)
}
