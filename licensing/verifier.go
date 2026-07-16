package licensing

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Verifier checks signed envelopes.
//
// A Verifier is safe for concurrent use by multiple goroutines AFTER
// construction is complete. Do not call WithKey, WithFingerprint, WithClock,
// or WithMinClock concurrently with Verify — configure the Verifier fully
// before sharing it.
type Verifier struct {
	keys     map[int]ed25519.PublicKey // envelope version -> trusted public key
	fpFunc   func() (string, error)    // machine fingerprint source (optional)
	clock    func() time.Time          // time source (overridable for tests)
	minClock int64                     // persisted max-seen unix seconds (anti rollback)
}

// NewVerifier creates a verifier that trusts pub for the given version.
// The public key must be a valid Ed25519 public key (32 bytes); otherwise
// an error is returned.
func NewVerifier(pub ed25519.PublicKey, version int) (*Verifier, error) {
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("license: ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	return &Verifier{
		keys:  map[int]ed25519.PublicKey{version: pub},
		clock: time.Now,
	}, nil
}

// WithKey registers an additional public key for a specific envelope version.
// This supports key rotation: old licenses keep verifying against the key
// version they were signed with, while new licenses use a fresh key.
//
// Must be called before any concurrent use of Verify. See the Verifier struct
// doc for the full concurrency contract.
func (v *Verifier) WithKey(version int, pub ed25519.PublicKey) *Verifier {
	v.keys[version] = pub
	return v
}

// WithFingerprint binds verification to a machine by supplying a function
// that returns the current machine's fingerprint. The function is invoked
// during Verify and its result is compared against License.Machine.
//
// Must be called before any concurrent use of Verify.
func (v *Verifier) WithFingerprint(f func() (string, error)) *Verifier {
	v.fpFunc = f
	return v
}

// WithClock overrides the time source. Use it for deterministic tests or
// to inject a custom clock.
//
// Must be called before any concurrent use of Verify.
func (v *Verifier) WithClock(f func() time.Time) *Verifier {
	v.clock = f
	return v
}

// WithMinClock sets the persisted "max time seen so far" (unix seconds).
// If the current clock is earlier than this, verification fails with
// ErrClockBackwards, defending against attackers rolling the OS clock back
// to keep an expired license alive. Persist the value returned by the clock
// (e.g. in your app's data dir) between runs.
//
// Must be called before any concurrent use of Verify.
func (v *Verifier) WithMinClock(minUnix int64) *Verifier {
	v.minClock = minUnix
	return v
}

// Verify parses and verifies raw envelope bytes.
func (v *Verifier) Verify(data []byte) (*License, error) {
	env, err := ParseEnvelope(data)
	if err != nil {
		return nil, err
	}
	return v.VerifyEnvelope(env)
}

// VerifyEnvelope verifies an already-parsed envelope. It checks, in order:
//  1. the signature against the key for env.Version,
//  2. structural decoding of the license,
//  3. time validity (not-yet-valid / expired),
//  4. clock-rollback (if WithMinClock was set),
//  5. machine binding (if the license carries one).
func (v *Verifier) VerifyEnvelope(env *Envelope) (*License, error) {
	pub, ok := v.keys[env.Version]
	if !ok {
		return nil, fmt.Errorf("%w: version %d", ErrUnknownVersion, env.Version)
	}
	licBytes, err := base64.StdEncoding.DecodeString(env.License)
	if err != nil {
		return nil, fmt.Errorf("%w: bad license encoding", ErrMalformed)
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return nil, fmt.Errorf("%w: bad signature encoding", ErrMalformed)
	}
	if !ed25519.Verify(pub, licBytes, sig) {
		return nil, ErrInvalidSignature
	}

	var lic License
	if err := json.Unmarshal(licBytes, &lic); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	now := v.clock()
	if !lic.NotBefore.IsZero() && now.Before(lic.NotBefore) {
		return nil, ErrNotYetValid
	}
	if !lic.Expiry.IsZero() && now.After(lic.Expiry) {
		return nil, ErrExpired
	}
	if v.minClock != 0 && now.Unix() < v.minClock {
		return nil, fmt.Errorf("%w: clock moved back", ErrClockBackwards)
	}
	if lic.Machine != nil {
		if err := v.checkMachine(lic.Machine); err != nil {
			return nil, err
		}
	}
	return &lic, nil
}

func (v *Verifier) checkMachine(b *MachineBinding) error {
	if v.fpFunc == nil {
		return fmt.Errorf("%w: verifier has no fingerprint source", ErrMachineMismatch)
	}
	fp, err := v.fpFunc()
	if err != nil {
		return fmt.Errorf("%w: cannot read fingerprint: %v", ErrMachineMismatch, err)
	}
	if b.Loose {
		candidates := make([]string, 0, 1+len(b.Aliases))
		candidates = append(candidates, b.Fingerprint)
		candidates = append(candidates, b.Aliases...)
		for _, c := range candidates {
			if subtle.ConstantTimeCompare([]byte(c), []byte(fp)) == 1 {
				return nil
			}
		}
		return ErrMachineMismatch
	}
	// Strict mode: check Fingerprint and Aliases with constant-time comparison.
	if subtle.ConstantTimeCompare([]byte(b.Fingerprint), []byte(fp)) == 1 {
		return nil
	}
	for _, a := range b.Aliases {
		if subtle.ConstantTimeCompare([]byte(a), []byte(fp)) == 1 {
			return nil
		}
	}
	return ErrMachineMismatch
}
