package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/synch/internal/design"
	"github.com/siddham/synch/internal/profile"
)

// View renders the current frame, centered in the terminal.
func (m model) View() string {
	if m.w == 0 {
		return "\n  initializing…"
	}
	var body string
	switch {
	case m.booting:
		body = m.bootView()
	case m.mode == viewSessions:
		body = m.sessionsView()
	case m.mode == viewTrends:
		body = m.trendsView()
	default:
		body = m.overviewView()
	}
	frame := header(m.p, width) + "\n" + body + "\n" + m.footer()
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, frame)
}

// --- boot animation ---

func (m model) bootView() string {
	radar := radarPanelWith(m.p, radarOpts{fraction: m.progress, selected: -1})

	pct := int(m.progress * 100)
	barW := 30
	fill := int(m.progress * float64(barW))
	bar := lipgloss.NewStyle().Foreground(design.Accent).Render(strings.Repeat("▓", fill)) +
		design.Dim.Render(strings.Repeat("░", barW-fill))
	scan := design.Dim.Render("[ INITIALIZING SESSION LEDGER ]")

	// Overall value counts up.
	overall := lipgloss.NewStyle().Foreground(design.ScoreColor(m.p.Overall)).Bold(true).
		Render(fmt.Sprintf("%.1f", m.p.Overall*m.progress))
	right := design.Panel.Width(28).Render(
		design.Label.Render("OVERALL") + "\n" + overall + design.Dim.Render(" / 10") + "\n\n" +
			scan + "\n" + bar + " " + design.Value.Render(fmt.Sprintf("%d%%", pct)))

	return lipgloss.JoinHorizontal(lipgloss.Top, radar, right)
}

// --- overview: radar + summary + dimensions + inspector + telemetry ---

func (m model) overviewView() string {
	opt := radarOpts{fraction: 1, selected: m.dimCursor}
	top := lipgloss.JoinHorizontal(lipgloss.Top, radarPanelWith(m.p, opt), summaryPanel(m.p))
	dims := dimensionsPanelWith(m.p, m.dimCursor)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top,
		inspectorPanel(m.p, m.dimCursor), statsPanel(m.p))
	return top + "\n" + dims + "\n" + bottom
}

// inspectorPanel shows exactly how the selected dimension's score was formed — the
// retained evidence counts + the matching tip. This is the "explainable" surface.
func inspectorPanel(p profile.Profile, idx int) string {
	if idx < 0 || idx >= len(p.Dimensions) {
		return ""
	}
	d := p.Dimensions[idx]
	var b strings.Builder
	b.WriteString(design.Label.Render("INSPECT · ") + design.Title.Render(strings.ToUpper(d.Title)) + "\n")

	if d.Signal.Sufficient {
		b.WriteString(lipgloss.NewStyle().Foreground(design.ScoreColor(d.Signal.Score)).Bold(true).
			Render(fmt.Sprintf("%.1f", d.Signal.Score)) + design.Dim.Render(" / 10   ") +
			design.Dim.Render(fmt.Sprintf("(%d obs)", int(d.Signal.Observations))) + "\n")
	} else {
		b.WriteString(design.Dim.Render("insufficient data — not enough history to grade fairly") + "\n")
	}

	for _, line := range evidenceLines(d) {
		b.WriteString(design.Dim.Render("· ") + line + "\n")
	}
	if tip := tipForDimension(p, d.Dimension); tip != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ") +
			design.Dim.Render(wrap(tip, 40)))
	}
	return design.Panel.Width(width - 34 - 2).Render(strings.TrimRight(b.String(), "\n"))
}

