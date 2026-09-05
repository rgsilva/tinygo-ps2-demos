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
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type (
	Pad struct {
		buf    unsafe.Pointer            // 256 bytes, 64-byte aligned, written by the IOP
		status *C.struct_padButtonStatus // in C memory: padRead is called every frame
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

func Init() error {
	if C.padInit(0) != 1 {
		return fmt.Errorf("libpad: padInit failed")
	}
	return nil
}

// PortOpen opens a controller port and waits for the pad to become stable.
func PortOpen(port int, slot int) (*Pad, error) {
	// libpad requires the pad buffer to be 64-byte aligned (it is written by
	// the IOP through DMA); with a misaligned buffer padPortOpen fails and
	// the pad never becomes stable.
	buf := unsafe.Pointer(C.memalign(64, 256))
	C.memset(buf, 0, 256)

	if C.padPortOpen(C.int(port), C.int(slot), buf) == 0 {
		C.free(buf)
		return nil, fmt.Errorf("libpad: padPortOpen(%d, %d) failed", port, slot)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		state := int(C.padGetState(C.int(port), C.int(slot)))
		if state == PAD_STATE_STABLE {
			break
		}
		if time.Now().After(deadline) {
			C.padPortClose(C.int(port), C.int(slot))
			C.free(buf)
			return nil, fmt.Errorf("libpad: pad %d/%d did not become stable (state %d)", port, slot, state)
		}
		time.Sleep(time.Millisecond)
	}

	return &Pad{
		buf:    buf,
		status: (*C.struct_padButtonStatus)(C.calloc(1, C.size_t(unsafe.Sizeof(C.struct_padButtonStatus{})))),
		port:   port,
		slot:   slot,
	}, nil
}

// Read returns the buttons held down. ok is false when the pad had no data
// this frame (the result is then all released).
func (p *Pad) Read() (r ReadResult, ok bool) {
	if C.padRead(C.int(p.port), C.int(p.slot), p.status) == 0 {
		return r, false
	}
	btns := p.status.btns // active low
	r.Up = btns&PAD_UP == 0
	r.Down = btns&PAD_DOWN == 0
	r.Left = btns&PAD_LEFT == 0
	r.Right = btns&PAD_RIGHT == 0
	r.L1 = btns&PAD_L1 == 0
	r.L2 = btns&PAD_L2 == 0
	r.R1 = btns&PAD_R1 == 0
	r.R2 = btns&PAD_R2 == 0
	r.Triangle = btns&PAD_TRIANGLE == 0
	r.Circle = btns&PAD_CIRCLE == 0
	r.Cross = btns&PAD_CROSS == 0
	r.Square = btns&PAD_SQUARE == 0
	r.Start = btns&PAD_START == 0
	r.Select = btns&PAD_SELECT == 0
	return r, true
}
