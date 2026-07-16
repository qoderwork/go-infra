package machine

import "testing"

func TestIsPlaceholder(t *testing.T) {
	cases := map[string]bool{
		"":                       true,
		"none":                   true,
		"Default string":         true,
		"To Be Filled By O.E.M.": true,
		"00000000":               true,
		"System Serial Number":   true,
		"Not Specified":          true,
		"ABC123XYZ":              false,
		"real-board-9f":          false,
	}
	for in, want := range cases {
		if got := isPlaceholder(in); got != want {
			t.Errorf("isPlaceholder(%q)=%v want %v", in, got, want)
		}
	}
}

func TestNormalizeStable(t *testing.T) {
	a := normalize("Board:XYZ")
	b := normalize("Board:XYZ")
	if a != b {
		t.Fatalf("normalize not stable: %q != %q", a, b)
	}
	if len(a) != 36 {
		t.Fatalf("normalize len = %d want 36 (UUID format)", len(a))
	}
	if normalize("x") == normalize("y") {
		t.Fatalf("normalize collides for different inputs")
	}
}

func TestUUIDv5Format(t *testing.T) {
	uuid := normalize("Board:TEST-123")
	// Check version nibble is 5
	if uuid[14] != '5' {
		t.Fatalf("UUID version nibble = %c want 5, uuid=%s", uuid[14], uuid)
	}
	// Check variant bits: position 19 should be 8, 9, a, or b
	v := uuid[19]
	if v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Fatalf("UUID variant = %c want [89ab], uuid=%s", v, uuid)
	}
	// Check dash positions: 8, 13, 18, 23
	for _, i := range []int{8, 13, 18, 23} {
		if uuid[i] != '-' {
			t.Fatalf("UUID dash missing at pos %d: %s", i, uuid)
		}
	}
}

func TestFingerprintNoPanic(t *testing.T) {
	fp, err := Fingerprint()
	// Fail-closed: either a valid UUID fingerprint, or a clean error. Never a panic.
	if err != nil && fp != "" {
		t.Fatalf("both error and fingerprint: %v %q", err, fp)
	}
	if err == nil && len(fp) != 36 {
		t.Fatalf("fingerprint len = %d want 36 (UUID format)", len(fp))
	}
}

// TestNormalizeSystemUUID mirrors the reference shell helper:
//
//	dmidecode -s system-uuid | sed 's/-//g' | awk '{print toupper($0)}'
func TestNormalizeSystemUUID(t *testing.T) {
	cases := map[string]string{
		"4c4c4544-004e-4d10-8034-b4a44c4c4634": "4C4C4544004E4D108034B4A44C4C4634",
		"4C4C4544-004E-4D10-8034-B4A44C4C4634": "4C4C4544004E4D108034B4A44C4C4634",
		"  a1b2c3d4-1111-2222-3333-444455556666  ": "A1B2C3D4111122223333444455556666",
		"A1B2C3D4111122223333444455556666":       "A1B2C3D4111122223333444455556666",
	}
	for in, want := range cases {
		if got := NormalizeSystemUUID(in); got != want {
			t.Errorf("NormalizeSystemUUID(%q)=%q want %q", in, got, want)
		}
	}
}

// TestFingerprintFromSystemUUID locks in that the token the issuer embeds via
// `sign -system-uuid` matches what the target's Fingerprint() produces at
// verify time, regardless of how the UUID was formatted on input.
func TestFingerprintFromSystemUUID(t *testing.T) {
	a, err := FingerprintFromSystemUUID("4c4c4544-004e-4d10-8034-b4a44c4c4634")
	if err != nil {
		t.Fatalf("FingerprintFromSystemUUID: %v", err)
	}
	b, err := FingerprintFromSystemUUID("4C4C4544004E4D108034B4A44C4C4634")
	if err != nil {
		t.Fatalf("FingerprintFromSystemUUID: %v", err)
	}
	if a != b {
		t.Fatalf("FingerprintFromSystemUUID not stable across formatting: %q != %q", a, b)
	}
	if len(a) != 36 {
		t.Fatalf("FingerprintFromSystemUUID len = %d want 36 (UUID v5)", len(a))
	}
	c, err := FingerprintFromSystemUUID("5B9A2D3E-4F5A-6B7C-8D9E-0F1A2B3C4D5E")
	if err != nil {
		t.Fatalf("FingerprintFromSystemUUID: %v", err)
	}
	if a == c {
		t.Fatalf("FingerprintFromSystemUUID collides for a different uuid")
	}
}

func TestFingerprintFromSystemUUIDEmpty(t *testing.T) {
	_, err := FingerprintFromSystemUUID("")
	if err == nil {
		t.Fatal("expected error for empty system UUID")
	}
	_, err = FingerprintFromSystemUUID("00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for placeholder system UUID")
	}
}
