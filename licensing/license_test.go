package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return pub, priv
}

func mustSign(t *testing.T, priv ed25519.PrivateKey, lic *License) *Envelope {
	t.Helper()
	env, err := NewSigner(priv, CurrentVersion).Sign(lic)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return env
}

func envBytes(t *testing.T, env *Envelope) []byte {
	t.Helper()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal env: %v", err)
	}
	return b
}

func TestGenerateKeyRoundtrip(t *testing.T) {
	pub, priv := newTestPair(t)
	privPEM, err := EncodePrivateKeyPEM(priv)
	if err != nil {
		t.Fatalf("EncodePrivateKeyPEM: %v", err)
	}
	pubPEM, err := EncodePublicKeyPEM(pub)
	if err != nil {
		t.Fatalf("EncodePublicKeyPEM: %v", err)
	}
	dpriv, err := DecodePrivateKeyPEM(privPEM)
	if err != nil {
		t.Fatalf("DecodePrivateKeyPEM: %v", err)
	}
	dpub, err := DecodePublicKeyPEM(pubPEM)
	if err != nil {
		t.Fatalf("DecodePublicKeyPEM: %v", err)
	}
	if string(dpriv) != string(priv) {
		t.Fatal("private key roundtrip mismatch")
	}
	if string(dpub) != string(pub) {
		t.Fatal("public key roundtrip mismatch")
	}
}

func TestSignAndVerify(t *testing.T) {
	pub, priv := newTestPair(t)
	now := time.Now()
	lic := &License{
		Product:   "acme",
		Subject:   "alice",
		Features:  []string{"pro", "support"},
		Capacity:  map[string]int64{"seats": 5},
		NotBefore: now.Add(-time.Hour),
		Expiry:    now.Add(24 * time.Hour),
		IssuedAt:  now,
	}
	env := mustSign(t, priv, lic)
	got, err := NewVerifier(pub, CurrentVersion).Verify(envBytes(t, env))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !got.HasFeature("pro") || got.CapacityOf("seats") != 5 {
		t.Fatal("decoded license missing fields")
	}
}

func TestTamperDetection(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(envBytes(t, env), &m); err != nil {
		t.Fatal(err)
	}
	// Replace the signed payload (base64 of canonical license JSON) with a
	// tampered one. Verification decodes and checks the signature, so this
	// must fail with ErrInvalidSignature.
	m["license"] = json.RawMessage(`"` + base64.StdEncoding.EncodeToString([]byte(`{"product":"evil"}`)) + `"`)
	tampered, _ := json.Marshal(m)
	if _, err := NewVerifier(pub, CurrentVersion).Verify(tampered); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("want ErrInvalidSignature got %v", err)
	}
}

func TestExpired(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(-time.Hour)}
	env := mustSign(t, priv, lic)
	if _, err := NewVerifier(pub, CurrentVersion).Verify(envBytes(t, env)); !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired got %v", err)
	}
}

func TestNotYetValid(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", NotBefore: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	if _, err := NewVerifier(pub, CurrentVersion).Verify(envBytes(t, env)); !errors.Is(err, ErrNotYetValid) {
		t.Fatalf("want ErrNotYetValid got %v", err)
	}
}

func TestMachineBindingStrict(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Machine: &MachineBinding{Fingerprint: "fp-good"}}
	env := mustSign(t, priv, lic)
	fpGood := func() (string, error) { return "fp-good", nil }
	fpBad := func() (string, error) { return "fp-bad", nil }
	if _, err := NewVerifier(pub, CurrentVersion).WithFingerprint(fpGood).Verify(envBytes(t, env)); err != nil {
		t.Fatalf("strict match should pass: %v", err)
	}
	if _, err := NewVerifier(pub, CurrentVersion).WithFingerprint(fpBad).Verify(envBytes(t, env)); !errors.Is(err, ErrMachineMismatch) {
		t.Fatalf("want ErrMachineMismatch got %v", err)
	}
}

func TestMachineBindingLoose(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Machine: &MachineBinding{Fingerprint: "fp-main", Loose: true, Aliases: []string{"fp-laptop"}}}
	env := mustSign(t, priv, lic)
	fpLaptop := func() (string, error) { return "fp-laptop", nil }
	fpUnknown := func() (string, error) { return "fp-unknown", nil }
	if _, err := NewVerifier(pub, CurrentVersion).WithFingerprint(fpLaptop).Verify(envBytes(t, env)); err != nil {
		t.Fatalf("loose alias should pass: %v", err)
	}
	if _, err := NewVerifier(pub, CurrentVersion).WithFingerprint(fpUnknown).Verify(envBytes(t, env)); !errors.Is(err, ErrMachineMismatch) {
		t.Fatalf("want ErrMachineMismatch got %v", err)
	}
}

func TestMachineBindingUnbound(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme"}
	env := mustSign(t, priv, lic)
	if _, err := NewVerifier(pub, CurrentVersion).
		WithFingerprint(func() (string, error) { return "whatever", nil }).
		Verify(envBytes(t, env)); err != nil {
		t.Fatalf("unbound license should verify: %v", err)
	}
}

func TestUnknownVersion(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme"}
	env := mustSign(t, priv, lic) // version 1
	v := NewVerifier(pub, 2)      // verifier only trusts version 2
	if _, err := v.Verify(envBytes(t, env)); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("want ErrUnknownVersion got %v", err)
	}
}

func TestClockBackwards(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	now := time.Now()
	fixed := now.Add(-time.Hour)
	v := NewVerifier(pub, CurrentVersion).
		WithClock(func() time.Time { return fixed }).
		WithMinClock(now.Unix())
	if _, err := v.Verify(envBytes(t, env)); !errors.Is(err, ErrClockBackwards) {
		t.Fatalf("want ErrClockBackwards got %v", err)
	}
}

func TestCanonicalStable(t *testing.T) {
	lic := &License{
		Product:  "acme",
		Features: []string{"b", "a"},
		Capacity: map[string]int64{"z": 1, "a": 2},
	}
	b1, _ := lic.CanonicalBytes()
	b2, _ := lic.CanonicalBytes()
	if string(b1) != string(b2) {
		t.Fatal("canonical bytes not stable")
	}
	s := string(b1)
	if !contains(s, `"a":2`) || !contains(s, `"z":1`) {
		t.Fatalf("unexpected canonical form: %s", s)
	}
	if indexOf(s, `"a":2`) > indexOf(s, `"z":1`) {
		t.Fatalf("map keys not sorted: %s", s)
	}
}

func TestConcurrentVerify(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	data := envBytes(t, env)
	v := NewVerifier(pub, CurrentVersion)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.Verify(data); err != nil {
				t.Errorf("concurrent verify failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestEnvelopeSaveLoad(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	path := filepath.Join(t.TempDir(), "lic.json")
	if err := SaveEnvelope(path, env); err != nil {
		t.Fatalf("SaveEnvelope: %v", err)
	}
	loaded, err := LoadEnvelope(path)
	if err != nil {
		t.Fatalf("LoadEnvelope: %v", err)
	}
	if _, err := NewVerifier(pub, CurrentVersion).VerifyEnvelope(loaded); err != nil {
		t.Fatalf("verify loaded envelope: %v", err)
	}
}

func contains(s, sub string) bool { return indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
