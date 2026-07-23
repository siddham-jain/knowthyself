package deepeval

import (
	"regexp"
	"strings"
)

// Redaction runs before anything is displayed or sent. It is deliberately
// conservative: a false positive costs a little judging signal, a false negative
// leaks a credential.
//
// Order matters. Secret patterns run first, before the path and URL rules can
// partially rewrite a token and stop it matching.

type rule struct {
	re   *regexp.Regexp
	with string
}

var secretRules = []rule{
	// PEM private key blocks, including the body.
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "<secret>"},
	// Provider-shaped keys.
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{16,}`), "<secret>"},
	{regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{16,}`), "<secret>"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "<secret>"},
	{regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{10,}`), "<secret>"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{30,}`), "<secret>"},
	// JWTs.
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}`), "<secret>"},
	// Assignments that name a secret, however the value is quoted.
	{regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_\-]?key|access[_\-]?key|auth)\b\s*[:=]\s*["']?[^\s"',;)]{4,}`), "$1=<secret>"},
	// Connection strings carrying inline credentials.
	{regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.\-]*://[^\s/@]+:[^\s/@]+@`), "<secret>@"},
}

var (
	emailRE = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	urlRE   = regexp.MustCompile(`\bhttps?://([^\s/?#]+)[^\s]*`)
	ipRE    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	// Absolute unix and windows paths. The basename is kept: which file was named is
	// the signal the rubric needs, the directory tree above it is not.
	unixPathRE = regexp.MustCompile(`(?:/[\w.\-+@]+){2,}/?`)
	winPathRE  = regexp.MustCompile(`\b[A-Za-z]:\\(?:[\w.\-+@]+\\)*[\w.\-+@]*`)
	// Long opaque runs: base64 payloads, hashes, blobs.
	blobRE = regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`)
)

// Redact strips credentials and identifying detail from a prompt while preserving
// the shape a judge needs: which file was named, that a URL was present, that a
// value was assigned.
func Redact(s string) string {
	for _, r := range secretRules {
		s = r.re.ReplaceAllString(s, r.with)
	}

	s = emailRE.ReplaceAllString(s, "<email>")
	s = urlRE.ReplaceAllString(s, "<url:$1>")
	s = blobRE.ReplaceAllString(s, "<blob>")

	s = winPathRE.ReplaceAllStringFunc(s, keepBase)
	s = unixPathRE.ReplaceAllStringFunc(s, keepBase)

	// After paths, so a dotted version inside a path isn't mistaken for an address.
	s = ipRE.ReplaceAllString(s, "<ip>")
	return s
}

// keepBase collapses an absolute path to its final component, which is what the
// rubric actually reads: whether the prompt located the work.
func keepBase(path string) string {
	sep := "/"
	if strings.Contains(path, `\`) {
		sep = `\`
	}
	trimmed := strings.TrimRight(path, sep)
	if trimmed == "" {
		return path
	}
	base := trimmed[strings.LastIndex(trimmed, sep)+1:]
	if base == "" {
		return path
	}
	return "<path>" + sep + base
}