// evidenceLines turns a dimension's retained counts into friendly, auditable lines.
func evidenceLines(d profile.DimensionResult) []string {
	e := d.Signal.Evidence
	num := func(k string) int { return int(e[k]) }
	pct := func(a, b string) string {
		if e[b] == 0 {
			return "0%"
		}
		return fmt.Sprintf("%.0f%%", 100*e[a]/e[b])
	}
	label := func(k, v string) string { return design.Label.Render(k+": ") + design.Value.Render(v) }

	switch d.Dimension {
	case profile.PromptQuality:
		return []string{
			label("prompts", fmt.Sprintf("%d", num("prompts"))),
			label("with file path", pct("prompts_with_path", "prompts")),
			label("with error/trace", pct("prompts_with_error", "prompts")),
			label("with code", pct("prompts_with_code", "prompts")),
			label("vague", fmt.Sprintf("%d", num("prompts_vague"))),
		}
	case profile.IterationEfficiency:
		return []string{
			label("clarification loops", fmt.Sprintf("%d", num("clarification_loops"))),
			label("correction re-prompts", fmt.Sprintf("%d", num("correction_reprompts"))),
			label("over prompts", fmt.Sprintf("%d", num("prompts"))),
		}
	case profile.ToolLeverage:
		return []string{
			label("tool calls", fmt.Sprintf("%d", num("tool_calls"))),
			label("sub-agent calls", fmt.Sprintf("%d", num("subagent_calls"))),
			label("MCP calls", fmt.Sprintf("%d", num("mcp_calls"))),
			label("skill / slash", fmt.Sprintf("%d / %d", num("skill_calls"), num("slash_commands"))),
		}
	case profile.ContextManagement:
		return []string{
			label("/clear used", fmt.Sprintf("%d", num("clears"))),
			label("plan-mode turns", fmt.Sprintf("%d", num("plan_mode_turns"))),
			label("forced compactions", fmt.Sprintf("%d", num("compactions"))),
		}
	case profile.TokenEconomy:
		return []string{
			label("cache read", humanTokens(int64(e["cache_read_tokens"]))),
			label("cache write", humanTokens(int64(e["cache_create_tokens"]))),
			label("output", humanTokens(int64(e["output_tokens"]))),
		}
	}
	return nil
}

func tipForDimension(p profile.Profile, d profile.Dimension) string {
	for _, in := range p.Insights {
		if in.Dimension == d {
			return in.Title + " — " + in.Body
		}
	}
	return ""
}

// --- sessions: list + selected detail ---

func (m model) sessionsView() string {
	if len(m.p.Sessions) == 0 {
		return design.Panel.Width(width - 2).Render(design.Dim.Render("No per-session data."))
	}
	var list strings.Builder
	list.WriteString(design.Label.Render("SESSIONS") + "\n")
	// Window the list to a viewport around the cursor.
	const rows = 14
	start := clampInt(m.sessCursor-rows/2, 0, maxInt(0, len(m.p.Sessions)-rows))
	end := minInt(start+rows, len(m.p.Sessions))
	for i := start; i < end; i++ {
		s := m.p.Sessions[i]
		cursor := "  "
		nameStyle := design.Label
		if i == m.sessCursor {
			cursor = lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ")
			nameStyle = lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
		}
		var sc string
		if s.Overall > 0 {
			sc = lipgloss.NewStyle().Foreground(design.ScoreColor(s.Overall)).Render(fmt.Sprintf("%4.1f", s.Overall))
		} else {
			sc = design.Dim.Render("   —") // ungradeable session (too little data)
		}
		list.WriteString(cursor + sc + " " + nameStyle.Render(truncate(s.Label, 26)) + "\n")
	}
	listPanel := design.Panel.Width(40).Render(strings.TrimRight(list.String(), "\n"))

	detail := m.sessionDetail(m.p.Sessions[m.sessCursor])
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detail)
}

