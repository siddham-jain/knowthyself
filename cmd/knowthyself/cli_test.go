package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/siddham-jain/knowthyself/internal/insight/deepeval"
	"github.com/siddham-jain/knowthyself/internal/tui"
)

// scratchConfig points knowthyself's state at a temp dir. Every test that touches
// configuration must use it — the real config holds a live API key.
func scratchConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	dir := filepath.Join(root, "knowthyself")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func seed(t *testing.T, dir string, providers map[string]deepeval.Provider, active string) {
	t.Helper()
	var s deepeval.Store
	for name, p := range providers {
		s.Add(name, p)
	}
	s.Active = active
	if err := deepeval.SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}
}

func TestDispatchResolvesNamesAndAliases(t *testing.T) {
	for _, name := range []string{"config", "settings", "status", "model", "use", "key", "provider", "providers", "update", "help"} {
		if _, ok := lookupCommand(name); !ok {
			t.Errorf("%q does not dispatch", name)
		}
	}
	if _, ok := lookupCommand("nonsense"); ok {
		t.Error("an unknown name should not resolve")
	}
}

// A typo should name the command it meant; an unrelated word should not guess.
func TestDidYouMean(t *testing.T) {
	cases := map[string]string{
		"proivder": "provider",
		"modle":    "model",
		"cofnig":   "config",
		"udpate":   "update",
		"hlep":     "help",
	}
	for typo, want := range cases {
		got, ok := nearest(typo)
		if !ok || got != want {
			t.Errorf("nearest(%q) = %q (%v), want %q", typo, got, ok, want)
		}
	}
	if got, ok := nearest("xyzzy"); ok {
		t.Errorf("nearest(\"xyzzy\") = %q, want no suggestion", got)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	var b strings.Builder
	printHelp(&b, false)
	out := b.String()
	for _, c := range commands() {
		if c.name == "help" {
			continue
		}
		if !strings.Contains(out, c.name) {
			t.Errorf("help omits %q", c.name)
		}
	}
	if strings.Contains(out, "--store") {
		t.Error("rare flags should be behind --help-all")
	}
	var all strings.Builder
	printHelp(&all, true)
	if !strings.Contains(all.String(), "--store") {
		t.Error("--help-all should show the rare flags")
	}
}

func TestModelShowAndSet(t *testing.T) {
	dir := scratchConfig(t)
	seed(t, dir, map[string]deepeval.Provider{
		"a": {BaseURL: "https://a.test/v1", Model: "old-model", APIKey: "k"},
	}, "a")

	if err := runModel([]string{"new-model"}); err != nil {
		t.Fatal(err)
	}
	store, _ := deepeval.LoadStore(dir)
	if got := store.Providers["a"].Model; got != "new-model" {
		t.Errorf("model = %q, want new-model", got)
	}
	// Unrelated fields must survive the write.
	if store.Providers["a"].APIKey != "k" || store.Providers["a"].BaseURL != "https://a.test/v1" {
		t.Errorf("setting the model clobbered other fields: %+v", store.Providers["a"])
	}
}

func TestFlatCommandsExplainWhenNothingConfigured(t *testing.T) {
	scratchConfig(t)
	for name, fn := range map[string]func([]string) error{
		"model": runModel,
		"key":   runKey,
	} {
		err := fn(nil)
		if err == nil {
			t.Fatalf("%s should fail with no provider", name)
		}
		if !strings.Contains(deepeval.Explain(err), "provider add") {
			t.Errorf("%s error does not name the fix: %s", name, deepeval.Explain(err))
		}
	}
}

// A key passed as argv would land in shell history, so it must be refused.
func TestKeyRefusesArgv(t *testing.T) {
	dir := scratchConfig(t)
	seed(t, dir, map[string]deepeval.Provider{"a": {BaseURL: "https://a.test/v1", Model: "m", APIKey: "k"}}, "a")

	err := runKey([]string{"sk-secret"})
	if err == nil {
		t.Fatal("passing a key as an argument must be refused")
	}
	if !strings.Contains(err.Error(), "shell history") {
		t.Errorf("the refusal should say why: %v", err)
	}
	store, _ := deepeval.LoadStore(dir)
	if store.Providers["a"].APIKey != "k" {
		t.Error("the refused key was written anyway")
	}
}

func TestKeyFromEnvClearsStoredKey(t *testing.T) {
	dir := scratchConfig(t)
	seed(t, dir, map[string]deepeval.Provider{"a": {BaseURL: "https://a.test/v1", Model: "m", APIKey: "stored"}}, "a")

	if err := runKey([]string{"--env", "SOME_VAR"}); err != nil {
		t.Fatal(err)
	}
	store, _ := deepeval.LoadStore(dir)
	p := store.Providers["a"]
	if p.KeyEnv != "SOME_VAR" || p.APIKey != "" {
		t.Errorf("switching to an env var must drop the stored key: %+v", p)
	}
}

// `provider add groq --key-env X` must parse the flags: Go's flag package stops at
// the first positional, so the name has to be pulled off first.
func TestProviderAddParsesFlagsAfterName(t *testing.T) {
	f, err := parseProviderFlags("add", []string{"groq", "--model", "m1", "--key-env", "GROQ_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	if f.name != "groq" || f.model != "m1" || f.keyEnv != "GROQ_KEY" {
		t.Fatalf("flags after the name were dropped: %+v", f)
	}
	// A bare known name should seed from that preset.
	d := draftFrom(providerFlags{name: "groq", keyEnv: "GROQ_KEY"}, tui.ProviderDraft{})
	if d.BaseURL == "" || d.Model == "" {
		t.Errorf("a preset name should fill base URL and model: %+v", d)
	}
	if !complete(d) {
		t.Errorf("preset + key-env should be enough to save without a wizard: %+v", d)
	}
}

func TestPrintConfigIsStableForScripts(t *testing.T) {
	dir := scratchConfig(t)
	seed(t, dir, map[string]deepeval.Provider{
		"a": {BaseURL: "https://a.test/v1", Model: "m", APIKey: "k"},
	}, "a")

	var b strings.Builder
	if err := printConfig(&b, dir); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"a", "https://a.test/v1", "m"} {
		if !strings.Contains(out, want) {
			t.Errorf("config output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "k") && strings.Contains(out, "key saved") == false {
		t.Error("the key itself must never be printed")
	}
}
