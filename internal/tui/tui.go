// Package tui renders the collaboration Profile as an industrial-instrument
// dashboard. On a real terminal it runs an interactive Bubble Tea program (boot
// animation, navigable axes, evidence inspector, session drill-down, trends) with a
// fluid, width-driven layout that reflows for narrow terminals; when output is
// piped or non-interactive (or in tests) it falls back to a single static frame.
package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/siddham/synch/internal/design"
	"github.com/siddham/synch/internal/profile"
)

// Reporter renders the dashboard to a writer (stdout by default; injectable for
// tests, which always take the static path).
type Reporter struct{ W io.Writer }

// New returns a TUI reporter writing to stdout.
func New() Reporter { return Reporter{W: os.Stdout} }

// Layout constants. Widths below are *rendered* totals; a panel rendered at total T
// sets a Lip Gloss content width of T-2 (border) and wraps text to T-4 (border +
// padding). See panelBox / textArea.
const (
	minWidth   = 46 // below this the terminal is too narrow to lay out
	maxWidth   = 96 // cap so the dashboard stays dense, not sprawling
	twoColMin  = 74 // at/above this we place panels side by side; below, we stack
	staticW    = 78 // width of the non-interactive frame
	legendCols = 22 // reserved width for the radar axis legend
)

// Render implements report.Reporter. It launches the interactive program on a TTY
// and renders a static frame otherwise.
func (r Reporter) Render(p profile.Profile) error {
	w := r.W
	if w == nil {
		w = os.Stdout
	}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return runInteractive(p)
	}
	return renderStatic(w, p)
}

func runInteractive(p profile.Profile) error {
	_, err := tea.NewProgram(newModel(p), tea.WithAltScreen()).Run()
	return err
}

// renderStatic writes the non-interactive single frame at a fixed width.
func renderStatic(w io.Writer, p profile.Profile) error {
	W := staticW
	var b strings.Builder
	b.WriteString(header(p, W))
	b.WriteByte('\n')
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		radarPanelWith(p, radarOpts{fraction: 1, selected: -1}, 48), summaryPanel(p, W-48)))
	b.WriteByte('\n')
	b.WriteString(dimensionsPanel(p, -1, W))
	b.WriteByte('\n')
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		statsPanel(p, 34), insightsPanel(p, W-34)))
	b.WriteByte('\n')
	fmt.Fprintln(w, b.String())
	return nil
}

// --- width helpers ---

// panelBox returns a panel style whose rendered width is total.
func panelBox(total int) lipgloss.Style { return design.Panel.Width(total - 2) }

// textArea is the usable text width inside a panel of the given rendered total.
func textArea(total int) int { return total - 4 }

// --- shared styled components (width-driven; used by static + interactive views) ---

func header(p profile.Profile, w int) string {
	title := design.Header.Render("SYNCH")
	sub := design.Dim.Render(" // AI COLLABORATION PROFILE")
	meta := design.Label.Render(fmt.Sprintf("SRC %s   GEN %s",
		strings.ToUpper(p.Source), p.GeneratedAt.Format("2006-01-02 15:04")))
	top := lipgloss.JoinHorizontal(lipgloss.Bottom, title, sub)
	// Drop the tagline, then the meta, as width shrinks.
	if w < lipgloss.Width(top)+lipgloss.Width(meta)+2 {
		top = design.Header.Render("SYNCH")
	}
	line := lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("━", w))
	gap := w - lipgloss.Width(top) - lipgloss.Width(meta)
	if gap < 1 {
		// Too tight even for SYNCH + meta: keep just the wordmark.
		return top + "\n" + line
	}
	return top + strings.Repeat(" ", gap) + meta + "\n" + line
}

