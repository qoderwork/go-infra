package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/licensing/machine"
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
	signer, err := NewSigner(priv, CurrentVersion)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	env, err := signer.Sign(lic)
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
	v, err := NewVerifier(pub, CurrentVersion)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	got, err := v.Verify(envBytes(t, env))
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
	v, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v.Verify(tampered); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("want ErrInvalidSignature got %v", err)
	}
}

func TestExpired(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(-time.Hour)}
	env := mustSign(t, priv, lic)
	v, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v.Verify(envBytes(t, env)); !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired got %v", err)
	}
}

func TestNotYetValid(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", NotBefore: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	v, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v.Verify(envBytes(t, env)); !errors.Is(err, ErrNotYetValid) {
		t.Fatalf("want ErrNotYetValid got %v", err)
	}
}

func TestMachineBindingStrict(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Machine: &MachineBinding{Fingerprint: "fp-good"}}
	env := mustSign(t, priv, lic)
	fpGood := func() (string, error) { return "fp-good", nil }
	fpBad := func() (string, error) { return "fp-bad", nil }
	v1, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v1.WithFingerprint(fpGood).Verify(envBytes(t, env)); err != nil {
		t.Fatalf("strict match should pass: %v", err)
	}
	v2, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v2.WithFingerprint(fpBad).Verify(envBytes(t, env)); !errors.Is(err, ErrMachineMismatch) {
		t.Fatalf("want ErrMachineMismatch got %v", err)
	}
}

func TestMachineBindingLoose(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Machine: &MachineBinding{Fingerprint: "fp-main", Loose: true, Aliases: []string{"fp-laptop"}}}
	env := mustSign(t, priv, lic)
	fpLaptop := func() (string, error) { return "fp-laptop", nil }
	fpUnknown := func() (string, error) { return "fp-unknown", nil }
	v1, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v1.WithFingerprint(fpLaptop).Verify(envBytes(t, env)); err != nil {
		t.Fatalf("loose alias should pass: %v", err)
	}
	v2, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v2.WithFingerprint(fpUnknown).Verify(envBytes(t, env)); !errors.Is(err, ErrMachineMismatch) {
		t.Fatalf("want ErrMachineMismatch got %v", err)
	}
}

func TestMachineBindingUnbound(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme"}
	env := mustSign(t, priv, lic)
	v, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v.
		WithFingerprint(func() (string, error) { return "whatever", nil }).
		Verify(envBytes(t, env)); err != nil {
		t.Fatalf("unbound license should verify: %v", err)
	}
}

func TestUnknownVersion(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme"}
	env := mustSign(t, priv, lic) // version 1
	v, _ := NewVerifier(pub, 2)  // verifier only trusts version 2
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
	v, _ := NewVerifier(pub, CurrentVersion)
	v.WithClock(func() time.Time { return fixed }).
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
	if !strings.Contains(s, `"a":2`) || !strings.Contains(s, `"z":1`) {
		t.Fatalf("unexpected canonical form: %s", s)
	}
	if strings.Index(s, `"a":2`) > strings.Index(s, `"z":1`) {
		t.Fatalf("map keys not sorted: %s", s)
	}
}

