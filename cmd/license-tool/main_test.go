package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/licensing"
)

// The CLI calls os.Exit on failure paths, so it can only be exercised as a
// subprocess. TestMain builds the binary once; each test drives it and asserts
// on exit codes and stdout. If the build fails the whole file skips.

var (
	toolBin  string
	buildErr string
)

func TestMain(m *testing.M) {
	goBin, err := exec.LookPath("go")
	tmp, tmpErr := os.MkdirTemp("", "license-tool-test")
	if err == nil && tmpErr == nil {
		toolBin = filepath.Join(tmp, "license-tool")
		if runtime.GOOS == "windows" {
			toolBin += ".exe"
		}
		build := exec.Command(goBin, "build", "-o", toolBin, ".")
		if out, berr := build.CombinedOutput(); berr != nil {
			buildErr = string(out)
		}
	} else if err != nil {
		buildErr = "go binary not found on PATH"
	} else {
		buildErr = "temp dir failed"
	}

	code := m.Run()
	if tmp != "" {
		_ = os.RemoveAll(tmp)
	}
	os.Exit(code)
}

func requireTool(t *testing.T) {
	t.Helper()
	if buildErr != "" {
		t.Skipf("license-tool build failed, skipping: %s", buildErr)
	}
}

// run executes the built tool and returns combined stdout+stderr and the
// process error (nil on exit code 0).
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(toolBin, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// writeTemplate marshals a minimal license (no machine binding) that verifies on
// any host. Expiry is far in the future so the time check always passes.
func writeTemplate(t *testing.T) string {
	t.Helper()
	lic := licensing.License{
		Version:  licensing.CurrentVersion,
		Product:  "AIO-NMS",
		Subject:  "acme-corp",
		Features: []string{"advanced", "clustering"},
		Capacity: map[string]int64{"nodes": 25},
		Expiry:   time.Now().AddDate(5, 0, 0).UTC(),
	}
	b, err := json.Marshal(lic)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	path := filepath.Join(t.TempDir(), "template.json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}
	return path
}

func genKeyPair(t *testing.T) (priv, pub string) {
	t.Helper()
	dir := t.TempDir()
	priv = filepath.Join(dir, "private.pem")
	pub = filepath.Join(dir, "public.pem")
	out, err := run(t, "genkey", "-priv", priv, "-pub", pub)
	if err != nil {
		t.Fatalf("genkey: %v\n%s", err, out)
	}
	for _, p := range []string{priv, pub} {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("genkey did not create %s: %v", p, statErr)
		}
	}
	return priv, pub
}

func signLicense(t *testing.T, priv, tpl string) string {
	t.Helper()
	licPath := filepath.Join(filepath.Dir(tpl), "license.lic")
	stdout, err := run(t, "sign", "-key", priv, "-in", tpl, "-out", licPath)
	if err != nil {
		t.Fatalf("sign: %v\n%s", err, stdout)
	}
	if !strings.Contains(stdout, "signed") {
		t.Fatalf("sign output missing confirmation: %q", stdout)
	}
	return licPath
}

func TestGenkeySignVerify(t *testing.T) {
	requireTool(t)

	priv, pub := genKeyPair(t)
	tpl := writeTemplate(t)
	licPath := signLicense(t, priv, tpl)

	out, err := run(t, "verify", "-pub", pub, "-in", licPath)
	if err != nil {
		t.Fatalf("verify should pass: %v\n%s", err, out)
	}
	for _, want := range []string{"VALID", "AIO-NMS", "acme-corp", "advanced", "nodes"} {
		if !strings.Contains(out, want) {
			t.Errorf("verify output missing %q; got:\n%s", want, out)
		}
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	requireTool(t)

	priv, pub := genKeyPair(t)
	licPath := signLicense(t, priv, writeTemplate(t))

	data, err := os.ReadFile(licPath)
	if err != nil {
		t.Fatalf("read lic: %v", err)
	}
	// Corrupt the base64 payload so the signature no longer matches.
	data[len(data)/2] ^= 0x01
	if err := os.WriteFile(licPath, data, 0o644); err != nil {
		t.Fatalf("write tampered lic: %v", err)
	}

	out, err := run(t, "verify", "-pub", pub, "-in", licPath)
	if err == nil {
		t.Fatalf("verify must fail on tampered license; output:\n%s", out)
	}
	if !strings.Contains(out, "INVALID") {
		t.Errorf("tampered verify should report INVALID; got:\n%s", out)
	}
}

func TestVerifyWrongKeyRejected(t *testing.T) {
	requireTool(t)

	priv, _ := genKeyPair(t)
	_, otherPub := genKeyPair(t) // a different key pair

	licPath := signLicense(t, priv, writeTemplate(t))

	out, err := run(t, "verify", "-pub", otherPub, "-in", licPath)
	if err == nil {
		t.Fatalf("verify must fail with an unrelated public key; output:\n%s", out)
	}
	if !strings.Contains(out, "INVALID") {
		t.Errorf("expected INVALID for wrong key; got:\n%s", out)
	}
}

func TestUsageExitCodes(t *testing.T) {
	requireTool(t)

	cases := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"unknown subcommand", []string{"bogus"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := run(t, tc.args...)
			var ee *exec.ExitError
			if !asExit(err, &ee) {
				t.Fatalf("expected non-zero exit, got err=%v", err)
			}
			if ee.ExitCode() != 2 {
				t.Errorf("usage error exit code = %d, want 2", ee.ExitCode())
			}
		})
	}
}

// asExit reports whether err is (or wraps) an *exec.ExitError, filling target.
func asExit(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}
