// Package machine computes a stable, normalized machine fingerprint used to
// bind a license to a physical (or virtual) machine.
//
// The fingerprint is a sha256 hex of a hardware identifier chosen from a
// fallback chain:
//
//	Linux   : /sys/class/dmi/id/board_serial -> /etc/machine-id
//	macOS   : system_profiler serial number
//	Windows : Win32_BaseBoard serial        -> Cryptography\MachineGuid
//
// If every source is missing or a known placeholder (common in VMs), it falls
// back to a hash of the hostname so the call never panics.
package machine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// placeholderWords are substrings that indicate a BIOS/VM filler value rather
// than a real hardware id.
var placeholderWords = []string{
	"none", "null", "nil", "unknown", "n/a", "na", "default",
	"to be filled", "system serial", "0x0", "o.e.m",
}

func isPlaceholder(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	if t == "" {
		return true
	}
	for _, w := range placeholderWords {
		if strings.Contains(t, w) {
			return true
		}
	}
	// all-identical characters, e.g. "00000000" or "xxxxxxxx"
	if len(t) > 0 {
		r := t[0]
		for i := 1; i < len(t); i++ {
			if t[i] != r {
				return false
			}
		}
		return true
	}
	return false
}

func normalize(raw string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(h[:])
}

func readFileTrim(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func powershellOutput(script string) (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func extractRegex(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func rawFingerprint() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if s := readFileTrim("/sys/class/dmi/id/board_serial"); s != "" && !isPlaceholder(s) {
			return "board:" + s, nil
		}
		if s := readFileTrim("/etc/machine-id"); s != "" {
			return "mid:" + s, nil
		}
		if s := readFileTrim("/var/lib/dbus/machine-id"); s != "" {
			return "mid:" + s, nil
		}
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPHardwareDataType").Output(); err == nil {
			if m := extractRegex(regexp.MustCompile(`Serial Number.*?:\s*(\S+)`), string(out)); m != "" {
				return "serial:" + m, nil
			}
		}
	case "windows":
		if s, err := powershellOutput("(Get-WmiObject Win32_BaseBoard).SerialNumber"); err == nil && s != "" && !isPlaceholder(s) {
			return "board:" + s, nil
		}
		if s, err := powershellOutput("(Get-ItemProperty 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography').MachineGuid"); err == nil && s != "" {
			return "guid:" + s, nil
		}
	}
	return "", fmt.Errorf("machine: no stable identifier found")
}

// Fingerprint returns a normalized, stable machine identifier, or an error if
// even the hostname fallback is unavailable. It never panics.
func Fingerprint() (string, error) {
	raw, err := rawFingerprint()
	if err != nil || isPlaceholder(raw) {
		host, herr := os.Hostname()
		if herr != nil {
			return "", fmt.Errorf("machine: fingerprint unavailable: %w", err)
		}
		raw = "host:" + host
	}
	return normalize(raw), nil
}
