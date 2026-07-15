package text

import "testing"

func TestIsCommandLike(t *testing.T) {
	cmd := []string{"git status", "npm run build", "!ls -la", "cd ../foo && make"}
	notCmd := []string{
		"fix the bug in main.go",
		"curl -L https://x.ai/vsix -o k.vsix && install it - this extension shows me ads and shares revenue with me",
		"why does the build fail when I run npm install",
		"",
	}
	for _, s := range cmd {
		if !IsCommandLike(s) {
			t.Errorf("expected command-like: %q", s)
		}
	}
	for _, s := range notCmd {
		if IsCommandLike(s) {
			t.Errorf("expected NOT command-like: %q", s)
		}
	}
}

func TestExtractSlashCommand(t *testing.T) {
	if got := ExtractSlashCommand("<command-name>/clear</command-name>"); got != "/clear" {
		t.Errorf("got %q", got)
	}
	if got := ExtractSlashCommand("<command-name>model</command-name>"); got != "/model" {
		t.Errorf("leading slash not added: %q", got)
	}
	if got := ExtractSlashCommand("just a normal prompt"); got != "" {
		t.Errorf("false positive: %q", got)
	}
}

func TestDominantScript(t *testing.T) {
	cases := map[string]string{
		"fix the bug": "latin",
		"मुख्य फ़ाइल में बग है": "devanagari",
		"main.py mein bug hai": "latin",
	}
	for in, want := range cases {
		if got := DominantScript(in); got != want {
			t.Errorf("DominantScript(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLengthBandScriptAware(t *testing.T) {
	if b := LengthBand("fix"); b != "vague" {
		t.Errorf("short latin should be vague, got %q", b)
	}
	// A compact Hindi phrase carries real intent; it must not be judged "vague"
	// against Latin-tuned thresholds.
	if b := LengthBand("बग ठीक करो फाइल में"); b == "vague" {
		t.Errorf("compact Hindi wrongly judged vague")
	}
}

func TestStructuralDetectors(t *testing.T) {
	if !HasFilePath("edit internal/store/sqlite.go now") {
		t.Error("path not detected")
	}
	if !HasFilePath("look at @config.yaml") {
		t.Error("@mention not detected")
	}
	if !HasError("got a TypeError: cannot read") {
		t.Error("error not detected")
	}
	if !HasError("panic at main.go:42") {
		t.Error("stack frame not detected")
	}
	if !HasCode("run `go test`") {
		t.Error("inline code not detected")
	}
	if !HasURL("see https://example.com/x") {
		t.Error("url not detected")
	}
}

func TestIsCorrectionMultilingual(t *testing.T) {
	pos := []string{"no, that's wrong", "actually use a map instead", "nahi ye galat hai", "फिर से करो", "still failing"}
	neg := []string{"add a test for this", "looks good, continue", "implement the parser"}
	for _, s := range pos {
		if !IsCorrection(s) {
			t.Errorf("expected correction: %q", s)
		}
	}
	for _, s := range neg {
		if IsCorrection(s) {
			t.Errorf("expected NOT correction: %q", s)
		}
	}
}

func TestEndsWithQuestion(t *testing.T) {
	if !EndsWithQuestion("which file?") {
		t.Error("latin ? not detected")
	}
	if !EndsWithQuestion("done. ready?  ") {
		t.Error("trailing space ? not detected")
	}
	if EndsWithQuestion("do it now.") {
		t.Error("false positive")
	}
}
