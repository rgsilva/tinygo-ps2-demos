package libpad

/*

#define _EE
#include <stdlib.h>
#include <tamtypes.h>
#include <kernel.h>

#include <libpad.h>

struct padButtonStatus status;
*/
import "C"
import (
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
	bufPtr := unsafe.Pointer(C.calloc(1, 256))

	C.padPortOpen(C.int(port), C.int(slot), bufPtr)
	for int(C.padGetState(C.int(port), C.int(slot))) != PAD_STATE_STABLE {
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
