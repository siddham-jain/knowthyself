package claude

import (
	"context"
	"strings"
	"testing"

	"github.com/siddham/synch/internal/model"
)

// parse is a test helper that runs the core parser over a JSONL string.
func parse(t *testing.T, jsonl string) model.Session {
	t.Helper()
	s, err := parseReader(context.Background(), strings.NewReader(jsonl), "test-session")
	if err != nil {
		t.Fatalf("parseReader returned error: %v", err)
	}
	return s
}

// scorableUserTurns returns the turns that count as real human prompts.
func scorableUserTurns(s model.Session) []model.Turn {
	var out []model.Turn
	for _, tn := range s.Turns {
		if tn.Scorable() {
			out = append(out, tn)
		}
	}
	return out
}

func TestPlainUserPrompt(t *testing.T) {
	in := `{"type":"user","timestamp":"2026-07-14T10:00:00Z","cwd":"/x","message":{"role":"user","content":"fix the bug in main.go"}}`
	s := parse(t, in)
	got := scorableUserTurns(s)
	if len(got) != 1 {
		t.Fatalf("want 1 scorable prompt, got %d", len(got))
	}
	if got[0].Text != "fix the bug in main.go" {
		t.Errorf("text = %q", got[0].Text)
	}
	if s.Cwd != "/x" {
		t.Errorf("cwd = %q", s.Cwd)
	}
}

func TestAssistantTokensAndTools(t *testing.T) {
	in := `{"type":"assistant","timestamp":"2026-07-14T10:00:01Z","message":{"role":"assistant","model":"claude-opus-4-8","stop_reason":"tool_use","content":[{"type":"thinking","thinking":"..."},{"type":"text","text":"Running it."},{"type":"tool_use","name":"Bash","input":{}},{"type":"tool_use","name":"mcp__context7__query-docs","input":{}},{"type":"tool_use","name":"Agent","input":{}}],"usage":{"input_tokens":5,"output_tokens":100,"cache_read_input_tokens":2000,"cache_creation_input_tokens":300}}}`
	s := parse(t, in)
	if len(s.Turns) != 1 {
		t.Fatalf("want 1 turn, got %d", len(s.Turns))
	}
	tn := s.Turns[0]
	if tn.Role != model.RoleAssistant {
		t.Fatalf("role = %v", tn.Role)
	}
	if tn.ThinkingBlocks != 1 {
		t.Errorf("thinking = %d", tn.ThinkingBlocks)
	}
	if len(tn.ToolCalls) != 3 {
		t.Fatalf("tool calls = %d", len(tn.ToolCalls))
	}
	if !tn.ToolCalls[1].IsMCP {
		t.Errorf("expected mcp tool flagged")
	}
	if !tn.ToolCalls[2].IsSubAgent {
		t.Errorf("expected Agent flagged as sub-agent")
	}
	if tn.Usage.CacheRead != 2000 || tn.Usage.Output != 100 {
		t.Errorf("usage = %+v", tn.Usage)
	}
	if s.Tokens.CacheRead != 2000 {
		t.Errorf("session tokens not accumulated: %+v", s.Tokens)
	}
	if len(s.Models) != 1 || s.Models[0] != "claude-opus-4-8" {
		t.Errorf("models = %v", s.Models)
	}
}

// EDGE: a truncated final line (session being written live) must not crash, and the
// valid earlier records must still parse.
func TestTruncatedFinalLine(t *testing.T) {
	in := `{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","tex`
	s := parse(t, in)
	if len(scorableUserTurns(s)) != 1 {
		t.Fatalf("valid record lost; turns=%d", len(s.Turns))
	}
}

// EDGE: unknown top-level type must be ignored, not fatal.
func TestUnknownRecordType(t *testing.T) {
	in := `{"type":"some-future-type","payload":{"x":1}}
{"type":"user","message":{"role":"user","content":"real prompt"}}`
	s := parse(t, in)
	if len(scorableUserTurns(s)) != 1 {
		t.Fatalf("turns=%d", len(s.Turns))
	}
}

