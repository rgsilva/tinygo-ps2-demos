package main

// The EE memory map is fixed at link time: program | libc heap | Go heap |
// stack (see LIBC_HEAP in the Makefile). These cases check that the kernel,
// libc and the Go runtime agree on it, that the libc heap really stops where
// the link says, and that C-side allocation by gsKit keeps working next to
// the Go heap while the collector runs.

/*
#define _EE
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <kernel.h>

extern char _end;
extern char _heap_size; // absolute symbol: its "address" is the value

// The EE thread's stack, as the kernel set it up for crt0.
static int ps2go_thread_stack(unsigned int *base, unsigned int *size) {
	ee_thread_status_t st;
	if (ReferThreadStatus(GetThreadId(), &st) < 0) {
		return -1;
	}
	*base = (unsigned int)st.stack;
	*size = (unsigned int)st.stack_size;
	return 0;
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"ps2go/lib/dmakit"
	"ps2go/lib/fonts"
	"ps2go/lib/gskit"
	"ps2go/lib/harness"
)

// A 256x256 CT32 texture for the upload test, filled with a gradient at run
// time; 16-byte aligned because the GS reads it by DMA.
//
//go:align 16
var testTexture [256 * 256 * 4]byte

// The kernel keeps the top page of RAM and puts the EE stack below it.
const stackTop = 0x02000000 - 0x1000

func programEnd() uintptr   { return uintptr(unsafe.Pointer(&C._end)) }
func libcHeapSize() uintptr { return uintptr(unsafe.Pointer(&C._heap_size)) }
func libcBreak() uintptr    { return uintptr(C.sbrk(0)) }
func libcHeapEnd() uintptr  { return uintptr(C.EndOfHeap()) }

// guard is a Go heap buffer with a known pattern; C allocations must never
// touch it.
type guard struct{ buf []byte }

func newGuard(n int) *guard {
	g := &guard{buf: make([]byte, n)}
	for i := range g.buf {
		g.buf[i] = byte(i*7 + 3)
	}
	return g
}

func (g *guard) check() error {
	for i, b := range g.buf {
		if b != byte(i*7+3) {
			return fmt.Errorf("Go buffer corrupted at %#x", uintptr(unsafe.Pointer(&g.buf[i])))
		}
	}
	return nil
}

func testMemoryLayout() error {
	end, size, brk, eoh := programEnd(), libcHeapSize(), libcBreak(), libcHeapEnd()
	var base, ssize C.uint
	if C.ps2go_thread_stack(&base, &ssize) < 0 {
		return fmt.Errorf("ReferThreadStatus failed")
	}
	stack := uintptr(base)
	harness.Logf("program end %#x, libc heap %#x..%#x (link: %d KB), Go heap %#x..%#x (%d KB), stack %#x..%#x",
		end, brk, eoh, size/1024, runtime.HeapStart, runtime.HeapEnd, (runtime.HeapEnd-runtime.HeapStart)/1024,
		stack, stack+uintptr(ssize))
	switch {
	case end < 0x00100000:
		return fmt.Errorf("program end %#x inside the kernel's 1 MB", end)
	case brk < end || brk > eoh:
		return fmt.Errorf("libc break %#x outside [%#x, %#x)", brk, end, eoh)
	case eoh != end+size:
		return fmt.Errorf("kernel heap end %#x, link says %#x", eoh, end+size)
	case runtime.HeapStart%16 != 0 || runtime.HeapStart < eoh:
		return fmt.Errorf("Go heap start %#x below the libc heap end %#x", runtime.HeapStart, eoh)
	case runtime.HeapStart-eoh >= 16:
		return fmt.Errorf("%d bytes wasted between the heaps", runtime.HeapStart-eoh)
	case runtime.HeapEnd <= runtime.HeapStart || runtime.HeapEnd != stack || stack+uintptr(ssize) != stackTop:
		return fmt.Errorf("Go heap end %#x, stack %#x..%#x", runtime.HeapEnd, stack, stack+uintptr(ssize))
	}
	return nil
}

func testLibcHeapCap() error {
	g := newGuard(1 << 20)
	end, brk0, eoh := programEnd(), libcBreak(), libcHeapEnd()
	const chunk = 64 << 10
	var chunks []unsafe.Pointer
	for len(chunks) < 4096 {
		p := C.malloc(chunk)
		if p == nil {
			break
		}
		a := uintptr(p)
		// newlib may hand back a free chunk from below the current break.
		if a < end || a+chunk > eoh {
			return fmt.Errorf("malloc returned %#x, outside the libc heap [%#x, %#x)", a, end, eoh)
		}
		C.memset(p, 0xA5, chunk)
		chunks = append(chunks, p)
	}
	got := uintptr(len(chunks)) * chunk
	harness.Logf("malloc gave %d KB of the %d KB left before NULL", got/1024, (eoh-brk0)/1024)
	if len(chunks) == 4096 {
		return fmt.Errorf("malloc never failed")
	}
	if got < (eoh-brk0)*3/4 {
		return fmt.Errorf("only %d of %d bytes were allocatable", got, eoh-brk0)
	}
	if libcBreak() > eoh {
		return fmt.Errorf("libc break %#x passed the heap end %#x", libcBreak(), eoh)
	}
	if p := C.malloc(C.size_t(runtime.HeapEnd - runtime.HeapStart)); p != nil {
		return fmt.Errorf("a Go-heap-sized malloc succeeded at %#x", uintptr(p))
	}
	for _, p := range chunks {
		C.free(p)
	}
	if err := g.check(); err != nil {
		return err
	}
	runtime.GC()
	// libc still works after the failures, and never hands out Go memory.
	p := C.malloc(chunk)
	if p == nil || uintptr(p) >= runtime.HeapStart {
		return fmt.Errorf("malloc after free: %#x", uintptr(p))
	}
	C.free(p)
	return nil
}

// testGsKitAlloc drives gsKit the way the demos do (DMA, screen, a texture
// and a font, a few frames) while allocating Go garbage and collecting.
func testGsKitAlloc() error {
	g := newGuard(1 << 20)
	brk0, eoh := libcBreak(), libcHeapEnd()

	dmakit.Init(dmakit.D_CTRL_RELE_OFF, dmakit.D_CTRL_MFD_OFF, dmakit.D_CTRL_STS_UNSPEC,
		dmakit.D_CTRL_STD_OFF, dmakit.D_CTRL_RCYC_8, 1<<dmakit.DMA_CHANNEL_GIF)
	dmakit.ChannelInit(dmakit.DMA_CHANNEL_GIF)

	gs := gskit.InitGlobal()
	gs.SetPSM(gskit.GS_PSM_CT24)
	gs.SetPSMZ(gskit.GS_PSMZ_16S)
	gs.SetDoubleBuffering(true)
	gs.SetZBuffering(false)
	gs.SetPrimAlphaEnable(false)
	gskit.InitScreen(gs)
	if gs.PSM() != gskit.GS_PSM_CT24 || !gs.DoubleBuffering() || gs.Width() == 0 {
		return fmt.Errorf("gsGlobal settings not applied: psm %d db %v %dx%d", gs.PSM(), gs.DoubleBuffering(), gs.Width(), gs.Height())
	}

	tex := gskit.NewTexture(256, 256, gskit.GS_PSM_CT32, gskit.GS_FILTER_NEAREST)
	for i := range testTexture {
		testTexture[i] = byte(i >> 8)
	}
	if err := tex.Upload(gs, testTexture[:]); err != nil {
		return err
	}
	if tex.VRAM() == 0 {
		return fmt.Errorf("texture VRAM address is 0")
	}

	font := gskit.InitFontFromMemory(fonts.Arial)
	if err := gskit.FontUpload(gs, font); err != nil {
		return err
	}

	brk1 := libcBreak()
	var junk [][]byte
	for frame := 0; frame < 30; frame++ {
		gskit.Clear(gs, 0x20, 0x20, 0x80, 0x80, 0x00)
		gskit.PrimSpriteTexture3D(gs, tex, 0, 0, 1, 0, 0, 256, 256, 1, 256, 256,
			gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00))
		gskit.FontPrint(gs, font, 0, 0, 2, 1.0, gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0x00),
			fmt.Sprintf("frame %d", frame))
		gskit.QueueExec(gs)
		gskit.SyncFlip(gs)
		junk = append(junk, make([]byte, 64<<10))
		if frame%10 == 9 {
			junk = nil
			runtime.GC()
		}
	}
	brk2 := libcBreak()
	harness.Logf("gsKit took %d KB of libc heap for init, %d KB more over 30 frames; %d KB left",
		(brk1-brk0)/1024, (brk2-brk1)/1024, (eoh-brk2)/1024)
	switch {
	case brk1 <= brk0:
		return fmt.Errorf("gsKit allocated nothing from the libc heap")
	case brk2 > eoh:
		return fmt.Errorf("libc break %#x passed the heap end %#x", brk2, eoh)
	}
	return g.check()
}
