// Package machine computes a stable, normalized machine fingerprint used to
// bind a license to a physical (or virtual) machine.
//
// The fingerprint is derived from a hardware identifier chosen from a
// platform-specific fallback chain. The most preferred source is the DMI
// system UUID (dmidecode -s system-uuid), which is the most stable host
// identifier (survives OS reinstall and container restart):
//
//	Linux   : dmidecode system-uuid -> /sys/class/dmi/id/board_serial -> /etc/machine-id
//	macOS   : system_profiler serial number
//	Windows : Win32_BaseBoard serial        -> Cryptography\MachineGuid
//
// Fail-closed policy: if no trusted hardware identifier can be read,
// Fingerprint returns an error. It deliberately does NOT fall back to
// mutable identifiers (hostname, MAC address) to prevent weak bindings.
//
// For node-locking a license to a physical host, use SystemUUID instead.
// It returns the DMI system UUID (dmidecode -s system-uuid, upper case, no
// dashes) — the canonical "machine code" shared between the issuer and the
// verifying application. See SystemUUID for the container mount requirements.
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
	"to be filled", "system serial", "not specified",
	"0x0", "o.e.m", "base board",
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

// normalize hashes the raw identifier and formats it as a UUID v5 string
// (xxxxxxxx-xxxx-5xxx-80xx-xxxxxxxxxxxx). This makes the fingerprint
// human-friendly and consistent with RFC 4122 naming conventions while
// remaining a one-way hash — the original hardware serial cannot be recovered.
func normalize(raw string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x50 // version 5
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	hex := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
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
		// Preferred: the DMI system UUID (dmidecode -s system-uuid). It is a
		// host hardware value read from /dev/mem, so it survives OS reinstalls
		// and container restarts — the most stable identifier available. In a
		// container, /dev/mem and /sbin/dmidecode must be bind-mounted from the
		// host for this to resolve to the host's UUID.
		if out, err := exec.Command("dmidecode", "-s", "system-uuid").Output(); err == nil {
			if t := systemUUIDToken(string(out)); t != "" {
				return t, nil
			}
		}
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
	return "", fmt.Errorf("machine: no stable hardware identifier found")
}

// Fingerprint returns a normalized, stable machine identifier in UUID v5
// format. It returns an error if no trusted hardware identifier can be read
// (fail-closed). It never panics.
//
// The underlying hardware identifier is chosen from a fallback chain whose
// most preferred member is the DMI system UUID (dmidecode -s system-uuid),
// which is the most stable host identifier (survives OS reinstall and
// container restart). See rawFingerprint for the full chain.
func Fingerprint() (string, error) {
	raw, err := rawFingerprint()
	if err != nil {
		return "", err
	}
	if isPlaceholder(raw) {
		return "", fmt.Errorf("machine: hardware identifier is a placeholder")
	}
	return normalize(raw), nil
}

// systemUUIDToken turns a raw dmidecode -s system-uuid value into the
// prefixed, normalized token used inside rawFingerprint. Empty/placeholder
// input yields "" so callers can fall through to the next source.
func systemUUIDToken(raw string) string {
	n := NormalizeSystemUUID(raw)
	if n == "" || isPlaceholder(n) {
		return ""
	}
	return "sysuuid:" + n
}

// FingerprintFromSystemUUID returns the same binding token Fingerprint would
// produce for the given host system UUID. Use it when an issuer needs to pin a
// license to a target host whose system UUID is known (e.g. from inventory)
// without running on that host: the resulting token matches what the target's
// Fingerprint() computes at verification time.
func FingerprintFromSystemUUID(raw string) string {
	return normalize(systemUUIDToken(raw))
}

// SystemUUID returns the host's DMI system UUID (the value of
// `dmidecode -s system-uuid`), normalized to upper case with the dashes
// removed. This is the canonical "machine code" used to node-lock a license
// to a physical host, matching the reference shell helper:
//
//	dmidecode -s system-uuid | sed 's/-//g' | awk '{print toupper($0)}'
//
// It shells out to dmidecode, which reads the DMI/SMBIOS table from /dev/mem.
// In container deployments /dev/mem and /sbin/dmidecode must be bind-mounted
// from the host (see the project's java-compose.yml) so the value matches the
// one used when the license was issued.
//
// Fail-closed: if dmidecode is unavailable, or it returns an empty/placeholder
// identifier, an error is returned. It deliberately does NOT fall back to
// another identifier (doing so would silently bind the license to the wrong
// machine).
func SystemUUID() (string, error) {
	out, err := exec.Command("dmidecode", "-s", "system-uuid").Output()
	if err != nil {
		return "", fmt.Errorf("machine: dmidecode system-uuid: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" || isPlaceholder(raw) {
		return "", fmt.Errorf("machine: no usable system-uuid from dmidecode")
	}
	return NormalizeSystemUUID(raw), nil
}

// NormalizeSystemUUID normalizes a raw DMI system UUID the same way the
// reference shell helper does: trim surrounding whitespace, drop dashes, and
// upper-case the result.
func NormalizeSystemUUID(raw string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(raw), "-", ""))
}
