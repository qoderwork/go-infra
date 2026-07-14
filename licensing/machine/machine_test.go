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
