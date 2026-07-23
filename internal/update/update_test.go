package update

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.1", "0.2.1", 0},
		{"v0.2.1", "0.2.1", 0},
		{"0.2.1", "0.3.0", -1},
		{"0.3.0", "0.2.1", 1},
		{"0.2", "0.2.0", 0},
		{"1.0.0", "0.99.99", 1},
		{"0.2.10", "0.2.9", 1},
		{"0.3.0-rc1", "0.3.0", 0}, // pre-release suffix is ignored, not ranked
		{"dev", "0.2.1", -1},      // an unversioned build always reports as behind
		{"0.2.1", "dev", 1},
		{"dev", "dev", 0},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestMethodForPath(t *testing.T) {
	cases := []struct {
		path string
		want Method
	}{
		{"/usr/local/lib/node_modules/knowthyself/bin/knowthyself", MethodNPM},
		{"/opt/homebrew/Cellar/knowthyself/0.2.1/bin/knowthyself", MethodHomebrew},
		{"/usr/local/bin/knowthyself", MethodDirect},
		{"/home/x/.local/bin/knowthyself", MethodDirect},
	}
	for _, c := range cases {
		if got := methodForPath(c.path); got != c.want {
			t.Errorf("methodForPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// A package-managed install must report a command; a direct one must not, since that
// is what decides whether knowthyself replaces the binary itself.
func TestMethodCommand(t *testing.T) {
	for _, m := range []Method{MethodNPM, MethodHomebrew, MethodGo} {
		if m.Command() == "" {
			t.Errorf("%q should report an upgrade command", m)
		}
	}
	if MethodDirect.Command() != "" {
		t.Error("a direct install must self-update, not print a command")
	}
}

func TestAssetName(t *testing.T) {
	got := Release{Version: "0.3.0", Tag: "v0.3.0"}.AssetName()
	if got == "" {
		t.Fatal("empty asset name")
	}
	// Must match the goreleaser name_template, which starts with the project name.
	if want := "knowthyself_0.3.0_"; len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("AssetName() = %q, want prefix %q", got, want)
	}
}