func TestConcurrentVerify(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	env := mustSign(t, priv, lic)
	data := envBytes(t, env)
	v, _ := NewVerifier(pub, CurrentVersion)
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
	v, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v.VerifyEnvelope(loaded); err != nil {
		t.Fatalf("verify loaded envelope: %v", err)
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	pub, priv := newTestPair(t)
	aesKey, err := GenerateAESKey()
	if err != nil {
		t.Fatalf("GenerateAESKey: %v", err)
	}

	lic := &License{
		Product:  "acme",
		Subject:  "alice",
		Features: []string{"pro", "encrypted"},
		Capacity: map[string]int64{"seats": 10},
		Expiry:   time.Now().Add(24 * time.Hour),
	}

	// Sign then encrypt.
	signer, err := NewSigner(priv, CurrentVersion)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	env, err := signer.SignEncrypted(lic, aesKey)
	if err != nil {
		t.Fatalf("SignEncrypted: %v", err)
	}

	// Serialize and parse back.
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Decrypt then verify.
	v, _ := NewVerifier(pub, CurrentVersion)
	got, err := v.VerifyEncrypted(data, aesKey)
	if err != nil {
		t.Fatalf("VerifyEncrypted: %v", err)
	}
	if got.Subject != "alice" || !got.HasFeature("encrypted") || got.CapacityOf("seats") != 10 {
		t.Fatalf("decrypted license fields mismatch: %+v", got)
	}
}

func TestEncryptedTamperDetection(t *testing.T) {
	pub, priv := newTestPair(t)
	aesKey, _ := GenerateAESKey()

	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	signer, _ := NewSigner(priv, CurrentVersion)
	env, err := signer.SignEncrypted(lic, aesKey)
	if err != nil {
		t.Fatalf("SignEncrypted: %v", err)
	}

	// Tamper with the ciphertext.
	env.Ciphertext = base64.StdEncoding.EncodeToString([]byte("tampered data here!"))
	data, _ := json.Marshal(env)

	wrongKey, _ := GenerateAESKey()
	v, _ := NewVerifier(pub, CurrentVersion)
	_, err = v.VerifyEncrypted(data, wrongKey)
	if err == nil {
		t.Fatal("expected error for tampered encrypted envelope")
	}
}

func TestEncryptedWrongKey(t *testing.T) {
	pub, priv := newTestPair(t)
	aesKey, _ := GenerateAESKey()
	wrongKey, _ := GenerateAESKey()

	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	signer, _ := NewSigner(priv, CurrentVersion)
	env, err := signer.SignEncrypted(lic, aesKey)
	if err != nil {
		t.Fatalf("SignEncrypted: %v", err)
	}
	data, _ := json.Marshal(env)

	// Decrypting with wrong key should fail.
	v, _ := NewVerifier(pub, CurrentVersion)
	_, err = v.VerifyEncrypted(data, wrongKey)
	if err == nil {
		t.Fatal("expected error for wrong AES key")
	}
}

func TestEncryptedSaveLoad(t *testing.T) {
	pub, priv := newTestPair(t)
	aesKey, _ := GenerateAESKey()

	lic := &License{Product: "acme", Subject: "bob", Expiry: time.Now().Add(time.Hour)}
	signer, _ := NewSigner(priv, CurrentVersion)
	env, err := signer.SignEncrypted(lic, aesKey)
	if err != nil {
		t.Fatalf("SignEncrypted: %v", err)
	}

	path := filepath.Join(t.TempDir(), "lic.enc")
	if err := SaveEncryptedEnvelope(path, env); err != nil {
		t.Fatalf("SaveEncryptedEnvelope: %v", err)
	}
	loaded, err := LoadEncryptedEnvelope(path)
	if err != nil {
		t.Fatalf("LoadEncryptedEnvelope: %v", err)
	}
	data, _ := json.Marshal(loaded)
	v, _ := NewVerifier(pub, CurrentVersion)
	got, err := v.VerifyEncrypted(data, aesKey)
	if err != nil {
		t.Fatalf("verify loaded encrypted: %v", err)
	}
	if got.Subject != "bob" {
		t.Fatalf("subject mismatch: got %q", got.Subject)
	}
}

func TestGenerateAESKey(t *testing.T) {
	key, err := GenerateAESKey()
	if err != nil {
		t.Fatalf("GenerateAESKey: %v", err)
	}
	if len(key) != AESKeySize {
		t.Fatalf("key length = %d, want %d", len(key), AESKeySize)
	}
	// Two generated keys should be different.
	key2, _ := GenerateAESKey()
	if string(key) == string(key2) {
		t.Fatal("two generated keys should not be identical")
	}
}

func TestEncryptedConcurrentVerify(t *testing.T) {
	pub, priv := newTestPair(t)
	aesKey, _ := GenerateAESKey()

	lic := &License{Product: "acme", Expiry: time.Now().Add(time.Hour)}
	signer, _ := NewSigner(priv, CurrentVersion)
	env, err := signer.SignEncrypted(lic, aesKey)
	if err != nil {
		t.Fatalf("SignEncrypted: %v", err)
	}
	data, _ := json.Marshal(env)

	v, _ := NewVerifier(pub, CurrentVersion)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.VerifyEncrypted(data, aesKey); err != nil {
				t.Errorf("concurrent encrypted verify failed: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestNewSignerKeyValidation(t *testing.T) {
	_, err := NewSigner(nil, CurrentVersion)
	if err == nil {
		t.Fatal("expected error for nil private key")
	}
	_, err = NewSigner(ed25519.PrivateKey(make([]byte, 10)), CurrentVersion)
	if err == nil {
		t.Fatal("expected error for short private key")
	}
}

func TestNewVerifierKeyValidation(t *testing.T) {
	_, err := NewVerifier(nil, CurrentVersion)
	if err == nil {
		t.Fatal("expected error for nil public key")
	}
	_, err = NewVerifier(ed25519.PublicKey(make([]byte, 10)), CurrentVersion)
	if err == nil {
		t.Fatal("expected error for short public key")
	}
}

func TestSignDoesNotMutateInput(t *testing.T) {
	_, priv := newTestPair(t)
	lic := &License{Product: "acme", Version: 0}
	signer, _ := NewSigner(priv, CurrentVersion)
	_, err := signer.Sign(lic)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if lic.Version != 0 {
		t.Fatalf("Sign mutated input License: version = %d, want 0", lic.Version)
	}
}

func TestParseEnvelopeErrors(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"invalid json", "{not json"},
		{"missing version", `{"alg":"ed25519","license":"dQ==","signature":"dQ=="}`},
		{"bad alg", `{"version":1,"alg":"rsa","license":"dQ==","signature":"dQ=="}`},
		{"empty license", `{"version":1,"alg":"ed25519","license":"","signature":"dQ=="}`},
		{"bad license base64", `{"version":1,"alg":"ed25519","license":"!!!","signature":"dQ=="}`},
		{"bad signature base64", `{"version":1,"alg":"ed25519","license":"dQ==","signature":"!!!"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseEnvelope([]byte(tc.data))
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("want ErrMalformed, got %v", err)
			}
		})
	}
}

func TestPrettyLicense(t *testing.T) {
	_, priv := newTestPair(t)
	lic := &License{Product: "acme", Subject: "bob", Features: []string{"pro"}}
	signer, _ := NewSigner(priv, CurrentVersion)
	env, err := signer.Sign(lic)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	pretty, err := env.PrettyLicense()
	if err != nil {
		t.Fatalf("PrettyLicense: %v", err)
	}
	if !strings.Contains(pretty, "acme") || !strings.Contains(pretty, "bob") {
		t.Fatalf("PrettyLicense output missing expected fields: %s", pretty)
	}
}

func TestStrictModeAliases(t *testing.T) {
	pub, priv := newTestPair(t)
	lic := &License{
		Product: "acme",
		Machine: &MachineBinding{Fingerprint: "fp-main", Aliases: []string{"fp-alias"}},
	}
	env := mustSign(t, priv, lic)
	// strict mode should match via alias
	fpAlias := func() (string, error) { return "fp-alias", nil }
	v, _ := NewVerifier(pub, CurrentVersion)
	if _, err := v.WithFingerprint(fpAlias).Verify(envBytes(t, env)); err != nil {
		t.Fatalf("strict alias match should pass: %v", err)
	}
}

func TestFingerprintFromSystemUUIDEmpty(t *testing.T) {
	_, err := machine.FingerprintFromSystemUUID("")
	if err == nil {
		t.Fatal("expected error for empty system UUID")
	}
}
