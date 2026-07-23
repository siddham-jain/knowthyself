// Package deepeval implements the optional, opt-in model-judged read of a
// developer's actual prompt text (--deep-eval).
//
// The deterministic scorers in internal/score measure structure: whether a prompt
// carries a path, a code fence, an error trace. They are deliberately blind to
// meaning, and so cannot answer whether an ask was understandable. That is the only
// question this package exists to answer, and the only thing that justifies sending
// anything off the machine.
//
// Contract: it consumes an already-computed Profile and produces qualitative output
// only. It never recomputes or changes a score.
package deepeval

import (
	"fmt"
	"strings"
)

// RubricVersion invalidates cached reads when the criteria or anchors change.
const RubricVersion = 1

// MaxLevel is the top of the anchored scale. Levels are ordinal with behavioural
// descriptors rather than a bare 1..10, because unanchored numeric scales agree
// poorly between models and between runs of the same model.
const MaxLevel = 4

// Criterion is one axis of the rubric. Anchors[i] describes level i.
type Criterion struct {
	Key   string
	Title string
	// Question is what the judge is being asked to decide.
	Question string
	// Conditional marks a criterion that does not apply to every prompt. Its mean is
	// divided by the applicable count, never by the sample size, so a developer who
	// rarely needs to correct the model is not punished for it.
	Conditional bool
	Anchors     [MaxLevel + 1]string
}

// Rubric is the single source of truth: the judge prompt, the response schema, and
// the validator are all derived from it, so they cannot drift apart.
var Rubric = []Criterion{
	{
		Key:      "goal_clarity",
		Title:    "Goal clarity",
		Question: "Does the prompt state the outcome wanted, or only an action to perform?",
		Anchors: [MaxLevel + 1]string{
			"No discernible goal (\"fix this\", \"continue\", \"make it better\").",
			"An action is named; the outcome is left implied.",
			"An outcome is stated but its scope is ambiguous.",
			"A concrete outcome is stated.",
			"Outcome and why it matters, so the model can resolve trade-offs unaided.",
		},
	},
	{
		Key:      "context_sufficiency",
		Title:    "Context sufficiency",
		Question: "Is what the model needs supplied, or assumed?",
		Anchors: [MaxLevel + 1]string{
			"Relies entirely on the model inferring or re-discovering the context.",
			"Gestures at context without locating it (\"the usual file\", \"that thing\").",
			"Partial: some context located, load-bearing parts assumed.",
			"The relevant location, state and inputs are supplied.",
			"Supplies context and pre-empts the obvious follow-up question.",
		},
	},
	{
		Key:      "constraints",
		Title:    "Constraints & acceptance",
		Question: "Are boundaries, non-goals and done-conditions stated?",
		Anchors: [MaxLevel + 1]string{
			"None.",
			"One implicit constraint, inferable but unstated.",
			"Some constraints, no done-condition.",
			"Explicit constraints or an explicit done-condition.",
			"Both, plus explicit non-goals (\"don't touch the schema\").",
		},
	},
	{
		Key:      "scope_discipline",
		Title:    "Scope discipline",
		Question: "Is this one coherent unit of work?",
		Anchors: [MaxLevel + 1]string{
			"Several unrelated asks bundled into one message.",
			"Multiple loosely-related asks.",
			"A single ask with unbounded scope (\"refactor the backend\").",
			"One coherent, bounded ask.",
			"Bounded and sequenced, with the ordering made explicit.",
		},
	},
	{
		Key:         "correction_quality",
		Title:       "Correction quality",
		Question:    "When correcting a previous response, does the prompt diagnose or merely re-assert?",
		Conditional: true,
		Anchors: [MaxLevel + 1]string{
			"Bare re-assertion (\"no\", \"still broken\", \"that's wrong\").",
			"Says it is wrong, not what is wrong.",
			"Identifies the symptom.",
			"Symptom and expected behaviour.",
			"Symptom, expected behaviour, and a hypothesis or the missing constraint that caused it.",
		},
	},
}

// Keys returns the rubric's criterion keys in declaration order.
func Keys() []string {
	out := make([]string, len(Rubric))
	for i, c := range Rubric {
		out[i] = c.Key
	}
	return out
}

// Lookup finds a criterion by key.
func Lookup(key string) (Criterion, bool) {
	for _, c := range Rubric {
		if c.Key == key {
			return c, true
		}
	}
	return Criterion{}, false
}

// describe renders the rubric as the instruction block sent to the judge.
func describe() string {
	var b strings.Builder
	for _, c := range Rubric {
		fmt.Fprintf(&b, "\n%s — %s\n%s\n", c.Key, c.Title, c.Question)
		if c.Conditional {
			b.WriteString("APPLIES ONLY to prompts that correct or push back on a previous response. " +
				"For any other prompt set \"applicable\": false and omit the level.\n")
		}
		for level, anchor := range c.Anchors {
			fmt.Fprintf(&b, "  %d = %s\n", level, anchor)
		}
	}
	return b.String()
}
