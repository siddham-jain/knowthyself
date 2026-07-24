package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
	"github.com/siddham-jain/knowthyself/internal/tui"
)

const providerUsage = `usage: knowthyself provider <command>

Manage the endpoints --deep-eval can call. Any OpenAI- or Anthropic-compatible
API works, including one running on your own machine.

commands:
  list              show saved providers (default)
  add               add one, guided
  edit [name]       change any field, base URL included
  use <name>        make one the default
  remove <name>     delete one
  test [name]       send one tiny request to check it works
  presets           list the known endpoints --preset accepts

flags for add/edit:
  --preset NAME     start from a known endpoint
  --base-url URL    API root
  --model ID        model the endpoint serves
  --key-env NAME    environment variable holding the key (preferred)
  --key KEY         the key itself (lands in shell history \u2014 prefer --key-env)
  --dialect D       anthropic or openai

  knowthyself provider add groq --model llama-3.3-70b-versatile --key-env GROQ_API_KEY
  knowthyself provider edit openrouter --model openai/gpt-4o-mini
`

func runProvider(args []string) error {
	cmd := "list"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}
	dir := stateDir()

	switch cmd {
	case "list", "ls":
		return providerList(dir)
	case "add", "new":
		return providerAdd(dir, args)
	case "presets":
		for _, p := range deepeval.Presets {
			fmt.Printf("  %-12s %s\n", p.Name, p.BaseURL)
		}
		return nil
	case "edit":
		return providerEdit(dir, args)
	case "use", "default":
		return providerUse(dir, args)
	case "remove", "rm", "delete":
		return providerRemove(dir, args)
	case "test", "check":
		return providerTest(dir, args)
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, providerUsage)
		return nil
	default:
		fmt.Fprint(os.Stderr, providerUsage)
		return fmt.Errorf("unknown provider command %q", cmd)
	}
}

func providerList(dir string) error {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	names := store.Names()
	if len(names) == 0 {
		fmt.Println("no providers configured yet\n\n  knowthyself provider add   — add one, guided")
		return nil
	}
	fmt.Printf("providers (%s)\n\n", deepeval.ConfigPath(dir))
	for _, name := range names {
		marker := "  "
		if name == store.Active {
			marker = "▸ "
		}
		p := store.Providers[name]
		fmt.Printf("%s%-14s %s\n", marker, name, p.BaseURL)
		fmt.Printf("  %-14s %s\n", "", p.Describe())
	}
	fmt.Println("\n▸ = used by default; change with `knowthyself provider use <name>`")
	return nil
}

// providerFlags are the non-interactive equivalents of the wizard fields. Supplying
// enough of them skips the wizard entirely; supplying some prefills it.
type providerFlags struct {
	name, model, baseURL, key, keyEnv, dialect, preset string
}

func parseProviderFlags(cmd string, args []string) (providerFlags, error) {
	var f providerFlags
	fs := flag.NewFlagSet("knowthyself provider "+cmd, flag.ContinueOnError)
	fs.StringVar(&f.preset, "preset", "", "start from a known endpoint (see `knowthyself provider presets`)")
	fs.StringVar(&f.baseURL, "base-url", "", "API root, e.g. https://host/v1")
	fs.StringVar(&f.model, "model", "", "model id the endpoint serves")
	fs.StringVar(&f.key, "key", "", "API key (prefer --key-env; an argument lands in shell history)")
	fs.StringVar(&f.keyEnv, "key-env", "", "environment variable holding the key")
	fs.StringVar(&f.dialect, "dialect", "", "wire format: anthropic or openai")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: knowthyself provider %s [name] [flags]\n\nflags:\n", cmd)
		fs.PrintDefaults()
	}
	// Go's flag package stops at the first non-flag argument, so the name is taken
	// off the front before parsing — otherwise `provider add groq --model x` would
	// silently ignore every flag after the name.
	var positional []string
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positional = append(positional, args[0])
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	positional = append(positional, fs.Args()...)
	if len(positional) > 0 {
		f.name = positional[0]
	}
	return f, nil
}

