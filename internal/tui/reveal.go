package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// tokensPerNovel is a deliberately round, clearly-approximate yardstick for turning
// a token count into something human ("≈ N novels"). ~90k words ≈ ~120k tokens.
const tokensPerNovel = 120_000

// revealView is the first-run "reveal": a celebratory identity portrait — the
// collaboration persona plus by-the-numbers scale — that the boot animation lands
// on. It is intentionally free of any judgmental/coach content; the graded
// dashboard is one keystroke away.
func (m model) revealView(lay layout) string {
	rows := clampInt(lay.h-8, 4, 14)
	radar := func(total int) string {
		return radarPanelWith(m.p, radarOpts{fraction: 1, selected: -1, maxRows: rows}, total)
	}
	hero := func(total int) string { return revealHero(m.p, total) }
	return lay.stack(radar, hero, radarWidth(lay))
}

// revealHero renders the persona headline, trait badges, and the "by the numbers"
// band. The archetype name is the star; every stat is framed as a reveal.
func revealHero(p profile.Profile, total int) string {
	area := textArea(total)
	a := p.Archetype
	var b strings.Builder

	b.WriteString(design.Dim.Render("YOU ARE A") + "\n")
	name := lipgloss.NewStyle().Foreground(design.Accent).Bold(true).Render(strings.ToUpper(a.Name))
	b.WriteString(lipgloss.NewStyle().Foreground(design.Accent).Render("▏") + name + "\n")
	if a.Blurb != "" {
		b.WriteString(design.Label.Render(wrap(a.Blurb, area)) + "\n")
	}
	if badges := traitBadges(a.Traits, 2); badges != "" {
		b.WriteString("\n" + badges + "\n")
	}

	nums := revealNumbers(p.Stats)
	if len(nums) > 0 {
		b.WriteString("\n")
		for _, line := range nums {
			b.WriteString(lipgloss.NewStyle().Foreground(design.Accent).Render("◆ ") + line + "\n")
		}
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

// traitBadges renders up to max deterministic badges as small technical tags.
func traitBadges(traits []string, max int) string {
	if len(traits) == 0 {
		return ""
	}
	if len(traits) > max {
		traits = traits[:max]
	}
	bracket := lipgloss.NewStyle().Foreground(design.Faint)
	tag := lipgloss.NewStyle().Foreground(design.Accent).Bold(true)
	var parts []string
	for _, t := range traits {
		parts = append(parts, bracket.Render("[")+tag.Render(" "+strings.ToUpper(t)+" ")+bracket.Render("]"))
	}
	return strings.Join(parts, " ")
}

// revealNumbers turns the headline Stats into the celebratory "by the numbers"
// lines. Every value is positive framing; empty inputs are simply omitted.
func revealNumbers(st profile.Stats) []string {
	value := lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
	label := design.Label
	var out []string

	out = append(out, value.Render(commas(st.Sessions))+label.Render(" conversations"))
	if st.CollabSeconds > 0 {
		out = append(out, value.Render(hoursLabel(st.CollabSeconds))+label.Render(" together"))
	}
	if st.TotalTokens > 0 {
		line := value.Render(humanTokens(st.TotalTokens)) + label.Render(" tokens")
		// The analogy only helps while the count is graspable; past a point the raw
		// figure is the more honest flex.
		if nov := st.TotalTokens / tokensPerNovel; nov >= 1 && nov < 1000 {
			line += label.Render(" · ≈") + value.Render(commas(int(nov))) + label.Render(" novels")
		}
		out = append(out, line)
	}
	if pn := projectsSince(st); pn != "" {
		out = append(out, pn)
	}
	if pl := peakAndLangs(st); pl != "" {
		out = append(out, pl)
	}
	if st.CacheHitRate > 0 {
		out = append(out, value.Render(fmt.Sprintf("%.0f%%", st.CacheHitRate*100))+
			label.Render(" cache — ")+design.Value.Render(cacheLabel(st.CacheHitRate)))
	}
	return out
}

func projectsSince(st profile.Stats) string {
	value := lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
	var parts []string
	if st.Projects > 0 {
		parts = append(parts, value.Render(fmt.Sprintf("%d", st.Projects))+design.Label.Render(" projects"))
	}
	if !st.FirstSessionAt.IsZero() {
		parts = append(parts, design.Label.Render("since ")+value.Render(st.FirstSessionAt.Format("Jan 2006")))
	}
	return strings.Join(parts, design.Dim.Render(" · "))
}

func peakAndLangs(st profile.Stats) string {
	value := lipgloss.NewStyle().Foreground(design.Ink).Bold(true)
	var parts []string
	if st.PeakHour >= 0 {
		parts = append(parts, design.Label.Render("peak ")+value.Render(hourLabel(st.PeakHour)))
	}
	if len(st.Languages) > 0 {
		langParts := make([]string, len(st.Languages))
		for i, l := range st.Languages {
			langParts[i] = value.Render(l)
		}
		parts = append(parts, strings.Join(langParts, design.Label.Render(" + ")))
	}
	return strings.Join(parts, design.Dim.Render(" · "))
}

// revealFooter is a minimal hint line for the reveal screen — no dashboard tabs yet,
// just the invitation to go deeper.
func (m model) revealFooter(w int) string {
	line := lipgloss.NewStyle().Foreground(design.Faint).Render(strings.Repeat("─", w))
	hint := lipgloss.NewStyle().Foreground(design.Accent).Render("→") +
		design.Dim.Render(" full breakdown") + design.Dim.Render("   ·   press any key   ·   q quit")
	gap := w - lipgloss.Width(hint)
	if gap < 0 {
		hint = design.Dim.Render("→ breakdown · q quit")
		gap = w - lipgloss.Width(hint)
	}
	if gap < 0 {
		gap = 0
	}
	return line + "\n" + hint + strings.Repeat(" ", gap)
}

// commas formats an integer with thousands separators (deterministic, locale-free).
func commas(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// hoursLabel renders a duration (in seconds) as a friendly span: minutes under an
// hour, one decimal up to ~10h, whole hours beyond.
func hoursLabel(sec int64) string {
	if sec < 3600 {
		m := sec / 60
		if m < 1 {
			m = 1
		}
		return fmt.Sprintf("%d min", m)
	}
	h := float64(sec) / 3600
	if h < 10 {
		return fmt.Sprintf("%.1f hrs", h)
	}
	return fmt.Sprintf("%s hrs", commas(int(h+0.5)))
}

// hourLabel renders a 0..23 hour as a 12-hour clock label, e.g. 23 → "11PM".
func hourLabel(h int) string {
	switch {
	case h == 0:
		return "12AM"
	case h < 12:
		return fmt.Sprintf("%dAM", h)
	case h == 12:
		return "12PM"
	default:
		return fmt.Sprintf("%dPM", h-12)
	}
}

// cacheLabel gives cache efficiency a positive, non-judgmental grade.
func cacheLabel(rate float64) string {
	switch {
	case rate >= 0.9:
		return "top-tier"
	case rate >= 0.75:
		return "strong"
	case rate >= 0.5:
		return "solid"
	default:
		return "warming up"
	}
}
