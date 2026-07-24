package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// One table drives dispatch, help, and did-you-mean, so the three can't drift apart
// as commands are added.

type command struct {
	name    string
	aliases []string
	args    string
	blurb   string
	// detail is the long help shown by `knowthyself help <name>`.
	detail string
	run    func(args []string) error
}

// commands is built lazily because the run funcs reference package-level helpers.
func commands() []command {
	return []command{
		{
			name: "config", aliases: []string{"settings", "status"}, args: "",
			blurb: "show and edit your deep-eval setup",
			detail: `Shows the active provider, model, endpoint and key source.

On a terminal this opens an editable settings screen; piped, it prints plain
text so it works in scripts.

  knowthyself config`,
			run: runConfig,
		},
		{
			name: "model", args: "[id]",
			blurb: "show or set the model used by --deep-eval",
			detail: `With no argument, prints the active provider's model.
With one, sets it and saves.

  knowthyself model                    # print it
  knowthyself model gpt-4o-mini        # set it

The id must be one your endpoint serves — check with:

  knowthyself provider test`,
			run: runModel,
		},
		{
			name: "use", args: "<provider>",
			blurb: "switch which saved provider --deep-eval uses",
			detail: `Makes a saved provider the default for --deep-eval.

  knowthyself use groq

List what you have with ` + "`knowthyself config`" + ` or ` + "`knowthyself provider list`" + `.`,
			run: runUse,
		},
		{
			name: "key", args: "[--env NAME]",
			blurb: "set the API key for the active provider",
			detail: `Reads a key without echoing it, so it never reaches your shell history:

  knowthyself key

Point at an environment variable instead, keeping the secret off disk entirely:

  knowthyself key --env GROQ_API_KEY

For scripts, pipe it in:

  echo "$MY_KEY" | knowthyself key`,
			run: runKey,
		},
		{
			name: "provider", aliases: []string{"providers"}, args: "<command>",
			blurb:  "add, edit, test and remove providers",
			detail: strings.TrimSpace(providerUsage),
			run:    runProvider,
		},
		{
			name: "update", args: "[--check]",
			blurb: "upgrade knowthyself to the latest release",
			detail: `Checks GitHub Releases and upgrades in place.

  knowthyself update           # upgrade
  knowthyself update --check   # just report

A binary installed by npm, Homebrew or ` + "`go install`" + ` is never replaced —
the right command for that manager is printed instead.`,
			run: runUpdate,
		},
		{
			name: "help", aliases: []string{"--help", "-h"}, args: "[command]",
			blurb: "show help for a command",
			run:   runHelp,
		},
	}
}

func lookupCommand(name string) (command, bool) {
	for _, c := range commands() {
		if c.name == name {
			return c, true
		}
		for _, a := range c.aliases {
			if a == name {
				return c, true
			}
		}
	}
	return command{}, false
}

func runHelp(args []string) error {
	if len(args) > 0 {
		c, ok := lookupCommand(args[0])
		if !ok {
			return unknownCommand(args[0])
		}
		fmt.Printf("knowthyself %s %s\n    %s\n", c.name, c.args, c.blurb)
		if c.detail != "" {
			fmt.Printf("\n%s\n", c.detail)
		}
		return nil
	}
	printHelp(os.Stdout, false)
	return nil
}

// commonFlags are shown by default; the rest are for people who already know they
// want them, and only clutter the first read.
var commonFlags = []struct{ flag, blurb string }{
	{"--deep-eval", "add a model-judged read of how you phrase things"},
	{"--json", "print the raw profile as JSON instead of the dashboard"},
	{"--sync", "refresh the local cache and exit"},
	{"--version, -v", "print the version"},
}

var rareFlags = []struct{ flag, blurb string }{
	{"--provider <name>", "use a specific saved provider for this run"},
	{"--model <id>", "override the model for this run"},
	{"--api-key <key>", "override the key for this run"},
	{"--base-url <url>", "override the endpoint for this run"},
	{"--api-dialect <d>", "wire format: anthropic or openai"},
	{"--source <id>", "session source to analyze (default claude-code)"},
	{"--store <path>", "path to the local cache database"},
}

func printHelp(w io.Writer, all bool) {
	fmt.Fprintln(w, "knowthyself — see how you actually collaborate with your AI coding assistant")
	fmt.Fprintln(w, "\nusage:\n  knowthyself                 open the dashboard\n  knowthyself <command>       everything else")

	fmt.Fprintln(w, "\ncommands:")
	for _, c := range commands() {
		if c.name == "help" {
			continue
		}
		fmt.Fprintf(w, "  %-20s %s\n", strings.TrimSpace(c.name+" "+c.args), c.blurb)
	}

	fmt.Fprintln(w, "\nflags:")
	for _, f := range commonFlags {
		fmt.Fprintf(w, "  %-20s %s\n", f.flag, f.blurb)
	}
	if all {
		for _, f := range rareFlags {
			fmt.Fprintf(w, "  %-20s %s\n", f.flag, f.blurb)
		}
	} else {
		fmt.Fprintf(w, "  %-20s %s\n", "--help-all", "show the remaining flags")
	}

	fmt.Fprintln(w, "\nexamples:\n"+
		"  knowthyself                          look at your profile\n"+
		"  knowthyself model gpt-4o-mini        change the judging model\n"+
		"  knowthyself use groq                 switch provider\n"+
		"  knowthyself --deep-eval              add a model-judged read\n"+
		"\n  knowthyself help <command>           detail on any command")
}

// unknownCommand suggests the closest real command instead of just refusing.
func unknownCommand(name string) error {
	if best, ok := nearest(name); ok {
		return fmt.Errorf("unknown command %q — did you mean `knowthyself %s`?", name, best)
	}
	return fmt.Errorf("unknown command %q — run `knowthyself help` to see them all", name)
}

// nearest finds the closest command name within a small edit distance, so an obvious
// typo is corrected but an unrelated word is not.
func nearest(name string) (string, bool) {
	type scored struct {
		name string
		dist int
	}
	var best []scored
	for _, c := range commands() {
		for _, candidate := range append([]string{c.name}, c.aliases...) {
			if strings.HasPrefix(candidate, "-") {
				continue
			}
			if d := editDistance(name, candidate); d <= 2 {
				best = append(best, scored{c.name, d})
			}
		}
	}
	if len(best) == 0 {
		return "", false
	}
	sort.SliceStable(best, func(i, j int) bool {
		if best[i].dist != best[j].dist {
			return best[i].dist < best[j].dist
		}
		return best[i].name < best[j].name
	})
	return best[0].name, true
}

// editDistance is Levenshtein, iterative with two rows.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = minOf(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}

func minOf(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
