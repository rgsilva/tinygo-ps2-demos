package gskit

/*
#define _EE
#include <gsKit.h>
*/
import "C"

// GSGlobal is gsKit's global state (GSGLOBAL). Reads and writes go straight
// to the native struct, so nothing gsKit changes itself is lost.
type GSGlobal struct {
	native *C.struct_gsGlobal
}

func (g GSGlobal) toNative() *C.struct_gsGlobal { return g.native }

func (g GSGlobal) Width() int            { return int(g.native.Width) }
func (g GSGlobal) Height() int           { return int(g.native.Height) }
func (g GSGlobal) PSM() int              { return int(g.native.PSM) }
func (g GSGlobal) PSMZ() int             { return int(g.native.PSMZ) }
func (g GSGlobal) DoubleBuffering() bool { return g.native.DoubleBuffering != 0 }
func (g GSGlobal) ZBuffering() bool      { return g.native.ZBuffering != 0 }
func (g GSGlobal) PrimAlphaEnable() bool { return g.native.PrimAlphaEnable != 0 }
func (g GSGlobal) PrimAlpha() uint64     { return uint64(g.native.PrimAlpha) }

// The mode settings take effect at InitScreen.
func (g GSGlobal) SetWidth(w int)            { g.native.Width = C.int(w) }
func (g GSGlobal) SetHeight(h int)           { g.native.Height = C.int(h) }
func (g GSGlobal) SetPSM(psm int)            { g.native.PSM = C.int(psm) }
func (g GSGlobal) SetPSMZ(psmz int)          { g.native.PSMZ = C.int(psmz) }
func (g GSGlobal) SetDoubleBuffering(b bool) { g.native.DoubleBuffering = boolToCuchar(b) }
func (g GSGlobal) SetZBuffering(b bool)      { g.native.ZBuffering = boolToCuchar(b) }
func (g GSGlobal) SetPrimAlphaEnable(b bool) { g.native.PrimAlphaEnable = boolToCint(b) }
