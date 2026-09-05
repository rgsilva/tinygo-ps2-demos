package main

import (
	"fmt"
	"math/rand/v2"

	"ps2go/lib/harness"
)

// testMathRand checks that the runtime seeds math/rand on the PS2. Without a
// provider the runtime falls back to xorshift64 from a constant state, whose
// first value is known; with one, the values must vary and spread.
func testMathRand() error {
	const unseededFirst = 0x47e4ce4b896cdd1d
	first := rand.Uint64()
	harness.Logf("math/rand: first value %#x", first)
	if first == unseededFirst {
		return fmt.Errorf("math/rand is not seeded (constant-state xorshift)")
	}
	seen := map[uint64]bool{first: true}
	for i := 0; i < 256; i++ {
		v := rand.Uint64()
		if seen[v] {
			return fmt.Errorf("repeated value %#x after %d draws", v, i+1)
		}
		seen[v] = true
	}
	var buckets [16]int
	for i := 0; i < 4096; i++ {
		buckets[rand.IntN(16)]++
	}
	for b, n := range buckets {
		if n < 128 || n > 384 { // expected 256 each
			return fmt.Errorf("bucket %d got %d of 4096 draws", b, n)
		}
	}
	return nil
}
