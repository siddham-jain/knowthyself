package deepeval

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Dialect is the wire format an endpoint speaks.
type Dialect string

const (
	DialectAnthropic Dialect = "anthropic"
	DialectOpenAI    Dialect = "openai"
)

const (
	defaultBaseURL = "https://api.anthropic.com/v1"
	defaultModel   = "claude-sonnet-5"
)

// Config is the resolved BYOK connection. Credentials are never logged or
// serialized.
type Config struct {
	APIKey string `json:"-"`
	// AuthToken is an OAuth bearer token (e.g. from `ant auth login`), used instead
	// of APIKey when present. Note a Claude Code subscription login is not one of
	// these and cannot authenticate the API.
	AuthToken string  `json:"-"`
	BaseURL   string  `json:"base_url,omitempty"`
	Model     string  `json:"model,omitempty"`
	Dialect   Dialect `json:"dialect,omitempty"`
	// Provider is the saved provider this came from, "" when it came from flags,
	// environment, or defaults.
	Provider string `json:"-"`

	MaxPrompts int `json:"max_prompts,omitempty"`
	CharBudget int `json:"char_budget,omitempty"`
}

// Host is the endpoint host alone — what the consent screen shows and what a
// DeepRead records. It must never carry credentials.
func (c Config) Host() string {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" {
		return c.BaseURL
	}
	return u.Host
}

// Flags are the command-line overrides, empty when unset.
type Flags struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	Dialect  string
}

// Resolve builds the connection from flags, then environment, then the selected
// saved provider, then defaults. dir is knowthyself's state directory.
func Resolve(f Flags, dir string) (Config, error) {
	store, err := LoadStore(dir)
	if err != nil {
		return Config{}, err
	}
	if f.Provider != "" {
		if _, ok := store.Providers[f.Provider]; !ok {
			return Config{}, ErrUnknownProvider{Name: f.Provider, Known: store.Names()}
		}
	}
	saved, name, _ := store.selected(f.Provider)

	cfg := Config{Provider: name}
	cfg.APIKey = firstNonEmpty(f.APIKey, os.Getenv("KNOWTHYSELF_API_KEY"), saved.Key(),
		os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("OPENAI_API_KEY"))
	cfg.AuthToken = firstNonEmpty(os.Getenv("KNOWTHYSELF_AUTH_TOKEN"), os.Getenv("ANTHROPIC_AUTH_TOKEN"))
	cfg.BaseURL = firstNonEmpty(f.BaseURL, os.Getenv("KNOWTHYSELF_BASE_URL"), saved.BaseURL, defaultBaseURL)
	cfg.Model = firstNonEmpty(f.Model, os.Getenv("KNOWTHYSELF_MODEL"), saved.Model, defaultModel)

	dialect := firstNonEmpty(f.Dialect, os.Getenv("KNOWTHYSELF_API_DIALECT"), string(saved.Dialect))
	switch Dialect(dialect) {
	case DialectAnthropic, DialectOpenAI:
		cfg.Dialect = Dialect(dialect)
	case "":
		cfg.Dialect = detectDialect(cfg.BaseURL)
	default:
		return Config{}, fmt.Errorf("unknown API dialect %q — use \"anthropic\" or \"openai\"", dialect)
	}

	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if u, err := url.Parse(cfg.BaseURL); err != nil || u.Host == "" {
		return cfg, fmt.Errorf("invalid base URL %q — it must be a full http(s) URL", cfg.BaseURL)
	}
	// A locally served endpoint generally has no credential to give.
	if cfg.APIKey == "" && cfg.AuthToken == "" && !isLocal(cfg.BaseURL) {
		return cfg, ErrNoKey{Host: cfg.Host()}
	}
	return cfg, nil
}

// detectDialect guesses from the host. Anthropic's own API is the only one assumed
// to speak the Anthropic format; everything else is treated as OpenAI-compatible,
// which is what the great majority of BYOK endpoints implement.
func detectDialect(baseURL string) Dialect {
	if strings.Contains(baseURL, "api.anthropic.com") {
		return DialectAnthropic
	}
	return DialectOpenAI
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
