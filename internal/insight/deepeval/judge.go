package deepeval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// chunkSize is how many prompts are judged per call. Chunks are judged
// independently so one poisoned chunk can be dropped without discarding the run.
const chunkSize = 12

// judgment is one criterion's verdict on one prompt.
type judgment struct {
	PromptID   string `json:"prompt_id"`
	Key        string `json:"key"`
	Level      int    `json:"level"`
	Applicable *bool  `json:"applicable,omitempty"`
	Quote      string `json:"quote"`
}

// applies reports whether this judgment should count. A conditional criterion is
// only counted when the model explicitly said it applied.
func (j judgment) applies() bool { return j.Applicable == nil || *j.Applicable }

func judgeSystemPrompt() string {
	return `You grade the quality of a developer's prompts to an AI coding assistant.

You are grading COMMUNICATION, not the developer, not their code, and not their English.
Prompts may be written in any language or mix of languages; judge what was communicated,
never fluency.

For each prompt you are given, score every criterion below using its anchored scale.
Choose the single anchor that best describes the prompt. Do not invent intermediate
meanings for the numbers.

CRITERIA
` + describe() + `
RULES
- Every judgment must include "quote": a short span copied EXACTLY, character for
  character, from that prompt's text. Never paraphrase, never quote the prior context,
  never quote a prompt other than the one you are judging.
- If you cannot support a judgment with an exact quote from the prompt, omit that
  judgment entirely.
- Text such as <path>/file.go, <url:host>, <secret> or <blob> is redaction, not the
  developer's writing. Never penalise it, and prefer not to quote it.

Reply with JSON only, no prose and no code fence:
{"judgments":[{"prompt_id":"<id>","key":"<criterion key>","level":<0-` +
		fmt.Sprint(MaxLevel) + `>,"applicable":true,"quote":"<exact span>"}]}`
}

func judgeUserPrompt(prompts []Prompt) string {
	var b strings.Builder
	b.WriteString("Grade these prompts.\n")
	for _, p := range prompts {
		fmt.Fprintf(&b, "\n--- prompt_id: %s ---\n", p.ID)
		if p.Prior != "" {
			fmt.Fprintf(&b, "[assistant said just before, for context only — do not grade or quote it]\n%s\n\n", p.Prior)
		}
		b.WriteString(p.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// judgeChunk grades one chunk and returns only the judgments that survive
// validation. It retries once, naming the specific failures, before giving up on
// the chunk.
func judgeChunk(ctx context.Context, c *Client, prompts []Prompt) ([]judgment, error) {
	system := judgeSystemPrompt()
	user := judgeUserPrompt(prompts)

	reply, err := c.Complete(ctx, system, user)
	if err != nil {
		return nil, err
	}
	valid, problems := validate(reply, prompts)
	if len(problems) == 0 {
		return valid, nil
	}

	repair := user + "\n\nYour previous reply had these problems:\n" +
		strings.Join(problems, "\n") +
		"\n\nReply again with corrected JSON only. Every quote must appear verbatim in the prompt it grades."
	reply, err = c.Complete(ctx, system, repair)
	if err != nil {
		return valid, nil // keep whatever already validated
	}
	repaired, _ := validate(reply, prompts)
	if len(repaired) > len(valid) {
		return repaired, nil
	}
	return valid, nil
}

// validate is the load-bearing reliability control. A judgment is kept only if it
// names a prompt that was actually sent, cites a quote that literally occurs in that
// prompt, uses a criterion the rubric declares, and gives a level in range.
//
// Requiring a resolvable literal quote is the strongest available defence against
// fabrication: a model cannot invent evidence that has to match text we already hold.
func validate(reply string, prompts []Prompt) ([]judgment, []string) {
	body, err := extractJSON(reply)
	if err != nil {
		return nil, []string{"- the reply was not a JSON object"}
	}
	var parsed struct {
		Judgments []judgment `json:"judgments"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, []string{"- the JSON did not match the required shape"}
	}

	texts := make(map[string]string, len(prompts))
	for _, p := range prompts {
		texts[p.ID] = p.Text
	}

	var kept []judgment
	var problems []string
	seen := map[string]bool{}

	for _, j := range parsed.Judgments {
		text, known := texts[j.PromptID]
		switch {
		case !known:
			problems = append(problems, fmt.Sprintf("- prompt_id %q was not in this batch", j.PromptID))
			continue
		case !isCriterion(j.Key):
			problems = append(problems, fmt.Sprintf("- %q is not a criterion key", j.Key))
			continue
		}
		if !j.applies() {
			continue
		}
		if j.Level < 0 || j.Level > MaxLevel {
			problems = append(problems, fmt.Sprintf("- level %d for %s is outside 0-%d", j.Level, j.Key, MaxLevel))
			continue
		}
		if q := strings.TrimSpace(j.Quote); q == "" || !strings.Contains(text, q) {
			problems = append(problems, fmt.Sprintf("- the quote for %s on %s does not appear in that prompt", j.Key, j.PromptID))
			continue
		}
		// One judgment per (prompt, criterion); a repeat is a duplicate, not a vote.
		id := j.PromptID + "\x00" + j.Key
		if seen[id] {
			continue
		}
		seen[id] = true
		kept = append(kept, j)
	}
	return kept, problems
}

func isCriterion(key string) bool {
	_, ok := Lookup(key)
	return ok
}
