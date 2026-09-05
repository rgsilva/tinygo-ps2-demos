package gskit

import "C"

/*
#define _EE
#include <stdlib.h>
#include <gsKit.h>
#include <gsToolkit.h>

extern float intToFloat(int i);

void resetAdditional(struct gsFont *font) {
	for (int i = 0; i < 256; i++) {
		font->Additional[i] -= 2;
	}
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type GSFont struct {
	native *C.struct_gsFont

	CharWidth uint32
}

func (g *GSFont) toNative() *C.struct_gsFont {
	return g.native
}

func InitFontFromMemory(ptr unsafe.Pointer, size int) GSFont {
	gsFont := (*C.struct_gsFont)(unsafe.Pointer(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsFont{})))))
	gsFont.Texture = (*C.struct_gsTexture)(unsafe.Pointer(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsTexture{})))))
	gsFont.RawData = (*C.uchar)(ptr)
	gsFont.RawSize = C.int(size)
	gsFont.Type = C.uchar(GSKIT_FTYPE_FNT)
	gsFont.Additional = (*C.short)(unsafe.Pointer(C.calloc(1, C.size_t(256*C.size_t(unsafe.Sizeof(C.short(0)))))))

	return GSFont{
		native: gsFont,
	}
}

func FontUpload(g GSGlobal, font GSFont) {
	ret := int(C.gsKit_font_upload(g.toNative(), font.toNative()))
	if ret < 0 {
		panic(fmt.Sprintf("gsKit_font_upload returned %d", ret))
	}
	font.CharWidth = uint32(font.native.CharWidth)
	C.resetAdditional(font.native)
}

func FontPrint(
	g GSGlobal,
	font GSFont,
	x, y, z int32,
	scale float32,
	color uint64,
	text string,
) {
	cText := C.CString(text)

	C.gsKit_font_print_scaled(
		g.toNative(),
		font.toNative(),
		C.intToFloat(C.int(x)), C.intToFloat(C.int(y)), C.int(z),
		C.float(scale),
		C.ulong(color),
		cText)

	C.free(unsafe.Pointer(cText))
}
