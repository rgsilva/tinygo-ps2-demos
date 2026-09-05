package main

// #include <stdint.h>
// uint32_t ps2go_cfile_mix(uint32_t a, uint32_t b);
import "C"

import (
	"fmt"

	"ps2go/lib/harness"
)

// testCFile calls a function from a .c file in the package (tests/cfile.c).
func testCFile() error {
	a := uint32(12345)
	got := uint32(C.ps2go_cfile_mix(C.uint32_t(a), 0xabcdef))
	want := a*2654435761 ^ 0xabcdef
	harness.Logf("cfile: %#x", got)
	if got != want {
		return fmt.Errorf("got %#x, want %#x", got, want)
	}
	return nil
}
