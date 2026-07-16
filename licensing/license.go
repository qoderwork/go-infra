package licensing

import (
	"encoding/json"
	"fmt"
	"time"
)

// License is the signed authorization document.
//
// Fields use time.Time so they round-trip cleanly through JSON (RFC3339)
// and can be formatted for display. Zero time values are omitted on the wire.
type License struct {
	Version   int              `json:"version"`
	ID        string           `json:"id,omitempty"`
	Product   string           `json:"product,omitempty"`
	Subject   string           `json:"subject,omitempty"`
	Issuer    string           `json:"issuer,omitempty"`
	Features  []string         `json:"features,omitempty"`
	Capacity  map[string]int64 `json:"capacity,omitempty"`
	NotBefore time.Time        `json:"not_before,omitempty"`
	Expiry    time.Time        `json:"expiry,omitempty"`
	Machine   *MachineBinding  `json:"machine,omitempty"`
	IssuedAt  time.Time        `json:"issued_at,omitempty"`
}

// MachineBinding optionally pins a license to one or more machines.
type MachineBinding struct {
	// Fingerprint is the value produced by the machine package (a sha256 hex
	// of a stable hardware id). It is compared, case-sensitively, against the
	// fingerprint the verifying application computes at runtime.
	Fingerprint string `json:"fp"`
	// Loose controls behavior when the fingerprint source is missing or fails:
	//   - false (strict, default): verification fails if the fingerprint
	//     source is nil, returns an error, or no candidate matches.
	//   - true (loose): verification passes when the fingerprint source is
	//     nil or returns an error (best-effort), but still fails when a
	//     fingerprint is successfully read and matches no candidate.
	Loose   bool     `json:"loose,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
}

// HasFeature reports whether the license grants a named feature.
func (l *License) HasFeature(name string) bool {
	for _, f := range l.Features {
		if f == name {
			return true
		}
	}
	return false
}

// CapacityOf returns the granted capacity for a named metric, or 0.
func (l *License) CapacityOf(name string) int64 {
	if l.Capacity == nil {
		return 0
	}
	return l.Capacity[name]
}

// CanonicalBytes returns a deterministic JSON encoding used for signing.
//
// Go's encoding/json emits struct fields in declaration order and sorts map
// keys lexicographically, so the output is stable across platforms and Go
// versions. The same bytes are stored inside the envelope, so verification
// re-checks the exact signed payload.
func (l *License) CanonicalBytes() ([]byte, error) {
	b, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("license: canonical marshal: %w", err)
	}
	return b, nil
}
