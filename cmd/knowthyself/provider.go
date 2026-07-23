package main

import (
	"context"
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
		return providerAdd(dir)
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

func providerAdd(dir string) error {
	if !isInteractive() {
		return fmt.Errorf("`provider add` is guided and needs a terminal — edit %s directly for scripted setup", deepeval.ConfigPath(dir))
	}
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	draft, ok, err := tui.RunProviderWizard(termWidth(), nil)
	if err != nil || !ok {
		if err == nil {
			fmt.Println("cancelled — nothing saved")
		}
		return err
	}
	return saveDraft(dir, store, draft, "added")
}

func providerEdit(dir string, args []string) error {
	if !isInteractive() {
		return fmt.Errorf("`provider edit` is guided and needs a terminal — edit %s directly for scripted setup", deepeval.ConfigPath(dir))
	}
	store, err := deepeval.LoadStore(dir)
	if err != nil {
		return err
	}
	name, err := pickProvider(store, args, "EDIT WHICH PROVIDER?")
	if err != nil {
		return err
	}
	if name == "" {
		return nil
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
