// Command knowthyself reads your AI coding assistant session logs and profiles how you
// collaborate with AI. Default: sync + open the TUI dashboard. --json emits the raw
// profile. See the plan for the full design.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/term"

	"github.com/siddham-jain/knowthyself/internal/engine"
	"github.com/siddham-jain/knowthyself/internal/insight"
	"github.com/siddham-jain/knowthyself/internal/provider"
	"github.com/siddham-jain/knowthyself/internal/provider/claude"
	"github.com/siddham-jain/knowthyself/internal/report"
	"github.com/siddham-jain/knowthyself/internal/store"
	"github.com/siddham-jain/knowthyself/internal/tui"
)

// Build metadata, injected at release time via -ldflags "-X main.version=..." by
// GoReleaser. Defaults keep a plain `go build` honest.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "knowthyself: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("knowthyself", flag.ContinueOnError)
	var (
		asJSON      = fs.Bool("json", false, "emit the raw profile as JSON instead of the TUI")
		syncOnly    = fs.Bool("sync", false, "sync the cache and print a summary, then exit")
		showVersion = fs.Bool("version", false, "print version and exit")
		deepEval    = fs.Bool("deep-eval", false, "use an LLM (BYO API key) to phrase qualitative tips (scores stay deterministic)")
		sourceID    = fs.String("source", claude.ID, "session source to analyze")
		storePath   = fs.String("store", store.DefaultPath(), "path to the local cache database")
	)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: knowthyself [flags]\n\nProfiles how you collaborate with your AI coding assistant.\n\nflags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Printf("knowthyself %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	// Playful cold-start screens are for a person at a real terminal; piped/JSON/sync
	// output stays plain and scriptable.
	greet := term.IsTerminal(int(os.Stdout.Fd())) && !*asJSON && !*syncOnly

	// No ~/.claude to read: greet the newcomer instead of failing blankly.
	if *sourceID == claude.ID && !claude.Available() {
		if greet {
			tui.RenderNoClaudeScreen(os.Stdout, termWidth(), claude.DefaultBase())
			return nil
		}
		return fmt.Errorf("no Claude Code history at %s — install Claude Code (claude.com/claude-code), then run knowthyself",
			claude.DefaultBase())
	}

	// First run on a real terminal: ask before profiling, so a newcomer opts in to
	// meeting themselves instead of being dropped straight into the dashboard. Asked
	// only once — running the command again is consent enough.
	if greet {
		marker := firstRunMarker(*storePath)
		if !fileExists(marker) {
			runNow, err := tui.RunFirstRunPrompt(termWidth())
			if err != nil {
				return err
			}
			_ = markFirstRun(marker)
			if !runNow {
				tui.RenderMaybeLater(os.Stdout, termWidth())
				return nil
			}
		}
	}

	ctx := context.Background()
	p, err := provider.Get(*sourceID)
	if err != nil {
		return err
	}
	repo, err := store.Open(*storePath)
	if err != nil {
		return err
	}
	defer repo.Close()

	sessions, res, err := engine.Sessions(ctx, p, repo, time.Now)
	if err != nil {
		return err
	}
	if *syncOnly {
		fmt.Printf("synced %s: %d discovered, %d parsed, %d unchanged, %d removed\n",
			*sourceID, res.Discovered, res.Parsed, res.Unchanged, res.Deleted)
		return nil
	}
	if len(sessions) == 0 {
		if greet {
			tui.RenderNoDataScreen(os.Stdout, termWidth(), 0)
			return nil
		}
		return fmt.Errorf("no %s sessions found yet — use Claude Code a bit, then run knowthyself", *sourceID)
	}

	ie := insight.Engine(insight.Heuristic{})
	if *deepEval {
		// Deep-eval only phrases tips; it never changes a score. Falls back to the
		// heuristic engine if no API key is configured.
		if de := insight.NewDeepEval(); de != nil {
			ie = de
		} else {
			fmt.Fprintln(os.Stderr, "knowthyself: --deep-eval set but no ANTHROPIC_API_KEY; using heuristic tips")
		}
	}

	prof, err := engine.Analyze(ctx, sessions, ie, time.Now)
	if err != nil {
		return err
	}

	var r report.Reporter
	if *asJSON {
		r = report.JSON{W: os.Stdout}
	} else {
		r = tui.New()
	}
	return r.Render(prof)
}

// firstRunMarker is the sentinel that records the welcome prompt has been shown. It
// lives beside the cache so it travels with knowthyself's state, not the working dir.
func firstRunMarker(storePath string) string {
	return filepath.Join(filepath.Dir(storePath), ".welcomed")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func markFirstRun(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("welcomed\n"), 0o644)
}

// termWidth returns the current terminal width, falling back to a sensible default
// when stdout isn't a measurable TTY.
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}
