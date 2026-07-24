package deepeval

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A slow endpoint must report progress and end on the budget, not hang silently.
func TestSlowEndpointReportsProgressAndTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(3 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	var stages []string
	progress := func(stage string, done, total int) {
		stages = append(stages, stage)
	}

	cfg := Config{APIKey: "k", BaseURL: srv.URL, Model: "slow", Dialect: DialectOpenAI}
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, cfg, t.TempDir(), testSessions(), alwaysConsent, progress)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a wedged endpoint must surface an error")
	}
	if elapsed > 10*time.Second {
		t.Errorf("took %s — the deadline is not bounding the run", elapsed)
	}
	if len(stages) == 0 {
		t.Error("no progress reported — the terminal would look frozen")
	}
	if !strings.Contains(Explain(err), "try") {
		t.Errorf("error offers no remedy: %s", Explain(err))
	}
	t.Logf("stopped after %s with: %s", elapsed.Round(time.Millisecond), Explain(err))
}
