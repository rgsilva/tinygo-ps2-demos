package gskit

/*

#define _EE
#include <stdlib.h>
#include <gsKit.h>
*/
import "C"
import (
	"unsafe"
)

type GSTexture struct {
	native *C.struct_gsTexture

	Width  uint32
	Height uint32
	PSM    int
	Clut   unsafe.Pointer
	VRAM   uint
	Mem    unsafe.Pointer
	Filter uint32
}

func NewGSTexture() GSTexture {
	ptr := unsafe.Pointer(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_gsTexture{}))))
	return GSTexture{
		native: (*C.struct_gsTexture)(ptr),
	}
}

func (g *GSTexture) toNative() *C.struct_gsTexture {
	g.native.Width = C.uint(g.Width)
	g.native.Height = C.uint(g.Height)
	g.native.PSM = C.uchar(g.PSM)
	g.native.Clut = (*C.uint)(g.Clut)
	g.native.Vram = C.uint(g.VRAM)
	g.native.Mem = (*C.uint)(g.Mem)
	g.native.Filter = C.uint(g.Filter)
	return g.native
}

func TextureUpload(g GSGlobal, tex GSTexture) {
	C.gsKit_texture_upload(g.toNative(), tex.toNative())
}

func TextureSize(width, height uint32, psm int) int {
	return int(C.gsKit_texture_size(C.int(width), C.int(height), C.int(psm)))
}

func PrimSpriteTexture3D(
	gs GSGlobal,
	tex GSTexture,
	x1, y1 int32, iz1 int32, u1, v1 int32,
	x2, y2 int32, iz2 int32, u2, v2 int32,
	mask uint64,
) {
	C.gsKit_prim_sprite_texture_3d(
		gs.toNative(),
		tex.toNative(),
		C.intToFloat(C.int(x1)), C.intToFloat(C.int(y1)), C.int(iz1),
		C.intToFloat(C.int(u1)), C.intToFloat(C.int(v1)),
		C.intToFloat(C.int(x2)), C.intToFloat(C.int(y2)), C.int(iz2),
		C.intToFloat(C.int(u2)), C.intToFloat(C.int(v2)),
		C.ulonglong(mask),
	)
}
