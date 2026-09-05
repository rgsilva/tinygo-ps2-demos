package main

/*
#define _EE
#include <stddef.h>
#include <gsKit.h>
#include <gsToolkit.h>

// Offsets as the C compiler lays the structs out.
static unsigned int off_global_PSM(void) { return offsetof(struct gsGlobal, PSM); }
static unsigned int off_global_PrimAlphaEnable(void) { return offsetof(struct gsGlobal, PrimAlphaEnable); }
static unsigned int off_global_PrimAlpha(void) { return offsetof(struct gsGlobal, PrimAlpha); }
static unsigned int off_global_dma_misc(void) { return offsetof(struct gsGlobal, dma_misc); }
static unsigned int size_global(void) { return sizeof(struct gsGlobal); }
static unsigned int off_tex_Vram(void) { return offsetof(struct gsTexture, Vram); }
static unsigned int size_tex(void) { return sizeof(struct gsTexture); }
static unsigned int off_font_Additional(void) { return offsetof(struct gsFont, Additional); }
static unsigned int size_font(void) { return sizeof(struct gsFont); }
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// testGsLayout checks that Go sees gsKit's structs the way the C compiler
// lays them out (gsGlobal has a 64-byte aligned field, which shifts
// everything after it).
func testGsLayout() error {
	var g C.struct_gsGlobal
	var t C.struct_gsTexture
	var f C.struct_gsFont
	for _, k := range []struct {
		name      string
		got, want uintptr
	}{
		{"gsGlobal.PSM", unsafe.Offsetof(g.PSM), uintptr(C.off_global_PSM())},
		{"gsGlobal.PrimAlphaEnable", unsafe.Offsetof(g.PrimAlphaEnable), uintptr(C.off_global_PrimAlphaEnable())},
		{"gsGlobal.PrimAlpha", unsafe.Offsetof(g.PrimAlpha), uintptr(C.off_global_PrimAlpha())},
		{"gsGlobal.dma_misc", unsafe.Offsetof(g.dma_misc), uintptr(C.off_global_dma_misc())},
		{"sizeof gsGlobal", unsafe.Sizeof(g), uintptr(C.size_global())},
		{"gsTexture.Vram", unsafe.Offsetof(t.Vram), uintptr(C.off_tex_Vram())},
		{"sizeof gsTexture", unsafe.Sizeof(t), uintptr(C.size_tex())},
		{"gsFont.Additional", unsafe.Offsetof(f.Additional), uintptr(C.off_font_Additional())},
		{"sizeof gsFont", unsafe.Sizeof(f), uintptr(C.size_font())},
	} {
		if k.got != k.want {
			return fmt.Errorf("%s: Go %d, C %d", k.name, k.got, k.want)
		}
	}
	return nil
}
