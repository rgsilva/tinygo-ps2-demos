// Package harness is the guest side of the PS2 test harness.
//
// A test ELF calls Run with a list of cases. Progress is reported on the EE
// serial port (SIO), which emulators write to their log, as lines of the form
//
//	PS2GO-BEGIN <cases>
//	PS2GO-CASE <name> RUN
//	PS2GO-CASE <name> PASS | FAIL <reason>
//	PS2GO-RESULT PASS|FAIL <passed>/<cases>
//
// and in a fixed-layout memory block (C symbol ps2go_harness_block) the host
// reads through the emulator's memory interface (PCSX2 PINE).
package harness

/*
#define _EE
#include <stdlib.h>
#include <kernel.h>
#include <sio.h>

// The block shared with the host, as a C global so the optimizer keeps it
// intact and the symbol name is plain. The host reads it through PINE after
// looking "ps2go_harness_block" up in the ELF symbol table. It lives outside
// the globals range the GC scans: its counters would look like heap pointers.
volatile unsigned int ps2go_harness_block[16] __attribute__((aligned(16), section(".ps2go.noscan")));

static void ps2go_block_set(int i, unsigned int v) { ps2go_harness_block[i] = v; }
static unsigned int ps2go_block_get(int i) { return ps2go_harness_block[i]; }
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Fields of the shared block, as indexes of 32-bit little-endian words. Keep
// in sync with BLOCK_FIELDS in harness/ps2test.py.
const (
	fMagic      = iota // "PS2G"
	fVersion           // layout version
	fState             // 0 booting, 1 running, 2 done
	fCases             // number of cases
	fPassed            //
	fFailed            //
	fCurrent           // 1-based index of the running case, 0 when none
	fHeapStart         //
	fHeapEnd           //
	fHeapInuse         //
	fHeapIdle          //
	fTotalAlloc        //
	fMallocs           //
	fFrees             //
	fXFailed           // cases that failed as expected (XFail set)
	fNumGC             // completed collections
)

const (
	Magic   = 0x47325350 // "PS2G" as a little-endian uint32
	Version = 2
)

func set(i int, v uint32) { C.ps2go_block_set(C.int(i), C.uint(v)) }
func get(i int) uint32    { return uint32(C.ps2go_block_get(C.int(i))) }

// Case is one test: Fn returns nil on success. A case known to fail on the
// current runtime carries the reason in XFail: its failure is reported as
// XFAIL and does not fail the run, and a pass is reported as XPASS and does,
// so the flag gets removed once the feature works.
type Case struct {
	Name  string
	Fn    func() error
	XFail string
}

var inited bool

// Init sets up the serial port. Run calls it; call it yourself only if you
// want to Log before Run.
func Init() {
	if inited {
		return
	}
	inited = true
	C.sio_init(38400, 0, 0, 0, 0)
}

// Log writes one line to the serial port.
func Log(s string) {
	cs := C.CString(s)
	C.sio_putsn(cs)
	C.free(unsafe.Pointer(cs))
	C.sio_putc('\n')
}

// Logf is Log with formatting.
func Logf(format string, args ...interface{}) {
	Log(fmt.Sprintf(format, args...))
}

// UpdateStats refreshes the runtime counters in the shared block.
func UpdateStats() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	set(fHeapStart, uint32(runtime.HeapStart))
	set(fHeapEnd, uint32(runtime.HeapEnd))
	set(fHeapInuse, uint32(ms.HeapInuse))
	set(fHeapIdle, uint32(ms.HeapIdle))
	set(fTotalAlloc, uint32(ms.TotalAlloc))
	set(fMallocs, uint32(ms.Mallocs))
	set(fFrees, uint32(ms.Frees))
	set(fNumGC, ms.NumGC)
}

// Run executes the cases in order, reports, and then parks the EE thread so
// the host can still inspect memory. It never returns.
func Run(cases []Case) {
	Init()
	set(fMagic, Magic)
	set(fVersion, Version)
	set(fCases, uint32(len(cases)))
	set(fState, 1)
	UpdateStats()
	Logf("PS2GO-BEGIN %d", len(cases))
	for i, c := range cases {
		set(fCurrent, uint32(i+1))
		Logf("PS2GO-CASE %s RUN", c.Name)
		err := runCase(c)
		switch {
		case err != nil && c.XFail != "":
			set(fXFailed, get(fXFailed)+1)
			Logf("PS2GO-CASE %s XFAIL %v (expected: %s)", c.Name, err, c.XFail)
		case err != nil:
			set(fFailed, get(fFailed)+1)
			Logf("PS2GO-CASE %s FAIL %v", c.Name, err)
		case c.XFail != "":
			set(fFailed, get(fFailed)+1)
			Logf("PS2GO-CASE %s XPASS unexpectedly passed, remove XFail (%s)", c.Name, c.XFail)
		default:
			set(fPassed, get(fPassed)+1)
			Logf("PS2GO-CASE %s PASS", c.Name)
		}
		UpdateStats()
	}
	set(fCurrent, 0)
	set(fState, 2)
	summary := fmt.Sprintf("%d/%d", get(fPassed), len(cases))
	if x := get(fXFailed); x != 0 {
		summary += fmt.Sprintf(" (%d expected failures)", x)
	}
	if get(fFailed) == 0 {
		Logf("PS2GO-RESULT PASS %s", summary)
	} else {
		Logf("PS2GO-RESULT FAIL %s", summary)
	}
	Park()
}

// Park halts the calling thread forever, keeping memory intact for the host.
func Park() {
	for {
		C.SleepThread()
	}
}

func runCase(c Case) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return c.Fn()
}
