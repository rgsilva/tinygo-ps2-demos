package libpad

/*
#cgo LDFLAGS: -lpad

#define _EE
#include <stdlib.h>
#include <malloc.h>
#include <string.h>
#include <tamtypes.h>
#include <kernel.h>

#include <libpad.h>

struct padButtonStatus status;
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type (
	Pad struct {
		bufPtr unsafe.Pointer
		port   int
		slot   int
	}

	ReadResult struct {
		Up, Down, Left, Right           bool
		L1, L2, R1, R2                  bool
		Triangle, Circle, Cross, Square bool
		Select, Start                   bool
	}
)

func Init() {
	C.padInit(0)
}

func PortOpen(port int, slot int) Pad {
	// libpad requires the pad buffer to be 64-byte aligned (it is written by
	// the IOP through DMA); with a misaligned buffer padPortOpen fails and
	// the pad never becomes stable.
	bufPtr := unsafe.Pointer(C.memalign(64, 256))
	C.memset(bufPtr, 0, 256)

	if C.padPortOpen(C.int(port), C.int(slot), bufPtr) == 0 {
		panic(fmt.Sprintf("libpad: padPortOpen(%d, %d) failed", port, slot))
	}
	deadline := time.Now().Add(5 * time.Second)
	for int(C.padGetState(C.int(port), C.int(slot))) != PAD_STATE_STABLE {
		if time.Now().After(deadline) {
			panic(fmt.Sprintf("libpad: pad %d/%d did not become stable (state %d)", port, slot, int(C.padGetState(C.int(port), C.int(slot)))))
		}
	}

	return Pad{
		bufPtr: bufPtr,
		port:   port,
		slot:   slot,
	}
}

func (p *Pad) Read() ReadResult {
	r := ReadResult{}

	if C.int(C.padRead(C.int(p.port), C.int(p.slot), &C.status)) > 0 {
		r.Up = C.status.btns&PAD_UP == 0
		r.Down = C.status.btns&PAD_DOWN == 0
		r.Left = C.status.btns&PAD_LEFT == 0
		r.Right = C.status.btns&PAD_RIGHT == 0
		r.L1 = C.status.btns&PAD_L1 == 0
		r.L2 = C.status.btns&PAD_L2 == 0
		r.R1 = C.status.btns&PAD_R1 == 0
		r.R2 = C.status.btns&PAD_R2 == 0
		r.Triangle = C.status.btns&PAD_TRIANGLE == 0
		r.Circle = C.status.btns&PAD_CIRCLE == 0
		r.Cross = C.status.btns&PAD_CROSS == 0
		r.Square = C.status.btns&PAD_SQUARE == 0
		r.Start = C.status.btns&PAD_START == 0
		r.Select = C.status.btns&PAD_SELECT == 0
	}

	return r
}
