package report

import (
	"encoding/json"
	"io"

	"github.com/siddham-jain/knowthyself/internal/profile"
)

// JSON renders the Profile as indented JSON to a writer. It's the machine-readable
// surface (`knowthyself --json`) and the contract future consumers (web, recruiter
// portal) parse — hence stable field names and profile.SchemaVersion.
type JSON struct {
	W io.Writer
}

func (j JSON) Render(p profile.Profile) error {
	enc := json.NewEncoder(j.W)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}
