package deepeval

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A provider is a saved endpoint: base URL, model, wire format, and how to get the
// key. Users register one once and select it by name, so switching between a local
// model and a hosted one is a flag rather than four.

// Provider is one saved endpoint.
type Provider struct {
	BaseURL string  `json:"base_url"`
	Model   string  `json:"model,omitempty"`
	Dialect Dialect `json:"dialect,omitempty"`
	// APIKey is stored only when the user chooses to; the file is written 0600.
	APIKey string `json:"api_key,omitempty"`
	// KeyEnv names an environment variable to read the key from instead of storing
	// it. Preferred over APIKey — the secret stays out of the file entirely.
	KeyEnv string `json:"api_key_env,omitempty"`
}

// Key returns the provider's credential, from the environment when KeyEnv is set.
func (p Provider) Key() string {
	if p.KeyEnv != "" {
		return os.Getenv(p.KeyEnv)
	}
	return p.APIKey
}

// NeedsKey reports whether this endpoint requires a credential. Endpoints served
// from the local machine (Ollama, LM Studio, llama.cpp) generally do not.
func (p Provider) NeedsKey() bool { return !isLocal(p.BaseURL) }

func isLocal(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

// Preset is a known endpoint, so registering one is a menu choice rather than a
// documentation lookup. Base URLs are stable; model names are not, so only the
// Anthropic preset suggests one.
type Preset struct {
	Name    string
	Label   string
	BaseURL string
	Model   string
	Dialect Dialect
	KeyEnv  string
}

// Presets are offered in this order when adding a provider.
var Presets = []Preset{
	{Name: "anthropic", Label: "Anthropic", BaseURL: "https://api.anthropic.com/v1", Model: defaultModel, Dialect: DialectAnthropic, KeyEnv: "ANTHROPIC_API_KEY"},
	{Name: "openai", Label: "OpenAI", BaseURL: "https://api.openai.com/v1", Dialect: DialectOpenAI, KeyEnv: "OPENAI_API_KEY"},
	{Name: "openrouter", Label: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", Dialect: DialectOpenAI, KeyEnv: "OPENROUTER_API_KEY"},
	{Name: "groq", Label: "Groq", BaseURL: "https://api.groq.com/openai/v1", Dialect: DialectOpenAI, KeyEnv: "GROQ_API_KEY"},
	{Name: "together", Label: "Together AI", BaseURL: "https://api.together.xyz/v1", Dialect: DialectOpenAI, KeyEnv: "TOGETHER_API_KEY"},
	{Name: "deepseek", Label: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Dialect: DialectOpenAI, KeyEnv: "DEEPSEEK_API_KEY"},
	{Name: "ollama", Label: "Ollama (local, no key)", BaseURL: "http://localhost:11434/v1", Dialect: DialectOpenAI},
	{Name: "lmstudio", Label: "LM Studio (local, no key)", BaseURL: "http://localhost:1234/v1", Dialect: DialectOpenAI},
	{Name: "custom", Label: "Something else — any OpenAI-compatible endpoint", Dialect: DialectOpenAI},
}

// LookupPreset finds a preset by name.
func LookupPreset(name string) (Preset, bool) {
	for _, p := range Presets {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}

// Store is the on-disk config: saved providers plus which one is active.
type Store struct {
	Active    string              `json:"active,omitempty"`
	Providers map[string]Provider `json:"providers,omitempty"`

	// Legacy single-endpoint fields, still honoured so an existing config keeps
	// working after the move to named providers.
	BaseURL string  `json:"base_url,omitempty"`
	Model   string  `json:"model,omitempty"`
	Dialect Dialect `json:"dialect,omitempty"`
}

// ConfigPath is where saved providers live.
func ConfigPath(dir string) string { return filepath.Join(dir, "config.json") }

// LoadStore reads the saved providers. A missing file is not an error.
func LoadStore(dir string) (Store, error) {
	var s Store
	b, err := os.ReadFile(ConfigPath(dir))
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("could not read %s: %w", ConfigPath(dir), err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("%s is not valid JSON: %w", ConfigPath(dir), err)
	}
	return s, nil
}

// SaveStore writes the providers back at 0600, since the file may hold a key.
func SaveStore(dir string, s Store) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(dir), append(b, '\n'), 0o600)
}

// Names lists saved providers alphabetically.
func (s Store) Names() []string {
	out := make([]string, 0, len(s.Providers))
	for name := range s.Providers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Add saves a provider and makes it active when it is the only one.
func (s *Store) Add(name string, p Provider) {
	if s.Providers == nil {
		s.Providers = map[string]Provider{}
	}
	s.Providers[name] = p
	if s.Active == "" || len(s.Providers) == 1 {
		s.Active = name
	}
}

// Remove deletes a provider, clearing the active selection if it pointed there.
func (s *Store) Remove(name string) bool {
	if _, ok := s.Providers[name]; !ok {
		return false
	}
	delete(s.Providers, name)
	if s.Active == name {
		s.Active = ""
		if rest := s.Names(); len(rest) > 0 {
			s.Active = rest[0]
		}
	}
	return true
}

// Use makes a saved provider active.
func (s *Store) Use(name string) bool {
	if _, ok := s.Providers[name]; !ok {
		return false
	}
	s.Active = name
	return true
}

// selected returns the provider to use: the one named, else the active one, else
// the legacy flat config if it has anything set.
func (s Store) selected(name string) (Provider, string, bool) {
	if name == "" {
		name = s.Active
	}
	if p, ok := s.Providers[name]; ok {
		return p, name, true
	}
	if s.BaseURL != "" || s.Model != "" {
		return Provider{BaseURL: s.BaseURL, Model: s.Model, Dialect: s.Dialect}, "", true
	}
	return Provider{}, "", false
}

// Describe renders a provider for the `provider list` output. It never prints a
// stored key — only where the key comes from.
func (p Provider) Describe() string {
	var parts []string
	if p.Model != "" {
		parts = append(parts, p.Model)
	}
	parts = append(parts, string(p.dialectOrDetected()))
	switch {
	case !p.NeedsKey():
		parts = append(parts, "no key needed")
	case p.KeyEnv != "":
		parts = append(parts, "key from $"+p.KeyEnv)
	case p.APIKey != "":
		parts = append(parts, "key saved")
	default:
		parts = append(parts, "no key configured")
	}
	return strings.Join(parts, " · ")
}

func (p Provider) dialectOrDetected() Dialect {
	if p.Dialect != "" {
		return p.Dialect
	}
	return detectDialect(p.BaseURL)
}
