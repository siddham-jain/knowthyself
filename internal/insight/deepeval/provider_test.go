package deepeval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	var s Store
	s.Add("groq", Provider{BaseURL: "https://api.groq.com/openai/v1", Model: "llama", Dialect: DialectOpenAI, APIKey: "gsk-x"})
	s.Add("local", Provider{BaseURL: "http://localhost:11434/v1", Model: "llama3.1", Dialect: DialectOpenAI})

	if s.Active != "groq" {
		t.Errorf("first provider added should become active, got %q", s.Active)
	}
	if err := SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}

	// The file may hold a key, so it must not be world- or group-readable.
	fi, err := os.Stat(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 600 — it can contain a key", perm)
	}

	back, err := LoadStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := back.Names(); len(got) != 2 || got[0] != "groq" || got[1] != "local" {
		t.Errorf("names = %v", got)
	}
	if back.Providers["groq"].APIKey != "gsk-x" {
		t.Error("key did not round-trip")
	}
}

func TestLoadStoreMissingFileIsNotAnError(t *testing.T) {
	s, err := LoadStore(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing config should be fine, got %v", err)
	}
	if len(s.Names()) != 0 {
		t.Error("expected an empty store")
	}
}

func TestRemoveReassignsActive(t *testing.T) {
	var s Store
	s.Add("a", Provider{BaseURL: "https://a.test/v1"})
	s.Add("b", Provider{BaseURL: "https://b.test/v1"})
	s.Use("a")

	if !s.Remove("a") {
		t.Fatal("remove failed")
	}
	if s.Active != "b" {
		t.Errorf("active = %q, want the remaining provider", s.Active)
	}
	if s.Remove("gone") {
		t.Error("removing an unknown provider should report false")
	}
}

// A key held in an env var must never be written to the config file.
func TestKeyFromEnvStaysOutOfTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TEST_PROVIDER_KEY", "super-secret")

	var s Store
	s.Add("envy", Provider{BaseURL: "https://x.test/v1", Model: "m", KeyEnv: "TEST_PROVIDER_KEY"})
	if err := SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "super-secret") {
		t.Error("env-sourced key was written to disk")
	}

	cfg, err := Resolve(Flags{Provider: "envy"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "super-secret" {
		t.Errorf("key not read from the env var: %q", cfg.APIKey)
	}
}

// A locally served endpoint needs no credential.
func TestLocalProviderNeedsNoKey(t *testing.T) {
	dir := t.TempDir()
	var s Store
	s.Add("ollama", Provider{BaseURL: "http://localhost:11434/v1", Model: "llama3.1", Dialect: DialectOpenAI})
	if err := SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}
	cfg, err := Resolve(Flags{Provider: "ollama"}, dir)
	if err != nil {
		t.Fatalf("a local endpoint should resolve without a key: %v", err)
	}
	if cfg.Host() != "localhost:11434" || cfg.Model != "llama3.1" {
		t.Errorf("cfg = %+v", cfg)
	}
}

// Flags beat the saved provider, which beats the defaults.
func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	var s Store
	s.Add("saved", Provider{BaseURL: "https://saved.test/v1", Model: "saved-model", Dialect: DialectOpenAI, APIKey: "k"})
	if err := SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}

	cfg, err := Resolve(Flags{Provider: "saved", Model: "flag-model"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "flag-model" || cfg.Host() != "saved.test" {
		t.Errorf("flag should override only the model: %+v", cfg)
	}
	if cfg.Provider != "saved" {
		t.Errorf("provider name not recorded: %q", cfg.Provider)
	}
}

func TestResolveUnknownProviderNamesTheKnownOnes(t *testing.T) {
	dir := t.TempDir()
	var s Store
	s.Add("one", Provider{BaseURL: "https://one.test/v1", APIKey: "k"})
	if err := SaveStore(dir, s); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(Flags{Provider: "missing"}, dir)
	var unknown ErrUnknownProvider
	if !asErr(err, &unknown) {
		t.Fatalf("err = %T, want ErrUnknownProvider", err)
	}
	if !strings.Contains(Explain(err), "one") {
		t.Errorf("remedy should list saved providers: %s", Explain(err))
	}
}

// A config written before named providers existed must keep working.
func TestLegacyFlatConfigStillResolves(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"base_url":"https://legacy.test/v1","model":"legacy-model","dialect":"openai"}`
	if err := os.WriteFile(ConfigPath(dir), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KNOWTHYSELF_API_KEY", "k")

	cfg, err := Resolve(Flags{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host() != "legacy.test" || cfg.Model != "legacy-model" {
		t.Errorf("legacy config ignored: %+v", cfg)
	}
}

func TestEveryPresetIsUsable(t *testing.T) {
	for _, p := range Presets {
		if p.Name == "" || p.Label == "" {
			t.Errorf("preset %+v is missing a name or label", p)
		}
		if p.Name == "custom" {
			continue // deliberately blank; the user fills it in
		}
		if !strings.HasPrefix(p.BaseURL, "http") {
			t.Errorf("preset %q has no usable base URL: %q", p.Name, p.BaseURL)
		}
		if p.Dialect != DialectOpenAI && p.Dialect != DialectAnthropic {
			t.Errorf("preset %q has an unknown dialect %q", p.Name, p.Dialect)
		}
		local := !Provider{BaseURL: p.BaseURL}.NeedsKey()
		if local && p.KeyEnv != "" {
			t.Errorf("local preset %q should not expect a key env var", p.Name)
		}
		if !local && p.KeyEnv == "" {
			t.Errorf("hosted preset %q should suggest a key env var", p.Name)
		}
	}
}

func asErr[T error](err error, target *T) bool {
	if v, ok := err.(T); ok {
		*target = v
		return true
	}
	return false
}
