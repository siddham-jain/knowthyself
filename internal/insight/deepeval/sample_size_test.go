package deepeval

import (
	"fmt"
	"testing"

	"github.com/siddham-jain/knowthyself/internal/model"
)

// corpus builds a synthetic history of `sessions` sessions, each with `promptsPer`
// scorable user turns, for exercising sample sizing.
func corpus(sessions, promptsPer int) []model.Session {
	var out []model.Session
	for s := 0; s < sessions; s++ {
		var turns []model.Turn
		for p := 0; p < promptsPer; p++ {
			turns = append(turns,
				model.Turn{Role: model.RoleUser, Text: fmt.Sprintf("refactor handler %d-%d to validate input and return early on error", s, p)},
				model.Turn{Role: model.RoleAssistant, Text: "done"},
			)
		}
		out = append(out, model.Session{ID: fmt.Sprintf("sess-%02d", s), Turns: turns})
	}
	return out
}

// The adaptive target grows with data but sub-linearly: floored, monotonic, and capped
// at the ceiling.
func TestTargetPromptsScalesWithData(t *testing.T) {
	prev := 0
	for _, a := range []int{1, 10, 47, 100, 130, 200, 294, 500, 2000} {
		got := targetPrompts(a)
		if got < prev {
			t.Errorf("available=%d: not monotonic, %d < %d", a, got, prev)
		}
		if a >= minAdaptivePrompts && got < minAdaptivePrompts {
			t.Errorf("available=%d fell below the floor: %d", a, got)
		}
		if got > DefaultMaxPrompts {
			t.Errorf("available=%d exceeded the ceiling: %d", a, got)
		}
		if got > a {
			t.Errorf("available=%d targeted more prompts than exist: %d", a, got)
		}
		prev = got
	}
	if got := targetPrompts(1000); got != DefaultMaxPrompts {
		t.Errorf("a large corpus should reach the ceiling: got %d, want %d", got, DefaultMaxPrompts)
	}
	if got := targetPrompts(130); got >= DefaultMaxPrompts {
		t.Errorf("a medium corpus should stay under the ceiling: got %d", got)
	}
}

// Build sizes the sample to the corpus when maxPrompts is unset, and an explicit
// override still wins.
func TestBuildSizesToAvailable(t *testing.T) {
	med := corpus(10, 13) // 130 scorable prompts
	s := Build(med, 0, 1<<30)
	if s.Available != 130 {
		t.Fatalf("available = %d, want 130", s.Available)
	}
	if want := targetPrompts(130); len(s.Prompts) != want {
		t.Errorf("adaptive sample = %d, want targetPrompts(130) = %d", len(s.Prompts), want)
	}
	if len(s.Prompts) >= DefaultMaxPrompts {
		t.Errorf("a medium corpus should sample fewer than the ceiling, got %d", len(s.Prompts))
	}

	big := corpus(40, 20) // 800 scorable prompts
	if b := Build(big, 0, 1<<30); len(b.Prompts) != DefaultMaxPrompts {
		t.Errorf("a large corpus should reach the ceiling: got %d, want %d", len(b.Prompts), DefaultMaxPrompts)
	}

	if o := Build(med, 12, 1<<30); len(o.Prompts) != 12 {
		t.Errorf("explicit maxPrompts=12 gave %d prompts", len(o.Prompts))
	}
}

// Sizing is deterministic: the same corpus always yields the same sample.
func TestBuildAdaptiveIsDeterministic(t *testing.T) {
	c := corpus(8, 11)
	a := Build(c, 0, 1<<30)
	b := Build(c, 0, 1<<30)
	if a.Fingerprint("m") != b.Fingerprint("m") {
		t.Error("adaptive sampling produced a different sample on a second run over the same corpus")
	}
}
