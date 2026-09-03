package main

// Embedded files: content must be exact, and //go:align on the variable must
// align the file data itself (a DMA source needs 16 bytes).

import (
	_ "embed"
	"fmt"
	"unsafe"

	"ps2go/harness"
)

//go:embed embed.bin
var embedPlain []byte

//go:align 16
//go:embed embed.bin
var embedAligned []byte

//go:align 64
//go:embed embed.bin
var embedAligned64 []byte

// Pragmas inside a var ( ... ) group must apply too.
var (
	//go:align 16
	//go:embed embed.bin
	embedGrouped []byte
)

func testEmbed() error {
	if len(embedPlain) != 4099 || len(embedAligned) != 4099 || len(embedAligned64) != 4099 {
		return fmt.Errorf("lengths %d %d %d", len(embedPlain), len(embedAligned), len(embedAligned64))
	}
	// Known content: generated with a fixed seed; check a few bytes and a
	// checksum computed over the plain copy against the aligned ones.
	if embedPlain[0] != 0xa5 || embedPlain[4098] != embedAligned[4098] {
		return fmt.Errorf("content: first byte %#x", embedPlain[0])
	}
	sum := checksum(embedPlain)
	if checksum(embedAligned) != sum || checksum(embedAligned64) != sum {
		return fmt.Errorf("aligned copies differ from the plain one")
	}
	a16 := uintptr(unsafe.Pointer(&embedAligned[0]))
	a64 := uintptr(unsafe.Pointer(&embedAligned64[0]))
	harness.Logf("embed: plain @%#x, align16 @%#x (%%16=%d), align64 @%#x (%%64=%d)", uintptr(unsafe.Pointer(&embedPlain[0])), a16, a16%16, a64, a64%64)
	if a16%16 != 0 || a64%64 != 0 {
		return fmt.Errorf("go:align not applied to embedded data")
	}
	if ag := uintptr(unsafe.Pointer(&embedGrouped[0])); ag%16 != 0 || checksum(embedGrouped) != sum {
		return fmt.Errorf("go:align inside a var group: data at %#x", ag)
	}
	return nil
}