// EDGE: user content as an array (tool_result plumbing) is NOT a human prompt.
func TestToolResultUserIsNotPrompt(t *testing.T) {
	in := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}}`
	s := parse(t, in)
	if n := len(scorableUserTurns(s)); n != 0 {
		t.Fatalf("tool_result user counted as prompt: %d", n)
	}
	if len(s.Turns) != 1 || !s.Turns[0].IsSynthetic {
		t.Fatalf("expected 1 synthetic meta turn, got %+v", s.Turns)
	}
}

// EDGE: slash-command envelope records the command but is not a NL prompt.
func TestSlashCommandEnvelope(t *testing.T) {
	in := `{"type":"user","message":{"role":"user","content":"<command-name>/clear</command-name>\n<command-message>clear</command-message>"}}`
	s := parse(t, in)
	if len(scorableUserTurns(s)) != 0 {
		t.Fatalf("slash command counted as prompt")
	}
	if len(s.Turns) != 1 || s.Turns[0].SlashCommand != "/clear" {
		t.Fatalf("slash command not captured: %+v", s.Turns)
	}
}

// EDGE: local-command caveat and isMeta are synthetic, not prompts.
func TestMetaAndCaveatExcluded(t *testing.T) {
	in := `{"type":"user","isMeta":true,"message":{"role":"user","content":"internal note"}}
{"type":"user","message":{"role":"user","content":"<local-command-caveat>output here</local-command-caveat>"}}`
	s := parse(t, in)
	if n := len(scorableUserTurns(s)); n != 0 {
		t.Fatalf("meta/caveat counted as prompt: %d", n)
	}
}

// EDGE: a pasted bare shell command is not a natural-language prompt...
func TestPastedCommandNotPrompt(t *testing.T) {
	in := `{"type":"user","message":{"role":"user","content":"git status"}}`
	s := parse(t, in)
	if len(scorableUserTurns(s)) != 0 {
		t.Fatalf("bare command counted as prompt")
	}
	if !s.Turns[0].IsCommandLike {
		t.Fatalf("expected IsCommandLike")
	}
}

// ...but a command WITH real prose IS a prompt (fairness: don't discard real input).
func TestCommandWithProseIsPrompt(t *testing.T) {
	in := `{"type":"user","message":{"role":"user","content":"curl -L https://x.ai/vsix -o k.vsix && code --install-extension k.vsix - this is the extension which will show me ads and share revenue with me"}}`
	s := parse(t, in)
	if len(scorableUserTurns(s)) != 1 {
		t.Fatalf("prompt-with-command wrongly discarded")
	}
}

// EDGE: sidechain (sub-agent) turns are attributed but excluded from human prompts.
func TestSidechainExcluded(t *testing.T) {
	in := `{"type":"user","isSidechain":true,"message":{"role":"user","content":"sub-agent internal prompt"}}`
	s := parse(t, in)
	if len(scorableUserTurns(s)) != 0 {
		t.Fatalf("sidechain counted as human prompt")
	}
	if len(s.Turns) != 1 || !s.Turns[0].IsSidechain {
		t.Fatalf("sidechain flag lost")
	}
}

// EDGE: empty / blank-line-only input must not panic and yields no turns.
func TestEmptyAndBlankLines(t *testing.T) {
	s := parse(t, "\n\n   \n")
	if len(s.Turns) != 0 {
		t.Fatalf("blank input produced turns: %d", len(s.Turns))
	}
}

// EDGE: out-of-order timestamps still yield correct start/end bounds.
func TestTimestampBounds(t *testing.T) {
	in := `{"type":"user","timestamp":"2026-07-14T12:00:00Z","message":{"role":"user","content":"b"}}
{"type":"user","timestamp":"2026-07-14T09:00:00Z","message":{"role":"user","content":"a"}}`
	s := parse(t, in)
	if s.StartedAt.Hour() != 9 || s.EndedAt.Hour() != 12 {
		t.Fatalf("bounds wrong: start=%v end=%v", s.StartedAt, s.EndedAt)
	}
}

// EDGE: permission mode carries forward to subsequent turns.
func TestPermissionModeCarryForward(t *testing.T) {
	in := `{"type":"user","permissionMode":"plan","message":{"role":"user","content":"plan this"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`
	s := parse(t, in)
	if len(s.Turns) != 2 {
		t.Fatalf("turns=%d", len(s.Turns))
	}
	if s.Turns[1].PermissionMode != "plan" {
		t.Fatalf("permission mode not carried: %q", s.Turns[1].PermissionMode)
	}
}

// FAIRNESS/MULTILINGUAL: a Hindi (Devanagari) prompt naming a file + error must be
// retained as a real, scorable prompt — never discarded for not being English.
func TestHindiPromptRetained(t *testing.T) {
	in := `{"type":"user","message":{"role":"user","content":"main.py mein yeh error aa raha hai: TypeError, ise theek karo — देखो fileना"}}`
	s := parse(t, in)
	got := scorableUserTurns(s)
	if len(got) != 1 {
		t.Fatalf("Hindi/Hinglish prompt discarded")
	}
}

// EDGE: a very long single line (large tool output style) must parse without the
// 64KB scanner limit tripping.
func TestVeryLongLine(t *testing.T) {
	big := strings.Repeat("x", 200_000)
	in := `{"type":"user","message":{"role":"user","content":"` + big + `"}}`
	s := parse(t, in)
	if len(s.Turns) != 1 {
		t.Fatalf("long line dropped")
	}
	if got := len(s.Turns[0].Text); got != 200_000 {
		t.Fatalf("long text truncated: %d", got)
	}
}

// EDGE: clarification-loop signal — assistant text ending in a question is flagged.
func TestAssistantQuestionSignal(t *testing.T) {
	in := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Which file should I edit?"}]}}`
	s := parse(t, in)
	if !s.Turns[0].EndsWithQ {
		t.Fatalf("assistant question not flagged")
	}
}
