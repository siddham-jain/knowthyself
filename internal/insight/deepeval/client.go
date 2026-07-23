package deepeval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Plain net/http and encoding/json rather than a vendored SDK: it keeps the build
// CGO-free and dependency-light, and BYOK means we cannot assume whose SDK is right.

const (
	requestTimeout = 120 * time.Second
	// maxTokens caps thinking and text together on models that think by default, so
	// it is set well above what a chunk of judgments actually needs.
	maxTokens = 12000
	// retries applies only to transient failures (429, 5xx). Auth and schema errors
	// are never retried.
	retries = 1
)

// Client talks to one endpoint in one dialect.
type Client struct {
	cfg  Config
	http *http.Client
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: requestTimeout}}
}

// Complete sends a system+user pair and returns the model's text.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		text, err := c.once(ctx, system, user)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !transient(err) {
			return "", err
		}
	}
	return "", lastErr
}

func transient(err error) bool {
	switch err.(type) {
	case ErrRateLimited, ErrUpstream, ErrUnreachable:
		return true
	}
	return false
}

func (c *Client) once(ctx context.Context, system, user string) (string, error) {
	url, body, err := c.request(system, user)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	switch {
	case c.cfg.Dialect == DialectAnthropic && c.cfg.AuthToken != "":
		// An OAuth token is not an API key: it goes on Authorization, and the beta
		// header is required on /v1/messages.
		req.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
		req.Header.Set("anthropic-version", "2023-06-01")
	case c.cfg.Dialect == DialectAnthropic:
		req.Header.Set("x-api-key", c.cfg.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	default:
		req.Header.Set("Authorization", "Bearer "+firstNonEmpty(c.cfg.AuthToken, c.cfg.APIKey))
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", ErrUnreachable{Host: c.cfg.Host(), Err: err}
	}
	defer resp.Body.Close()

	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return "", classify(c.cfg.Host(), c.cfg.Model, resp)
	}
	if readErr != nil {
		return "", ErrUnreachable{Host: c.cfg.Host(), Err: readErr}
	}
	return c.parse(payload)
}

func (c *Client) request(system, user string) (string, []byte, error) {
	if c.cfg.Dialect == DialectAnthropic {
		// No sampling parameters: Sonnet 5, Opus 4.7/4.8 and Fable 5 reject a
		// non-default temperature with a 400, and `thinking: disabled` is itself
		// rejected by Fable 5 — so the only model-agnostic request is one that sends
		// neither. Models that think by default are given headroom by maxTokens
		// instead, since it caps thinking and text together.
		body, err := json.Marshal(map[string]any{
			"model":      c.cfg.Model,
			"max_tokens": maxTokens,
			"system":     system,
			"messages":   []map[string]string{{"role": "user", "content": user}},
		})
		return c.cfg.BaseURL + "/messages", body, err
	}
	body, err := json.Marshal(map[string]any{
		"model":       c.cfg.Model,
		"max_tokens":  maxTokens,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	return c.cfg.BaseURL + "/chat/completions", body, err
}

func (c *Client) parse(payload []byte) (string, error) {
	badFormat := func() error {
		return ErrBadFormat{Host: c.cfg.Host(), Dialect: c.cfg.Dialect, Snippet: snippet(payload)}
	}
	if c.cfg.Dialect == DialectAnthropic {
		var out struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(payload, &out); err != nil || len(out.Content) == 0 {
			return "", badFormat()
		}
		var b strings.Builder
		for _, blk := range out.Content {
			if blk.Type == "text" {
				b.WriteString(blk.Text)
			}
		}
		if b.Len() == 0 {
			return "", badFormat()
		}
		return b.String(), nil
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &out); err != nil || len(out.Choices) == 0 {
		return "", badFormat()
	}
	if out.Choices[0].Message.Content == "" {
		return "", badFormat()
	}
	return out.Choices[0].Message.Content, nil
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// extractJSON pulls the first JSON object out of a model reply. BYOK means we cannot
// rely on native structured-output support, which varies widely across
// "OpenAI-compatible" endpoints, so the object is located in the text instead.
func extractJSON(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		if j := strings.Index(rest, "```"); j >= 0 {
			s = strings.TrimSpace(rest[:j])
		}
	}
	start := strings.Index(s, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object in the reply")
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		ch := s[i]
		switch {
		case esc:
			esc = false
		case ch == '\\' && inStr:
			esc = true
		case ch == '"':
			inStr = !inStr
		case inStr:
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth == 0 {
				return []byte(s[start : i+1]), nil
			}
		}
	}
	return nil, fmt.Errorf("JSON object in the reply is unterminated")
}
