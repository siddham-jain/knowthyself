package deepeval

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/siddham-jain/knowthyself/internal/profile"
)

// maxFindings caps the synthesised advice. Four is what fits the panel and what a
// person will actually act on.
const maxFindings = 4

// Coverage thresholds on how much of the sample survived validation. Below
// abandonAt the read is not reported at all.
const (
	confidentAt = 0.8
	usableAt    = 0.6
	abandonAt   = 0.3
)

// aggregate reduces per-prompt judgments to one result per criterion. A conditional
// criterion divides by the number of prompts it applied to, never by the sample size.
func aggregate(judgments []judgment) []profile.CriterionResult {
	sums := map[string]int{}
	counts := map[string]int{}
	for _, j := range judgments {
		sums[j.Key] += j.Level
		counts[j.Key]++
	}

	out := make([]profile.CriterionResult, 0, len(Rubric))
	for _, c := range Rubric {
		n := counts[c.Key]
		var mean float64
		if n > 0 {
			mean = float64(sums[c.Key]) / float64(n)
		}
		out = append(out, profile.CriterionResult{
			Key:    c.Key,
			Title:  c.Title,
			Mean:   mean,
			Max:    MaxLevel,
			Judged: n,
		})
	}
	return out
}

// coverage is the share of the sample that produced at least one valid judgment.
func coverage(judgments []judgment, sample Sample) float64 {
	if len(sample.Prompts) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, j := range judgments {
		seen[j.PromptID] = true
	}
	return float64(len(seen)) / float64(len(sample.Prompts))
}

func confidenceFor(c float64) string {
	switch {
	case c >= confidentAt:
		return profile.ConfidenceHigh
	case c >= usableAt:
		return profile.ConfidenceMedium
	default:
		return profile.ConfidenceLow
	}
}

// weakest returns the criteria with the lowest means, worst first, ignoring any that
// were never judged. Advice is worth most where the score is lowest.
func weakest(results []profile.CriterionResult, n int) []profile.CriterionResult {
	var judged []profile.CriterionResult
	for _, r := range results {
		if r.Judged > 0 {
			judged = append(judged, r)
		}
	}
	sort.SliceStable(judged, func(i, j int) bool { return judged[i].Mean < judged[j].Mean })
	if len(judged) > n {
		judged = judged[:n]
	}
	return judged
}

func synthesisSystemPrompt() string {
	return `You write short, concrete coaching for a developer, based on graded evidence from
their own prompts to an AI coding assistant.

Rules:
- Address the developer as "you". Be specific and practical. No praise padding, no
  hedging, no restating the score back to them.
- Each finding must be something they can do differently on their very next prompt.
- Ground every finding in the quoted evidence you are given. Never invent a quote.
- Body must be at most two sentences.

Reply with JSON only, no prose and no code fence:
{"findings":[{"criterion":"<criterion key>","title":"<max 6 words>","body":"<max 2 sentences>"}]}`
}

func synthesisUserPrompt(results []profile.CriterionResult, judgments []judgment, sample Sample) string {
	texts := map[string]string{}
	for _, p := range sample.Prompts {
		texts[p.ID] = p.Text
	}

	var b strings.Builder
	b.WriteString("Graded results (0 worst, ")
	fmt.Fprintf(&b, "%d best):\n", MaxLevel)
	for _, r := range results {
		if r.Judged == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s (%s): mean %.1f over %d prompts\n", r.Title, r.Key, r.Mean, r.Judged)
	}

	targets := weakest(results, maxFindings)
	b.WriteString("\nWrite one finding for each of these weakest criteria, worst first:\n")
	for _, t := range targets {
		fmt.Fprintf(&b, "- %s (%s)\n", t.Title, t.Key)
		for _, ex := range examplesFor(judgments, t.Key, 3) {
			fmt.Fprintf(&b, "    level %d, they wrote: %q\n", ex.Level, ex.Quote)
		}
	}
	return b.String()
}

// examplesFor returns the lowest-scoring quoted evidence for a criterion.
func examplesFor(judgments []judgment, key string, n int) []judgment {
	var out []judgment
	for _, j := range judgments {
		if j.Key == key {
			out = append(out, j)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Level < out[j].Level })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// synthesise asks for the findings and keeps only those naming a real criterion.
// Failure here is not fatal: the criterion scores stand on their own.
func synthesise(ctx context.Context, c *Client, results []profile.CriterionResult, judgments []judgment, sample Sample) []profile.Insight {
	reply, err := c.Complete(ctx, synthesisSystemPrompt(), synthesisUserPrompt(results, judgments, sample))
	if err != nil {
		return nil
	}
	body, err := extractJSON(reply)
	if err != nil {
		return nil
	}
	var parsed struct {
		Findings []struct {
			Criterion string `json:"criterion"`
			Title     string `json:"title"`
			Body      string `json:"body"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}

	var out []profile.Insight
	for _, f := range parsed.Findings {
		if !isCriterion(f.Criterion) || strings.TrimSpace(f.Title) == "" || strings.TrimSpace(f.Body) == "" {
			continue
		}
		out = append(out, profile.Insight{
			Title:  strings.TrimSpace(f.Title),
			Body:   strings.TrimSpace(f.Body),
			Source: "deep-eval",
		})
		if len(out) >= maxFindings {
			break
		}
	}
	return out
}
