package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// Verifier checks signed envelopes.
type Verifier struct {
	keys     map[int]ed25519.PublicKey // envelope version -> trusted public key
	fpFunc   func() (string, error)    // machine fingerprint source (optional)
	clock    func() time.Time           // time source (overridable for tests)
	minClock int64                     // persisted max-seen unix seconds (anti rollback)
}

// NewVerifier creates a verifier that trusts pub for the given version.
func NewVerifier(pub ed25519.PublicKey, version int) *Verifier {
	return &Verifier{
		keys:  map[int]ed25519.PublicKey{version: pub},
		clock: time.Now,
	}
}

// WithKey registers an additional public key for a specific envelope version.
// This supports key rotation: old licenses keep verifying against the key
// version they were signed with, while new licenses use a fresh key.
func (v *Verifier) WithKey(version int, pub ed25519.PublicKey) *Verifier {
	v.keys[version] = pub
	return v
}

// WithFingerprint binds verification to a machine by supplying a function
// that returns the current machine's fingerprint. The function is invoked
// during Verify and its result is compared against License.Machine.
func (v *Verifier) WithFingerprint(f func() (string, error)) *Verifier {
	v.fpFunc = f
	return v
}

// WithClock overrides the time source. Use it for deterministic tests or
// to inject a custom clock.
func (v *Verifier) WithClock(f func() time.Time) *Verifier {
	v.clock = f
	return v
}

// WithMinClock sets the persisted "max time seen so far" (unix seconds).
// If the current clock is earlier than this, verification fails with
// ErrClockBackwards, defending against attackers rolling the OS clock back
// to keep an expired license alive. Persist the value returned by the clock
// (e.g. in your app's data dir) between runs.
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
		return nil, fmt.Errorf("%w: bad license encoding", ErrInvalidSignature)
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return nil, fmt.Errorf("%w: bad signature encoding", ErrInvalidSignature)
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
		for _, c := range append([]string{b.Fingerprint}, b.Aliases...) {
			if c == fp {
				return nil
			}
		}
		return ErrMachineMismatch
	}
	if b.Fingerprint != fp {
		return ErrMachineMismatch
	}
	return nil
}
