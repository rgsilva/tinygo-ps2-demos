package gskit

/*
#define _EE
#include <gsKit.h>
*/
import "C"

type GSGlobal struct {
	native *C.struct_gsGlobal

	// NOTE: this definitely does not include all the fields.
	Width           uint32
	Height          uint32
	PSM             int
	PSMZ            int
	DoubleBuffering bool
	ZBuffering      bool
	PrimAlphaEnable bool
	PrimAlpha       uint64
}

func (g *GSGlobal) toNative() *C.struct_gsGlobal {
	g.native.PSM = C.int(g.PSM)
	g.native.PSMZ = C.int(g.PSMZ)
	g.native.Width = C.int(g.Width)
	g.native.Height = C.int(g.Height)
	g.native.DoubleBuffering = boolToCuchar(g.DoubleBuffering)
	g.native.ZBuffering = boolToCuchar(g.ZBuffering)
	g.native.PrimAlphaEnable = boolToCint(g.PrimAlphaEnable)
	g.native.PrimAlpha = C.ulonglong(g.PrimAlpha)
	return g.native
}
