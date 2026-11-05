// Copyright (c) 2021 The Gnet Authors. All rights reserved.
// Copyright (c) 2019-present Andy Pan
// Copyright (c) 2017 Joshua J Baker
//
// Derived from github.com/panjf2000/gnet, licensed under Apache-2.0.
// Modified by qoderwork: package renamed and dependencies retargeted.
// Full text: LICENSES/Apache-2.0.txt; see THIRD_PARTY_LICENSES.md.

// Package bytesconv provides zero-copy conversion utilities
// between byte slices and strings.
package bytesconv

import "unsafe"

// BytesToString converts a byte slice to a string without memory allocation.
//
// The returned string shares the underlying memory with the byte slice.
// The byte slice must not be modified during the lifetime of the string.
func BytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes converts a string to a byte slice without memory allocation.
//
// The returned byte slice shares the underlying memory with the string.
// The byte slice must not be modified (strings are immutable in Go).
func StringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
