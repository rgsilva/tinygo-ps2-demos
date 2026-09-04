// Package gotest is exercised with `tinygo test -target ps2 ./gotest`: Go's
// own testing package running in PCSX2 through the harness (the emulator of
// the ps2 target). Build with -tags ps2fail to include a failing test.
package gotest

// Add returns a + b.
func Add(a, b int) int { return a + b }
