package envutil

import (
	"os"
	"testing"
)

func TestString(t *testing.T) {
	os.Setenv("TEST_STRING", "hello")
	defer os.Unsetenv("TEST_STRING")

	if v := String("TEST_STRING", "default"); v != "hello" {
		t.Fatalf("String = %q, want hello", v)
	}
	if v := String("NOT_SET", "default"); v != "default" {
		t.Fatalf("String = %q, want default", v)
	}
}

func TestInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	if v := Int("TEST_INT", 0); v != 42 {
		t.Fatalf("Int = %d, want 42", v)
	}
	if v := Int("NOT_SET", 99); v != 99 {
		t.Fatalf("Int = %d, want 99", v)
	}

	os.Setenv("TEST_BAD_INT", "not a number")
	defer os.Unsetenv("TEST_BAD_INT")
	if v := Int("TEST_BAD_INT", 100); v != 100 {
		t.Fatalf("Int with bad value = %d, want 100", v)
	}
}

func TestBool(t *testing.T) {
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")
	os.Setenv("TEST_BOOL_1", "1")
	os.Setenv("TEST_BOOL_0", "0")
	defer os.Unsetenv("TEST_BOOL_TRUE")
	defer os.Unsetenv("TEST_BOOL_FALSE")
	defer os.Unsetenv("TEST_BOOL_1")
	defer os.Unsetenv("TEST_BOOL_0")

	if !Bool("TEST_BOOL_TRUE", false) {
		t.Fatal("Bool(true) = false, want true")
	}
	if Bool("TEST_BOOL_FALSE", true) {
		t.Fatal("Bool(false) = true, want false")
	}
	if !Bool("TEST_BOOL_1", false) {
		t.Fatal("Bool(1) = false, want true")
	}
	if Bool("TEST_BOOL_0", true) {
		t.Fatal("Bool(0) = true, want false")
	}
	if !Bool("NOT_SET", true) {
		t.Fatal("Bool(NOT_SET) = false, want default true")
	}
}

func TestFloat64(t *testing.T) {
	os.Setenv("TEST_FLOAT", "3.14")
	defer os.Unsetenv("TEST_FLOAT")

	if v := Float64("TEST_FLOAT", 0); v != 3.14 {
		t.Fatalf("Float64 = %f, want 3.14", v)
	}
	if v := Float64("NOT_SET", 2.71); v != 2.71 {
		t.Fatalf("Float64 = %f, want 2.71", v)
	}
}

func TestInt64(t *testing.T) {
	os.Setenv("TEST_INT64", "9223372036854775807")
	defer os.Unsetenv("TEST_INT64")

	if v := Int64("TEST_INT64", 0); v != 9223372036854775807 {
		t.Fatalf("Int64 = %d, want max int64", v)
	}
}

func TestLookup(t *testing.T) {
	os.Setenv("TEST_LOOKUP", "value")
	defer os.Unsetenv("TEST_LOOKUP")

	v, ok := Lookup("TEST_LOOKUP")
	if !ok || v != "value" {
		t.Fatalf("Lookup = %q, %v, want value, true", v, ok)
	}

	_, ok = Lookup("NOT_SET")
	if ok {
		t.Fatal("Lookup(NOT_SET) should return false")
	}
}

func TestMust(t *testing.T) {
	os.Setenv("TEST_MUST", "value")
	defer os.Unsetenv("TEST_MUST")

	if v := Must("TEST_MUST"); v != "value" {
		t.Fatalf("Must = %q, want value", v)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Must(NOT_SET) should panic")
		}
	}()
	Must("NOT_SET_THAT_DEFINITELY_DOES_NOT_EXIST_12345")
}

func TestSetUnset(t *testing.T) {
	if err := Set("TEST_SET", "setvalue"); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if os.Getenv("TEST_SET") != "setvalue" {
		t.Fatal("Set did not set the value")
	}
	if err := Unset("TEST_SET"); err != nil {
		t.Fatalf("Unset error: %v", err)
	}
	if _, ok := os.LookupEnv("TEST_SET"); ok {
		t.Fatal("Unset did not unset the value")
	}
}