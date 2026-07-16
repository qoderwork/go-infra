// Package envutil provides environment variable parsing utilities.
package envutil

import (
	"os"
	"strconv"
)

// String returns the value of the environment variable named by the key.
// If the variable is not present, returns the default value.
func String(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// Int returns the value of the environment variable named by the key as an int.
// If the variable is not present or cannot be parsed, returns the default value.
func Int(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

// Int64 returns the value of the environment variable named by the key as an int64.
// If the variable is not present or cannot be parsed, returns the default value.
func Int64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}

// Float64 returns the value of the environment variable named by the key as a float64.
// If the variable is not present or cannot be parsed, returns the default value.
func Float64(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// Bool returns the value of the environment variable named by the key as a bool.
// If the variable is not present or cannot be parsed, returns the default value.
// Accepts "1", "t", "T", "TRUE", "true", "True" as true.
// Accepts "0", "f", "F", "FALSE", "false", "False" as false.
func Bool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// Lookup returns the value of the environment variable named by the key.
// Returns the value and a boolean indicating whether the variable was present.
func Lookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// Must returns the value of the environment variable named by the key.
// Panics if the variable is not present.
func Must(key string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	panic("envutil: required environment variable " + key + " is not set")
}

// MustInt returns the value of the environment variable named by the key as an int.
// Panics if the variable is not present or cannot be parsed.
func MustInt(key string) int {
	v := Must(key)
	i, err := strconv.Atoi(v)
	if err != nil {
		panic("envutil: environment variable " + key + " is not a valid int: " + v)
	}
	return i
}

// MustBool returns the value of the environment variable named by the key as a bool.
// Panics if the variable is not present or cannot be parsed.
func MustBool(key string) bool {
	v := Must(key)
	b, err := strconv.ParseBool(v)
	if err != nil {
		panic("envutil: environment variable " + key + " is not a valid bool: " + v)
	}
	return b
}

// Set sets the value of the environment variable named by the key.
func Set(key, value string) error {
	return os.Setenv(key, value)
}

// Unset unsets the environment variable named by the key.
func Unset(key string) error {
	return os.Unsetenv(key)
}