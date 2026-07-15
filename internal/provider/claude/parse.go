package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/siddham/synch/internal/model"
	"github.com/siddham/synch/internal/provider"
	"github.com/siddham/synch/internal/text"
)

// Parse opens the session file and parses it defensively. A malformed file yields
// whatever could be salvaged plus a nil error where possible; only unopenable files
// return an error.
func (p *Provider) Parse(ctx context.Context, ref provider.SessionRef) (model.Session, error) {
	f, err := os.Open(ref.Path)
	if err != nil {
		return model.Session{}, err
	}
	defer f.Close()
	return parseReader(ctx, f, ref.SessionID)
}

// rawRecord is the shared envelope of a Claude Code JSONL line. Only fields synch
// uses are declared; unknown fields are tolerated by encoding/json.
type rawRecord struct {
	Type             string          `json:"type"`
	SessionID        string          `json:"sessionId"`
	SessionIDSnake   string          `json:"session_id"`
	Timestamp        string          `json:"timestamp"`
	Cwd              string          `json:"cwd"`
	GitBranch        string          `json:"gitBranch"`
	Version          string          `json:"version"`
	IsMeta           bool            `json:"isMeta"`
	IsSidechain      bool            `json:"isSidechain"`
	IsCompactSummary bool            `json:"isCompactSummary"`
	PermissionMode   string          `json:"permissionMode"`
	Mode             string          `json:"mode"` // some records carry mode here
	Message          json.RawMessage `json:"message"`
}

type rawMessage struct {
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	Content    json.RawMessage `json:"content"` // string XOR array of blocks
	StopReason *string         `json:"stop_reason"`
	Usage      *rawUsage       `json:"usage"`
}

type rawUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

type rawBlock struct {
	Type string `json:"type"`
	// text / thinking
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
	// tool_use
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
	// tool_result
	ToolUseID string `json:"tool_use_id"`
}

// parseReader is the testable core: it consumes JSONL from r and builds a Session.
// It never returns an error for malformed lines — those are skipped — so partial or
// live-being-written files still produce a usable (if smaller) Session.
func parseReader(ctx context.Context, r io.Reader, sessionID string) (model.Session, error) {
	s := model.Session{ID: sessionID, Source: ID}
	models := map[string]bool{}
	currentPerm := ""

	br := bufio.NewReaderSize(r, 256*1024)
	for {
		if err := ctx.Err(); err != nil {
			return s, err
		}
		// ReadBytes grows to fit arbitrarily long lines (tool outputs can be huge),
		// avoiding bufio.Scanner's 64KB token limit.
		lineBytes, readErr := br.ReadBytes('\n')
		line := bytes.TrimSpace(lineBytes)
		if len(line) > 0 {
			if rec, ok := decodeRecord(line); ok {
				applyRecord(&s, rec, models, &currentPerm)
			}
		}
		if readErr != nil {
			break // io.EOF or a read error: stop, keep what we have
		}
	}

	// Finalize distinct models in stable (insertion-ish) order via a sorted set.
	for m := range models {
		s.Models = append(s.Models, m)
	}
	sortStrings(s.Models)
	return s, nil
}

// decodeRecord unmarshals one line, returning ok=false for malformed JSON.
func decodeRecord(line []byte) (rawRecord, bool) {
	var rec rawRecord
	if err := json.Unmarshal(line, &rec); err != nil {
		return rawRecord{}, false
	}
	return rec, true
}

// applyRecord folds one raw record into the session being built.
func applyRecord(s *model.Session, rec rawRecord, models map[string]bool, currentPerm *string) {
	ts := parseTime(rec.Timestamp)
	trackMeta(s, rec, ts)

	// Carry permission mode forward: it appears on dedicated records and inline.
	if rec.PermissionMode != "" {
		*currentPerm = rec.PermissionMode
	}
	if rec.Type == "permission-mode" && rec.Mode != "" {
		*currentPerm = rec.Mode
	}

	switch rec.Type {
	case "user":
		if t, ok := buildUserTurn(rec, ts, *currentPerm); ok {
			s.Turns = append(s.Turns, t)
		}
	case "assistant":
		if t, ok := buildAssistantTurn(rec, ts, *currentPerm, models); ok {
			s.Turns = append(s.Turns, t)
			s.Tokens.Add(t.Usage)
		}
	default:
		// attachment, mode, permission-mode, ai-title, last-prompt,
		// file-history-snapshot, summary, system: not conversation turns. The
		// session-level metadata we need was already captured in trackMeta.
	}
}

// trackMeta captures session-level fields from any record type.
func trackMeta(s *model.Session, rec rawRecord, ts time.Time) {
	if s.Cwd == "" && rec.Cwd != "" {
		s.Cwd = rec.Cwd
	}
	if s.GitBranch == "" && rec.GitBranch != "" {
		s.GitBranch = rec.GitBranch
	}
	if rec.Version != "" {
		s.Version = rec.Version
	}
	if !ts.IsZero() {
		if s.StartedAt.IsZero() || ts.Before(s.StartedAt) {
			s.StartedAt = ts
		}
		if ts.After(s.EndedAt) {
			s.EndedAt = ts
		}
	}
}

