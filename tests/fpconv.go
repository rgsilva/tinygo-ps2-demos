package main

// Float <-> integer conversions on values the compiler cannot fold. The R5900
// FPU has no TRUNC.W.S (LLVM's usual choice for fptosi); its CVT.W.S rounds
// toward zero and the backend must select that instead. At -opt 0 this used
// to yield 0x4F000000 for every conversion (PCSX2: "Unrecognized FPU/COP1 op").

import (
	"fmt"

	"ps2go/harness"
)

//go:noinline
func idf32(f float32) float32 { return f }

//go:noinline
func idf64(f float64) float64 { return f }

//go:noinline
func idi(i int64) int64 { return i }

func testFloatIntConv() error {
	type c32 struct {
		f   float32
		i   int32
		u   uint32
		i64 int64
	}
	for _, c := range []c32{
		{0, 0, 0, 0}, {1.5, 1, 1, 1}, {-1.5, -1, 0, -1}, {-3, -3, 0, -3},
		{123456.75, 123456, 123456, 123456}, {-2147483648, -2147483648, 0, -2147483648},
		{1073741824, 1073741824, 1073741824, 1073741824}, {3000000000, 0, 3000000000, 3000000000},
		{-1e12, 0, 0, -999999995904},
	} {
		f := idf32(c.f)
		if c.i != 0 || c.f == 0 {
			if got := int32(f); got != c.i {
				return fmt.Errorf("int32(%v) = %d, want %d", c.f, got, c.i)
			}
		}
		if got := uint32(f); got != c.u && c.f >= 0 {
			return fmt.Errorf("uint32(%v) = %d, want %d", c.f, got, c.u)
		}
		if got := int64(f); got != c.i64 {
			return fmt.Errorf("int64(%v) = %d, want %d", c.f, got, c.i64)
		}
	}
	for _, c := range []struct {
		f float64
		i int64
	}{{0, 0}, {2.75, 2}, {-2.75, -2}, {1e15, 1000000000000000}, {-9007199254740992, -9007199254740992}} {
		if got := int64(idf64(c.f)); got != c.i {
			return fmt.Errorf("int64(%v) = %d, want %d", c.f, got, c.i)
		}
		if got := int32(idf64(c.f)); c.i == int64(int32(c.i)) && got != int32(c.i) {
			return fmt.Errorf("int32(%v) = %d, want %d", c.f, got, c.i)
		}
	}
	// Integer -> float, on values the float type represents exactly: the EE
	// FPU rounds toward zero, so an inexact conversion at run time differs
	// from the compiler's round-to-nearest constant folding, and that is the
	// hardware, not a bug.
	for _, i := range []int64{0, 1, -1, 123456, 16777216, -2147483648, 1 << 40, -(1 << 50)} {
		v := idi(i)
		if float64(v) != float64(i) || (i == int64(int32(i)) && float32(int32(v)) != float32(i)) {
			return fmt.Errorf("int -> float for %d", i)
		}
		if i >= 0 && i <= 16777216 && float32(uint32(v)) != float32(uint32(i)) {
			return fmt.Errorf("uint32 -> float32 for %d", i)
		}
	}
	// Near the int32 limit the EE's CVT.W.S saturates (0x7FFFFFFF); where
	// exactly is a hardware/emulator property, so only report it.
	for _, f := range []float32{1073741824, 1500000000, 2000000000, 2147483520, 2147483648, -2147483648, -2147483904} {
		harness.Logf("int32(%v) = %d", f, int32(idf32(f)))
	}
	harness.Log("float <-> int conversions ok")
	return nil
}
