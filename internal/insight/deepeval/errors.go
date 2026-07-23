package deepeval

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Every failure carries a remedy. A user who opted into deep-eval and hit a wall
// should be told what to do next, never handed a bare HTTP status.
type remediable interface {
	Remedy() string
}

// Explain renders an error plus its remedy, for the one line the CLI prints when a
// deep read could not run.
func Explain(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var r remediable
	if errors.As(err, &r) {
		return msg + "\n  " + r.Remedy()
	}
	return msg
}

// ErrNoKey is returned when no API key could be resolved from any source.
type ErrNoKey struct{ Host string }

func (e ErrNoKey) Error() string { return "deep-eval needs an API key and none is configured" }

// The remedy names the Claude Code case explicitly: this tool's users are Claude
// Code users, and most will assume the subscription they already log in with is
// usable here. It is not — that login does not authenticate the API.
func (e ErrNoKey) Remedy() string {
	return "run `knowthyself provider add` to set one up, or pass --api-key / export KNOWTHYSELF_API_KEY.\n" +
		"  a Claude Code subscription login is not an API key — get one at console.anthropic.com,\n" +
		"  or export ANTHROPIC_AUTH_TOKEN if you use an OAuth token from `ant auth login`"
}

// ErrUnknownProvider names a --provider that was never saved.
type ErrUnknownProvider struct {
	Name  string
	Known []string
}

func (e ErrUnknownProvider) Error() string {
	return fmt.Sprintf("no saved provider called %q", e.Name)
}
func (e ErrUnknownProvider) Remedy() string {
	if len(e.Known) == 0 {
		return "add one with `knowthyself provider add`"
	}
	return "saved providers: " + strings.Join(e.Known, ", ") + " — or add one with `knowthyself provider add`"
}

// ErrAuth is a key the endpoint rejected.
type ErrAuth struct {
	Host   string
	Status int
}

func (e ErrAuth) Error() string {
	return fmt.Sprintf("%s rejected the API key (HTTP %d)", e.Host, e.Status)
}
func (e ErrAuth) Remedy() string {
	return "check the key belongs to " + e.Host + " — a key from a different provider will not work here"
}

// ErrModel is a model the endpoint does not serve.
type ErrModel struct{ Host, Model string }

func (e ErrModel) Error() string {
	return fmt.Sprintf("%s does not serve a model called %q", e.Host, e.Model)
}
func (e ErrModel) Remedy() string {
	return "pick one the endpoint lists with --model, or set KNOWTHYSELF_MODEL"
}

// ErrRateLimited is a 429.
type ErrRateLimited struct {
	Host       string
	RetryAfter string
}

func (e ErrRateLimited) Error() string {
	if e.RetryAfter != "" {
		return fmt.Sprintf("%s rate-limited the request (retry after %s)", e.Host, e.RetryAfter)
	}
	return e.Host + " rate-limited the request"
}
func (e ErrRateLimited) Remedy() string { return "wait a moment and run --deep-eval again" }

// ErrUpstream is a 5xx that survived a retry.
type ErrUpstream struct {
	Host   string
	Status int
}

func (e ErrUpstream) Error() string {
	return fmt.Sprintf("%s returned a server error (HTTP %d)", e.Host, e.Status)
}
func (e ErrUpstream) Remedy() string { return "this is the provider's side — try again shortly" }

// ErrUnreachable covers DNS, refused connections, timeouts and TLS failures.
type ErrUnreachable struct {
	Host string
	Err  error
}

func (e ErrUnreachable) Error() string { return "could not reach " + e.Host }
func (e ErrUnreachable) Unwrap() error { return e.Err }
func (e ErrUnreachable) Remedy() string {
	if isTLS(e.Err) {
		return "the TLS handshake failed — check a proxy or corporate certificate is not intercepting the connection"
	}
	return "check your connection, and that --base-url is right (currently " + e.Host + ")"
}

// ErrBadFormat is a body that is not the JSON this dialect expects — usually the
// wrong dialect for the endpoint.
type ErrBadFormat struct {
	Host    string
	Dialect Dialect
	Snippet string
}

func (e ErrBadFormat) Error() string {
	return fmt.Sprintf("%s did not answer in the %s format", e.Host, e.Dialect)
}
func (e ErrBadFormat) Remedy() string {
	other := DialectOpenAI
	if e.Dialect == DialectOpenAI {
		other = DialectAnthropic
	}
	return fmt.Sprintf("try --api-dialect %s, or check --base-url points at the API root", other)
}

// ErrUnusable is a model that could not produce judgments matching the rubric
// schema, even after a repair attempt.
type ErrUnusable struct {
	Model  string
	Valid  int
	Sample int
}

func (e ErrUnusable) Error() string {
	return fmt.Sprintf("%s produced only %d usable judgments out of %d prompts", e.Model, e.Valid, e.Sample)
}
func (e ErrUnusable) Remedy() string {
	return "the model could not follow the response schema — try a more capable model with --model"
}

// classify turns an HTTP response into the typed error for its status.
func classify(host, model string, resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrAuth{Host: host, Status: resp.StatusCode}
	case resp.StatusCode == http.StatusNotFound:
		return ErrModel{Host: host, Model: model}
	case resp.StatusCode == http.StatusTooManyRequests:
		return ErrRateLimited{Host: host, RetryAfter: resp.Header.Get("Retry-After")}
	case resp.StatusCode >= 500:
		return ErrUpstream{Host: host, Status: resp.StatusCode}
	default:
		return fmt.Errorf("%s returned HTTP %d", host, resp.StatusCode)
	}
}

func isTLS(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "certificate")
}
