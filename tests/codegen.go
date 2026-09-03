package main

// Cases that pin the R5900-specific code generation: 64-bit multiply and
// divide go through libcalls, selects use MOVN/MOVZ, atomics are libcalls
// (no LL/SC), and the FPU is single precision with float64 in software.

import (
	"fmt"
	"math"
	"sync/atomic"
)

// Inputs live in a global slice so the compiler cannot fold the arithmetic.
var vec = []uint64{
	0, 1, 2, 3, 7, 0xFF, 0x1234, 0xFFFFFFFF, 0x100000000, 0x123456789ABCDEF0,
	0x8000000000000000, 0xFFFFFFFFFFFFFFFF, 0xDEADBEEFCAFEBABE, 0x0000000100000001,
}

// refMul is a 32-bit-limb schoolbook multiply, independent of __muldi3.
func refMul(a, b uint64) uint64 {
	al, ah := a&0xFFFFFFFF, a>>32
	bl, bh := b&0xFFFFFFFF, b>>32
	// Only the low 64 bits of the product matter.
	lo := mul32(uint32(al), uint32(bl))
	cross := uint64(uint32(al))*0 + (mul32(uint32(al), uint32(bh)) << 32) + (mul32(uint32(ah), uint32(bl)) << 32)
	return lo + cross
}

//go:noinline
func mul32(a, b uint32) uint64 {
	// 16-bit limbs so that this stays a 32x32->64 computation done with
	// 32-bit operations only.
	a0, a1 := uint64(a&0xFFFF), uint64(a>>16)
	b0, b1 := uint64(b&0xFFFF), uint64(b>>16)
	return a0*b0 + ((a0*b1 + a1*b0) << 16) + ((a1 * b1) << 32)
}

// refDiv is shift-subtract long division, independent of __udivdi3.
func refDiv(n, d uint64) (q, r uint64) {
	for i := 63; i >= 0; i-- {
		r <<= 1
		r |= (n >> uint(i)) & 1
		if r >= d {
			r -= d
			q |= 1 << uint(i)
		}
	}
	return
}

//go:noinline
func mul64(a, b uint64) uint64 { return a * b }

//go:noinline
func div64(a, b uint64) (uint64, uint64) { return a / b, a % b }

//go:noinline
func sdiv64(a, b int64) (int64, int64) { return a / b, a % b }

func testInt64Mul() error {
	for _, a := range vec {
		for _, b := range vec {
			got, want := mul64(a, b), refMul(a, b)
			if got != want {
				return fmt.Errorf("%#x*%#x = %#x, want %#x", a, b, got, want)
			}
		}
	}
	return nil
}

func testInt64Div() error {
	for _, a := range vec {
		for _, b := range vec {
			if b == 0 {
				continue
			}
			q, r := div64(a, b)
			wq, wr := refDiv(a, b)
			if q != wq || r != wr {
				return fmt.Errorf("%#x/%#x = %#x r %#x, want %#x r %#x", a, b, q, r, wq, wr)
			}
			sa, sb := int64(a), int64(b)
			sq, sr := sdiv64(sa, sb)
			if sq*sb+sr != sa || (sr != 0 && (sr < 0) != (sa < 0)) {
				return fmt.Errorf("signed %d/%d = %d r %d", sa, sb, sq, sr)
			}
		}
	}
	return nil
}

//go:noinline
func sel(c bool, a, b int32) int32 {
	if c {
		return a
	}
	return b
}

//go:noinline
func selU(c bool, a, b uint64) uint64 {
	if c {
		return a
	}
	return b
}

func testSelect() error {
	for i := int32(-5); i <= 5; i++ {
		if sel(i < 0, -1, 1) != map[bool]int32{true: -1, false: 1}[i < 0] {
			return fmt.Errorf("sel %d", i)
		}
		if sel(i == 0, 100, i) != map[bool]int32{true: 100, false: i}[i == 0] {
			return fmt.Errorf("sel eq %d", i)
		}
	}
	if selU(true, 0x1122334455667788, 0) != 0x1122334455667788 || selU(false, 0, 0x8877665544332211) != 0x8877665544332211 {
		return fmt.Errorf("sel64")
	}
	return nil
}

var f32vec = []float32{-3.5, -1, -0.0, 0, 0.5, 1, 2.25, 1e10, -1e10, 1e-10}

//go:noinline
func cmp32(a, b float32) (lt, le, gt, ge, eq bool) {
	return a < b, a <= b, a > b, a >= b, a == b
}

func testFloat32Compare() error {
	for i, a := range f32vec {
		for j, b := range f32vec {
			lt, le, gt, ge, eq := cmp32(a, b)
			// Reference through the index order of the sorted vector, ignoring -0/0.
			wa, wb := a, b
			if lt != (wa < wb) || le != (wa <= wb) || gt != (wa > wb) || ge != (wa >= wb) || eq != (wa == wb) {
				return fmt.Errorf("cmp %v(%d) %v(%d): %v %v %v %v %v", a, i, b, j, lt, le, gt, ge, eq)
			}
		}
	}
	nan := float32(math.NaN())
	if nan == nan || !(nan != nan) {
		return fmt.Errorf("NaN self-compare")
	}
	var x float32 = 3
	if x*x != 9 || float32(math.Sqrt(float64(x*x))) != 3 {
		return fmt.Errorf("mul/sqrt")
	}
	return nil
}

func testFloat64() error {
	a, b := 1.5, 2.25
	if a+b != 3.75 || b-a != 0.75 || a*b != 3.375 || b/a != 1.5 {
		return fmt.Errorf("f64 arith")
	}
	if !(a < b) || a > b || a == b || !(a != b) {
		return fmt.Errorf("f64 compare")
	}
	if float32(b) != 2.25 || float64(float32(0.1)) == 0.1 {
		return fmt.Errorf("f64 conversions")
	}
	s := fmt.Sprintf("%.3f", math.Pi)
	if s != "3.142" {
		return fmt.Errorf("f64 format %q", s)
	}
	return nil
}

func testAtomics() error {
	var n32 int32
	var n64 int64
	var u32 uint32 = 5
	for i := 0; i < 100; i++ {
		atomic.AddInt32(&n32, 3)
		atomic.AddInt64(&n64, 1<<33)
	}
	if atomic.LoadInt32(&n32) != 300 || atomic.LoadInt64(&n64) != 100<<33 {
		return fmt.Errorf("add %d %d", n32, n64)
	}
	if !atomic.CompareAndSwapUint32(&u32, 5, 6) || atomic.CompareAndSwapUint32(&u32, 5, 7) || u32 != 6 {
		return fmt.Errorf("cas %d", u32)
	}
	if atomic.SwapInt32(&n32, -1) != 300 || n32 != -1 {
		return fmt.Errorf("swap")
	}
	return nil
}
