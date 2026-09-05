package gskit

/*
#define _EE
#include <stdlib.h>
#include <gsKit.h>
#include <gsToolkit.h>

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

// GSFont is a gsKit font (GSFONT) in C memory.
type GSFont struct {
	native *C.struct_gsFont
	raw    []byte // keeps the font data alive for the native struct
}

// InitFontFromMemory prepares a FNT font from its file contents (16-byte
// aligned). Upload it with FontUpload before use.
//
// Built by hand: gsKit_init_font_raw is declared in gsToolkit.h but not
// compiled into the SDK's libgskit_toolkit.
func InitFontFromMemory(data []byte) *GSFont {
	f := (*C.struct_gsFont)(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsFont{}))))
	f.Texture = (*C.struct_gsTexture)(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsTexture{}))))
	f.Type = GSKIT_FTYPE_FNT
	f.RawData = (*C.uchar)(unsafe.Pointer(&data[0]))
	f.RawSize = C.int(len(data))
	// The FNT upload writes the advance widths through Additional.
	f.Additional = (*C.short)(C.calloc(256, C.size_t(unsafe.Sizeof(C.short(0)))))
	return &GSFont{native: f, raw: data}
}

func FontUpload(g GSGlobal, font *GSFont) error {
	if ret := int(C.gsKit_font_upload(g.toNative(), font.native)); ret < 0 {
		return fmt.Errorf("gskit: gsKit_font_upload returned %d", ret)
	}
	return nil
}

func (f *GSFont) CharWidth() int  { return int(f.native.CharWidth) }
func (f *GSFont) CharHeight() int { return int(f.native.CharHeight) }

// AddSpacing changes the advance width of every character by delta pixels.
// Call it after FontUpload, which sets the widths.
func (f *GSFont) AddSpacing(delta int) {
	C.addSpacing(f.native, C.short(delta))
}

// Free releases the font's C memory (the texture's VRAM stays allocated).
func (f *GSFont) Free() {
	C.free(unsafe.Pointer(f.native.Additional))
	C.free(unsafe.Pointer(f.native.Texture))
	C.free(unsafe.Pointer(f.native))
	f.native = nil
	f.raw = nil
}

func FontPrint(g GSGlobal, font *GSFont, x, y float32, z int, scale float32, color uint64, text string) {
	cText := C.CString(text)
	C.gsKit_font_print_scaled(g.toNative(), font.native, C.float(x), C.float(y), C.int(z),
		C.float(scale), C.ulong(color), cText)
	C.free(unsafe.Pointer(cText))
}
