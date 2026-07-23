package deepeval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/siddham-jain/knowthyself/internal/profile"
)

// Reads are cached by sample fingerprint, so re-running with no new sessions is free
// and instant. Consent is recorded per (host, model): pointing knowthyself at a
// different endpoint or model asks again.

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

// consentKey identifies an endpoint+model pair, without recording either in the
// clear on disk.
func consentKey(cfg Config) string {
	sum := sha256.Sum256([]byte(cfg.Host() + "\x00" + cfg.Model))
	return hex.EncodeToString(sum[:8])
}

func consentFile(dir string, cfg Config) string {
	return filepath.Join(cacheDir(dir), "consent-"+consentKey(cfg))
}

// HasConsent reports whether this endpoint and model were already approved.
func HasConsent(dir string, cfg Config) bool {
	_, err := os.Stat(consentFile(dir, cfg))
	return err == nil
}

// RecordConsent marks this endpoint and model as approved.
func RecordConsent(dir string, cfg Config) error {
	if err := os.MkdirAll(cacheDir(dir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(consentFile(dir, cfg), []byte(cfg.Host()+" "+cfg.Model+"\n"), 0o600)
}
