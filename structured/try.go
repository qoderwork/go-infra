// Copyright (c) 2023 Sourcegraph
// Derived from github.com/sourcegraph/conc (MIT License), file: panics/try.go.
// Modified by qoderwork: merged into the single structured package.
// See THIRD_PARTY_LICENSES.md for the full license text.
package structured

// Try executes f, catching and returning any panic it might spawn.
//
// The recovered panic can be propagated with panic(), or handled as a normal error with
// (*Recovered).AsError().
func Try(f func()) *Recovered {
	var c Catcher
	c.Try(f)
	return c.Recovered()
}
