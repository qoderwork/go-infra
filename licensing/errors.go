package licensing

import "errors"

// Sentinel errors returned by verification. Use errors.Is to test for them.
var (
	ErrInvalidSignature = errors.New("license: invalid signature")
	ErrExpired          = errors.New("license: license expired")
	ErrNotYetValid      = errors.New("license: license not yet valid")
	ErrClockBackwards   = errors.New("license: system clock moved backwards")
	ErrMachineMismatch  = errors.New("license: machine fingerprint mismatch")
	ErrUnknownVersion   = errors.New("license: unknown license version")
	ErrMalformed        = errors.New("license: malformed license data")
)
