package gskit

/*
#cgo LDFLAGS: -lgskit_toolkit -lgskit -ldmakit -lpng -ljpeg -lz
#include <stdlib.h>
#define _EE
#include <gsKit.h>
#include <gsToolkit.h>

float intToFloat(int i) {
	return (float)i;
}
*/
import "C"

func InitGlobal() GSGlobal {
	gsGlobal := C.gsKit_init_global_custom(
		C.int(GS_RENDER_QUEUE_OS_POOLSIZE),
		C.int(GS_RENDER_QUEUE_PER_POOLSIZE))
	return GSGlobal{
		native:          gsGlobal,
		Width:           uint32(gsGlobal.Width),
		Height:          uint32(gsGlobal.Height),
		PSM:             int(gsGlobal.PSM),
		PSMZ:            int(gsGlobal.PSMZ),
		DoubleBuffering: bool(gsGlobal.DoubleBuffering == 0x01),
		ZBuffering:      bool(gsGlobal.ZBuffering == 0x01),
		PrimAlphaEnable: bool(gsGlobal.PrimAlphaEnable == 0x01),
	}
}

func InitScreen(g GSGlobal) {
	C.gsKit_init_screen(g.toNative())
}

func VRAMAlloc(g GSGlobal, size int, typ int) uint {
	return uint(C.gsKit_vram_alloc(g.toNative(), C.uint(size), C.uchar(typ)))
}

func SyncFlip(g GSGlobal) {
	C.gsKit_sync_flip(g.toNative())
}

func SetActive(g GSGlobal) {
	C.gsKit_setactive(g.toNative())
}

func Clear(gs GSGlobal, r, g, b, a, q byte) {
	C.gsKit_clear(gs.toNative(), C.ulonglong(GS_SETREG_RGBAQ(r, g, b, a, q)))
}

func QueueExec(g GSGlobal) {
	C.gsKit_queue_exec(g.toNative())
}
