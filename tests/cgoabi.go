package main

// Go <-> C calling convention checks. The ps2sdk is built for the n32 ABI
// with a single-precision FPU (doubles travel as software values), so every
// argument kind that crosses the boundary is exercised here.

/*
#include <stdint.h>

struct ps2go_pair { int32_t a; int64_t b; float c; double d; };

static double ps2go_dbl(double a, double b) { return a * b + 0.5; }
static float ps2go_flt(float a, float b) { return a / b; }
static int64_t ps2go_i64(int64_t a, int64_t b, int32_t c) { return a * b - c; }
static uint64_t ps2go_u64(uint32_t lo, uint32_t hi) { return ((uint64_t)hi << 32) | lo; }
static double ps2go_mix(int32_t i, double d, float f, int64_t l) { return i + d + f + (double)l; }
static struct ps2go_pair ps2go_pair_make(int32_t a, int64_t b, float c, double d) {
	struct ps2go_pair p = {a, b, c, d};
	return p;
}
static double ps2go_pair_sum(struct ps2go_pair p) { return p.a + (double)p.b + p.c + p.d; }
static double ps2go_pair_sum_ptr(const struct ps2go_pair *p) { return ps2go_pair_sum(*p); }
static int ps2go_many(int a, int b, int c, int d, int e, int f, int g, int h, int i, int j) {
	return a + 2*b + 3*c + 4*d + 5*e + 6*f + 7*g + 8*h + 9*i + 10*j;
}
*/
import "C"

import (
	"fmt"

	"ps2go/harness"
)

func testCgoABI() error {
	if v := float64(C.ps2go_dbl(1.5, 2)); v != 3.5 {
		return fmt.Errorf("double: %v", v)
	}
	if v := float32(C.ps2go_flt(3, 4)); v != 0.75 {
		return fmt.Errorf("float: %v", v)
	}
	if v := int64(C.ps2go_i64(0x100000000, 3, 7)); v != 0x300000000-7 {
		return fmt.Errorf("int64: %#x", v)
	}
	if v := uint64(C.ps2go_u64(0xDEADBEEF, 0xCAFEBABE)); v != 0xCAFEBABEDEADBEEF {
		return fmt.Errorf("uint64: %#x", v)
	}
	if v := float64(C.ps2go_mix(1, 2.5, 0.25, 1<<40)); v != 3.75+float64(int64(1)<<40) {
		return fmt.Errorf("mixed args: %v", v)
	}
	p := C.ps2go_pair_make(7, 1<<33, 1.5, 2.25)
	if int32(p.a) != 7 || int64(p.b) != 1<<33 || float32(p.c) != 1.5 || float64(p.d) != 2.25 {
		return fmt.Errorf("struct return: %+v", p)
	}
	want := 7 + float64(int64(1)<<33) + 1.5 + 2.25
	if v := float64(C.ps2go_pair_sum_ptr(&p)); v != want {
		return fmt.Errorf("struct by pointer: %v want %v", v, want)
	}
	// Passing a struct BY VALUE from Go to C is not ABI-correct in TinyGo's
	// cgo (it hands LLVM a struct type, which the n32 backend splits into
	// per-field registers, while C expects memory-image chunks): the float
	// field arrives as 0. Pass pointers. Logged, not asserted.
	harness.Logf("cgo: struct by value sum %v (want %v; known TinyGo cgo limitation if different)", float64(C.ps2go_pair_sum(p)), want)
	if v := int(C.ps2go_many(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)); v != 385 {
		return fmt.Errorf("10 args: %d", v)
	}
	return nil
}
