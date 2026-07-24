package main

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// The progress line must animate, update its counter, and erase itself so the
// dashboard that renders next starts on a clean row.
func TestSpinnerDrawsAndErases(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan string, 1)
	go func() {
		// Read to EOF: a single Read returns only the first frame written.
		all, _ := io.ReadAll(r)
		out <- string(all)
	}()

	s := newSpinner(w, true)
	s.Update("judging your prompts", 8, 60)
	time.Sleep(300 * time.Millisecond)
	s.Update("judging your prompts", 24, 60)
	time.Sleep(300 * time.Millisecond)
	s.Stop()
	w.Close()

	got := <-out
	if !strings.Contains(got, "judging your prompts") {
		t.Errorf("stage not drawn: %q", got)
	}
	if !strings.Contains(got, "8/60") || !strings.Contains(got, "24/60") {
		t.Errorf("counter did not update: %q", got)
	}
	frames := 0
	for _, f := range spinnerFrames {
		if strings.ContainsRune(got, f) {
			frames++
		}
	}
	if frames < 2 {
		t.Errorf("only %d distinct frames — it is not animating", frames)
	}
	if !strings.HasSuffix(got, "\r") {
		t.Error("line not erased — the next render would start mid-line")
	}
}

// Piped output must stay clean: no spinner, nothing on the stream at all.
func TestSpinnerSilentWhenNotInteractive(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	s := newSpinner(w, false)
	s.Update("judging your prompts", 1, 10)
	time.Sleep(200 * time.Millisecond)
	s.Stop()
	w.Close()

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	if n != 0 {
		t.Errorf("wrote %d bytes to a non-terminal: %q", n, buf[:n])
	}
}

// Stop must be safe to call twice — it runs both deferred and inline.
func TestSpinnerStopIsIdempotent(t *testing.T) {
	_, w, _ := os.Pipe()
	s := newSpinner(w, true)
	s.Update("x", 0, 0)
	s.Stop()
	s.Stop()
}
