package deepeval

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/siddham-jain/knowthyself/internal/model"
	"github.com/siddham-jain/knowthyself/internal/profile"
)

// recorder is a fake OpenAI-dialect endpoint that replies with judgments grounded in
// whatever prompts it was actually sent.
type recorder struct {
	mu     sync.Mutex
	bodies []string
	status int
}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.bodies = append(r.bodies, string(body))
	r.mu.Unlock()

	if r.status != 0 {
		w.WriteHeader(r.status)
		return
	}

	var in struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.Unmarshal(body, &in)
	user := in.Messages[len(in.Messages)-1].Content

	// The synthesis call asks for findings, not judgments.
	if strings.Contains(user, "Graded results") {
		writeChoice(w, `{"findings":[{"criterion":"goal_clarity","title":"Say what done means","body":"You name the action but not the outcome."}]}`)
		return
	}

	// Quote the first line of each prompt back, so every judgment is grounded. The
	// prior-context block must be skipped: quoting it is exactly what the validator
	// is supposed to reject.
	var js []string
	for _, block := range strings.Split(user, "--- prompt_id: ")[1:] {
		id, rest, _ := strings.Cut(block, " ---\n")
		if strings.HasPrefix(rest, "[assistant said") {
			_, rest, _ = strings.Cut(rest, "\n\n")
		}
		text := strings.TrimSpace(rest)
		quote := strings.SplitN(text, "\n", 2)[0]
		if len(quote) > 20 {
			quote = quote[:20]
		}
		js = append(js, fmt.Sprintf(`{"prompt_id":%q,"key":"goal_clarity","level":2,"quote":%q}`, id, quote))
	}
	writeChoice(w, `{"judgments":[`+strings.Join(js, ",")+`]}`)
}

func writeChoice(w http.ResponseWriter, content string) {
	payload, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{{"message": map[string]string{"content": content}}},
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}

func testSessions() []model.Session {
	return []model.Session{{ID: "sess-a", Turns: []model.Turn{
		{Role: model.RoleUser, Text: "make the login button submit the form on mobile"},
		{Role: model.RoleAssistant, Text: "done"},
		{Role: model.RoleUser, Text: "deploy /Users/siddham/secretproj/main.go with key sk-abcdefghijklmnopqrstuvwxyz012345"},
		{Role: model.RoleAssistant, Text: "ok"},
		{Role: model.RoleUser, Text: "fix it"},
	}}}
}

func alwaysConsent(Config, Sample) (bool, error) { return true, nil }

