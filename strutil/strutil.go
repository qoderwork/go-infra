// Package strutil provides string manipulation utilities.
package strutil

import (
	"strings"
	"unicode"
)

// SwapCase returns a copy of the string with letter case swapped.
// Upper case letters are converted to lower case and vice versa.
func SwapCase(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsUpper(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if unicode.IsLower(r) {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Capitalize returns a copy of the string with the first letter capitalized.
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// CapitalizeAll returns a copy of the string with the first letter of each word capitalized.
func CapitalizeAll(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prev := ' '
	for _, r := range s {
		if prev == ' ' {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

// Initials returns the first letter of each word in the string.
// Words are separated by whitespace. The result is a concatenation of the initials.
func Initials(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	prev := ' '
	for _, r := range s {
		if prev == ' ' && r != ' ' {
			b.WriteRune(unicode.ToUpper(r))
		}
		prev = r
	}
	return b.String()
}

// InitialsWithSeparator returns the first letter of each word in the string,
// separated by the given separator.
func InitialsWithSeparator(s, sep string) string {
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	initials := make([]string, len(parts))
	for i, p := range parts {
		if len(p) > 0 {
			initials[i] = strings.ToUpper(string(p[0]))
		}
	}
	return strings.Join(initials, sep)
}

// Abbreviate abbreviates the string using the first n letters of each word.
// If n <= 0, returns an empty string.
func Abbreviate(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	parts := strings.Fields(s)
	var b strings.Builder
	for _, p := range parts {
		if len(p) >= n {
			b.WriteString(p[:n])
		} else {
			b.WriteString(p)
		}
	}
	return b.String()
}

// AbbreviateWithSeparator abbreviates the string using the first n letters of each word,
// separated by the given separator.
func AbbreviateWithSeparator(s string, n int, sep string) string {
	if n <= 0 || s == "" {
		return ""
	}
	parts := strings.Fields(s)
	abbrevs := make([]string, len(parts))
	for i, p := range parts {
		if len(p) >= n {
			abbrevs[i] = p[:n]
		} else {
			abbrevs[i] = p
		}
	}
	return strings.Join(abbrevs, sep)
}

// Reverse returns a reversed copy of the string.
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// Truncate truncates the string to the given length, adding "..." if truncated.
// If length <= 0, returns an empty string.
func Truncate(s string, length int) string {
	if length <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= length {
		return s
	}
	return string(r[:length]) + "..."
}

// TruncateWithoutEllipsis truncates the string to the given length without adding "...".
func TruncateWithoutEllipsis(s string, length int) string {
	if length <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= length {
		return s
	}
	return string(r[:length])
}

// IsEmpty returns true if the string is empty or contains only whitespace.
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// IsBlank returns true if the string is empty (no whitespace check).
func IsBlank(s string) bool {
	return s == ""
}

// DefaultIfEmpty returns the default value if the string is empty or contains only whitespace.
func DefaultIfEmpty(s, def string) string {
	if IsEmpty(s) {
		return def
	}
	return s
}

// RemoveAll removes all occurrences of the substrings from the string.
func RemoveAll(s string, substrs ...string) string {
	for _, substr := range substrs {
		s = strings.ReplaceAll(s, substr, "")
	}
	return s
}

// Quote returns the string wrapped in double quotes.
func Quote(s string) string {
	return `"` + s + `"`
}

// Unquote removes surrounding double quotes from the string.
// Returns the original string if it's not quoted.
func Unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}