func (m model) sessionDetail(s profile.SessionSummary) string {
	var b strings.Builder
	b.WriteString(design.Label.Render("SESSION · ") + design.Title.Render(truncate(s.Label, 22)) + "\n")

	var overall string
	if s.Overall > 0 {
		overall = lipgloss.NewStyle().Foreground(design.ScoreColor(s.Overall)).Bold(true).Render(fmt.Sprintf("%.1f", s.Overall))
	} else {
		overall = design.Dim.Render("—")
	}
	b.WriteString(design.Label.Render("overall ") + overall +
		design.Dim.Render(fmt.Sprintf("   %d prompts · %s tok", s.Prompts, humanTokens(s.Tokens))) + "\n")

	// Radar on top, centered; breakdown rows below — avoids side-by-side wrap.
	radar := radarBlockWith(summaryDimensions(s), 28, 28, radarOpts{fraction: 1, selected: -1})
	b.WriteString("\n" + radar + "\n\n")

	for _, d := range profile.Order {
		v, ok := s.Dimensions[d]
		var val, bar string
		if ok {
			st := lipgloss.NewStyle().Foreground(design.ScoreColor(v))
			bar = st.Render(design.Bar(v, 12))
			val = st.Render(fmt.Sprintf(" %4.1f", v))
		} else {
			bar = design.Dim.Render(strings.Repeat("░", 12))
			val = design.Dim.Render("  n/a")
		}
		b.WriteString(design.Label.Width(16).Render(shortDim(d)) + bar + val + "\n")
	}
	return design.Panel.Width(width - 40 - 2).Render(strings.TrimRight(b.String(), "\n"))
}

// --- trends: chronological sparklines ---

func (m model) trendsView() string {
	sessions := m.p.Sessions
	if len(sessions) < 2 {
		return design.Panel.Width(width - 2).Render(
			design.Dim.Render("Not enough sessions yet for a trend. Come back after a few more."))
	}
	overall := make([]float64, len(sessions))
	for i, s := range sessions {
		overall[i] = s.Overall
	}

	var b strings.Builder
	b.WriteString(design.Label.Render("JOURNEY") + design.Dim.Render(fmt.Sprintf("  · %d sessions, oldest → newest", len(sessions))) + "\n\n")
	b.WriteString(design.Label.Width(18).Render("OVERALL") +
		sparkline(overall, lipgloss.NewStyle().Foreground(design.Accent)) + "  " + trendArrow(overall) + "\n\n")

	for _, d := range profile.Order {
		vals := make([]float64, len(sessions))
		for i, s := range sessions {
			vals[i] = s.Dimensions[d] // 0 if not sufficient => gap dot
		}
		style := lipgloss.NewStyle().Foreground(design.Muted)
		b.WriteString(design.Label.Width(18).Render(shortDim(d)) +
			sparkline(vals, style) + "  " + trendArrow(vals) + "\n")
	}

	// Timeline strip: one score-colored block per session.
	b.WriteString("\n" + design.Label.Render("TIMELINE") + "  ")
	for _, s := range sessions {
		b.WriteString(lipgloss.NewStyle().Foreground(design.ScoreColor(s.Overall)).Render("▉"))
	}
	return design.Panel.Width(width - 2).Render(strings.TrimRight(b.String(), "\n"))
}

// --- footer ---

func (m model) footer() string {
	tabs := []string{"1 OVERVIEW", "2 SESSIONS", "3 TRENDS"}
	var rendered []string
	for i, t := range tabs {
		if viewMode(i) == m.mode {
			rendered = append(rendered, lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render("["+t+"]"))
		} else {
			rendered = append(rendered, design.Dim.Render(" "+t+" "))
		}
	}
	left := strings.Join(rendered, " ")
	keys := design.Dim.Render("↑↓ select · ←→/tab view · r replay · q quit")
	gap := width - lipgloss.Width(left) - lipgloss.Width(keys)
	if gap < 1 {
		gap = 1
	}
	line := lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("─", width))
	return line + "\n" + left + strings.Repeat(" ", gap) + keys
}

// --- helpers ---

// summaryDimensions builds DimensionResults from a session summary so the radar
// renderer can draw a per-session chart.
func summaryDimensions(s profile.SessionSummary) []profile.DimensionResult {
	out := make([]profile.DimensionResult, 0, len(profile.Order))
	for _, d := range profile.Order {
		v, ok := s.Dimensions[d]
		out = append(out, profile.DimensionResult{
			Dimension: d, Title: d.Title(),
			Signal: profile.Signal{Score: v, Sufficient: ok},
		})
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
