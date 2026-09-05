package gskit

/*
#define _EE
#include <stdlib.h>
#include <gsKit.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// GSTexture is a gsKit texture (GSTEXTURE) in C memory.
type GSTexture struct {
	native *C.struct_gsTexture
	data   []byte // keeps the pixel data alive while the native struct points at it
}

// NewTexture allocates a texture of the given size, pixel format (GS_PSM_*)
// and filter (GS_FILTER_*), without a CLUT. Upload it before drawing.
func NewTexture(width, height, psm, filter int) *GSTexture {
	t := (*C.struct_gsTexture)(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsTexture{}))))
	t.Width = C.uint(width)
	t.Height = C.uint(height)
	t.PSM = C.uchar(psm)
	t.Filter = C.uint(filter)
	return &GSTexture{native: t}
}

func (t *GSTexture) Width() int   { return int(t.native.Width) }
func (t *GSTexture) Height() int  { return int(t.native.Height) }
func (t *GSTexture) PSM() int     { return int(t.native.PSM) }
func (t *GSTexture) VRAM() uint32 { return uint32(t.native.Vram) }

// Size is the texture's size in VRAM.
func (t *GSTexture) Size() int { return TextureSize(t.Width(), t.Height(), t.PSM()) }

// Upload sends the pixel data (raw, in the texture's format, 16-byte
// aligned) to VRAM, allocating the VRAM on the first call.
func (t *GSTexture) Upload(gs GSGlobal, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("gskit: empty texture data")
	}
	if t.native.Vram == 0 {
		vram, err := VRAMAlloc(gs, t.Size(), GSKIT_ALLOC_USERBUFFER)
		if err != nil {
			return err
		}
		t.native.Vram = C.uint(vram)
	}
	t.native.Mem = (*C.uint)(unsafe.Pointer(&data[0]))
	t.data = data
	C.gsKit_texture_upload(gs.toNative(), t.native)
	return nil
}

// Free releases the texture's C memory. Its VRAM stays allocated (see
// VRAMClear).
func (t *GSTexture) Free() {
	C.free(unsafe.Pointer(t.native))
	t.native = nil
	t.data = nil
}

func TextureSize(width, height, psm int) int {
	return int(C.gsKit_texture_size(C.int(width), C.int(height), C.int(psm)))
}

// PrimSpriteTexture3D draws the texture region (u1,v1)-(u2,v2) as the
// sprite (x1,y1)-(x2,y2) at depth iz, modulated by color (GS_SETREG_RGBAQ).
func PrimSpriteTexture3D(
	gs GSGlobal,
	tex *GSTexture,
	x1, y1 float32, iz1 int, u1, v1 float32,
	x2, y2 float32, iz2 int, u2, v2 float32,
	color uint64,
) {
	C.gsKit_prim_sprite_texture_3d(
		gs.toNative(),
		tex.native,
		C.float(x1), C.float(y1), C.int(iz1),
		C.float(u1), C.float(v1),
		C.float(x2), C.float(y2), C.int(iz2),
		C.float(u2), C.float(v2),
		C.ulonglong(color),
	)
}

// PrimSprite draws a flat-colored rectangle (x1,y1)-(x2,y2) at depth z.
func PrimSprite(gs GSGlobal, x1, y1, x2, y2 float32, z int, color uint64) {
	C.gsKit_prim_sprite(gs.toNative(), C.float(x1), C.float(y1), C.float(x2), C.float(y2), C.int(z), C.ulonglong(color))
}
