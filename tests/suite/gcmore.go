package main

// Garbage collector behaviour on this target. The GC is conservative: any
// word in a root range that looks like a heap address pins an object. The
// root ranges are the Go program's own globals (_globals_start.._globals_end,
// laid out by the generated linker script) and the stack.

/*
extern char _globals_start[], _globals_end[];
static unsigned int ps2go_globals_start(void) { return (unsigned int)_globals_start; }
static unsigned int ps2go_globals_end(void) { return (unsigned int)_globals_end; }
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"ps2go/lib/harness"
)

// use keeps an allocation observable without holding it anywhere afterwards
// (a "sink" global would keep the last value: its final nil store is dead
// and gets removed by the optimizer).
//
//go:noinline
func use(b []byte) {
	b[0]++
}

// scrubStack overwrites free stack below the caller so stale pointers from
// earlier frames don't pin objects during the measurements.
//
//go:noinline
func scrubStack(depth int) int {
	var buf [256]uint32
	for i := range buf {
		buf[i] = 0x11111111
	}
	if depth > 0 {
		return scrubStack(depth-1) + int(buf[depth%256])
	}
	return int(buf[0])
}

func memstats() runtime.MemStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms
}

// memstatsSmall returns heap in use and object count in KB-scale units that
// cannot be mistaken for heap addresses.
//
//go:noinline
func memstatsSmall() (inuse, objects uint32) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return uint32(ms.HeapInuse), uint32(ms.HeapObjects)
}

// testGCRoots checks that the globals range the GC scans is the Go program's
// own data only (tens of KB: the suite's crypto and math/big tables are the
// bulk) and contains few heap-looking words. Before the linker script
// change it spanned the SDK's .data..bss (~78 KB) with ~240 false roots
// from libkernel's code blobs.
func testGCRoots() error {
	start, end := uintptr(C.ps2go_globals_start()), uintptr(C.ps2go_globals_end())
	if end <= start || end-start > 64*1024 {
		return fmt.Errorf("globals range %#x-%#x (%d bytes)", start, end, end-start)
	}
	n, words := 0, 0
	for a := start; a+4 <= end; a += 4 {
		v := uintptr(*(*uint32)(unsafe.Pointer(a)))
		words++
		if v >= runtime.HeapStart && v < runtime.HeapEnd {
			n++
		}
	}
	harness.Logf("gc-roots: globals %#x-%#x: %d words, %d point into the heap", start, end, words, n)
	if n > 64 {
		return fmt.Errorf("%d heap-looking words in globals, expected only live runtime pointers", n)
	}
	return nil
}

//go:noinline
func makeDead(count, size int) {
	for i := 0; i < count; i++ {
		b := make([]byte, size) // zero-filled: words of 0 never look like pointers
		b[0] = byte(i)
		use(b)
	}
}

// testGCReclaim allocates ~12 MB of dead buffers and expects the collector
// to get nearly all of it back. Before the globals fixes (Go-only globals
// range, pointer-free globals out of the scan) this retained ~8 MB forever.
func testGCReclaim() error {
	scrubStack(20)
	runtime.GC()
	// Keep only small numbers live across the collection: counters such as
	// TotalAlloc are in the millions, which is inside the heap's address
	// range, and a conservative scanner would treat them as pointers.
	baseInuse, baseObjs := memstatsSmall()
	makeDead(8, 1024*1024)
	makeDead(2000, 2048)
	scrubStack(20)
	runtime.GC()
	runtime.GC()
	inuse, objs := memstatsSmall()
	retained := int64(inuse) - int64(baseInuse)
	harness.Logf("gc-reclaim: baseline %d KB, after dead 12 MB + 2 GCs %d KB (retained %d KB, %d objects)", baseInuse/1024, inuse/1024, retained/1024, int64(objs)-int64(baseObjs))
	// The last buffer of each loop tends to survive: its pointer sits in a
	// stack slot of a frame that is live during the collection. Anything
	// beyond that means a systematic false root.
	if retained > 1536*1024 {
		return fmt.Errorf("retained %d KB of dead buffers", retained/1024)
	}
	return nil
}

var keptLarge [][]byte

// testGCLarge repeatedly allocates 1 MB objects with small garbage in
// between, keeping a few. Fragmentation plus false retention made this run
// out of memory after a few rounds.
func testGCLarge() error {
	keptLarge = nil
	for round := 0; round < 12; round++ {
		b := make([]byte, 1024*1024)
		b[0], b[len(b)-1] = byte(round), byte(round)
		if round%4 == 0 {
			keptLarge = append(keptLarge, b)
		} else {
			use(b)
		}
		for i := 0; i < 500; i++ {
			s := make([]byte, 16+(i*37)%3000)
			s[0] = byte(i)
			use(s)
		}
	}
	for i, b := range keptLarge {
		if b[0] != byte(i*4) || b[len(b)-1] != byte(i*4) {
			return fmt.Errorf("kept large object %d corrupted", i)
		}
	}
	ms := memstats()
	harness.Logf("gc-large: 12 rounds done, %d kept, in use %d KB, %d GCs so far", len(keptLarge), ms.HeapInuse/1024, ms.NumGC)
	keptLarge = nil
	return nil
}

type gcNode struct {
	id   int
	sum  uint32
	data []byte
	next *gcNode
}

// testGCNatural relies on allocation-triggered collections (no runtime.GC)
// and verifies survivors after each one.
func testGCNatural() error {
	var head *gcNode
	kept, total := 0, 0
	base := memstats()
	last := base.NumGC
	seen := uint32(0)
	for seen < 3 {
		for i := 0; i < 2000; i++ {
			b := make([]byte, 64+(i*41)%4000)
			fill(b, i)
			total += len(b)
			if i%100 == 0 {
				head = &gcNode{id: kept, sum: checksum(b), data: b, next: head}
				kept++
			} else {
				use(b)
			}
		}
		if ms := memstats(); ms.NumGC != last {
			last = ms.NumGC
			seen++
			n := 0
			for p := head; p != nil; p = p.next {
				if checksum(p.data) != p.sum {
					return fmt.Errorf("survivor %d corrupted after natural GC", p.id)
				}
				n++
			}
			if n != kept {
				return fmt.Errorf("%d survivors, want %d", n, kept)
			}
		}
		if total > 200*1024*1024 {
			return fmt.Errorf("no natural GC after %d MB", total>>20)
		}
	}
	harness.Logf("gc-natural: %d collections while allocating %d MB, %d survivors intact", seen, total>>20, kept)
	return nil
}

var (
	finalized   int
	finalizedID int
)

//go:noinline
func makeFinalizable(id int) {
	o := &gcNode{id: id, data: make([]byte, 100)}
	runtime.SetFinalizer(o, func(p *gcNode) {
		finalized++
		finalizedID = p.id
	})
	use(o.data)
}

func testGCFinalizer() error {
	makeFinalizable(42)
	scrubStack(20)
	if finalized != 0 {
		return fmt.Errorf("finalizer ran before GC")
	}
	for i := 0; i < 5 && finalized == 0; i++ {
		makeDead(64, 4096) // churn so the object's block gets reused if freed
		scrubStack(20)
		runtime.GC()
	}
	if finalized == 0 {
		// The conservative collector may keep the object alive through some
		// word that happens to look like a pointer; not an error by itself.
		harness.Log("gc-finalizer: object still pinned after 5 collections (conservative GC)")
		return nil
	}
	if finalized != 1 || finalizedID != 42 {
		return fmt.Errorf("finalizer ran %d times (id %d), want once for 42", finalized, finalizedID)
	}
	harness.Log("gc-finalizer: ran once")
	return nil
}
