package licensing

import "errors"

// Sentinel errors returned by verification and decryption. Use errors.Is to
// test for them.
var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrExpired          = errors.New("license expired")
	ErrNotYetValid      = errors.New("license not yet valid")
	ErrClockBackwards   = errors.New("system clock moved backwards")
	ErrMachineMismatch  = errors.New("machine fingerprint mismatch")
	ErrUnknownVersion   = errors.New("unknown license version")
	ErrMalformed        = errors.New("malformed license data")
	ErrDecryptionFailed = errors.New("decryption failed")
)
