package strutil

import "testing"

func TestSwapCase(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Hello", "hELLO"},
		{"hELLO", "Hello"},
		{"HeLLo WoRLd", "hEllO wOrlD"},
		{"123", "123"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SwapCase(tt.in); got != tt.want {
			t.Errorf("SwapCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "Hello"},
		{"Hello", "Hello"},
		{"hELLO", "HELLO"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Capitalize(tt.in); got != tt.want {
			t.Errorf("Capitalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCapitalizeAll(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello world", "Hello World"},
		{"hello  world", "Hello  World"},
		{"HELLO WORLD", "HELLO WORLD"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := CapitalizeAll(tt.in); got != tt.want {
			t.Errorf("CapitalizeAll(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestInitials(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"John Doe", "JD"},
		{"john doe", "JD"},
		{"John Middle Doe", "JMD"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Initials(tt.in); got != tt.want {
			t.Errorf("Initials(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestInitialsWithSeparator(t *testing.T) {
	if got := InitialsWithSeparator("John Doe", "."); got != "J.D" {
		t.Errorf("InitialsWithSeparator = %q, want J.D", got)
	}
}

func TestAbbreviate(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"Hello World", 1, "HW"},
		{"Hello World", 2, "HeWo"},
		{"Hello", 1, "H"},
		{"Hello", 10, "Hello"},
		{"", 1, ""},
		{"Hello", 0, ""},
	}
	for _, tt := range tests {
		if got := Abbreviate(tt.in, tt.n); got != tt.want {
			t.Errorf("Abbreviate(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

func TestReverse(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"hello", "olleh"},
		{"Hello", "olleH"},
		{"", ""},
		{"a", "a"},
	}
	for _, tt := range tests {
		if got := Reverse(tt.in); got != tt.want {
			t.Errorf("Reverse(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in      string
		length  int
		want    string
	}{
		{"Hello World", 5, "Hello..."},
		{"Hello", 10, "Hello"},
		{"Hello", 0, ""},
		{"", 5, ""},
	}
	for _, tt := range tests {
		if got := Truncate(tt.in, tt.length); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.in, tt.length, got, tt.want)
		}
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"   ", true},
		{"\t\n", true},
		{"a", false},
		{" a ", false},
	}
	for _, tt := range tests {
		if got := IsEmpty(tt.in); got != tt.want {
			t.Errorf("IsEmpty(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestDefaultIfEmpty(t *testing.T) {
	if got := DefaultIfEmpty("", "default"); got != "default" {
		t.Errorf("DefaultIfEmpty = %q, want default", got)
	}
	if got := DefaultIfEmpty("   ", "default"); got != "default" {
		t.Errorf("DefaultIfEmpty(whitespace) = %q, want default", got)
	}
	if got := DefaultIfEmpty("value", "default"); got != "value" {
		t.Errorf("DefaultIfEmpty = %q, want value", got)
	}
}

func TestRemoveAll(t *testing.T) {
	if got := RemoveAll("hello world", "l"); got != "heo word" {
		t.Errorf("RemoveAll = %q, want heo word", got)
	}
	if got := RemoveAll("hello world", "l", "o"); got != "he wrd" {
		t.Errorf("RemoveAll = %q, want he wrd", got)
	}
}

func TestQuoteUnquote(t *testing.T) {
	if got := Quote("hello"); got != `"hello"` {
		t.Errorf("Quote = %q, want %q", got, `"hello"`)
	}
	if got := Unquote(`"hello"`); got != "hello" {
		t.Errorf("Unquote = %q, want hello", got)
	}
	if got := Unquote("hello"); got != "hello" {
		t.Errorf("Unquote(unquoted) = %q, want hello", got)
	}
}