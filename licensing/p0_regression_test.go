package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

// TestCheckMachineRejectsEmptyFingerprint: constant-time comparison of two
// empty strings returns 1, which used to silently bypass strict binding.
func TestCheckMachineRejectsEmptyFingerprint(t *testing.T) {
	v := &Verifier{fpFunc: func() (string, error) { return "", nil }}
	if err := v.checkMachine(&MachineBinding{Fingerprint: ""}); err == nil {
		t.Fatal("empty runtime fingerprint must not satisfy an empty binding")
	}
	if err := v.checkMachine(&MachineBinding{Fingerprint: "some-fp"}); err == nil {
		t.Fatal("empty runtime fingerprint must not satisfy a non-empty binding")
	}
	if err := v.checkMachine(&MachineBinding{Fingerprint: "", Loose: true, Aliases: []string{""}}); err == nil {
		t.Fatal("empty loose candidates must not satisfy an empty fingerprint")
	}
}

func TestCheckMachineRejectsEmptyStrictBinding(t *testing.T) {
	v := &Verifier{fpFunc: func() (string, error) { return "fp-123", nil }}
	if err := v.checkMachine(&MachineBinding{Fingerprint: ""}); err == nil {
		t.Fatal("strict binding with an empty fingerprint must be rejected")
	}
	if err := v.checkMachine(&MachineBinding{Fingerprint: "fp-123"}); err != nil {
		t.Fatalf("valid strict binding was rejected: %v", err)
	}
}

func TestSignRejectsEmptyStrictBinding(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSigner(priv, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sign(&License{Machine: &MachineBinding{}}); err == nil {
		t.Fatal("Sign must reject a strict binding with no fingerprint")
	}
	if _, err := s.Sign(&License{Machine: &MachineBinding{Fingerprint: "fp-123"}}); err != nil {
		t.Fatalf("valid strict binding was rejected by Sign: %v", err)
	}
}