func radarPanelWith(p profile.Profile, opt radarOpts, total int) string {
	area := textArea(total)
	radarCells := clampInt(area-legendCols-2, 12, 24)
	dots := radarCells * 2
	opt.numbers = true
	radar := radarBlockWith(p.Dimensions, dots, dots, opt)
	legend := axisLegend(p.Dimensions, opt.selected)
	body := lipgloss.JoinHorizontal(lipgloss.Center, radar, "  ", legend)
	title := design.Label.Render("COLLABORATION RADAR") + " " + scaleCaption()
	return panelBox(total).Render(title + "\n" + body)
}

func axisLegend(dims []profile.DimensionResult, selected int) string {
	var b strings.Builder
	for i, d := range dims {
		mark := fmt.Sprintf("%d", i+1)
		nameStyle := design.Label
		markStyle := lipgloss.NewStyle().Foreground(design.Accent)
		if i == selected {
			mark = "▸" + mark
			nameStyle = lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
			markStyle = markStyle.Bold(true)
		}
		b.WriteString(markStyle.Render(mark) + " " + nameStyle.Render(shortDim(d.Dimension)) +
			"  " + reading(d.Signal) + "\n")
	}
	return b.String()
}

func summaryPanel(p profile.Profile, total int) string {
	area := textArea(total)
	big := lipgloss.NewStyle().Foreground(design.ScoreColor(p.Overall)).Bold(true).
		Render(fmt.Sprintf("%.1f", p.Overall))
	body := design.Label.Render("OVERALL") + "\n" +
		big + design.Dim.Render(" / 10") + "\n\n" +
		design.Label.Render("ARCHETYPE") + "\n" +
		design.Value.Render(strings.ToUpper(p.Archetype.Name)) + "\n" +
		design.Label.Render(wrap(p.Archetype.Blurb, area)) + "\n\n" +
		design.Dim.Render(wrap(p.Archetype.Explanation, area))
	return panelBox(total).Render(body)
}

func dimensionsPanel(p profile.Profile, selected, total int) string {
	area := textArea(total)
	nameW := clampInt(area/3, 14, 22)
	barW := clampInt(area-nameW-8, 6, 44) // 2 cursor + 6 value
	var b strings.Builder
	b.WriteString(design.Label.Render("DIMENSIONS") + "\n")
	for i, d := range p.Dimensions {
		cursor := "  "
		nameStyle := lipgloss.NewStyle().Foreground(design.Ink)
		if i == selected {
			cursor = lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ")
			nameStyle = nameStyle.Bold(true)
		}
		name := nameStyle.Width(nameW).Render(truncate(d.Title, nameW))
		var meter, val string
		if d.Signal.Sufficient {
			ms := lipgloss.NewStyle().Foreground(design.ScoreColor(d.Signal.Score))
			meter = ms.Render(design.Bar(d.Signal.Score, barW))
			val = ms.Bold(true).Render(fmt.Sprintf(" %4.1f", d.Signal.Score))
		} else {
			meter = design.Dim.Render(strings.Repeat("░", barW))
			val = design.Dim.Render("  n/a")
		}
		b.WriteString(cursor + name + meter + val + "\n")
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

func statsPanel(p profile.Profile, total int) string {
	s := p.Stats
	kw := clampInt(textArea(total)/2, 10, 16)
	row := func(k, v string) string {
		return design.Label.Width(kw).Render(k) + design.Value.Render(v) + "\n"
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
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

func insightsPanel(p profile.Profile, total int) string {
	tw := textArea(total) - 2
	var b strings.Builder
	b.WriteString(design.Label.Render("SIGNALS & TIPS") + "\n")
	if len(p.Insights) == 0 {
		b.WriteString(design.Dim.Render("No tips — clean run."))
	}
	for i, in := range p.Insights {
		bullet := lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ")
		b.WriteString(bullet + design.Title.Render(in.Title) + "\n")
		b.WriteString(design.Dim.Render(wrap(in.Body, tw)))
		if i < len(p.Insights)-1 {
			b.WriteString("\n\n")
		}
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
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

// wrap hard-wraps text to w columns (rune-aware; keeps words intact).
func wrap(s string, w int) string {
	if w < 1 {
		w = 1
	}
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
