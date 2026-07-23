package deepeval

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siddham-jain/knowthyself/internal/model"
)

func sessions(n, promptsEach int) []model.Session {
	out := make([]model.Session, n)
	for s := range out {
		turns := make([]model.Turn, 0, promptsEach*2)
		for p := 0; p < promptsEach; p++ {
			turns = append(turns,
				model.Turn{Role: model.RoleAssistant, Text: fmt.Sprintf("assistant reply %d", p)},
				model.Turn{Role: model.RoleUser, Text: fmt.Sprintf("session %d prompt %d, please fix the thing", s, p)},
			)
		}
		out[s] = model.Session{ID: fmt.Sprintf("sess-%02d", s), Turns: turns}
	}
	return out
}

// Two runs over the same corpus must select the same prompts, or scores are not
// comparable between runs and the cache key is meaningless.
func TestSampleIsDeterministic(t *testing.T) {
	ss := sessions(5, 20)
	a, b := Build(ss, 30, 0), Build(ss, 30, 0)
	if len(a.Prompts) != len(b.Prompts) {
		t.Fatalf("sample sizes differ: %d vs %d", len(a.Prompts), len(b.Prompts))
	}
	for i := range a.Prompts {
		if a.Prompts[i].ID != b.Prompts[i].ID {
			t.Fatalf("sample differs at %d: %s vs %s", i, a.Prompts[i].ID, b.Prompts[i].ID)
		}
	}
	if a.Fingerprint("m") != b.Fingerprint("m") {
		t.Error("fingerprint is not stable across runs")
	}
}

func TestSampleRespectsBounds(t *testing.T) {
	s := Build(sessions(10, 50), 25, 0)
	if len(s.Prompts) > 25 {
		t.Errorf("sample of %d exceeds the cap of 25", len(s.Prompts))
	}
	if s.Available != 500 {
		t.Errorf("available = %d, want 500", s.Available)
	}
}

// One huge session must not crowd out the small ones, or the read describes a single
// project rather than the developer.
func TestSampleSpreadsAcrossSessions(t *testing.T) {
	ss := append(sessions(4, 5), model.Session{ID: "sess-huge", Turns: sessions(1, 400)[0].Turns})
	s := Build(ss, 40, 0)
	if s.Sessions < 4 {
		t.Errorf("only %d sessions represented, want the small ones included too", s.Sessions)
	}
	from := map[string]int{}
	for _, p := range s.Prompts {
		from[strings.SplitN(p.ID, "#", 2)[0]]++
	}
	if huge := from["sess-hug"]; huge > len(s.Prompts)/2 {
		t.Errorf("the 400-prompt session took %d of %d slots", huge, len(s.Prompts))
	}
}

func TestSampleRedactsBeforeLeaving(t *testing.T) {
	ss := []model.Session{{ID: "s1", Turns: []model.Turn{
		{Role: model.RoleUser, Text: "deploy with sk-abcdefghijklmnopqrstuvwxyz012345 to /Users/me/app/main.go"},
	}}}
	s := Build(ss, 10, 0)
	if len(s.Prompts) != 1 {
		t.Fatalf("want 1 prompt, got %d", len(s.Prompts))
	}
	if strings.Contains(s.Prompts[0].Text, "sk-abcdefghijklmnopqrstuvwxyz012345") {
		t.Error("secret reached the sample")
	}
	if strings.Contains(s.Prompts[0].Text, "/Users/me") {
		t.Error("home path reached the sample")
	}
}

// Non-scorable turns are excluded upstream by Scorable(); confirm nothing slips in.
func TestSampleSkipsNonScorable(t *testing.T) {
	ss := []model.Session{{ID: "s1", Turns: []model.Turn{
		{Role: model.RoleUser, Text: "real prompt here"},
		{Role: model.RoleUser, Text: "/clear", SlashCommand: "/clear"},
		{Role: model.RoleUser, Text: "tool plumbing", IsSynthetic: true},
		{Role: model.RoleUser, Text: "sidechain work", IsSidechain: true},
		{Role: model.RoleAssistant, Text: "not a prompt"},
	}}}
	if got := len(Build(ss, 10, 0).Prompts); got != 1 {
		t.Errorf("sampled %d prompts, want only the 1 scorable one", got)
	}
}

func promptSet() []Prompt {
	return []Prompt{
		{ID: "p1", Text: "make the login button submit the form"},
		{ID: "p2", Text: "fix it"},
	}
}

