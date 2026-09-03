package main

// Floating point on the R5900: float32 is the hardware FPU (single precision
// only, rounds toward zero, no NaN or infinity), float64 is software.
// Compare forms are written out one by one, including the negations, because
// each becomes a different LLVM condition code.

import (
	"fmt"
	"math"

	"ps2go/harness"
)

// Inputs live in globals so nothing is folded at compile time.
var fa = []float32{-3.5, -1, -0.5, 0, 0.5, 1, 2.25, 3, 1e10, -1e10, 1e-10, 123456.75}
var da = []float64{-3.5, -1, -0.5, 0, 0.5, 1, 2.25, 3, 1e10, -1e10, 1e-10, 123456.75, 1e300, -1e-300}

//go:noinline
func c32(a, b float32) [12]bool {
	return [12]bool{a < b, a <= b, a > b, a >= b, a == b, a != b,
		!(a < b), !(a <= b), !(a > b), !(a >= b), !(a == b), !(a != b)}
}

//go:noinline
func c64(a, b float64) [12]bool {
	return [12]bool{a < b, a <= b, a > b, a >= b, a == b, a != b,
		!(a < b), !(a <= b), !(a > b), !(a >= b), !(a == b), !(a != b)}
}

// ref computes the twelve outcomes from an exact integer ordering.
func ref(lt, eq bool) [12]bool {
	gt := !lt && !eq
	return [12]bool{lt, lt || eq, gt, gt || eq, eq, !eq,
		!lt, !(lt || eq), !gt, !(gt || eq), !eq, eq}
}

func testFloat32Compare() error {
	for _, a := range fa {
		for _, b := range fa {
			got := c32(a, b)
			// Reference ordering from float64, which is software on this target.
			want := ref(float64(a) < float64(b), float64(a) == float64(b))
			if got != want {
				return fmt.Errorf("%v vs %v: got %v want %v", a, b, got, want)
			}
		}
	}
	// The R5900 has no NaN: a NaN bit pattern is just an ordinary number to
	// the FPU, so Go's NaN rules cannot hold for float32 on this target.
	nan := float32(math.NaN())
	harness.Logf("float32: NaN==NaN is %v on this target (would be false on IEEE hardware)", nan == nan)
	return nil
}

func testFloat64Compare() error {
	for _, a := range da {
		for _, b := range da {
			got := c64(a, b)
			want := ref(a < b, a == b)
			if got != want {
				return fmt.Errorf("%v vs %v: got %v want %v", a, b, got, want)
			}
		}
	}
	nan := math.NaN()
	if nan == nan || nan < 1 || nan > 1 || !(nan != nan) || !math.IsNaN(nan) {
		return fmt.Errorf("float64 NaN semantics")
	}
	if !math.IsInf(math.Inf(1), 1) || math.Inf(1) < math.MaxFloat64 {
		return fmt.Errorf("float64 Inf")
	}
	return nil
}

//go:noinline
func f32ops(a, b float32) (add, sub, mul, div float32) { return a + b, a - b, a * b, a / b }

//go:noinline
func f64ops(a, b float64) (add, sub, mul, div float64) { return a + b, a - b, a * b, a / b }

// within1ulp accepts the truncating FPU: the result may be one unit below the
// correctly rounded value.
func within1ulp32(got, want float32) bool {
	if got == want {
		return true
	}
	return math.Abs(float64(got-want)) <= math.Abs(float64(math.Nextafter32(want, float32(math.Inf(1)))-want))
}

func testFloat32Arith() error {
	// Exact in single precision: must match bit for bit.
	exact := []struct{ a, b, add, sub, mul, div float32 }{
		{1.5, 2.25, 3.75, -0.75, 3.375, 0.6666666865348816}, // div inexact, checked below
		{3, 4, 7, -1, 12, 0.75},
		{-8, 0.5, -7.5, -8.5, -4, -16},
		{1e10, 1e10, 2e10, 0, 1e20, 1},
	}
	for _, e := range exact {
		add, sub, mul, div := f32ops(e.a, e.b)
		if add != e.add || sub != e.sub {
			return fmt.Errorf("%v,%v: add %v sub %v", e.a, e.b, add, sub)
		}
		// Products and quotients may be inexact; the FPU truncates.
		if !within1ulp32(mul, e.mul) || !within1ulp32(div, e.div) {
			return fmt.Errorf("%v,%v: mul %v want ~%v, div %v want ~%v", e.a, e.b, mul, e.mul, div, e.div)
		}
	}
	// Inexact: within one ulp of the correctly rounded value.
	third := float32(1) / float32(3)
	_, _, _, div := f32ops(1, 3)
	if !within1ulp32(div, third) {
		return fmt.Errorf("1/3 = %v", div)
	}
	_, _, nine, _ := f32ops(3, 3)
	if s := float32(math.Sqrt(float64(nine))); s != 3 { // sqrt(9) is exact
		return fmt.Errorf("sqrt(%v) = %v", nine, s)
	}
	// Conversions.
	if int32(fa[4]) != 0 || int32(fa[0]) != -3 || int32(fa[11]) != 123456 {
		return fmt.Errorf("f32->int truncation: %d %d %d", int32(fa[4]), int32(fa[0]), int32(fa[11]))
	}
	if float32(int32(-7)) != -7 || float32(uint32(3000000000)) != 3e9 {
		return fmt.Errorf("int->f32")
	}
	if float64(fa[6]) != 2.25 || float32(da[11]) != 123456.75 {
		return fmt.Errorf("f32<->f64")
	}
	_, _, _, q := f32ops(1.5, 2.25)
	harness.Logf("float32: 1/3 = %.9g (IEEE nearest %.9g), 1.5/2.25 = %.9g", div, third, q)
	return nil
}

func testFloat64Arith() error {
	cases := []struct{ a, b, add, sub, mul, div float64 }{
		{1.5, 2.25, 3.75, -0.75, 3.375, 1.5 / 2.25},
		{3, 4, 7, -1, 12, 0.75},
		{1, 3, 4, -2, 3, 1.0 / 3},
		{1e150, 2, 1e150, 1e150, 2e150, 5e149},
		{-8, 0.5, -7.5, -8.5, -4, -16},
	}
	for _, e := range cases {
		add, sub, mul, div := f64ops(e.a, e.b)
		if add != e.add || sub != e.sub || mul != e.mul || div != e.div {
			return fmt.Errorf("%v,%v: add %v sub %v mul %v div %v", e.a, e.b, add, sub, mul, div)
		}
	}
	if math.Sqrt(2)*math.Sqrt(2) != 2.0000000000000004 || math.Floor(-2.5) != -3 || math.Round(2.5) != 3 {
		return fmt.Errorf("math funcs")
	}
	if int64(da[12]/1e290) != 10000000000 || float64(int64(1)<<53) != 9007199254740992 {
		return fmt.Errorf("f64<->int64")
	}
	if float32(da[6]) != 2.25 || float64(float32(0.1)) == 0.1 {
		return fmt.Errorf("f64<->f32")
	}
	s := fmt.Sprintf("%.6f %.3e %g", math.Pi, 123456.789, 1e-7)
	if s != "3.141593 1.235e+05 1e-07" {
		return fmt.Errorf("format %q", s)
	}
	return nil
}