// buildUserTurn classifies a "user" record. It distinguishes real human prompts
// from the many non-prompt shapes that share type=="user": tool-result plumbing,
// slash-command envelopes, local-command caveats, meta records, and pasted commands.
func buildUserTurn(rec rawRecord, ts time.Time, perm string) (model.Turn, bool) {
	t := model.Turn{
		Role:           model.RoleUser,
		Timestamp:      ts,
		IsSidechain:    rec.IsSidechain,
		PermissionMode: perm,
	}

	str, isString, hadContent := decodeContentString(rec.Message)
	if !isString {
		// content is an array => tool_result plumbing (the harness returning tool
		// output as a "user" record). Not a human prompt; keep as a meta turn so
		// turn-shape metrics see it but prompt scoring ignores it.
		if !hadContent {
			return model.Turn{}, false
		}
		t.Role = model.RoleMeta
		t.IsSynthetic = true
		return t, true
	}

	t.Text = str
	// Slash-command envelope: record the command (Tool-Leverage stat) but it's not NL.
	if cmd := text.ExtractSlashCommand(str); cmd != "" {
		t.SlashCommand = cmd
		return t, true
	}
	// Meta / synthetic user records.
	if rec.IsMeta || rec.IsCompactSummary || text.IsLocalCommandCaveat(str) {
		t.Role = model.RoleMeta
		t.IsSynthetic = true
		t.IsCompaction = rec.IsCompactSummary
		return t, true
	}
	// Pasted shell command with no prose: not a natural-language prompt.
	if text.IsCommandLike(str) {
		t.IsCommandLike = true
	}
	return t, true
}

// buildAssistantTurn extracts text, thinking count, tool calls, tokens, and the
// clarification-loop signal from an "assistant" record.
func buildAssistantTurn(rec rawRecord, ts time.Time, perm string, models map[string]bool) (model.Turn, bool) {
	var msg rawMessage
	if len(rec.Message) == 0 || json.Unmarshal(rec.Message, &msg) != nil {
		return model.Turn{}, false
	}
	t := model.Turn{
		Role:           model.RoleAssistant,
		Timestamp:      ts,
		IsSidechain:    rec.IsSidechain,
		PermissionMode: perm,
	}
	if msg.Model != "" {
		models[msg.Model] = true
	}
	if msg.StopReason != nil {
		t.StopReason = *msg.StopReason
	}
	if msg.Usage != nil {
		t.Usage = model.TokenUsage{
			Input:         msg.Usage.InputTokens,
			Output:        msg.Usage.OutputTokens,
			CacheRead:     msg.Usage.CacheReadInputTokens,
			CacheCreation: msg.Usage.CacheCreationInputTokens,
		}
	}

	// content is an array of blocks for assistant turns.
	var blocks []rawBlock
	if len(msg.Content) > 0 {
		_ = json.Unmarshal(msg.Content, &blocks) // tolerate; empty on failure
	}
	var textBuf bytes.Buffer
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if textBuf.Len() > 0 {
				textBuf.WriteByte('\n')
			}
			textBuf.WriteString(b.Text)
		case "thinking":
			t.ThinkingBlocks++
		case "tool_use":
			t.ToolCalls = append(t.ToolCalls, model.ToolCall{
				Name:       b.Name,
				IsMCP:      isMCPTool(b.Name),
				IsSubAgent: isSubAgentTool(b.Name),
			})
		}
	}
	t.Text = textBuf.String()
	t.EndsWithQ = text.EndsWithQuestion(t.Text)
	return t, true
}

// decodeContentString reports whether message.content is a JSON string, returning
// the decoded string. hadContent is false when content is absent/empty.
func decodeContentString(raw json.RawMessage) (str string, isString, hadContent bool) {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &msg) != nil {
		return "", false, false
	}
	c := bytes.TrimSpace(msg.Content)
	if len(c) == 0 || string(c) == "null" {
		return "", false, false
	}
	if c[0] == '"' {
		var decoded string
		if err := json.Unmarshal(c, &decoded); err == nil {
			return decoded, true, true
		}
		return "", true, true
	}
	// Array (or other): content present but not a plain string.
	return "", false, true
}

func isMCPTool(name string) bool { return len(name) > 5 && name[:5] == "mcp__" }

func isSubAgentTool(name string) bool { return name == "Agent" || name == "Task" }

// parseTime parses an ISO-8601 timestamp, returning the zero time on failure.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// sortStrings sorts a string slice in place (small helper to avoid an import churn).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j-1] > ss[j]; j-- {
			ss[j-1], ss[j] = ss[j], ss[j-1]
		}
	}
}
