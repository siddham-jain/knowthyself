// Package update checks GitHub Releases for a newer knowthyself and, when the
// running binary was installed by plain download, replaces it in place. Binaries
// owned by a package manager are never touched — the manager's own command is
// reported instead, so the two can't fight over the same file.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repo    = "siddham-jain/knowthyself"
	binName = "knowthyself"
	timeout = 15 * time.Second
)

// Method is how the running binary was installed.
type Method string

const (
	MethodDirect   Method = "direct"
	MethodNPM      Method = "npm"
	MethodHomebrew Method = "homebrew"
	MethodGo       Method = "go"
)

// Command returns the upgrade command a user must run themselves, or "" when
// knowthyself can do the upgrade itself.
func (m Method) Command() string {
	switch m {
	case MethodNPM:
		return "npm install -g " + binName + "@latest"
	case MethodHomebrew:
		return "brew upgrade " + binName
	case MethodGo:
		return "go install github.com/" + repo + "/cmd/" + binName + "@latest"
	default:
		return ""
	}
}

// Detect infers the install method from the path of the running binary.
func Detect() Method {
	exe, err := os.Executable()
	if err != nil {
		return MethodDirect
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return methodForPath(exe)
}

func methodForPath(exe string) Method {
	p := filepath.ToSlash(exe)
	switch {
	case strings.Contains(p, "/node_modules/"):
		return MethodNPM
	case strings.Contains(p, "/Cellar/"), strings.Contains(p, "/homebrew/"):
		return MethodHomebrew
	case isUnder(p, goBinDirs()):
		return MethodGo
	default:
		return MethodDirect
	}
}

func isUnder(path string, dirs []string) bool {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if strings.HasPrefix(path, filepath.ToSlash(d)+"/") {
			return true
		}
	}
	return false
}

// goBinDirs returns the directories `go install` would write to. `go env` is
// consulted only if the toolchain is present; its absence is not an error.
func goBinDirs() []string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "go", "bin"))
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return dirs
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, goBin, "env", "GOBIN", "GOPATH").Output()
	if err != nil {
		return dirs
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		dirs = append(dirs, lines[0])
	}
	if len(lines) > 1 && lines[1] != "" {
		dirs = append(dirs, filepath.Join(lines[1], "bin"))
	}
	return dirs
}

// Release is the subset of a GitHub release this package needs.
type Release struct {
	Version string // without the leading "v"
	Tag     string
}

// AssetName is the release archive for the running platform, matching the
// name_template in .goreleaser.yaml.
func (r Release) AssetName() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", binName, r.Version, runtime.GOOS, runtime.GOARCH, ext)
}

func (r Release) assetURL() string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, r.Tag, r.AssetName())
}

func (r Release) checksumURL() string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/checksums.txt", repo, r.Tag)
}

// Latest fetches the most recent published release.
func Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("could not reach github.com — check your connection: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return Release{}, fmt.Errorf("no published release found at github.com/%s", repo)
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return Release{}, fmt.Errorf("github rate limit reached — try again later, or download from https://github.com/%s/releases", repo)
	case resp.StatusCode != http.StatusOK:
		return Release{}, fmt.Errorf("github returned %s while checking for updates", resp.Status)
	}

	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("could not read the release list from github: %w", err)
	}
	if body.TagName == "" {
		return Release{}, fmt.Errorf("github returned a release with no tag")
	}
	return Release{Tag: body.TagName, Version: strings.TrimPrefix(body.TagName, "v")}, nil
}

// Compare orders two dotted numeric versions: -1 if a < b, 0 if equal, 1 if a > b.
// Non-numeric components (a "dev" build, a "-rc1" suffix) sort below any release, so
// an unversioned local build always reports as behind.
func Compare(a, b string) int {
	as, aok := versionParts(a)
	bs, bok := versionParts(b)
	switch {
	case !aok && !bok:
		return 0
	case !aok:
		return -1
	case !bok:
		return 1
	}
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return nil, false
	}
	fields := strings.Split(v, ".")
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
