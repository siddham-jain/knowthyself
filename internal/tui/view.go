package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham/synch/internal/design"
	"github.com/siddham/synch/internal/profile"
)

// layout captures the responsive decisions for the current terminal width.
type layout struct {
	w      int  // dashboard content width (rendered)
	twoCol bool // place paired panels side by side (else stack vertically)
}

func (m model) layout() layout {
	w := clampInt(m.w-2, minWidth, maxWidth)
	return layout{w: w, twoCol: w >= twoColMin}
}

// easeOutCubic decelerates the boot animation so it settles smoothly (no bounce —
// this is instrumentation, not a toy).
func easeOutCubic(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	u := 1 - t
	return 1 - u*u*u
}

// View renders the current frame, centered in the terminal.
func (m model) View() string {
	if m.w == 0 || m.h == 0 {
		return "\n  initializing…"
	}
	if m.w < minWidth {
		msg := design.Dim.Render("⚠ terminal too narrow\n  widen to ≥ " +
			fmt.Sprintf("%d", minWidth+2) + " columns")
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, msg)
	}

	lay := m.layout()
	// The reveal is a full-screen first-run portrait with its own minimal footer.
	if m.atReveal && !m.booting {
		frame := header(m.p, lay.w) + "\n" + m.revealView(lay) + "\n" + m.revealFooter(lay.w)
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, frame)
	}

	var body string
	switch {
	case m.booting:
		body = m.bootView(lay)
	case m.mode == viewSessions:
		body = m.sessionsView(lay)
	case m.mode == viewTrends:
		body = m.trendsView(lay)
	default:
		body = m.overviewView(lay)
	}
	frame := header(m.p, lay.w) + "\n" + body + "\n" + m.footer(lay.w)
	return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, frame)
}

// stack places two panels side by side when there's room, else vertically. leftW is
// the rendered width of the left panel in two-column mode.
func (lay layout) stack(left, right func(total int) string, leftW int) string {
	if lay.twoCol {
		return lipgloss.JoinHorizontal(lipgloss.Top, left(leftW), right(lay.w-leftW))
	}
	return lipgloss.JoinVertical(lipgloss.Left, left(lay.w), right(lay.w))
}

// --- boot animation ---

func (m model) bootView(lay layout) string {
	e := easeOutCubic(m.progress)
	radar := func(total int) string {
		return radarPanelWith(m.p, radarOpts{fraction: e, selected: -1}, total)
	}
	gauge := func(total int) string {
		barW := clampInt(textArea(total)-8, 8, 30)
		fill := int(e * float64(barW))
		bar := lipgloss.NewStyle().Foreground(design.Accent).Render(strings.Repeat("▓", fill)) +
			design.Dim.Render(strings.Repeat("░", barW-fill))
		overall := lipgloss.NewStyle().Foreground(design.ScoreColor(m.p.Overall)).Bold(true).
			Render(fmt.Sprintf("%.1f", m.p.Overall*e))
		body := design.Label.Render("OVERALL") + "\n" + overall + design.Dim.Render(" / 10") + "\n\n" +
			design.Dim.Render("[ INITIALIZING SESSION LEDGER ]") + "\n" +
			bar + " " + design.Value.Render(fmt.Sprintf("%d%%", int(e*100)))
		return panelBox(total).Render(body)
	}
	return lay.stack(radar, gauge, 48)
}

// --- overview ---

func (m model) overviewView(lay layout) string {
	radar := func(total int) string {
		return radarPanelWith(m.p, radarOpts{fraction: 1, selected: m.dimCursor}, total)
	}
	summary := func(total int) string { return summaryPanel(m.p, total) }
	inspector := func(total int) string { return inspectorPanel(m.p, m.dimCursor, total) }
	stats := func(total int) string { return statsPanel(m.p, total) }

	top := lay.stack(radar, summary, radarWidth(lay))
	dims := dimensionsPanel(m.p, m.dimCursor, lay.w)
	bottom := lay.stack(stats, inspector, 34)
	return top + "\n" + dims + "\n" + bottom
}

