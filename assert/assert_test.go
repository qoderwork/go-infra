package assert

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// fakeT records the outcome of an assertion without failing the surrounding
// test. testing.TB has an unexported method and cannot be implemented from
// outside the testing package, so it is embedded (satisfying the full method
// set) while Helper and Error — the only methods these assertions call — are
// overridden to capture rather than report.
type fakeT struct {
	testing.TB
	failed bool
	msgs   []string
}

func (f *fakeT) Helper() {}
func (f *fakeT) Error(args ...any) {
	f.failed = true
	f.msgs = append(f.msgs, fmt.Sprint(args...))
}

func (f *fakeT) reset() { f.failed = false; f.msgs = nil }

func fail(t *testing.T, label string, f *fakeT) {
	t.Helper()
	if !f.failed {
		t.Fatalf("%s: expected a reported failure, got none", label)
	}
}

func TestEqual(t *testing.T) {
	f := &fakeT{}
	if !Equal(f, 1, 1) || f.failed {
		t.Error("Equal matching values must pass")
	}
	f.reset()
	if Equal(f, 1, 2) {
		t.Error("Equal mismatching values must return false")
	}
	fail(t, "Equal mismatch", f)
	f.reset()
	if !Equal(f, []int{1, 2}, []int{1, 2}) {
		t.Error("Equal should deep-compare slices")
	}
}

func TestTrueFalse(t *testing.T) {
	f := &fakeT{}
	if !True(f, true) || !False(f, false) {
		t.Error("True/False with matching value must pass")
	}
	f.reset()
	if True(f, false) {
		t.Error("True(false) must return false")
	}
	fail(t, "True(false)", f)

	f.reset()
	if False(f, true) {
		t.Error("False(true) must return false")
	}
	fail(t, "False(true)", f)
}

func TestErrorAssertions(t *testing.T) {
	f := &fakeT{}
	if !NoError(f, nil) {
		t.Error("NoError(nil) must pass")
	}
	f.reset()
	if Error(f, nil) {
		t.Error("Error(nil) must return false")
	}
	fail(t, "Error(nil)", f)

	target := errors.New("sentinel")
	f.reset()
	if !ErrorIs(f, fmt.Errorf("wrap: %w", target), target) {
		t.Error("ErrorIs should match a wrapped error")
	}
	f.reset()
	if ErrorIs(f, errors.New("other"), target) {
		t.Error("ErrorIs must return false for unrelated error")
	}
	fail(t, "ErrorIs mismatch", f)
}

func TestContains(t *testing.T) {
	f := &fakeT{}
	if !Contains(f, "hello world", "world") {
		t.Error("Contains string should match")
	}
	if !Contains(f, []byte("binary payload"), "payload") {
		t.Error("Contains []byte should match")
	}
	f.reset()
	if Contains(f, "abc", "xyz") {
		t.Error("Contains must return false when absent")
	}
	fail(t, "Contains absent", f)

	f.reset()
	if Contains(f, 42, "x") {
		t.Error("Contains must reject unsupported type")
	}
	fail(t, "Contains bad type", f)
}

func TestNilEmpty(t *testing.T) {
	f := &fakeT{}
	if !Nil(f, error(nil)) || !Nil(f, []int(nil)) || !Nil(f, nil) {
		t.Error("Nil should accept typed and plain nil")
	}
	f.reset()
	if NotNil(f, nil) {
		t.Error("NotNil(nil) must return false")
	}
	fail(t, "NotNil(nil)", f)

	f.reset()
	if !Empty(f, "") || !Empty(f, []int{}) || !Empty(f, 0) {
		t.Error("Empty should accept zero-length/zero-value")
	}
	f.reset()
	if NotEmpty(f, "") {
		t.Error("NotEmpty(\"\") must return false")
	}
	fail(t, "NotEmpty empty", f)
}

func TestLenAndZero(t *testing.T) {
	f := &fakeT{}
	if !Len(f, []int{1, 2, 3}, 3) || !Len(f, "abcd", 4) {
		t.Error("Len should match collection lengths")
	}
	f.reset()
	if Len(f, []int{1}, 2) {
		t.Error("Len must return false on mismatch")
	}
	fail(t, "Len mismatch", f)

	f.reset()
	if Len(f, 42, 1) {
		t.Error("Len must reject unsupported type")
	}
	fail(t, "Len bad type", f)

	f.reset()
	if !Zero(f, 0) || !Zero(f, "") || !Zero(f, time.Duration(0)) {
		t.Error("Zero should accept zero values")
	}
	f.reset()
	if Zero(f, 7) {
		t.Error("Zero(7) must return false")
	}
}

func TestEventually(t *testing.T) {
	f := &fakeT{}
	start := time.Now()
	hits := 0
	if !Eventually(f, func() bool { hits++; return hits > 3 }, time.Second, time.Millisecond) {
		t.Error("Eventually should succeed once the condition turns true")
	}
	if f.failed {
		t.Errorf("Eventually false positive: %v", f.msgs)
	}

	f.reset()
	if Eventually(f, func() bool { return false }, 50*time.Millisecond, time.Millisecond) {
		t.Error("Eventually must return false when condition never holds")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("Eventually should bound its wait by waitFor")
	}
	fail(t, "Eventually timeout", f)
}

func TestNever(t *testing.T) {
	f := &fakeT{}
	if !Never(f, func() bool { return false }, 30*time.Millisecond, time.Millisecond) {
		t.Error("Never should pass while the condition stays false")
	}
	if f.failed {
		t.Errorf("Never false positive: %v", f.msgs)
	}

	f.reset()
	if Never(f, func() bool { return true }, 30*time.Millisecond, time.Millisecond) {
		t.Error("Never must return false when the condition becomes true")
	}
	fail(t, "Never fired", f)
}

func TestFormattedFailureMessage(t *testing.T) {
	f := &fakeT{}
	False(f, true, "value %d is not %s", 1, "false")
	if len(f.msgs) == 0 {
		t.Fatal("expected a failure message")
	}
	want := "value 1 is not false: "
	if got := f.msgs[len(f.msgs)-1]; len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("message = %q, want prefix %q", got, want)
	}
}
