package machine

import "testing"

func TestIsPlaceholder(t *testing.T) {
	cases := map[string]bool{
		"":                        true,
		"none":                    true,
		"Default string":          true,
		"To Be Filled By O.E.M.": true,
		"00000000":                true,
		"System Serial Number":    true,
		"ABC123XYZ":               false,
		"real-board-9f":           false,
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
	if len(a) != 64 {
		t.Fatalf("normalize len = %d want 64", len(a))
	}
	if normalize("x") == normalize("y") {
		t.Fatalf("normalize collides for different inputs")
	}
}

func TestFingerprintNoPanic(t *testing.T) {
	fp, err := Fingerprint()
	// Either we get a fingerprint, or a clean error. Never a panic.
	if err != nil && fp != "" {
		t.Fatalf("both error and fingerprint: %v %q", err, fp)
	}
}
