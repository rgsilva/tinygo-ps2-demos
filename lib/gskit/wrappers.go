package gskit

/*
#cgo LDFLAGS: -lgskit_toolkit -lgskit -ldmakit -lpng -ljpeg -lz
#include <stdlib.h>
#define _EE
#include <gsKit.h>
#include <gsToolkit.h>
*/
import "C"
import "fmt"

func InitGlobal() GSGlobal {
	return GSGlobal{native: C.gsKit_init_global_custom(
		C.int(GS_RENDER_QUEUE_OS_POOLSIZE),
		C.int(GS_RENDER_QUEUE_PER_POOLSIZE))}
}

func InitScreen(g GSGlobal) {
	C.gsKit_init_screen(g.toNative())
}

// VRAMAlloc allocates size bytes of VRAM (GSKIT_ALLOC_*) and returns the
// address. Call it after InitScreen, which takes the frame and Z buffers
// from the start of VRAM: after that no allocation can be at 0, which is
// gsKit's error value.
func VRAMAlloc(g GSGlobal, size int, typ int) (uint32, error) {
	addr := uint32(C.gsKit_vram_alloc(g.toNative(), C.uint(size), C.uchar(typ)))
	if addr == GSKIT_ALLOC_ERROR {
		return 0, fmt.Errorf("gskit: out of VRAM allocating %d bytes", size)
	}
	return addr, nil
}

// VRAMClear frees all VRAM allocations (gsKit's allocator is linear).
func VRAMClear(g GSGlobal) {
	C.gsKit_vram_clear(g.toNative())
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
