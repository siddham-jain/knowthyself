package deepeval

import (
	"regexp"
	"strings"
	"testing"
)

// Anything that looks like a credential must not survive redaction. These are the
// cases that matter most: a miss here leaks a secret off the machine.
func TestRedactRemovesSecrets(t *testing.T) {
	cases := []struct{ name, in, leak string }{
		{"openai key", "use sk-abcdefghijklmnopqrstuvwxyz012345 please", "sk-abcdefghijklmnopqrstuvwxyz012345"},
		{"github pat", "token ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"},
		{"aws key", "AKIAIOSFODNN7EXAMPLE is the id", "AKIAIOSFODNN7EXAMPLE"},
		{"slack token", "xoxb-123456789012-abcdefghijkl", "xoxb-123456789012-abcdefghijkl"},
		{"google key", "AIzaSyA1234567890123456789012345678901 here", "AIzaSyA1234567890123456789012345678901"},
		{"jwt", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N", "eyJhbGciOiJIUzI1NiJ9"},
		{"password assign", `password = "hunter2placeholder"`, "hunter2placeholder"},
		{"api_key assign", `api_key: abcd1234efgh5678`, "abcd1234efgh5678"},
		{"conn string", "postgres://admin:s3cretpw@db.internal/app", "s3cretpw"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----", "MIIEowIBAAKCAQEA"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Redact(c.in)
			if strings.Contains(got, c.leak) {
				t.Errorf("secret survived redaction\n in: %s\nout: %s\nleak: %s", c.in, got, c.leak)
			}
		})
	}
}

func TestRedactPathsKeepBasename(t *testing.T) {
	got := Redact("please fix /Users/someone/Projects/secret-client/internal/tui/view.go now")
	if strings.Contains(got, "someone") || strings.Contains(got, "secret-client") {
		t.Errorf("path not collapsed: %s", got)
	}
	// The basename is the signal the rubric reads — it must survive.
	if !strings.Contains(got, "view.go") {
		t.Errorf("basename lost, judge can no longer tell the prompt located the work: %s", got)
	}
}

func TestRedactWindowsPath(t *testing.T) {
	got := Redact(`open C:\Users\someone\src\app\main.go`)
	if strings.Contains(got, "someone") {
		t.Errorf("windows path not collapsed: %s", got)
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("windows basename lost: %s", got)
	}
}

func TestRedactEmailURLAndIP(t *testing.T) {
	got := Redact("mail bob.smith@corp.example.com or hit https://api.internal.corp/v1/users?id=9 at 10.1.2.3")
	for _, leak := range []string{"bob.smith@corp.example.com", "/v1/users", "10.1.2.3"} {
		if strings.Contains(got, leak) {
			t.Errorf("%q survived redaction: %s", leak, got)
		}
	}
	// The host is retained so the judge can still see that an API was referenced.
	if !strings.Contains(got, "api.internal.corp") {
		t.Errorf("url host lost: %s", got)
	}
}

// Ordinary prose must come through essentially untouched, or the rubric has nothing
// to judge.
func TestRedactLeavesProseAlone(t *testing.T) {
	in := "The login button does nothing on mobile. Expected it to submit the form and redirect to the dashboard."
	if got := Redact(in); got != in {
		t.Errorf("prose was altered\n in: %s\nout: %s", in, got)
	}
}

// Every quote a judge returns must be findable in the redacted text, so redaction
// must be deterministic.
func TestRedactIsDeterministic(t *testing.T) {
	in := "fix /a/b/c.go and email x@y.com with sk-abcdefghijklmnopqrstuvwxyz012345"
	if Redact(in) != Redact(in) {
		t.Error("redaction is not deterministic")
	}
}

// A redacted prompt must never still match the obvious secret shapes.
func TestRedactOutputHasNoKeyShapes(t *testing.T) {
	shapes := regexp.MustCompile(`sk-[A-Za-z0-9]{16,}|ghp_[A-Za-z0-9]{16,}|AKIA[0-9A-Z]{16}`)
	in := "sk-aaaaaaaaaaaaaaaaaaaaaaaaaaaa ghp_bbbbbbbbbbbbbbbbbbbbbbbb AKIAIOSFODNN7EXAMPLE"
	if shapes.MatchString(Redact(in)) {
		t.Errorf("key shape survived: %s", Redact(in))
	}
}