func TestRunEndToEnd(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	cfg := Config{APIKey: "k", BaseURL: srv.URL, Model: "test-model", Dialect: DialectOpenAI}
	read, err := Run(context.Background(), cfg, t.TempDir(), testSessions(), alwaysConsent, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if read.Model != "test-model" || read.RubricVer != RubricVersion {
		t.Errorf("provenance not recorded: %+v", read)
	}
	if read.Sample.Prompts != 3 || read.Sample.Available != 3 {
		t.Errorf("sample = %+v, want 3 prompts", read.Sample)
	}
	if read.Confidence != profile.ConfidenceHigh {
		t.Errorf("confidence = %q, want high (every prompt was judged)", read.Confidence)
	}

	var goal profile.CriterionResult
	for _, c := range read.Criteria {
		if c.Key == "goal_clarity" {
			goal = c
		}
	}
	if goal.Judged != 3 || goal.Mean != 2 {
		t.Errorf("goal_clarity = mean %.1f over %d, want 2.0 over 3", goal.Mean, goal.Judged)
	}
	if len(read.Findings) != 1 || read.Findings[0].Source != "deep-eval" {
		t.Errorf("findings = %+v", read.Findings)
	}
}

// The single most important assertion in this package: nothing secret may appear in
// any request body actually put on the wire.
func TestRunSendsNothingSecret(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	cfg := Config{APIKey: "k", BaseURL: srv.URL, Model: "test-model", Dialect: DialectOpenAI}
	if _, err := Run(context.Background(), cfg, t.TempDir(), testSessions(), alwaysConsent, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.bodies) == 0 {
		t.Fatal("nothing was sent")
	}
	for i, body := range rec.bodies {
		for _, leak := range []string{"sk-abcdefghijklmnopqrstuvwxyz012345", "/Users/siddham", "secretproj"} {
			if strings.Contains(body, leak) {
				t.Errorf("request %d leaked %q", i, leak)
			}
		}
	}
}

// Declining must send nothing at all.
func TestRunRespectsRefusal(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	cfg := Config{APIKey: "k", BaseURL: srv.URL, Model: "m", Dialect: DialectOpenAI}
	_, err := Run(context.Background(), cfg, t.TempDir(), testSessions(),
		func(Config, Sample) (bool, error) { return false, nil }, nil)

	if _, ok := err.(ErrDeclined); !ok {
		t.Fatalf("err = %v, want ErrDeclined", err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.bodies) != 0 {
		t.Errorf("declining still sent %d requests", len(rec.bodies))
	}
}

// A second run over the same corpus must be served from cache without re-consenting
// or re-sending.
func TestRunUsesCache(t *testing.T) {
	rec := &recorder{}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	dir := t.TempDir()
	cfg := Config{APIKey: "k", BaseURL: srv.URL, Model: "m", Dialect: DialectOpenAI}
	if _, err := Run(context.Background(), cfg, dir, testSessions(), alwaysConsent, nil); err != nil {
		t.Fatalf("first run: %v", err)
	}
	rec.mu.Lock()
	first := len(rec.bodies)
	rec.mu.Unlock()

	if _, err := Run(context.Background(), cfg, dir, testSessions(), func(Config, Sample) (bool, error) {
		t.Error("cached run must not ask for consent again")
		return true, nil
	}, nil); err != nil {
		t.Fatalf("second run: %v", err)
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.bodies) != first {
		t.Errorf("cached run sent %d more requests", len(rec.bodies)-first)
	}
}

// An endpoint that rejects the key must surface as ErrAuth with a remedy, not as a
// bare status code.
func TestRunAuthFailure(t *testing.T) {
	rec := &recorder{status: http.StatusUnauthorized}
	srv := httptest.NewServer(rec)
	defer srv.Close()

	cfg := Config{APIKey: "bad", BaseURL: srv.URL, Model: "m", Dialect: DialectOpenAI}
	_, err := Run(context.Background(), cfg, t.TempDir(), testSessions(), alwaysConsent, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, ok := err.(ErrAuth); !ok {
		t.Fatalf("err = %T (%v), want ErrAuth", err, err)
	}
	if !strings.Contains(Explain(err), "check the key belongs to") {
		t.Errorf("no remedy offered: %s", Explain(err))
	}
}

// A model that cannot follow the schema must abandon cleanly rather than report a
// read built on nothing.
func TestRunAbandonsOnGarbage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeChoice(w, "I'm afraid I can't help with that.")
	}))
	defer srv.Close()

	cfg := Config{APIKey: "k", BaseURL: srv.URL, Model: "weak", Dialect: DialectOpenAI}
	_, err := Run(context.Background(), cfg, t.TempDir(), testSessions(), alwaysConsent, nil)
	if _, ok := err.(ErrUnusable); !ok {
		t.Fatalf("err = %T (%v), want ErrUnusable", err, err)
	}
}

// The Anthropic request must carry no sampling parameters: Sonnet 5, Opus 4.7/4.8
// and Fable 5 reject a non-default temperature with a 400, so sending one breaks
// the default model on its very first real call.
func TestAnthropicRequestOmitsSamplingParams(t *testing.T) {
	cfg := Config{APIKey: "k", BaseURL: "https://api.anthropic.com/v1", Model: "claude-sonnet-5", Dialect: DialectAnthropic}
	_, body, err := NewClient(cfg).request("sys", "user")
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"temperature", "top_p", "top_k", "thinking"} {
		if _, present := sent[banned]; present {
			t.Errorf("request sends %q; it is rejected or unsafe on current models", banned)
		}
	}
	if sent["model"] != "claude-sonnet-5" {
		t.Errorf("model = %v", sent["model"])
	}
	// max_tokens caps thinking and text together on models that think by default.
	if mt, _ := sent["max_tokens"].(float64); mt < 8000 {
		t.Errorf("max_tokens = %v, too tight once thinking shares the budget", mt)
	}
}

// An OAuth token authenticates differently from an API key.
func TestAuthHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Write([]byte(`{"content":[{"type":"text","text":"{}"}]}`))
	}))
	defer srv.Close()

	key := Config{APIKey: "secret-key", BaseURL: srv.URL, Model: "m", Dialect: DialectAnthropic}
	if _, err := NewClient(key).Complete(context.Background(), "s", "u"); err != nil {
		t.Fatal(err)
	}
	if got.Get("x-api-key") != "secret-key" || got.Get("anthropic-version") == "" {
		t.Errorf("api-key auth headers wrong: %v", got)
	}
	if got.Get("Authorization") != "" {
		t.Error("an API key must not be sent as a bearer token")
	}

	tok := Config{AuthToken: "oauth-tok", BaseURL: srv.URL, Model: "m", Dialect: DialectAnthropic}
	if _, err := NewClient(tok).Complete(context.Background(), "s", "u"); err != nil {
		t.Fatal(err)
	}
	if got.Get("Authorization") != "Bearer oauth-tok" {
		t.Errorf("oauth token not sent as bearer: %v", got.Get("Authorization"))
	}
	if got.Get("anthropic-beta") != "oauth-2025-04-20" {
		t.Errorf("oauth beta header missing: %v", got.Get("anthropic-beta"))
	}
	if got.Get("x-api-key") != "" {
		t.Error("an OAuth token must not be sent as x-api-key")
	}
}