// The quote-grounding rule is the anti-fabrication control; these are the cases it
// exists to reject.
func TestValidateRejectsUngrounded(t *testing.T) {
	cases := []struct{ name, reply string }{
		{"quote not in prompt", `{"judgments":[{"prompt_id":"p1","key":"goal_clarity","level":3,"quote":"never written"}]}`},
		{"unknown prompt", `{"judgments":[{"prompt_id":"p99","key":"goal_clarity","level":3,"quote":"fix it"}]}`},
		{"unknown criterion", `{"judgments":[{"prompt_id":"p2","key":"vibes","level":3,"quote":"fix it"}]}`},
		{"level too high", `{"judgments":[{"prompt_id":"p2","key":"goal_clarity","level":9,"quote":"fix it"}]}`},
		{"level negative", `{"judgments":[{"prompt_id":"p2","key":"goal_clarity","level":-1,"quote":"fix it"}]}`},
		{"empty quote", `{"judgments":[{"prompt_id":"p2","key":"goal_clarity","level":1,"quote":""}]}`},
		{"quote from other prompt", `{"judgments":[{"prompt_id":"p2","key":"goal_clarity","level":1,"quote":"login button"}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kept, problems := validate(c.reply, promptSet())
			if len(kept) != 0 {
				t.Errorf("kept %d judgments, want 0", len(kept))
			}
			if len(problems) == 0 {
				t.Error("no problem reported, so the repair retry would have nothing to say")
			}
		})
	}
}

func TestValidateKeepsGrounded(t *testing.T) {
	reply := `{"judgments":[
		{"prompt_id":"p1","key":"goal_clarity","level":3,"quote":"submit the form"},
		{"prompt_id":"p2","key":"goal_clarity","level":0,"quote":"fix it"}]}`
	kept, problems := validate(reply, promptSet())
	if len(kept) != 2 {
		t.Fatalf("kept %d, want 2 (problems: %v)", len(kept), problems)
	}
	if len(problems) != 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
}

// A conditional criterion marked inapplicable must be dropped, not counted as zero.
func TestValidateDropsInapplicable(t *testing.T) {
	reply := `{"judgments":[{"prompt_id":"p1","key":"correction_quality","level":0,"applicable":false,"quote":"submit the form"}]}`
	if kept, _ := validate(reply, promptSet()); len(kept) != 0 {
		t.Errorf("inapplicable judgment was kept, which would drag the mean down unfairly")
	}
}

func TestValidateDedupes(t *testing.T) {
	reply := `{"judgments":[
		{"prompt_id":"p2","key":"goal_clarity","level":0,"quote":"fix it"},
		{"prompt_id":"p2","key":"goal_clarity","level":4,"quote":"fix it"}]}`
	kept, _ := validate(reply, promptSet())
	if len(kept) != 1 || kept[0].Level != 0 {
		t.Errorf("duplicate judgment not collapsed to the first: %+v", kept)
	}
}

func TestValidateHandlesFencedAndJunk(t *testing.T) {
	fenced := "Sure!\n```json\n{\"judgments\":[{\"prompt_id\":\"p2\",\"key\":\"goal_clarity\",\"level\":0,\"quote\":\"fix it\"}]}\n```"
	if kept, _ := validate(fenced, promptSet()); len(kept) != 1 {
		t.Errorf("fenced JSON not parsed, got %d judgments", len(kept))
	}
	if kept, problems := validate("I can't do that.", promptSet()); len(kept) != 0 || len(problems) == 0 {
		t.Error("a non-JSON reply should yield no judgments and a reported problem")
	}
}

// A conditional criterion divides by how often it applied, not by the sample size.
func TestAggregateConditionalDenominator(t *testing.T) {
	js := []judgment{
		{PromptID: "p1", Key: "correction_quality", Level: 4},
		{PromptID: "p2", Key: "correction_quality", Level: 2},
		{PromptID: "p1", Key: "goal_clarity", Level: 1},
	}
	for _, r := range aggregate(js) {
		switch r.Key {
		case "correction_quality":
			if r.Judged != 2 || r.Mean != 3 {
				t.Errorf("correction_quality: mean %.1f over %d, want 3.0 over 2", r.Mean, r.Judged)
			}
		case "goal_clarity":
			if r.Judged != 1 || r.Mean != 1 {
				t.Errorf("goal_clarity: mean %.1f over %d, want 1.0 over 1", r.Mean, r.Judged)
			}
		default:
			if r.Judged != 0 {
				t.Errorf("%s was never judged but reports %d", r.Key, r.Judged)
			}
		}
	}
}

func TestCoverageAndConfidence(t *testing.T) {
	s := Sample{Prompts: []Prompt{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}}
	js := []judgment{{PromptID: "a"}, {PromptID: "b"}, {PromptID: "b"}, {PromptID: "c"}}
	if got := coverage(js, s); got != 0.75 {
		t.Errorf("coverage = %v, want 0.75", got)
	}
	if got := confidenceFor(0.9); got != "high" {
		t.Errorf("confidence(0.9) = %s", got)
	}
	if got := confidenceFor(0.65); got != "medium" {
		t.Errorf("confidence(0.65) = %s", got)
	}
	if got := confidenceFor(0.4); got != "low" {
		t.Errorf("confidence(0.4) = %s", got)
	}
}

// Every criterion must be reachable from the generated prompt text, or the judge is
// being asked for something the validator will reject.
func TestRubricDescribesEveryCriterion(t *testing.T) {
	desc := describe()
	for _, c := range Rubric {
		if !strings.Contains(desc, c.Key) {
			t.Errorf("rubric prompt omits %q", c.Key)
		}
		for level, anchor := range c.Anchors {
			if anchor == "" {
				t.Errorf("%s level %d has no anchor", c.Key, level)
			}
		}
	}
	if !strings.Contains(judgeSystemPrompt(), "goal_clarity") {
		t.Error("system prompt does not embed the rubric")
	}
}

func TestDialectDetection(t *testing.T) {
	if got := detectDialect("https://api.anthropic.com/v1"); got != DialectAnthropic {
		t.Errorf("anthropic base URL detected as %q", got)
	}
	for _, url := range []string{"https://api.openai.com/v1", "http://localhost:11434/v1", "https://openrouter.ai/api/v1"} {
		if got := detectDialect(url); got != DialectOpenAI {
			t.Errorf("%s detected as %q, want openai", url, got)
		}
	}
}

func TestConfigHostHidesCredentials(t *testing.T) {
	cfg := Config{BaseURL: "https://api.example.com/v1", APIKey: "sk-supersecret"}
	if got := cfg.Host(); strings.Contains(got, "sk-") || got != "api.example.com" {
		t.Errorf("Host() = %q", got)
	}
}
