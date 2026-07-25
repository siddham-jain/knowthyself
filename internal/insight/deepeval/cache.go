package deepeval

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Reads are cached by sample fingerprint, so re-running with no new sessions is free
// and instant, and sends nothing. Consent is not cached: any run that would put new
// prompts on the wire asks first.

func cacheDir(dir string) string { return filepath.Join(dir, "deep-eval") }

func cacheFile(dir, fingerprint string) string {
	return filepath.Join(cacheDir(dir), fingerprint+".json")
}

// LoadCached returns a previous read for this exact sample and rubric, if any.
func LoadCached(dir, fingerprint string) *profile.DeepRead {
	b, err := os.ReadFile(cacheFile(dir, fingerprint))
	if err != nil {
		return nil
	}
	var dr profile.DeepRead
	if err := json.Unmarshal(b, &dr); err != nil || dr.RubricVer != RubricVersion {
		return nil
	}
	return &dr
}

// Save stores a read. A cache write failure is not worth failing the run over.
func Save(dir, fingerprint string, dr *profile.DeepRead) {
	b, err := json.MarshalIndent(dr, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(cacheDir(dir), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(cacheFile(dir, fingerprint), b, 0o600)
}
