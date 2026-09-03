package main

/*
#include <stdlib.h>
extern char _fdata[], _edata[], _fbss[], _end[], _ftext[];
static unsigned int sym_fdata(void) { return (unsigned int)_fdata; }
static unsigned int sym_edata(void) { return (unsigned int)_edata; }
static unsigned int sym_fbss(void)  { return (unsigned int)_fbss; }
static unsigned int sym_end(void)   { return (unsigned int)_end; }
static unsigned int sym_ftext(void) { return (unsigned int)_ftext; }
static unsigned int get_sp(void) { unsigned int v; __asm__ volatile("move %0, $sp" : "=r"(v)); return v; }

// A Go pointer stored only in C memory (see testCMemoryPointer).
static void **cslot;
static void cslot_alloc(void) { cslot = (void **)malloc(64); *cslot = 0; }
static void cslot_set(void *p) { *cslot = p; }
static void *cslot_get(void) { return *cslot; }
static unsigned int cslot_addr(void) { return (unsigned int)cslot; }
*/
import "C"

import (
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

import "ps2go/harness"

// ---------------------------------------------------------------------------
// Layout / sanity
// ---------------------------------------------------------------------------

func testLayout() error {
	sp := uintptr(C.get_sp())
	harness.Logf("layout: text=%#x data=%#x-%#x bss=%#x-%#x sp=%#x", uint32(C.sym_ftext()), uint32(C.sym_fdata()), uint32(C.sym_edata()), uint32(C.sym_fbss()), uint32(C.sym_end()), sp)
	harness.Logf("layout: heap=%#x-%#x (%d bytes) heapStart%%16=%d heapStart%%64=%d", runtime.HeapStart, runtime.HeapEnd, runtime.HeapEnd-runtime.HeapStart, runtime.HeapStart%16, runtime.HeapStart%64)
	if runtime.HeapStart == 0 {
		return fmt.Errorf("heap malloc failed (heapStart=0)")
	}
	if runtime.HeapStart%16 != 0 {
		return fmt.Errorf("heap start %#x not 16-byte aligned", runtime.HeapStart)
	}
	ms0 := logStats("layout: before GC")
	runtime.GC()
	ms1 := logStats("layout: after GC")
	if ms1.NumGC != ms0.NumGC+1 {
		return fmt.Errorf("NumGC not populated: %d -> %d", ms0.NumGC, ms1.NumGC)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 1. Correctness under pressure
// ---------------------------------------------------------------------------

type obj struct {
	id   int
	sum  uint32
	data []byte
	next *obj
}

var keptSmall []*obj

func verifyList(head *obj, want int, tag string) error {
	n := 0
	for p := head; p != nil; p = p.next {
		if checksum(p.data) != p.sum {
			return fmt.Errorf("%s: object %d corrupted", tag, p.id)
		}
		n++
	}
	if n != want {
		return fmt.Errorf("%s: %d survivors, want %d", tag, n, want)
	}
	return nil
}

func testSmallObjects() error {
	var head *obj
	kept := 0
	base := memstats()
	naturalGCs := uint32(0)
	// 300k small allocations, keep 1 in 50: 6000 survivors of 8..72 bytes.
	for i := 0; i < 300000; i++ {
		b := make([]byte, 8+(i*13)%64)
		fillSafe(b, i)
		if i%50 == 0 {
			head = &obj{id: i, sum: checksum(b), data: b, next: head}
			kept++
		} else {
			sink = b
		}
		if i%50000 == 49999 {
			ms := memstats()
			if ms.NumGC != base.NumGC+naturalGCs {
				// A natural collection happened: verify survivors.
				naturalGCs = ms.NumGC - base.NumGC
				if err := verifyList(head, kept, "after natural GC"); err != nil {
					return err
				}
			}
		}
	}
	sink = nil
	runtime.GC()
	if err := verifyList(head, kept, "after runtime.GC"); err != nil {
		return err
	}
	ms := logStats("small: end")
	harness.Logf("small: kept %d objects, natural GCs during run: %d", kept, ms.NumGC-base.NumGC)
	keptSmall = nil
	head = nil
	return nil
}

var keptLarge [][]byte
var keptLargeSums []uint32

func testLargeObjects() error {
	sizes := []int{4 * 1024, 20 * 1024, 70 * 1024, 100 * 1024, 300 * 1024, 1024 * 1024, 1024 * 1024}
	base := memstats()
	for round := 0; round < 8; round++ {
		for i, sz := range sizes {
			if sz >= 1024*1024 {
				r := heapFreeRuns()
				if r.largest < uintptr(sz)+16 {
					harness.Logf("large: round %d: no free run for %d bytes before GC (largest %d KB, free %d KB in %d runs); collecting", round, sz, r.largest/1024, r.total/1024, r.runs)
					runtime.GC()
					r = logFrag(fmt.Sprintf("large: round %d after GC", round))
					if r.largest < uintptr(sz)+16 {
						ms := logStats("large: FRAGMENTED")
						kept := len(keptLarge)
						keptLarge, keptLargeSums = nil, nil
						_ = kept
						return fmt.Errorf("round %d: cannot allocate %d bytes: idle=%d KB but largest free run=%d KB (%d objects live, %d kept)", round, sz, ms.HeapIdle/1024, r.largest/1024, ms.HeapObjects, kept)
					}
				}
			}
			b := make([]byte, sz)
			fillSafe(b, round*100+i)
			if round%2 == 0 {
				keptLarge = append(keptLarge, b)
				keptLargeSums = append(keptLargeSums, checksum(b))
			} else {
				sink = b
			}
		}
		churn(2 * 1024 * 1024)
		if round%2 == 1 {
			runtime.GC()
			ms := logStats(fmt.Sprintf("large: round %d after GC (no stack scrub)", round))
			scrubStack(20)
			runtime.GC()
			ms2 := logStats(fmt.Sprintf("large: round %d after GC (stack scrubbed)", round))
			harness.Logf("large: round %d: scrub-sensitive retention = %d KB / %d objects; kept = %d objects / %d KB", round, (int64(ms.HeapInuse)-int64(ms2.HeapInuse))/1024, int64(ms.HeapObjects)-int64(ms2.HeapObjects), len(keptLarge), sumLens(keptLarge)/1024)
		}
		logFrag(fmt.Sprintf("large: end of round %d", round))
		for i, b := range keptLarge {
			if checksum(b) != keptLargeSums[i] {
				return fmt.Errorf("round %d: large object %d (%d bytes) corrupted", round, i, len(b))
			}
		}
	}
	sink = nil
	ms := logStats("large: end")
	harness.Logf("large: kept %d objects totalling %d bytes, GCs: %d", len(keptLarge), sumLens(keptLarge), ms.NumGC-base.NumGC)
	keptLarge = nil
	keptLargeSums = nil
	runtime.GC()
	logStats("large: after release")
	logFrag("large: after release")
	return nil
}

func sumLens(bs [][]byte) int {
	n := 0
	for _, b := range bs {
		n += len(b)
	}
	return n
}

var keptMap map[string][]byte

func testMapsStrings() error {
	keptMap = map[string][]byte{}
	base := memstats()
	for round := 0; round < 40; round++ {
		for i := 0; i < 500; i++ {
			k := fmt.Sprintf("key-%d-%s", i, strings.Repeat("x", i%17))
			v := []byte(strings.Repeat(string(rune('a'+i%26)), 10+i%200))
			if i%3 == 0 {
				keptMap[k] = v
			} else {
				delete(keptMap, k)
			}
			s := strings.ToUpper(k) + strings.Repeat("!", i%50)
			sinkInt += len(s)
		}
		if round%10 == 9 {
			runtime.GC()
		}
		// Verify map contents.
		for k, v := range keptMap {
			var i int
			var rest string
			if _, err := fmt.Sscanf(k, "key-%d-%s", &i, &rest); err != nil {
				// keys without the x-suffix have no trailing field
				if _, err2 := fmt.Sscanf(k, "key-%d-", &i); err2 != nil {
					return fmt.Errorf("round %d: bad key %q", round, k)
				}
			}
			want := strings.Repeat(string(rune('a'+i%26)), 10+i%200)
			if string(v) != want {
				return fmt.Errorf("round %d: key %q value corrupted (len %d want %d)", round, k, len(v), len(want))
			}
		}
	}
	ms := logStats("maps: end")
	harness.Logf("maps: %d entries live, GCs: %d", len(keptMap), ms.NumGC-base.NumGC)
	keptMap = nil
	return nil
}

var keptPtrs [][]*obj

func testSliceOfPointers() error {
	base := memstats()
	for round := 0; round < 10; round++ {
		s := make([]*obj, 0, 100)
		for i := 0; i < 1000; i++ {
			b := make([]byte, 32+(i*7)%500)
			fillSafe(b, round*1000+i)
			o := &obj{id: round*1000 + i, sum: checksum(b), data: b}
			if i%10 == 0 {
				s = append(s, o)
			}
		}
		keptPtrs = append(keptPtrs, s)
		churn(1024 * 1024)
		runtime.GC()
		for _, s := range keptPtrs {
			for _, o := range s {
				if checksum(o.data) != o.sum {
					return fmt.Errorf("round %d: object %d corrupted", round, o.id)
				}
			}
		}
	}
	ms := logStats("ptrslices: end")
	harness.Logf("ptrslices: %d slices kept, GCs: %d", len(keptPtrs), ms.NumGC-base.NumGC)
	keptPtrs = nil
	return nil
}

// Interior pointers: the only reference is into the middle of the object.
var (
	gInterior    unsafe.Pointer // raw pointer into the middle of a 64 KB object
	gInteriorSub []byte         // sub-slice of another object
	gInteriorEnd []byte         // zero-length slice at the very end of an object
	gInteriorSum uint32
	gInteriorSum2 uint32
	gInteriorEndAddr uintptr
)

//go:noinline
func setupInterior() {
	b := make([]byte, 64*1024)
	fillSafe(b, 7)
	gInteriorSum = checksum(b[40000:41000])
	gInterior = unsafe.Pointer(&b[40000])
	c := make([]byte, 5000)
	fillSafe(c, 9)
	gInteriorSum2 = checksum(c[1000:4000])
	gInteriorSub = c[1000:4000]
	d := make([]byte, 4088) // 4088+8 header = exactly 256 blocks: end pointer lands on the header
	fillSafe(d, 11)
	gInteriorEnd = d[len(d):]
	gInteriorEndAddr = uintptr(unsafe.Pointer(&d[0]))
	sink = nil
}

func testInteriorPointers() error {
	setupInterior()
	scrubStack(20)
	runtime.GC()
	churn(3 * 1024 * 1024)
	runtime.GC()
	churn(3 * 1024 * 1024)
	view := unsafe.Slice((*byte)(gInterior), 1000)
	if checksum(view) != gInteriorSum {
		return fmt.Errorf("64K object reachable only via interior unsafe.Pointer was corrupted")
	}
	if checksum(gInteriorSub) != gInteriorSum2 {
		return fmt.Errorf("object reachable only via sub-slice was corrupted")
	}
	// Check whether the object referenced only by an end pointer survived
	// by looking at its content through the recorded address.
	end := unsafe.Slice((*byte)(unsafe.Pointer(gInteriorEndAddr)), 4088)
	sum := checksum(end)
	want := make([]byte, 4088)
	fillSafe(want, 11)
	harness.Logf("interior: end-pointer object (%#x, len 4088) survived: %v", gInteriorEndAddr, sum == checksum(want))
	gInterior = nil
	gInteriorSub = nil
	gInteriorEnd = nil
	return nil
}

// Pointers kept only in globals. gDataSlot is initialised so it lives in
// .data; gBssSlot is zero-initialised so it lives in .bss.
var gDataSlot = []byte{1, 2, 3}
var gBssSlot []byte
var gDataPtr *obj = &obj{id: -1}
var gBssPtr *obj

//go:noinline
func setupGlobals() (uint32, uint32) {
	a := make([]byte, 3000)
	fillSafe(a, 21)
	gDataSlot = a
	b := make([]byte, 3000)
	fillSafe(b, 22)
	gBssSlot = b
	c := make([]byte, 700)
	fillSafe(c, 23)
	gDataPtr = &obj{id: 1, sum: checksum(c), data: c}
	d := make([]byte, 700)
	fillSafe(d, 24)
	gBssPtr = &obj{id: 2, sum: checksum(d), data: d}
	return checksum(a), checksum(b)
}

func testGlobalsOnly() error {
	fd, ed, fb, en := uintptr(C.sym_fdata()), uintptr(C.sym_edata()), uintptr(C.sym_fbss()), uintptr(C.sym_end())
	in := func(p uintptr, lo, hi uintptr) bool { return p >= lo && p < hi }
	addrData := uintptr(unsafe.Pointer(&gDataSlot))
	addrBss := uintptr(unsafe.Pointer(&gBssSlot))
	harness.Logf("globals: &gDataSlot=%#x in .data=%v; &gBssSlot=%#x in .bss=%v; &gDataPtr=%#x in .data=%v; &gBssPtr=%#x in .bss=%v",
		addrData, in(addrData, fd, ed), addrBss, in(addrBss, fb, en),
		uintptr(unsafe.Pointer(&gDataPtr)), in(uintptr(unsafe.Pointer(&gDataPtr)), fd, ed),
		uintptr(unsafe.Pointer(&gBssPtr)), in(uintptr(unsafe.Pointer(&gBssPtr)), fb, en))
	sa, sb := setupGlobals()
	scrubStack(20)
	runtime.GC()
	churn(3 * 1024 * 1024)
	runtime.GC()
	churn(3 * 1024 * 1024)
	if checksum(gDataSlot) != sa {
		return fmt.Errorf(".data slice corrupted")
	}
	if checksum(gBssSlot) != sb {
		return fmt.Errorf(".bss slice corrupted")
	}
	if checksum(gDataPtr.data) != gDataPtr.sum || gDataPtr.id != 1 {
		return fmt.Errorf(".data pointer object corrupted")
	}
	if checksum(gBssPtr.data) != gBssPtr.sum || gBssPtr.id != 2 {
		return fmt.Errorf(".bss pointer object corrupted")
	}
	gDataSlot, gBssSlot, gDataPtr, gBssPtr = nil, nil, nil, nil
	return nil
}

// Pointers kept only on the stack across deep recursion.
//
//go:noinline
func recurse(depth, max int) error {
	var pad [96]uint32 // ~400 bytes of frame per level
	b := make([]byte, 100+depth*3)
	fillSafe(b, depth)
	sum := checksum(b)
	o := &obj{id: depth, sum: sum, data: b}
	for i := range pad {
		pad[i] = uint32(depth) * 0x01010101
	}
	if depth < max {
		if err := recurse(depth+1, max); err != nil {
			return err
		}
	} else {
		sp := uintptr(C.get_sp())
		harness.Logf("stack: at depth %d, sp=%#x (%d bytes used of 128K)", depth, sp, 0x02000000-sp)
		runtime.GC()
		churn(4 * 1024 * 1024) // forces at least one natural GC? (heap is 24 MB, so not necessarily)
		runtime.GC()
		churn(2 * 1024 * 1024)
	}
	if checksum(o.data) != o.sum || o.id != depth || checksum(b) != sum {
		return fmt.Errorf("stack-held object at depth %d corrupted", depth)
	}
	sinkInt += int(pad[depth%96])
	return nil
}

func testStackDeep() error {
	return recurse(0, 120)
}

var keptClosures []func() (int, uint32)

func testClosures() error {
	for i := 0; i < 300; i++ {
		b := make([]byte, 50+i*3)
		fillSafe(b, i)
		id := i
		want := checksum(b)
		keptClosures = append(keptClosures, func() (int, uint32) {
			if checksum(b) != want {
				return -1, 0
			}
			return id, want
		})
		churn(20000)
	}
	scrubStack(20)
	runtime.GC()
	churn(3 * 1024 * 1024)
	runtime.GC()
	for i, f := range keptClosures {
		id, _ := f()
		if id != i {
			return fmt.Errorf("closure %d: captured object corrupted (got %d)", i, id)
		}
	}
	keptClosures = nil
	return nil
}

// Natural collections only: never call runtime.GC(); allocate until the
// collector ran a few times and verify survivors after each.
func testNaturalGC() error {
	var head *obj
	kept := 0
	base := memstats()
	last := base.NumGC
	total := 0
	for last-base.NumGC < 3 {
		for i := 0; i < 2000; i++ {
			b := make([]byte, 64+(i*41)%4000)
			fillSafe(b, i)
			total += len(b)
			if i%100 == 0 {
				head = &obj{id: kept, sum: checksum(b), data: b, next: head}
				kept++
			} else {
				sink = b
			}
		}
		ms := memstats()
		if ms.NumGC != last {
			last = ms.NumGC
			if err := verifyList(head, kept, "natural"); err != nil {
				return err
			}
			harness.Logf("natural: GC #%d after %d bytes allocated; inuse=%d kept=%d", last, total, ms.HeapInuse, kept)
		}
		if total > 400*1024*1024 {
			return fmt.Errorf("no natural GC after %d bytes", total)
		}
	}
	sink = nil
	return nil
}

// Leak detection: repeated cycles of allocate/drop/GC; heap in use must
// return to a baseline.
//go:noinline
func leakCycleBody(cycle int) int {
	m := map[int][]byte{}
	for i := 0; i < 3000; i++ {
		m[i] = make([]byte, 16+(i*53)%700)
	}
	var ss []string
	for i := 0; i < 2000; i++ {
		ss = append(ss, fmt.Sprintf("s%d-%s", i, strings.Repeat("y", i%30)))
	}
	var big [][]byte
	for i := 0; i < 8; i++ {
		big = append(big, make([]byte, 256*1024))
	}
	return len(m) + len(ss) + len(big)
}

func leakCycles(viaCall bool) error {
	scrubStack(10)
	runtime.GC()
	base := logStats("leak: baseline")
	var worst int64
	for cycle := 0; cycle < 8; cycle++ {
		if ms := memstats(); ms.HeapIdle < 6*1024*1024 {
			return fmt.Errorf("stopping before cycle %d: only %d KB idle (retained %d KB)", cycle, ms.HeapIdle/1024, (int64(ms.HeapInuse)-int64(base.HeapInuse))/1024)
		}
		if viaCall {
			sinkInt += leakCycleBody(cycle)
		} else {
			m := map[int][]byte{}
			for i := 0; i < 3000; i++ {
				m[i] = make([]byte, 16+(i*53)%700)
			}
			var ss []string
			for i := 0; i < 2000; i++ {
				ss = append(ss, fmt.Sprintf("s%d-%s", i, strings.Repeat("y", i%30)))
			}
			var big [][]byte
			for i := 0; i < 8; i++ {
				big = append(big, make([]byte, 256*1024))
			}
			sinkInt += len(m) + len(ss) + len(big)
			m, ss, big = nil, nil, nil
		}
		scrubStack(10)
		runtime.GC()
		ms := memstats()
		d := int64(ms.HeapInuse) - int64(base.HeapInuse)
		harness.Logf("leak: cycle %d inuse=%d (delta %+d KB) objects=%d", cycle, ms.HeapInuse, d/1024, ms.HeapObjects)
		if d > worst {
			worst = d
		}
	}
	if worst > 64*1024 {
		return fmt.Errorf("heap in use grew by %d KB over baseline", worst/1024)
	}
	return nil
}

// Leak detection: repeated cycles of allocate/drop/GC in the same frame; heap
// in use must return to a baseline.
func testLeakCycles() error { return leakCycles(false) }

// Same, but the allocations happen inside a function that has returned.
func testLeakCyclesViaCall() error { return leakCycles(true) }

// Which "drop" patterns actually release memory.
//go:noinline
func buildMap(n int) map[int][]byte {
	m := map[int][]byte{}
	for i := 0; i < n; i++ {
		m[i] = make([]byte, 200)
	}
	return m
}

//go:noinline
func buildAndDropMap(n int) int {
	m := buildMap(n)
	return len(m)
}

func testDropPatterns() error {
	scrubStack(10)
	runtime.GC()
	base := memstats()
	report := func(tag string) {
		scrubStack(10)
		runtime.GC()
		ms := memstats()
		harness.Logf("drop[%s]: retained %d KB / %d objects", tag, (int64(ms.HeapInuse)-int64(base.HeapInuse))/1024, int64(ms.HeapObjects)-int64(base.HeapObjects))
	}
	// A: build in this frame, set to nil, GC.
	m := buildMap(2000)
	sinkInt += len(m)
	m = nil
	report("same-frame nil")
	// B: build and drop inside a callee that returned.
	sinkInt += buildAndDropMap(2000)
	report("dropped in callee")
	// C: build in this frame, overwrite the variable with a new small map.
	m2 := buildMap(2000)
	sinkInt += len(m2)
	m2 = buildMap(1)
	sinkInt += len(m2)
	report("same-frame overwrite")
	// D: keep a value on the stack in a global slot, then clear the global.
	gBssSlot = make([]byte, 512*1024)
	gBssSlot[0] = 1
	gBssSlot = nil
	report("global nil")
	return nil
}

// ---------------------------------------------------------------------------
// 3. Finalizers
// ---------------------------------------------------------------------------

var finalized int
var finalizedID int
var finalizerAllocOK bool

//go:noinline
func makeFinalizable(id int) {
	o := &obj{id: id, data: make([]byte, 100)}
	runtime.SetFinalizer(o, func(p *obj) {
		finalized++
		finalizedID = p.id
		// Allocating inside a finalizer must be legal.
		x := make([]byte, 1000)
		x[0] = 1
		finalizerAllocOK = x[0] == 1
	})
	sink = o.data
	sink = nil
}

//go:noinline
func makeCleared(id int) {
	o := &obj{id: id}
	runtime.SetFinalizer(o, func(p *obj) { finalized += 100 })
	runtime.SetFinalizer(o, nil)
}

func testFinalizers() error {
	makeFinalizable(42)
	makeCleared(43)
	scrubStack(20)
	if finalized != 0 {
		return fmt.Errorf("finalizer ran before GC")
	}
	runtime.GC()
	harness.Logf("finalizer: after 1st GC: ran=%d id=%d allocOK=%v", finalized, finalizedID, finalizerAllocOK)
	runtime.GC()
	churn(1024 * 1024)
	runtime.GC()
	harness.Logf("finalizer: after 3 GCs: ran=%d", finalized)
	if finalized == 0 {
		return fmt.Errorf("finalizer never ran")
	}
	if finalized != 1 || finalizedID != 42 {
		return fmt.Errorf("finalizer ran %d times (id %d), want once for 42", finalized, finalizedID)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 4. GC cost
// ---------------------------------------------------------------------------

var costKeep [][]byte
var costKeepPtrs []*obj

//go:noinline
func costEmpty() {
	runtime.GC()
	empty := memstats()
	mn, av := timeGC(5)
	harness.Logf("gccost: empty heap (inuse=%d): min %v avg %v", empty.HeapInuse, mn, av)
}

//go:noinline
func costOneMB() {
	costKeep = [][]byte{make([]byte, 1024*1024)}
	fillSafe(costKeep[0], 1)
	mn, av := timeGC(5)
	harness.Logf("gccost: 1 MB live (1 x 1MB []byte): min %v avg %v", mn, av)
	costKeep = nil
}

//go:noinline
func costSmall(n int) {
	costKeep = make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		costKeep = append(costKeep, make([]byte, 64))
	}
	runtime.GC()
	ms := memstats()
	mn, av := timeGC(3)
	harness.Logf("gccost: %d KB live (%d x 64B []byte, %d objects): min %v avg %v", ms.HeapInuse/1024, n, ms.HeapObjects, mn, av)
	costKeep = nil
}

//go:noinline
func costLinked(n int) {
	costKeepPtrs = make([]*obj, 0, n)
	for i := 0; i < n; i++ {
		o := &obj{id: i, data: make([]byte, 16)}
		if i > 0 {
			o.next = costKeepPtrs[i-1]
		}
		costKeepPtrs = append(costKeepPtrs, o)
	}
	runtime.GC()
	ms := memstats()
	mn, av := timeGC(5)
	harness.Logf("gccost: %d KB live (%d linked objs, %d objects): min %v avg %v", ms.HeapInuse/1024, n, ms.HeapObjects, mn, av)
	costKeepPtrs = nil
}

//go:noinline
func costBig(n int) {
	costKeep = make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		b := make([]byte, 1024*1024)
		fillSafe(b, i)
		costKeep = append(costKeep, b)
	}
	runtime.GC()
	ms := memstats()
	mn, av := timeGC(5)
	harness.Logf("gccost: %d KB live (%d x 1MB []byte): min %v avg %v", ms.HeapInuse/1024, n, mn, av)
	costKeep = nil
}

func testGCCost() error {
	costEmpty()
	scrubStack(10)
	costOneMB()
	scrubStack(10)
	runtime.GC()
	logStats("gccost: after 1MB release")
	costSmall(16384)
	scrubStack(10)
	runtime.GC()
	costLinked(20000)
	scrubStack(10)
	runtime.GC()
	logStats("gccost: after linked release")
	costBig(8)
	scrubStack(10)
	runtime.GC()
	logStats("gccost: after 8MB release")
	costSmall(65536)
	scrubStack(10)
	runtime.GC()
	logStats("gccost: after 65536 release")

	// Stack scanning cost: GC from a shallow stack vs. from ~100 KB deep.
	mnShallow, _ := timeGC(5)
	deep := gcAtDepth(0, 240) // 240 x ~420 B = ~100 KB
	harness.Logf("gccost: stack scan: shallow %v, with ~100 KB of stack in use %v (delta %v)", mnShallow, deep, deep-mnShallow)
	return nil
}

//go:noinline
func gcAtDepth(depth, max int) time.Duration {
	var pad [96]uint32
	for i := range pad {
		pad[i] = 0x02000000 - uint32(i) // non-heap values
	}
	if depth < max {
		d := gcAtDepth(depth+1, max)
		sinkInt += int(pad[depth%96])
		return d
	}
	sp := uintptr(C.get_sp())
	harness.Logf("gccost: sp at depth %d = %#x (%d bytes of stack in use)", depth, sp, 0x02000000-sp)
	mn, _ := timeGC(5)
	sinkInt += int(pad[depth%96])
	return mn
}

// ---------------------------------------------------------------------------
// 5. Go pointer stored only in C memory
// ---------------------------------------------------------------------------

//go:noinline
func stashInC() uint32 {
	b := make([]byte, 4096)
	fillSafe(b, 77)
	C.cslot_set(unsafe.Pointer(&b[0]))
	return checksum(b)
}

func testCMemoryPointer() error {
	C.cslot_alloc()
	harness.Logf("cmem: C slot at %#x (outside Go heap %#x-%#x: %v)", uint32(C.cslot_addr()), runtime.HeapStart, runtime.HeapEnd,
		uintptr(C.cslot_addr()) < runtime.HeapStart || uintptr(C.cslot_addr()) >= runtime.HeapEnd)
	want := stashInC()
	scrubStack(20)
	before := memstats()
	runtime.GC()
	after := memstats()
	p := C.cslot_get()
	view := unsafe.Slice((*byte)(p), 4096)
	sumAfterGC := checksum(view)
	churn(6 * 1024 * 1024)
	runtime.GC()
	churn(6 * 1024 * 1024)
	sumAfterChurn := checksum(view)
	harness.Logf("cmem: objects before GC %d, after %d; intact after GC: %v; intact after churn: %v",
		before.HeapObjects, after.HeapObjects, sumAfterGC == want, sumAfterChurn == want)
	if sumAfterChurn == want && after.HeapObjects == before.HeapObjects {
		return fmt.Errorf("object referenced only from C memory was NOT collected (unexpected)")
	}
	harness.Log("cmem: as expected, the object referenced only from C memory was collected and its memory reused")
	return nil
}


// ---------------------------------------------------------------------------
// False retention: how much dead pointer-free data survives a GC depending on
// the byte pattern it holds. The Go heap occupies 0x0016xxxx-0x0196xxxx, so any
// 32-bit word whose high byte is 0x00 or 0x01 looks like a heap pointer.
// ---------------------------------------------------------------------------

//go:noinline
func makeGarbage(n, size int, pattern int) {
	for i := 0; i < n; i++ {
		b := make([]byte, size)
		switch pattern {
		case 0: // zeros: never a pointer
		case 1: // high bit set in every byte: never a pointer
			fillSafe(b, i)
		case 2: // sequential bytes (0.5% of words look like heap pointers)
			fill(b, i)
		case 3: // "RGBA-like" 0x00RRGGBB words: high byte 0
			for j := 0; j+3 < len(b); j += 4 {
				b[j], b[j+1], b[j+2], b[j+3] = byte(j), byte(j>>8), byte(j>>16), 0
			}
		case 4: // small ints (0..1000): never in the heap range
			for j := 0; j+3 < len(b); j += 4 {
				b[j], b[j+1], b[j+2], b[j+3] = byte(j/4%1000), byte(j/4%1000>>8), 0, 0
			}
		}
		sink = b
	}
	sink = nil
}

func testFalseRetention() error {
	names := []string{"zeros", "high-bit-set", "sequential-bytes", "0x00RRGGBB-words", "small-ints"}
	for pat, name := range names {
		scrubStack(20)
		runtime.GC()
		base := memstats()
		// 8 x 1 MB and 2000 x 2 KB of garbage in this pattern.
		makeGarbage(8, 1024*1024, pat)
		makeGarbage(2000, 2048, pat)
		scrubStack(20)
		ms0 := memstats()
		runtime.GC()
		ms1 := memstats()
		runtime.GC()
		ms2 := memstats()
		harness.Logf("retention[%s]: allocated %d KB dead; after GC1 inuse=%d KB (retained %d KB, %d objs), after GC2 %d KB (%d objs)",
			name, (int64(ms0.HeapInuse)-int64(base.HeapInuse))/1024, ms1.HeapInuse/1024, (int64(ms1.HeapInuse)-int64(base.HeapInuse))/1024, int64(ms1.HeapObjects)-int64(base.HeapObjects),
			ms2.HeapInuse/1024, int64(ms2.HeapObjects)-int64(base.HeapObjects))
	}
	return nil
}


// ---------------------------------------------------------------------------
// False roots: words in the conservatively scanned ranges (globals and the
// live stack) whose value falls inside the Go heap.
// ---------------------------------------------------------------------------

func scanRangeForHeapWords(tag string, lo, hi uintptr, maxList int) int {
	n := 0
	listed := 0
	for a := lo; a+4 <= hi; a += 4 {
		v := *(*uintptr)(unsafe.Pointer(a))
		if v >= runtime.HeapStart && v < runtime.HeapEnd {
			n++
			if listed < maxList {
				harness.Logf("falseroot[%s]: addr=%#x value=%#x (heap+%d)", tag, a, v, v-runtime.HeapStart)
				listed++
			}
		}
	}
	return n
}

func testFalseRoots() error {
	runtime.GC()
	fd, en := uintptr(C.sym_fdata()), uintptr(C.sym_end())
	ng := scanRangeForHeapWords("globals", fd, en, 60)
	sp := uintptr(C.get_sp())
	ns := scanRangeForHeapWords("stack", sp, 0x02000000, 40)
	harness.Logf("falseroots: globals %#x-%#x: %d heap-range words of %d; stack %#x-0x2000000: %d heap-range words of %d",
		fd, en, ng, (en-fd)/4, sp, ns, (0x02000000-sp)/4)
	return nil
}
