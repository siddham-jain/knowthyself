// Package text holds language-agnostic content analysis shared by the parser and
// the scorers. The guiding rule (see plan, "Multilingual & language-agnostic
// scoring"): grade communication quality, not English proficiency. Structural
// signals — file paths, code fences, error patterns, URLs, command shape — work
// identically in English, Hindi, Hinglish, or code-switched text. Lexical cues are
// optional boosters only and never lower a score.
package text

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// commandEnvelope matches Claude Code's slash-command framing:
	// <command-name>/clear</command-name>.
	commandEnvelope = regexp.MustCompile(`(?s)<command-name>([^<]*)</command-name>`)

	// filePath matches path-like tokens: a file with a known-ish extension, an
	// @-mention, or a segmented path. Language-independent.
	filePath = regexp.MustCompile(`(?:@[\w./-]+)|(?:\b[\w./-]+\.[A-Za-z][A-Za-z0-9]{0,4}\b)|(?:(?:\.{0,2}/)[\w./-]+/[\w./-]+)`)

	// codeFence matches Markdown code fences and inline backtick spans.
	codeFence  = regexp.MustCompile("```")
	inlineCode = regexp.MustCompile("`[^`]+`")

	// url matches http(s) URLs.
	url = regexp.MustCompile(`https?://[^\s)]+`)

	// errorPattern matches stack-trace / error markers, which are near-always in
	// English/code regardless of the surrounding prompt language.
	errorPattern = regexp.MustCompile(`(?i)traceback|exception|\b\w*error\b|\bpanic\b|stack ?trace|\bat [\w.$<>]+\([^)]*:\d+\)|\w+\.\w+:\d+|:\d+:\d+|\bundefined\b|\bnullpointer\b|segfault|exit(?: code|status) \d+|errno`)

	// shellStart matches a line beginning with a common shell command token.
	shellStart = regexp.MustCompile(`^(?:sudo\s+)?(?:curl|wget|git|npm|npx|pnpm|yarn|bun|go|python3?|pip3?|node|deno|cargo|rustc|make|docker|kubectl|brew|apt|apt-get|yum|cd|ls|cat|rm|cp|mv|mkdir|touch|chmod|chown|grep|sed|awk|find|tar|unzip|ssh|scp|code|vim|nano|export|source|echo|which|ps|kill|systemctl|bash|sh|zsh)\b`)

	// shellOperator matches shell control operators that signal a command line.
	shellOperator = regexp.MustCompile(`&&|\|\||[|;>]|\$\(`)
)

// ExtractSlashCommand returns the slash command from a command-envelope user turn,
// e.g. "/clear". Returns "" if the text is not a slash-command envelope.
func ExtractSlashCommand(s string) string {
	m := commandEnvelope.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	cmd := strings.TrimSpace(m[1])
	if cmd == "" {
		return ""
	}
	if !strings.HasPrefix(cmd, "/") {
		cmd = "/" + cmd
	}
	return cmd
}

// IsLocalCommandCaveat reports whether the text is locally-run command output that
// Claude Code frames with <local-command-caveat>. Such records are not human input.
func IsLocalCommandCaveat(s string) bool {
	return strings.Contains(s, "<local-command-caveat>")
}

// IsCommandLike reports whether a user "prompt" is really a pasted shell command
// with no natural-language intent, and should be excluded from prompt scoring.
//
// Conservative by design: a prompt that merely *contains* a command but also has
// prose (e.g. "curl ... && code --install-extension ... - this is the extension
// that shows ads") is a REAL prompt and must return false. We only flag input that
// is essentially just a command.
func IsCommandLike(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	// Explicit bash-bang invocation.
	if strings.HasPrefix(t, "!") {
		return true
	}
	lines := strings.Split(t, "\n")
	if len(lines) > 4 {
		return false // multi-paragraph input is prose, not a one-off command
	}
	first := strings.TrimSpace(lines[0])
	if !shellStart.MatchString(first) {
		return false
	}
	// First line looks like a command. It's only "command-like" if there's little
	// prose: count natural-language-ish words (alphabetic tokens that aren't flags,
	// paths, or command tokens). A low prose count => it's just a command.
	if proseWordCount(t) <= 3 {
		return true
	}
	return false
}

// proseWordCount counts tokens that look like natural-language words (>=2 letters,
// not a flag, not a path/URL, not containing typical command punctuation).
func proseWordCount(s string) int {
	n := 0
	for _, tok := range strings.Fields(s) {
		if strings.HasPrefix(tok, "-") || strings.ContainsAny(tok, "/\\.@:=$|&`") {
			continue
		}
		letters := 0
		for _, r := range tok {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= 2 {
			n++
		}
	}
	return n
}

// EndsWithQuestion reports whether the (trimmed) text ends in a question mark —
// Latin '?' or Devanagari danda variants are not used for questions, so '?' plus
// the Arabic/fullwidth question marks cover the multilingual cases we care about.
func EndsWithQuestion(s string) bool {
	t := strings.TrimRightFunc(strings.TrimSpace(s), func(r rune) bool {
		return unicode.IsSpace(r) || r == '"' || r == '\'' || r == ')' || r == ']'
	})
	if t == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(t)
	return r == '?' || r == '？' || r == '؟'
}

// --- Structural feature detectors (language-independent) ---

// HasFilePath reports whether the text references a file path or @-mention.
func HasFilePath(s string) bool { return filePath.MatchString(s) }

// HasCode reports whether the text contains a code fence or inline code span.
func HasCode(s string) bool { return codeFence.MatchString(s) || inlineCode.MatchString(s) }

// HasURL reports whether the text contains an http(s) URL.
func HasURL(s string) bool { return url.MatchString(s) }

// HasError reports whether the text includes an error/stack-trace marker.
func HasError(s string) bool { return errorPattern.MatchString(s) }

// RuneLen returns the length in runes (never bytes), so multibyte scripts such as
// Devanagari are measured fairly.
func RuneLen(s string) int { return utf8.RuneCountInString(s) }

// DominantScript classifies the dominant Unicode script of the text as "latin",
// "devanagari", or "other". Used to normalize length/specificity thresholds so a
// Hindi prompt is not penalized for tokenizing differently than English.
func DominantScript(s string) string {
	var latin, deva, other int
	for _, r := range s {
		switch {
		case r >= 0x0900 && r <= 0x097F:
			deva++
		case unicode.In(r, unicode.Latin):
			latin++
		case unicode.IsLetter(r):
			other++
		}
	}
	switch {
	case deva == 0 && latin == 0 && other == 0:
		return "latin" // no letters; treat neutrally
	case deva >= latin && deva >= other:
		return "devanagari"
	case latin >= other:
		return "latin"
	default:
		return "other"
	}
}
