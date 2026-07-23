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
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/siddham-jain/knowthyself/internal/engine"
	"github.com/siddham-jain/knowthyself/internal/insight"
	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
	"github.com/siddham-jain/knowthyself/internal/provider"
	"github.com/siddham-jain/knowthyself/internal/provider/claude"
	"github.com/siddham-jain/knowthyself/internal/report"
	"github.com/siddham-jain/knowthyself/internal/store"
	"github.com/siddham-jain/knowthyself/internal/tui"
	"github.com/siddham-jain/knowthyself/internal/update"
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
		// Explain appends the error's remedy when it carries one, so guidance is not
		// lost on the way out of a subcommand.
		fmt.Fprintln(os.Stderr, "knowthyself: "+deepeval.Explain(err))
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "update":
			return runUpdate(args[1:])
		case "provider", "providers":
			return runProvider(args[1:])
		default:
			return fmt.Errorf("unknown command %q — run `knowthyself --help` for usage", args[0])
		}
	}

	fs := flag.NewFlagSet("knowthyself", flag.ContinueOnError)
	var (
		asJSON      = fs.Bool("json", false, "emit the raw profile as JSON instead of the TUI")
		syncOnly    = fs.Bool("sync", false, "sync the cache and print a summary, then exit")
		showVersion = fs.Bool("version", false, "print version and exit")
		deepEval    = fs.Bool("deep-eval", false, "add a model-judged read of your prompts, using your own API key (scores stay deterministic and local)")
		sourceID    = fs.String("source", claude.ID, "session source to analyze")
		storePath   = fs.String("store", store.DefaultPath(), "path to the local cache database")

		providerName = fs.String("provider", "", "saved provider to use for --deep-eval (see `knowthyself provider`)")
		apiKey       = fs.String("api-key", "", "API key for --deep-eval (or set KNOWTHYSELF_API_KEY)")
		baseURL      = fs.String("base-url", "", "API base URL for --deep-eval (default https://api.anthropic.com/v1)")
		modelName    = fs.String("model", "", "model to judge with (default claude-sonnet-5)")
		apiDialect   = fs.String("api-dialect", "", "wire format: anthropic or openai (default: inferred from --base-url)")
	)
	fs.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: knowthyself [flags]\n       knowthyself update [--check]\n       knowthyself provider <list|add|edit|use|remove|test>\n\nProfiles how you collaborate with your AI coding assistant.\n\nflags:")
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

	prof, err := engine.Analyze(ctx, sessions, insight.Heuristic{}, time.Now)
	if err != nil {
		return err
	}

	// The deep read is additive and strictly opt-in: it is layered onto a profile
	// that is already complete, and any failure leaves that profile untouched.
	if *deepEval {
		flags := deepeval.Flags{Provider: *providerName, APIKey: *apiKey, BaseURL: *baseURL, Model: *modelName, Dialect: *apiDialect}
		if err := attachDeepRead(ctx, &prof, flags, filepath.Dir(*storePath), sessions, greet); err != nil {
			fmt.Fprintln(os.Stderr, "knowthyself: deep-eval skipped — "+deepeval.Explain(err))
		}
	}

	var r report.Reporter
	if *asJSON {
		r = report.JSON{W: os.Stdout}
	} else {
		r = tui.New(update.Notice(filepath.Dir(*storePath), version))
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

// stateDir is where knowthyself keeps its cache, config, and sentinels.
func stateDir() string { return filepath.Dir(store.DefaultPath()) }

// isInteractive reports whether there is a terminal to run a guided flow in.
func isInteractive() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

// termWidth returns the current terminal width, falling back to a sensible default
// when stdout isn't a measurable TTY.
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}
