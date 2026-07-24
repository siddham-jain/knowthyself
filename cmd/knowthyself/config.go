package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
	"github.com/siddham-jain/knowthyself/internal/tui"
)

// The one-line commands for the adjustments people make constantly. Each operates on
// the active provider and reuses the store API in internal/insight/deepeval.

// errNoProvider is returned by every flat command when nothing is configured.
type errNoProvider struct{ what string }

func (e errNoProvider) Error() string {
	return "no provider configured, so there is no " + e.what + " to change"
}
func (e errNoProvider) Remedy() string {
	return "run `knowthyself provider add` to set one up (or `knowthyself config`)"
}

// activeProvider loads the store and the provider currently selected.
func activeProvider(dir string) (deepeval.Store, string, deepeval.Provider, error) {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return store, "", deepeval.Provider{}, err
	}
	name := store.Active
	if name == "" {
		if names := store.Names(); len(names) > 0 {
			name = names[0]
		}
	}
	p, ok := store.Providers[name]
	if !ok {
		return store, "", deepeval.Provider{}, errNoProvider{what: "model"}
	}
	return store, name, p, nil
}

// runModel prints or sets the active provider's model.
func runModel(args []string) error {
	dir := stateDir()
	store, name, p, err := activeProvider(dir)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		fmt.Println(p.Model)
		return nil
	}
	want := strings.TrimSpace(args[0])
	if want == "" {
		return fmt.Errorf("give a model id, e.g. `knowthyself model gpt-4o-mini`")
	}
	if want == p.Model {
		fmt.Printf("%s is already using %s\n", name, want)
		return nil
	}
	was := p.Model
	p.Model = want
	store.Add(name, p)
	if err := deepeval.SaveStore(dir, store); err != nil {
		return err
	}
	fmt.Printf("%s: %s → %s\n\ncheck it works:  knowthyself provider test\n", name, was, want)
	return nil
}

// runUse switches which saved provider is the default.
func runUse(args []string) error {
	return providerUse(stateDir(), args)
}

// runKey sets the credential for the active provider. The key is never accepted as
// an argument — argv lands in shell history and in the process list.
func runKey(args []string) error {
	fs := flag.NewFlagSet("knowthyself key", flag.ContinueOnError)
	env := fs.String("env", "", "read the key from this environment variable instead of storing it")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: knowthyself key [--env NAME]\n\nReads the key without echoing it. For scripts, pipe it in.\n\nflags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if rest := fs.Args(); len(rest) > 0 {
		return fmt.Errorf("don't pass the key as an argument — it would be saved in your shell history.\n" +
			"  run `knowthyself key` and paste it at the prompt, or `knowthyself key --env NAME`")
	}

	dir := stateDir()
	store, name, p, err := activeProvider(dir)
	if err != nil {
		return errNoProvider{what: "key"}
	}

	if *env != "" {
		p.KeyEnv, p.APIKey = *env, ""
		store.Add(name, p)
		if err := deepeval.SaveStore(dir, store); err != nil {
			return err
		}
		fmt.Printf("%s now reads its key from $%s\n", name, *env)
		if os.Getenv(*env) == "" {
			fmt.Printf("\nheads up: $%s is not set in this shell\n", *env)
		}
		return nil
	}

	key, err := readSecret(fmt.Sprintf("paste the key for %s: ", name))
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("no key entered — nothing changed")
	}
	p.APIKey, p.KeyEnv = key, ""
	store.Add(name, p)
	if err := deepeval.SaveStore(dir, store); err != nil {
		return err
	}
	fmt.Printf("key saved for %s (%s, readable only by you)\n\ncheck it works:  knowthyself provider test\n",
		name, deepeval.ConfigPath(dir))
	return nil
}

// readSecret reads a line without echoing it when stdin is a terminal, and plainly
// when it is piped, so both a human and a script can supply a key.
func readSecret(prompt string) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, prompt)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// runConfig opens the settings screen on a terminal, and prints the same information
// as plain text otherwise so it doubles as a status command for scripts.
func runConfig(args []string) error {
	dir := stateDir()
	if !isInteractive() {
		return printConfig(os.Stdout, dir)
	}
	return tui.RunSettings(termWidth(), dir)
}

func printConfig(w io.Writer, dir string) error {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	names := store.Names()
	if len(names) == 0 {
		fmt.Fprintln(w, "no provider configured\n\n  knowthyself provider add   — set one up")
		return nil
	}
	fmt.Fprintf(w, "config  %s\n\n", deepeval.ConfigPath(dir))
	for _, name := range names {
		marker := "  "
		if name == store.Active {
			marker = "▸ "
		}
		p := store.Providers[name]
		fmt.Fprintf(w, "%s%-14s %s\n  %-14s %s\n", marker, name, p.BaseURL, "", p.Describe())
	}
	fmt.Fprintln(w, "\n▸ = used by --deep-eval    change with `knowthyself use <name>`")
	return nil
}
