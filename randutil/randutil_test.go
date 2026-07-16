package randutil

import (
	"testing"
)

func TestStringLength(t *testing.T) {
	s := String(10, CharsetAlphaNumeric)
	if len(s) != 10 {
		t.Fatalf("len(String(10)) = %d, want 10", len(s))
	}
}

func TestStringCharset(t *testing.T) {
	s := String(1000, CharsetNumeric)
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("String with CharsetNumeric contains non-digit: %c", c)
		}
	}
}

func TestStringZeroLength(t *testing.T) {
	s := String(0, CharsetAlpha)
	if s != "" {
		t.Fatalf("String(0) = %q, want empty", s)
	}
}

func TestCryptoStringLength(t *testing.T) {
	s, err := CryptoString(10, CharsetAlphaNumeric)
	if err != nil {
		t.Fatalf("CryptoString error: %v", err)
	}
	if len(s) != 10 {
		t.Fatalf("len(CryptoString(10)) = %d, want 10", len(s))
	}
}

func TestCryptoStringCharset(t *testing.T) {
	s, err := CryptoString(1000, CharsetHex)
	if err != nil {
		t.Fatalf("CryptoString error: %v", err)
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("CryptoString with CharsetHex contains invalid char: %c", c)
		}
	}
}

func TestDNS1035Label(t *testing.T) {
	for i := 0; i < 100; i++ {
		s := DNS1035Label(10)
		if len(s) != 10 {
			t.Fatalf("len(DNS1035Label(10)) = %d, want 10", len(s))
		}
		// First char must be a letter.
		if s[0] < 'a' || s[0] > 'z' {
			t.Fatalf("DNS1035Label first char = %c, want letter", s[0])
		}
		// Last char must be alphanumeric (no hyphen).
		last := s[len(s)-1]
		if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
			t.Fatalf("DNS1035Label last char = %c, want alphanumeric", last)
		}
	}
}

func TestBytes(t *testing.T) {
	b, err := Bytes(32)
	if err != nil {
		t.Fatalf("Bytes error: %v", err)
	}
	if len(b) != 32 {
		t.Fatalf("len(Bytes(32)) = %d, want 32", len(b))
	}
}

func TestBytesZeroLength(t *testing.T) {
	b, err := Bytes(0)
	if err != nil {
		t.Fatalf("Bytes(0) error: %v", err)
	}
	if b != nil {
		t.Fatalf("Bytes(0) = %v, want nil", b)
	}
}

func TestAlphaFuncs(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(int) string
		charset string
	}{
		{"Alpha", Alpha, CharsetAlphaLower},
		{"AlphaUpper", AlphaUpper, CharsetAlphaUpper},
		{"AlphaMixed", AlphaMixed, CharsetAlpha},
		{"Numeric", Numeric, CharsetNumeric},
		{"AlphaNumeric", AlphaNumeric, CharsetAlphaNumeric},
		{"Hex", Hex, CharsetHex},
		{"URLSafe", URLSafe, CharsetURLSafe},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.fn(100)
			if len(s) != 100 {
				t.Fatalf("len(%s(100)) = %d, want 100", tt.name, len(s))
			}
			for _, c := range s {
				found := false
				for _, valid := range tt.charset {
					if byte(c) == byte(valid) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s() contains invalid char %c", tt.name, c)
				}
			}
		})
	}
}

func TestCryptoFuncs(t *testing.T) {
	tests := []struct {
		name    string
		fn      func(int) (string, error)
		charset string
	}{
		{"CryptoAlpha", CryptoAlpha, CharsetAlphaLower},
		{"CryptoAlphaNumeric", CryptoAlphaNumeric, CharsetAlphaNumeric},
		{"CryptoHex", CryptoHex, CharsetHex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := tt.fn(100)
			if err != nil {
				t.Fatalf("%s error: %v", tt.name, err)
			}
			if len(s) != 100 {
				t.Fatalf("len(%s(100)) = %d, want 100", tt.name, len(s))
			}
			for _, c := range s {
				found := false
				for _, valid := range tt.charset {
					if byte(c) == byte(valid) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s() contains invalid char %c", tt.name, c)
				}
			}
		})
	}
}