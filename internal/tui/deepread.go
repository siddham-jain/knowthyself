package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/siddham-jain/knowthyself/internal/design"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// The deep read is model-judged, on its own anchored scale. It must never be
// mistaken for the deterministic radar, so the panel states the model, the sample
// size and the confidence, and the scale is labelled on every row.

// MaxLevelFallback is used only when a stored read carries no per-criterion Max,
// which older cache entries may not.
const MaxLevelFallback = 4

func (m model) deepReadView(lay layout) string {
	dr := m.p.DeepRead
	if dr == nil {
		return panelBox(lay.w).Render(design.Dim.Render(wrap(
			"No deep read yet. Run `knowthyself --deep-eval` with your own API key to add a written read of how you phrase things. Your scores stay local either way.",
			textArea(lay.w))))
	}

	criteria := func(total int) string { return criteriaPanel(dr, total) }
	findings := func(total int) string { return findingsPanel(dr, total) }
	return lay.stack(criteria, findings, radarWidth(lay))
}

func criteriaPanel(dr *profile.DeepRead, total int) string {
	area := textArea(total)
	nameW := clampInt(area/2, 16, 26)
	barW := clampInt(area-nameW-8, 6, 32)

	scale := MaxLevelFallback
	if len(dr.Criteria) > 0 && dr.Criteria[0].Max > 0 {
		scale = dr.Criteria[0].Max
	}

	var b strings.Builder
	b.WriteString(design.Label.Render("DEEP READ") + " " +
		design.Dim.Render(fmt.Sprintf("· model-judged · 0–%d scale", scale)) + "\n")

	for _, c := range dr.Criteria {
		// Truncate one short of the column so the label never touches the meter.
		name := lipgloss.NewStyle().Foreground(design.Ink).Width(nameW).Render(truncate(c.Title, nameW-1))
		if c.Judged == 0 {
			b.WriteString(name + design.Dim.Render(strings.Repeat("░", barW)) + design.Dim.Render("  n/a") + "\n")
			continue
		}
		// Scale the 0..MaxLevel mean onto the shared meter without restating it as a
		// 0..10 score, which would invite comparison with the radar.
		frac := c.Mean / float64(c.Max)
		st := lipgloss.NewStyle().Foreground(design.ScoreColor(frac * 10))
		b.WriteString(name +
			st.Render(design.Bar(frac*10, barW)) +
			st.Bold(true).Render(fmt.Sprintf(" %3.1f", c.Mean)) +
			design.Dim.Render(fmt.Sprintf("/%d", c.Max)) + "\n")
	}

	b.WriteString("\n" + design.Dim.Render(wrap(
		fmt.Sprintf("judged by %s over %d prompts from %d sessions (%d available) · %s confidence",
			dr.Model, dr.Sample.Prompts, dr.Sample.Sessions, dr.Sample.Available, dr.Confidence), area)))
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}

func findingsPanel(dr *profile.DeepRead, total int) string {
	tw := textArea(total) - 2
	var b strings.Builder
	b.WriteString(design.Label.Render("WHAT TO CHANGE") + "\n")
	if len(dr.Findings) == 0 {
		b.WriteString(design.Dim.Render(wrap("No findings — nothing stood out worth changing.", tw)))
		return panelBox(total).Render(b.String())
	}
	for i, f := range dr.Findings {
		b.WriteString(lipgloss.NewStyle().Foreground(design.Accent).Render("▸ ") + design.Title.Render(f.Title) + "\n")
		b.WriteString(design.Dim.Render(wrap(f.Body, tw)))
		if i < len(dr.Findings)-1 {
			b.WriteString("\n\n")
		}
	}
	return panelBox(total).Render(strings.TrimRight(b.String(), "\n"))
}
