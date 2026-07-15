// Command synch reads your AI coding assistant session logs and profiles how you
// collaborate with AI. Default: sync + open the TUI dashboard. --json emits the raw
// profile. See the plan for the full design.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/siddham/synch/internal/engine"
	"github.com/siddham/synch/internal/insight"
	"github.com/siddham/synch/internal/provider"
	"github.com/siddham/synch/internal/provider/claude"
	"github.com/siddham/synch/internal/report"
	"github.com/siddham/synch/internal/store"
	"github.com/siddham/synch/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "synch: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("synch", flag.ContinueOnError)
	var (
		asJSON    = fs.Bool("json", false, "emit the raw profile as JSON instead of the TUI")
		syncOnly  = fs.Bool("sync", false, "sync the cache and print a summary, then exit")
		deepEval  = fs.Bool("deep-eval", false, "use an LLM (BYO API key) to phrase qualitative tips (scores stay deterministic)")
		sourceID  = fs.String("source", claude.ID, "session source to analyze")
		storePath = fs.String("store", store.DefaultPath(), "path to the local cache database")
	)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: synch [flags]\n\nProfiles how you collaborate with your AI coding assistant.\n\nflags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
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
		return fmt.Errorf("no %s sessions found. Have you used it yet?", *sourceID)
	}

	ie := insight.Engine(insight.Heuristic{})
	if *deepEval {
		// Deep-eval only phrases tips; it never changes a score. Falls back to the
		// heuristic engine if no API key is configured.
		if de := insight.NewDeepEval(); de != nil {
			ie = de
		} else {
			fmt.Fprintln(os.Stderr, "synch: --deep-eval set but no ANTHROPIC_API_KEY; using heuristic tips")
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