// draftFrom builds a wizard draft from a preset plus flag overrides.
func draftFrom(f providerFlags, base tui.ProviderDraft) tui.ProviderDraft {
	// A bare name that matches a known endpoint is treated as that preset, so
	// `provider add groq --key-env GROQ_API_KEY` is all it takes.
	from := f.preset
	if from == "" && base.BaseURL == "" {
		from = f.name
	}
	if from != "" {
		if p, ok := deepeval.LookupPreset(from); ok {
			base = tui.ProviderDraft{
				Name: p.Name, BaseURL: p.BaseURL, Model: p.Model, Dialect: string(p.Dialect),
			}
		}
	}
	if f.name != "" {
		base.Name = f.name
	}
	if f.baseURL != "" {
		base.BaseURL = f.baseURL
	}
	if f.model != "" {
		base.Model = f.model
	}
	if f.dialect != "" {
		base.Dialect = f.dialect
	}
	if f.keyEnv != "" {
		base.KeyEnv, base.APIKey = f.keyEnv, ""
	}
	if f.key != "" {
		base.APIKey, base.KeyEnv = f.key, ""
	}
	return base
}

// complete reports whether a draft can be saved without asking anything else.
func complete(d tui.ProviderDraft) bool {
	if d.Name == "" || d.BaseURL == "" || d.Model == "" {
		return false
	}
	return d.APIKey != "" || d.KeyEnv != "" || isLocalURL(d.BaseURL)
}

func isLocalURL(u string) bool {
	return strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1")
}

func providerAdd(dir string, args []string) error {
	f, err := parseProviderFlags("add", args)
	if err != nil {
		return err
	}
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}

	draft := draftFrom(f, tui.ProviderDraft{})
	if complete(draft) {
		return saveDraft(dir, store, draft, "added")
	}
	if !isInteractive() {
		return fmt.Errorf("not enough to add a provider without a terminal.\n" +
			"  from a known endpoint:  knowthyself provider add groq --key-env GROQ_API_KEY\n" +
			"  or fully specified:     knowthyself provider add NAME --base-url URL --model ID --key-env VAR\n" +
			"  known endpoints:        knowthyself provider presets")
	}

	var seed *tui.ProviderDraft
	if draft != (tui.ProviderDraft{}) {
		seed = &draft
	}
	saved, ok, err := tui.RunProviderWizard(termWidth(), seed)
	if err != nil || !ok {
		if err == nil {
			fmt.Println("cancelled — nothing saved")
		}
		return err
	}
	return saveDraft(dir, store, saved, "added")
}

func providerEdit(dir string, args []string) error {
	f, err := parseProviderFlags("edit", args)
	if err != nil {
		return err
	}
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}

	var pick []string
	if f.name != "" {
		pick = []string{f.name}
	}
	name, err := pickProvider(store, pick, "EDIT WHICH PROVIDER?")
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	if _, ok := store.Providers[name]; !ok {
		return unknownProvider(store, name)
	}

	// Flags alone are enough to change a field without opening the wizard.
	if f.model != "" || f.baseURL != "" || f.key != "" || f.keyEnv != "" || f.dialect != "" {
		p := store.Providers[name]
		edited := draftFrom(f, tui.ProviderDraft{
			Name: name, BaseURL: p.BaseURL, Model: p.Model,
			Dialect: string(p.Dialect), APIKey: p.APIKey, KeyEnv: p.KeyEnv,
		})
		edited.Name = name
		return saveDraft(dir, store, edited, "updated")
	}
	if !isInteractive() {
		return fmt.Errorf("nothing to change — pass --model, --base-url, --key-env or --dialect")
	}

	p := store.Providers[name]
	existing := tui.ProviderDraft{
		Name: name, BaseURL: p.BaseURL, Model: p.Model,
		Dialect: string(p.Dialect), APIKey: p.APIKey, KeyEnv: p.KeyEnv,
	}
	draft, ok, err := tui.RunProviderWizard(termWidth(), &existing)
	if err != nil || !ok {
		if err == nil {
			fmt.Println("cancelled — nothing changed")
		}
		return err
	}
	// A rename replaces the old entry rather than leaving a duplicate behind.
	if draft.Name != name {
		store.Remove(name)
	}
	return saveDraft(dir, store, draft, "updated")
}

