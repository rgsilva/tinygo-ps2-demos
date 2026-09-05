package main

/*
#define _EE
#include <gsKit.h>
#include <gsToolkit.h>

static unsigned long long rgbaq(unsigned char r, unsigned char g, unsigned char b, unsigned char a, unsigned char q) {
	return GS_SETREG_RGBAQ(r, g, b, a, q);
}
static unsigned long long alpha(unsigned char A, unsigned char B, unsigned char C, unsigned char D, unsigned short FIX) {
	return GS_SETREG_ALPHA(A, B, C, D, FIX);
}
*/
import "C"

import (
	"fmt"

	"ps2go/lib/gskit"
)

// testGsRegisters checks the Go register builders and constants against
// gsKit's macros.
func testGsRegisters() error {
	for _, c := range [][5]uint8{
		{0, 0, 0, 0, 0}, {0x80, 0x80, 0x80, 0x80, 0}, {0xFF, 0xFF, 0xFF, 0x80, 0}, {1, 2, 3, 4, 5}, {0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	} {
		want := uint64(C.rgbaq(C.uchar(c[0]), C.uchar(c[1]), C.uchar(c[2]), C.uchar(c[3]), C.uchar(c[4])))
		if got := gskit.GS_SETREG_RGBAQ(c[0], c[1], c[2], c[3], c[4]); got != want {
			return fmt.Errorf("RGBAQ%v: %#x, gsKit %#x", c, got, want)
		}
	}
	for _, c := range [][5]uint16{
		{0, 0, 0, 0, 0}, {0, 1, 0, 1, 0x80}, {1, 0, 2, 1, 0xFF}, {3, 3, 3, 3, 0xFFFF},
	} {
		want := uint64(C.alpha(C.uchar(c[0]), C.uchar(c[1]), C.uchar(c[2]), C.uchar(c[3]), C.ushort(c[4])))
		if got := gskit.GS_SETREG_ALPHA(uint8(c[0]), uint8(c[1]), uint8(c[2]), uint8(c[3]), c[4]); got != want {
			return fmt.Errorf("ALPHA%v: %#x, gsKit %#x", c, got, want)
		}
	}
	for _, k := range []struct {
		name      string
		got, want int
	}{
		{"GS_PSM_CT32", gskit.GS_PSM_CT32, C.GS_PSM_CT32},
		{"GS_PSM_CT24", gskit.GS_PSM_CT24, C.GS_PSM_CT24},
		{"GS_PSMZ_16S", gskit.GS_PSMZ_16S, C.GS_PSMZ_16S},
		{"GS_FILTER_NEAREST", gskit.GS_FILTER_NEAREST, C.GS_FILTER_NEAREST},
		{"GS_FILTER_LINEAR", gskit.GS_FILTER_LINEAR, C.GS_FILTER_LINEAR},
		{"GSKIT_ALLOC_SYSBUFFER", gskit.GSKIT_ALLOC_SYSBUFFER, C.GSKIT_ALLOC_SYSBUFFER},
		{"GSKIT_ALLOC_USERBUFFER", gskit.GSKIT_ALLOC_USERBUFFER, C.GSKIT_ALLOC_USERBUFFER},
		{"GSKIT_ALLOC_ERROR", gskit.GSKIT_ALLOC_ERROR, C.GSKIT_ALLOC_ERROR},
		{"GSKIT_FTYPE_FNT", gskit.GSKIT_FTYPE_FNT, C.GSKIT_FTYPE_FNT},
		{"GS_BLEND_FRONT2BACK", gskit.GS_BLEND_FRONT2BACK, C.GS_BLEND_FRONT2BACK},
		{"GS_BLEND_BACK2FRONT", gskit.GS_BLEND_BACK2FRONT, C.GS_BLEND_BACK2FRONT},
		{"GS_RENDER_QUEUE_OS_POOLSIZE", gskit.GS_RENDER_QUEUE_OS_POOLSIZE, C.GS_RENDER_QUEUE_OS_POOLSIZE},
		{"GS_RENDER_QUEUE_PER_POOLSIZE", gskit.GS_RENDER_QUEUE_PER_POOLSIZE, C.GS_RENDER_QUEUE_PER_POOLSIZE},
	} {
		if k.got != k.want {
			return fmt.Errorf("%s: %#x, gsKit %#x", k.name, k.got, k.want)
		}
	}
	return nil
}