// radarWidth picks the radar panel's width in two-column mode, leaving the rest for
// the summary.
func radarWidth(lay layout) int {
	return clampInt(lay.w/2+6, 44, 52)
}

// inspectorPanel shows exactly how the selected dimension's score was formed — the
// retained evidence counts + the matching tip. The "explainable" surface.
func inspectorPanel(p profile.Profile, idx, total int) string {
	if idx < 0 || idx >= len(p.Dimensions) {
		idx = 0
	}
	d := p.Dimensions[idx]
	tw := textArea(total)
	var b strings.Builder
	b.WriteString(design.Label.Render("INSPECT · ") + design.Title.Render(strings.ToUpper(d.Title)) + "\n")

	if d.Signal.Sufficient {
		b.WriteString(lipgloss.NewStyle().Foreground(design.ScoreColor(d.Signal.Score)).Bold(true).
			Render(fmt.Sprintf("%.1f", d.Signal.Score)) + design.Dim.Render(" / 10   ") +
			design.Dim.Render(fmt.Sprintf("(%d obs)", int(d.Signal.Observations))) + "\n")
	} else {
		b.WriteString(design.Dim.Render(wrap("insufficient data — not enough history to grade fairly", tw)) + "\n")
	}
	for _, line := range evidenceLines(d) {
		b.WriteString(design.Dim.Render("· ") + line + "\n")
	}
	if tip := tipForDimension(p, d.Dimension); tip != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ") +
			design.Dim.Render(wrap(tip, tw-2)))
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

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

// --- sessions ---

func (m model) sessionsView(lay layout) string {
	if len(m.p.Sessions) == 0 {
		return panelBox(lay.w).Render(design.Dim.Render("No per-session data."))
	}
	listW := 40
	if !lay.twoCol {
		listW = lay.w
	}
	list := m.sessionList(listW)
	detailW := lay.w - 40
	if !lay.twoCol {
		detailW = lay.w
	}
	detail := m.sessionDetail(m.p.Sessions[m.sessCursor], detailW)
	if lay.twoCol {
		return lipgloss.JoinHorizontal(lipgloss.Top, list, detail)
	}
	return lipgloss.JoinVertical(lipgloss.Left, list, detail)
}

func (m model) sessionList(total int) string {
	nameW := clampInt(textArea(total)-8, 12, 30)
	var b strings.Builder
	b.WriteString(design.Label.Render("SESSIONS") + "\n")
	const rows = 12
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
			sc = design.Dim.Render("   —")
		}
		b.WriteString(cursor + sc + " " + nameStyle.Render(truncate(s.Label, nameW)) + "\n")
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

