package main

// Collection time. The sweep used to walk the whole heap, so an idle program
// paid a fixed cost per collection that depended on the heap size (about
// 300 ms for 26 MB); now it stops at the highest live object and handles a
// word of block states at a time. The case runs early (nearly empty heap:
// tight bound, the floor) and late (whatever earlier cases left live: loose
// bound, informational).

import (
	"fmt"
	"runtime"
	"time"

	"ps2go/lib/harness"
)

type sweepNode struct {
	next *sweepNode
	pad  [2]uintptr
}

var gcKeep *sweepNode

func gcTime() time.Duration {
	start := time.Now()
	runtime.GC()
	return time.Since(start)
}

func idleGCTime() time.Duration {
	runtime.GC()
	idle := gcTime()
	for i := 0; i < 2; i++ {
		if d := gcTime(); d < idle {
			idle = d
		}
	}
	return idle
}

// testGCFloor runs early, on a nearly empty heap: the fixed cost of a
// collection. It allocates nothing that could stay pinned for later cases.
func testGCFloor() error {
	idle := idleGCTime()
	harness.Logf("gc floor: idle collection %.1f ms (heap %d KB)", float64(idle)/1e6, (runtime.HeapEnd-runtime.HeapStart)/1024)
	if idle > 20*time.Millisecond {
		return fmt.Errorf("idle collection took %.1f ms", float64(idle)/1e6)
	}
	return nil
}

func testGCSweepTime() error {
	idle := idleGCTime()

	// 100k live small objects: the per-object cost on top of the floor.
	for i := 0; i < 100000; i++ {
		gcKeep = &sweepNode{next: gcKeep}
	}
	live := gcTime()
	n := 0
	for p := gcKeep; p != nil; p = p.next {
		n++
	}
	gcKeep = nil
	touchSink()
	dead := gcTime()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	harness.Logf("gc: idle %.1f ms, %d live objects %.1f ms, after freeing them %.1f ms (heap %d KB, %d collections)",
		float64(idle)/1e6, n, float64(live)/1e6, float64(dead)/1e6,
		(runtime.HeapEnd-runtime.HeapStart)/1024, ms.NumGC)
	if n != 100000 {
		return fmt.Errorf("kept %d of 100000 objects", n)
	}
	if idle > 100*time.Millisecond {
		return fmt.Errorf("idle collection took %.1f ms", float64(idle)/1e6)
	}
	return nil
}

//go:noinline
func touchSink() {}
