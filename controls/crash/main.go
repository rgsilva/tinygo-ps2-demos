// Negative control: the harness must report CRASH.
package main

import (
	"unsafe"

	"ps2go/harness"
)

var unmapped uintptr = 0x30000000 // kuseg address with no TLB mapping on the PS2

func main() {
	harness.Run([]harness.Case{
		{Name: "ok", Fn: func() error { return nil }},
		{Name: "crash", Fn: func() error {
			*(*uint32)(unsafe.Pointer(unmapped)) = 1
			return nil
		}},
	})
}
