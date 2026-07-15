// Package model defines the source-agnostic normalized representation of an AI
// coding session: Session -> Turn -> ToolCall. Every provider (Claude Code today;
// Codex, OpenCode, Gemini later) parses its own on-disk format into these types,
// and everything downstream (scoring, reporting) depends only on this model.
//
// This is one of the two load-bearing contracts in synch (the other is
// profile.Profile). Keep it stable and provider-neutral: nothing here may leak a
// detail specific to one tool's log format.
package model

import "time"

// Role identifies who produced a Turn.
type Role string

const (
	RoleUser      Role = "user"      // a human natural-language prompt
	RoleAssistant Role = "assistant" // the model's response
	RoleMeta      Role = "meta"      // synthetic/system records (compaction, caveats, tool output framing)
)

// Session is one conversation with an AI coding assistant, normalized from a
// provider's on-disk log.
type Session struct {
	ID         string    // provider-unique session id
	Source     string    // provider id, e.g. "claude-code"
	Cwd        string    // working directory (read from records, never from the encoded dir name)
	GitBranch  string    // best-effort branch at session time
	Models     []string  // distinct model ids seen in the session
	Version    string    // provider/client version string (for schema branching)
	StartedAt  time.Time // earliest record timestamp (zero if unknown)
	EndedAt    time.Time // latest record timestamp (zero if unknown)
	Turns      []Turn    // ordered conversation turns (synthetic/meta included, flagged)
	Tokens     TokenUsage // session-level token totals (sum of assistant usage)
}

// Turn is a single step in the conversation.
type Turn struct {
	Role      Role
	Text      string    // plain user text, or concatenated assistant text blocks
	Timestamp time.Time // zero if the record had no timestamp

	// Classification flags. These decide what a Turn contributes to scoring; getting
	// them right is fairness-critical (see plan: "Records that must NOT be scored").
	IsSidechain   bool // belongs to a sub-agent/Task branch, not the human thread
	IsSynthetic   bool // injected by the client (tool-result plumbing, caveat, meta)
	IsCompaction  bool // specifically a /compact summary record (context-overflow signal)
	IsCommandLike bool // user "prompt" that is really a pasted shell command / non-NL input

	// User-turn signals.
	SlashCommand string // e.g. "/clear" if this turn was a slash-command envelope ("" otherwise)

    // Assistant-turn signals.
	ToolCalls      []ToolCall // tool invocations issued in this assistant turn
	ThinkingBlocks int        // count of reasoning/thinking blocks (plaintext ones)
	EndsWithQ      bool       // assistant text ends in a question (clarification-loop signal)
	StopReason     string     // provider stop reason, "" if unknown/interrupted

	// PermissionMode captured at this point in the session ("default","auto",
	// "acceptEdits","plan", ...). Empty if not present. Used by Context/autonomy signals.
	PermissionMode string

	// Usage is the assistant turn's token accounting (zero value for user/meta turns).
	Usage TokenUsage
}

// ToolCall is one tool invocation inside an assistant turn.
type ToolCall struct {
	Name         string // e.g. "Bash", "Read", "Agent", "Skill", "mcp__context7__query-docs"
	IsMCP        bool   // true for MCP-provided tools (name prefixed "mcp__" or attribution set)
	IsSubAgent   bool   // true for the Task/Agent tool (delegation signal)
	InputSummary string // short, non-sensitive summary of the input (never raw secrets)
	ResultOK     bool   // best-effort success flag from the paired tool_result
	DurationMs   int64  // best-effort duration if the provider records it
}

// TokenUsage is the normalized token accounting shared by turns and sessions.
// Field names are provider-neutral; each provider maps its own shape onto these.
type TokenUsage struct {
	Input         int64
	Output        int64
	CacheRead     int64 // tokens served from cache (the big Token-Economy lever)
	CacheCreation int64 // tokens written to cache
	Reasoning     int64 // reasoning/thinking output tokens (0 if unknown)
}

// TotalInput returns input + cache reads + cache creation — the full billed input side.
func (t TokenUsage) TotalInput() int64 { return t.Input + t.CacheRead + t.CacheCreation }

// Add accumulates another usage into the receiver.
func (t *TokenUsage) Add(o TokenUsage) {
	t.Input += o.Input
	t.Output += o.Output
	t.CacheRead += o.CacheRead
	t.CacheCreation += o.CacheCreation
	t.Reasoning += o.Reasoning
}

// Scorable reports whether a user Turn should count as a real human prompt for
// Prompt-Quality/Iteration scoring. Synthetic, sidechain, slash-command, and
// command-like turns are excluded so they can't inflate or deflate a grade.
func (t Turn) Scorable() bool {
	return t.Role == RoleUser &&
		!t.IsSynthetic &&
		!t.IsSidechain &&
		!t.IsCommandLike &&
		t.SlashCommand == ""
}
