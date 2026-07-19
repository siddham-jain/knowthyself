// Package report defines the Reporter seam: how a computed Profile is presented.
// The TUI dashboard and the JSON emitter are Reporters; future surfaces (a web
// dashboard, a portable recruiter export) are just additional Reporters consuming
// the same profile.Profile.
package report

import "github.com/siddham-jain/knowthyself/internal/profile"

// Reporter renders a Profile to some surface (terminal, stdout JSON, ...).
type Reporter interface {
	// Render presents the profile. It must not mutate p.
	Render(p profile.Profile) error
}
