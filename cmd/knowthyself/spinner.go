package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// A deep read is a long series of network round trips. Without something moving on
// screen the terminal looks hung, so progress is drawn on a single line that is
// rewritten in place and erased when the work finishes.

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type spinner struct {
	w        io.Writer
	mu       sync.Mutex
	stop     chan struct{}
	done     sync.WaitGroup
	stage    string
	at, of   int
	lastWide int
}

// newSpinner starts an animated progress line, or a no-op when out is not a
// terminal — piped output must stay clean and scriptable.
func newSpinner(out *os.File, interactive bool) *spinner {
	s := &spinner{w: out, stop: make(chan struct{})}
	if !interactive {
		return s
	}
	s.done.Add(1)
	go s.animate()
	return s
}

func (s *spinner) animate() {
	defer s.done.Done()
	tick := time.NewTicker(90 * time.Millisecond)
	defer tick.Stop()
	frame := 0
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
			s.draw(spinnerFrames[frame%len(spinnerFrames)])
			frame++
		}
	}
}

func (s *spinner) draw(f rune) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stage == "" {
		return
	}
	line := fmt.Sprintf("  %c %s", f, s.stage)
	if s.of > 0 {
		line += fmt.Sprintf("  %d/%d", s.at, s.of)
	}
	// Pad to the previous width so a shorter line can't leave stale characters.
	pad := ""
	if n := s.lastWide - len([]rune(line)); n > 0 {
		pad = strings.Repeat(" ", n)
	}
	s.lastWide = len([]rune(line))
	fmt.Fprint(s.w, "\r"+line+pad)
}

// Update sets what the line reports. Safe to call from the judging goroutine.
func (s *spinner) Update(stage string, done, total int) {
	s.mu.Lock()
	s.stage, s.at, s.of = stage, done, total
	s.mu.Unlock()
}

// Stop halts the animation and erases the line, leaving the cursor at column 0 so
// whatever prints next starts clean.
func (s *spinner) Stop() {
	select {
	case <-s.stop:
		return // already stopped
	default:
	}
	close(s.stop)
	s.done.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastWide > 0 {
		fmt.Fprint(s.w, "\r"+strings.Repeat(" ", s.lastWide)+"\r")
		s.lastWide = 0
	}
}
