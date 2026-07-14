package licensing

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

// CurrentVersion is the envelope/signature scheme version. Bump this and
// register a new public key via Verifier.WithKey when you rotate keys.
const CurrentVersion = 1

// Envelope wraps a signed license.
//
//	{
//	  "version":  1,
//	  "alg":      "ed25519",
//	  "license":  "<base64 of canonical license JSON>",
//	  "signature": "<base64 of ed25519 over the canonical license bytes>"
//	}
//
// The license payload is stored as base64 (not embedded JSON) on purpose: it
// makes the signed bytes immune to JSON pretty-printing / re-indentation, so a
// license file stays verifiable no matter how it is formatted on disk. Use
// PrettyLicense to get a human-readable view.
type Envelope struct {
	Version   int    `json:"version"`
	Alg       string `json:"alg"`
	License   string `json:"license"`
	Signature string `json:"signature"`
}

// ParseEnvelope decodes and structurally validates an envelope without
// verifying the signature.
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if env.Version == 0 {
		return nil, fmt.Errorf("%w: missing version", ErrMalformed)
	}
	if env.Alg != "ed25519" {
		return nil, fmt.Errorf("%w: unsupported alg %q", ErrMalformed, env.Alg)
	}
	if env.License == "" {
		return nil, fmt.Errorf("%w: empty license", ErrMalformed)
	}
	if _, err := base64.StdEncoding.DecodeString(env.License); err != nil {
		return nil, fmt.Errorf("%w: bad license encoding", ErrMalformed)
	}
	if _, err := base64.StdEncoding.DecodeString(env.Signature); err != nil {
		return nil, fmt.Errorf("%w: bad signature encoding", ErrMalformed)
	}
	return &env, nil
}

// LoadEnvelope reads an envelope from a file.
func LoadEnvelope(path string) (*Envelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseEnvelope(data)
}

// SaveEnvelope writes an envelope as pretty-printed JSON.
func SaveEnvelope(path string, env *Envelope) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// PrettyLicense decodes the canonical (base64) license payload and returns it
// as indented JSON for human-friendly display. It does not affect verification,
// which always works off the raw canonical bytes.
func (e *Envelope) PrettyLicense() (string, error) {
	b, err := base64.StdEncoding.DecodeString(e.License)
	if err != nil {
		return "", fmt.Errorf("%w: bad license encoding", ErrMalformed)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return "", err
	}
	return out.String(), nil
}
