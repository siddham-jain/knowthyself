// Package tui renders the collaboration Profile as an industrial-instrument
// dashboard in the terminal: a braille radar, dimensional readouts, headline stats,
// and tips, styled from the shared design tokens. It's a report.Reporter.
//
// v1 renders a single static frame (the profile is computed once). Interactive
// drill-down (Bubble Tea model/update) is a natural later addition behind the same
// Reporter seam.
package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/synch/internal/design"
	"github.com/siddham/synch/internal/profile"
)

// Reporter renders the dashboard to a writer (stdout by default; injectable for
// tests).
type Reporter struct{ W io.Writer }

// New returns a TUI reporter writing to stdout.
func New() Reporter { return Reporter{W: os.Stdout} }

const width = 78

// Render implements report.Reporter.
func (r Reporter) Render(p profile.Profile) error {
	w := r.W
	if w == nil {
		w = os.Stdout
	}
	var b strings.Builder
	b.WriteString(header(p))
	b.WriteByte('\n')
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, radarPanel(p), summaryPanel(p)))
	b.WriteByte('\n')
	b.WriteString(dimensionsPanel(p))
	b.WriteByte('\n')
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, statsPanel(p), insightsPanel(p)))
	b.WriteByte('\n')
	fmt.Fprintln(w, b.String())
	return nil
}

func header(p profile.Profile) string {
	title := design.Header.Render("SYNCH")
	sub := design.Dim.Render(" // AI COLLABORATION PROFILE")
	meta := design.Label.Render(fmt.Sprintf("SRC %s   GEN %s",
		strings.ToUpper(p.Source), p.GeneratedAt.Format("2006-01-02 15:04")))
	line := lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("━", width))
	top := lipgloss.JoinHorizontal(lipgloss.Bottom, title, sub)
	gap := width - lipgloss.Width(top) - lipgloss.Width(meta)
	if gap < 1 {
		gap = 1
	}
	return top + strings.Repeat(" ", gap) + meta + "\n" + line
}

func radarPanel(p profile.Profile) string {
	radar := radarBlock(p.Dimensions, 44, 44)
	legend := axisLegend(p.Dimensions)
	body := lipgloss.JoinHorizontal(lipgloss.Center, radar, "  ", legend)
	return design.Panel.Width(48).Render(
		design.Label.Render("COLLABORATION RADAR") + "\n" + body)
}

// axisLegend lists the axes with their readings beside the radar.
func axisLegend(dims []profile.DimensionResult) string {
	var b strings.Builder
	for i, d := range dims {
		marker := lipgloss.NewStyle().Foreground(design.Accent).Render(fmt.Sprintf("%d", i+1))
		name := design.Label.Render(shortDim(d.Dimension))
		b.WriteString(marker + " " + name + "  " + reading(d.Signal) + "\n")
	}
	return b.String()
}

func summaryPanel(p profile.Profile) string {
	score := p.Overall
	big := lipgloss.NewStyle().Foreground(design.ScoreColor(score)).Bold(true).
		Render(fmt.Sprintf("%.1f", score))
	scale := design.Dim.Render(" / 10")
	arch := design.Value.Render(strings.ToUpper(p.Archetype.Name))
	blurb := design.Label.Render(wrap(p.Archetype.Blurb, 24))
	expl := design.Dim.Render(wrap(p.Archetype.Explanation, 24))

	body := design.Label.Render("OVERALL") + "\n" +
		big + scale + "\n\n" +
		design.Label.Render("ARCHETYPE") + "\n" +
		arch + "\n" + blurb + "\n\n" + expl
	return design.Panel.Width(28).Render(body)
}

func dimensionsPanel(p profile.Profile) string {
	var b strings.Builder
	b.WriteString(design.Label.Render("DIMENSIONS") + "\n")
	for _, d := range p.Dimensions {
		name := lipgloss.NewStyle().Foreground(design.Ink).Width(22).Render(d.Title)
		var meter, val string
		if d.Signal.Sufficient {
			meterStyle := lipgloss.NewStyle().Foreground(design.ScoreColor(d.Signal.Score))
			meter = meterStyle.Render(design.Bar(d.Signal.Score, 32))
			val = meterStyle.Bold(true).Render(fmt.Sprintf(" %4.1f", d.Signal.Score))
		} else {
			meter = design.Dim.Render(strings.Repeat("░", 32))
			val = design.Dim.Render("  n/a")
		}
		b.WriteString(name + meter + val + "\n")
	}
	return design.Panel.Width(width - 2).Render(strings.TrimRight(b.String(), "\n"))
}

func statsPanel(p profile.Profile) string {
	s := p.Stats
	row := func(k, v string) string {
		return design.Label.Width(16).Render(k) + design.Value.Render(v) + "\n"
	}
	var b strings.Builder
	b.WriteString(design.Label.Render("TELEMETRY") + "\n")
	b.WriteString(row("SESSIONS", fmt.Sprintf("%d", s.Sessions)))
	b.WriteString(row("PROMPTS", fmt.Sprintf("%d", s.UserPrompts)))
	b.WriteString(row("CACHE HIT", fmt.Sprintf("%.0f%%", s.CacheHitRate*100)))
	b.WriteString(row("TOKENS", humanTokens(s.TotalTokens)))
	if len(s.TopTools) > 0 {
		b.WriteString(row("TOP TOOL", fmt.Sprintf("%s (%d)", s.TopTools[0].Name, s.TopTools[0].Count)))
	}
	if len(s.TopSlashCommands) > 0 {
		b.WriteString(row("TOP CMD", fmt.Sprintf("%s (%d)", s.TopSlashCommands[0].Name, s.TopSlashCommands[0].Count)))
	}
	return design.Panel.Width(34).Render(strings.TrimRight(b.String(), "\n"))
}

func insightsPanel(p profile.Profile) string {
	var b strings.Builder
	b.WriteString(design.Label.Render("SIGNALS & TIPS") + "\n")
	if len(p.Insights) == 0 {
		b.WriteString(design.Dim.Render("No tips — clean run."))
	}
	for i, in := range p.Insights {
		bullet := lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ")
		b.WriteString(bullet + design.Title.Render(in.Title) + "\n")
		b.WriteString(design.Dim.Render(wrap(in.Body, 40)))
		if i < len(p.Insights)-1 {
			b.WriteString("\n\n")
		}
	}
	return design.Panel.Width(width - 34 - 2).Render(strings.TrimRight(b.String(), "\n"))
}

// --- small helpers ---

func reading(sig profile.Signal) string {
	if !sig.Sufficient {
		return design.Dim.Render("n/a")
	}
	return lipgloss.NewStyle().Foreground(design.ScoreColor(sig.Score)).Bold(true).
		Render(fmt.Sprintf("%.1f", sig.Score))
}

func shortDim(d profile.Dimension) string {
	switch d {
	case profile.PromptQuality:
		return "Prompt Quality"
	case profile.IterationEfficiency:
		return "Iteration Eff."
	case profile.ToolLeverage:
		return "Tool Leverage"
	case profile.ContextManagement:
		return "Context Mgmt"
	case profile.TokenEconomy:
		return "Token Economy"
	default:
		return d.Title()
	}
}

func humanTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// wrap hard-wraps text to width columns (rune-aware; keeps words intact).
func wrap(s string, w int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := 0
	for i, word := range words {
		wl := len([]rune(word))
		if line > 0 && line+1+wl > w {
			b.WriteByte('\n')
			line = 0
		} else if i > 0 && line > 0 {
			b.WriteByte(' ')
			line++
		}
		b.WriteString(word)
		line += wl
	}
	return b.String()
}
