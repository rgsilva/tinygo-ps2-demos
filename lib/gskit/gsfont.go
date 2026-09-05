package gskit

/*
#define _EE
#include <stdlib.h>
#include <gsKit.h>
#include <gsToolkit.h>

extern float intToFloat(int i);

static void addSpacing(struct gsFont *font, short delta) {
	for (int i = 0; i < 256; i++) {
		font->Additional[i] += delta;
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
	raw    []byte // keeps the font data alive for the native struct

	CharWidth uint32
}

func (g *GSFont) toNative() *C.struct_gsFont {
	return g.native
}

// InitFontFromMemory prepares a FNT font from its file contents (16-byte
// aligned). Upload it with FontUpload before use.
func InitFontFromMemory(data []byte) *GSFont {
	gsFont := (*C.struct_gsFont)(unsafe.Pointer(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsFont{})))))
	gsFont.Texture = (*C.struct_gsTexture)(unsafe.Pointer(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsTexture{})))))
	gsFont.RawData = (*C.uchar)(unsafe.Pointer(&data[0]))
	gsFont.RawSize = C.int(len(data))
	gsFont.Type = C.uchar(GSKIT_FTYPE_FNT)
	gsFont.Additional = (*C.short)(unsafe.Pointer(C.calloc(1, C.size_t(256*C.size_t(unsafe.Sizeof(C.short(0)))))))

	return &GSFont{
		native: gsFont,
		raw:    data,
	}
}

func FontUpload(g GSGlobal, font *GSFont) error {
	ret := int(C.gsKit_font_upload(g.toNative(), font.native))
	if ret < 0 {
		return fmt.Errorf("gskit: gsKit_font_upload returned %d", ret)
	}
	font.CharWidth = uint32(font.native.CharWidth)
	return nil
}

// AddSpacing changes the advance width of every character by delta pixels.
// Call it after FontUpload, which sets the widths.
func (g *GSFont) AddSpacing(delta int) {
	C.addSpacing(g.native, C.short(delta))
}

func FontPrint(
	g GSGlobal,
	font *GSFont,
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