func saveDraft(dir string, store deepeval.Store, draft tui.ProviderDraft, verb string) error {
	store.Add(draft.Name, deepeval.Provider{
		BaseURL: draft.BaseURL,
		Model:   draft.Model,
		Dialect: deepeval.Dialect(draft.Dialect),
		APIKey:  draft.APIKey,
		KeyEnv:  draft.KeyEnv,
	})
	if err := deepeval.SaveStore(dir, store); err != nil {
		return err
	}
	fmt.Printf("%s %s → %s\n", verb, draft.Name, draft.BaseURL)
	if store.Active == draft.Name {
		fmt.Println("it is now the default for --deep-eval")
	}
	fmt.Printf("\ntry it:  knowthyself provider test %s\n", draft.Name)
	return nil
}

func providerUse(dir string, args []string) error {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	name, err := pickProvider(store, args, "USE WHICH PROVIDER BY DEFAULT?")
	if err != nil || name == "" {
		return err
	}
	if !store.Use(name) {
		return unknownProvider(store, name)
	}
	if err := deepeval.SaveStore(dir, store); err != nil {
		return err
	}
	fmt.Printf("--deep-eval now uses %s by default\n", name)
	return nil
}

func providerRemove(dir string, args []string) error {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	name, err := pickProvider(store, args, "REMOVE WHICH PROVIDER?")
	if err != nil || name == "" {
		return err
	}
	if !store.Remove(name) {
		return unknownProvider(store, name)
	}
	if err := deepeval.SaveStore(dir, store); err != nil {
		return err
	}
	fmt.Printf("removed %s\n", name)
	if store.Active != "" {
		fmt.Printf("default is now %s\n", store.Active)
	}
	return nil
}

// providerTest sends one minimal request so a misconfiguration surfaces here rather
// than part-way through a deep read.
func providerTest(dir string, args []string) error {
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else if store.Active != "" {
		name = store.Active
	} else if name, err = pickProvider(store, args, "TEST WHICH PROVIDER?"); err != nil || name == "" {
		return err
	}

	cfg, err := deepeval.Resolve(deepeval.Flags{Provider: name}, dir)
	if err != nil {
		return err
	}
	fmt.Printf("testing %s\n  %s\n  %s · %s\n\n", name, cfg.BaseURL, cfg.Model, cfg.Dialect)

	reply, err := deepeval.NewClient(cfg).Complete(context.Background(),
		"Reply with the single word: ok", "Reply with the single word: ok")
	if err != nil {
		return err
	}
	fmt.Printf("  reachable — replied %q\n", strings.TrimSpace(truncateLine(reply, 60)))
	return nil
}

// pickProvider takes the name from args, or asks when there's a terminal.
func pickProvider(store deepeval.Store, args []string, title string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	names := store.Names()
	if len(names) == 0 {
		return "", fmt.Errorf("no providers configured — add one with `knowthyself provider add`")
	}
	if !isInteractive() {
		return "", fmt.Errorf("name a provider: %s", strings.Join(names, ", "))
	}
	details := make([]string, len(names))
	for i, n := range names {
		details[i] = store.Providers[n].BaseURL
	}
	return tui.RunProviderPicker(termWidth(), title, names, details)
}

func unknownProvider(store deepeval.Store, name string) error {
	return deepeval.ErrUnknownProvider{Name: name, Known: store.Names()}
}

func truncateLine(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
