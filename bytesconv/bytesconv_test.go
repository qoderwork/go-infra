package bytesconv

import "testing"

func TestBytesToString(t *testing.T) {
	b := []byte("hello world")
	s := BytesToString(b)
	if s != "hello world" {
		t.Errorf("BytesToString = %q, want %q", s, "hello world")
	}
}

func TestStringToBytes(t *testing.T) {
	s := "hello world"
	b := StringToBytes(s)
	if string(b) != "hello world" {
		t.Errorf("StringToBytes = %q, want %q", string(b), "hello world")
	}
}

func TestRoundtrip(t *testing.T) {
	tests := []string{
		"",
		"a",
		"hello",
		"hello world",
		"你好世界",
	}
	for _, tt := range tests {
		b := StringToBytes(tt)
		s := BytesToString(b)
		if s != tt {
			t.Errorf("roundtrip(%q) = %q", tt, s)
		}
	}
}

func BenchmarkBytesToString(b *testing.B) {
	data := []byte("hello world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BytesToString(data)
	}
}

func BenchmarkStringToBytes(b *testing.B) {
	s := "hello world"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StringToBytes(s)
	}
}