func (m model) sessionDetail(s profile.SessionSummary, total int) string {
	area := textArea(total)
	var b strings.Builder
	b.WriteString(design.Label.Render("SESSION · ") + design.Title.Render(truncate(s.Label, maxInt(8, area-12))) + "\n")

	var overall string
	if s.Overall > 0 {
		overall = lipgloss.NewStyle().Foreground(design.ScoreColor(s.Overall)).Bold(true).Render(fmt.Sprintf("%.1f", s.Overall))
	} else {
		overall = design.Dim.Render("—")
	}
	b.WriteString(design.Label.Render("overall ") + overall +
		design.Dim.Render(fmt.Sprintf("   %d prompts · %s tok", s.Prompts, humanTokens(s.Tokens))) + "\n")

	radarCells := clampInt(area-4, 12, 20)
	radar := radarBlockWith(summaryDimensions(s), radarCells*2, radarCells*2, radarOpts{fraction: 1, selected: -1})
	b.WriteString("\n" + radar + "\n\n")

	barW := clampInt(area-24, 8, 16)
	for _, d := range profile.Order {
		v, ok := s.Dimensions[d]
		var val, bar string
		if ok {
			st := lipgloss.NewStyle().Foreground(design.ScoreColor(v))
			bar = st.Render(design.Bar(v, barW))
			val = st.Render(fmt.Sprintf(" %4.1f", v))
		} else {
			bar = design.Dim.Render(strings.Repeat("░", barW))
			val = design.Dim.Render("  n/a")
		}
		b.WriteString(design.Label.Width(16).Render(shortDim(d)) + bar + val + "\n")
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

// --- trends ---

func (m model) trendsView(lay layout) string {
	sessions := m.p.Sessions
	if len(sessions) < 2 {
		return panelBox(lay.w).Render(
			design.Dim.Render(wrap("Not enough sessions yet for a trend. Come back after a few more.", textArea(lay.w))))
	}
	labelW := 18
	// Sparkline gets whatever's left after the label and the trend arrow (~9).
	sparkMax := clampInt(textArea(lay.w)-labelW-10, 8, 60)

	overall := seriesOverall(sessions)
	var b strings.Builder
	b.WriteString(design.Label.Render("JOURNEY") +
		design.Dim.Render(fmt.Sprintf("  · %d sessions, oldest → newest", len(sessions))) + "\n\n")
	b.WriteString(design.Label.Width(labelW).Render("OVERALL") +
		sparkline(fit(overall, sparkMax), lipgloss.NewStyle().Foreground(design.Accent)) +
		"  " + trendArrow(overall) + "\n\n")
	for _, d := range profile.Order {
		vals := seriesDim(sessions, d)
		b.WriteString(design.Label.Width(labelW).Render(shortDim(d)) +
			sparkline(fit(vals, sparkMax), lipgloss.NewStyle().Foreground(design.Muted)) +
			"  " + trendArrow(vals) + "\n")
	}
	b.WriteString("\n" + design.Label.Render("TIMELINE") + "  ")
	for _, s := range fitTimeline(sessions, sparkMax) {
		b.WriteString(lipgloss.NewStyle().Foreground(design.ScoreColor(s)).Render("▉"))
	}
	return panelBox(lay.w).Render(strings.TrimRight(b.String(), "\n"))
}

func seriesOverall(ss []profile.SessionSummary) []float64 {
	out := make([]float64, len(ss))
	for i, s := range ss {
		out[i] = s.Overall
	}
	return out
}

func seriesDim(ss []profile.SessionSummary, d profile.Dimension) []float64 {
	out := make([]float64, len(ss))
	for i, s := range ss {
		out[i] = s.Dimensions[d] // 0 => rendered as a gap
	}
	return out
}

// fit downsamples a series to at most n points by averaging buckets, so long
// histories still fit a narrow terminal without overflow.
func fit(vals []float64, n int) []float64 {
	if len(vals) <= n || n < 1 {
		return vals
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		lo := i * len(vals) / n
		hi := (i + 1) * len(vals) / n
		if hi <= lo {
			hi = lo + 1
		}
		var sum float64
		var cnt int
		for j := lo; j < hi && j < len(vals); j++ {
			if vals[j] > 0 {
				sum += vals[j]
				cnt++
			}
		}
		if cnt > 0 {
			out[i] = sum / float64(cnt)
		}
	}
	return out
}

func fitTimeline(ss []profile.SessionSummary, n int) []float64 {
	return fit(seriesOverall(ss), n)
}

// --- footer ---

func (m model) footer(w int) string {
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
	line := lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("─", w))

	gap := w - lipgloss.Width(left) - lipgloss.Width(keys)
	if gap < 1 {
		// Not enough room on one line: stack keys under the tabs.
		short := design.Dim.Render("tab view · q quit")
		if w-lipgloss.Width(left) >= lipgloss.Width(short)+1 {
			g := w - lipgloss.Width(left) - lipgloss.Width(short)
			return line + "\n" + left + strings.Repeat(" ", g) + short
		}
		return line + "\n" + left + "\n" + keys
	}
	return line + "\n" + left + strings.Repeat(" ", gap) + keys
}

// --- helpers ---

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
	if n < 1 {
		n = 1
	}
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
