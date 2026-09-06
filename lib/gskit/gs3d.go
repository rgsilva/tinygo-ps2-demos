package gskit

/*
#define _EE
#include <gsKit.h>
#include <gsToolkit.h>
void ps2go_prim_list_stq(struct gsGlobal *gs, struct gsTexture *tex, int count, const void *vertices);
*/
import "C"

import (
	"math"
	"unsafe"
)

// GS register tags of the vertex words (GIF A+D mode).
const (
	tagRGBAQ = 0x01
	tagST    = 0x02
	tagXYZ2  = 0x05
)

// STQVertex is one vertex for PrimListTriangleTextureSTQ: three GS
// registers, each as its 64-bit value followed by its tag (gsKit's
// GSPRIMSTQPOINT, 48 bytes). Build it with the Vertex* helpers.
type STQVertex struct {
	RGBAQ, rgbaqTag uint64
	STQ, stqTag     uint64
	XYZ2, xyz2Tag   uint64
}

// VertexColor packs a color and the perspective divisor q (1/w) into the
// RGBAQ word.
func VertexColor(r, g, b, a uint8, q float32) uint64 {
	return uint64(r) | uint64(g)<<8 | uint64(b)<<16 | uint64(a)<<24 | uint64(math.Float32bits(q))<<32
}

// VertexSTQ packs perspective-correct texture coordinates: s = u*q,
// t = v*q with u, v in 0..1 across the texture.
func VertexSTQ(s, t float32) uint64 {
	return uint64(math.Float32bits(s)) | uint64(math.Float32bits(t))<<32
}

// VertexXYZ packs a screen position (pixels, the GS window offset added)
// and the 16-bit depth z (larger is nearer with the default test).
func (g GSGlobal) VertexXYZ(x, y float32, z uint32) uint64 {
	ix := clampXY(int32(x*16) + int32(g.native.OffsetX))
	iy := clampXY(int32(y*16) + int32(g.native.OffsetY))
	return uint64(uint16(ix)) | uint64(uint16(iy))<<16 | uint64(z)<<32
}

func clampXY(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v >= 4096*16 {
		return 4096*16 - 1
	}
	return v
}

// NewSTQVertex assembles a vertex from the three packed words.
func NewSTQVertex(rgbaq, stq, xyz2 uint64) STQVertex {
	return STQVertex{RGBAQ: rgbaq, rgbaqTag: tagRGBAQ, STQ: stq, stqTag: tagST, XYZ2: xyz2, xyz2Tag: tagXYZ2}
}

// PrimListTriangleTextureSTQ draws len(v)/3 textured, Gouraud-shaded
// triangles with perspective-correct texturing.
func PrimListTriangleTextureSTQ(gs GSGlobal, tex *GSTexture, v []STQVertex) {
	if len(v) < 3 {
		return
	}
	C.ps2go_prim_list_stq(gs.toNative(), tex.native, C.int(len(v)-len(v)%3), unsafe.Pointer(&v[0]))
}

func (g GSGlobal) OffsetX() int { return int(g.native.OffsetX) }
func (g GSGlobal) OffsetY() int { return int(g.native.OffsetY) }

// SetTexFilter sets the texture sampling filter (GS_FILTER_*) for the
// primitives drawn from now on.
func SetTexFilter(g GSGlobal, filter int) {
	C.gsKit_set_texfilter(g.toNative(), C.uchar(filter))
}
