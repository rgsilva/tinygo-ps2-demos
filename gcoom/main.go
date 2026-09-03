// Heap exhaustion experiment: grow live data in 512 KB steps until the
// allocator fails. Build: make DEMOS=gcoom gcoom BUILD=build-gc
package main

import (
	"runtime"
	"time"

	"ps2go/harness"
)

var keep [][]byte
var sink []byte

func stats(tag string) runtime.MemStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	harness.Logf("%s: inuse=%d idle=%d objects=%d numgc=%d", tag, ms.HeapInuse, ms.HeapIdle, ms.HeapObjects, ms.NumGC)
	return ms
}

func testExhaust() error {
	harness.Logf("oom: heap %#x-%#x (%d bytes)", runtime.HeapStart, runtime.HeapEnd, runtime.HeapEnd-runtime.HeapStart)
	stats("oom: start")
	const step = 512 * 1024
	for i := 0; ; i++ {
		harness.Logf("oom: allocating chunk %d (live so far %d KB)", i, i*step/1024)
		b := make([]byte, step)
		for j := 0; j < len(b); j += 4096 {
			b[j] = byte(i)
		}
		keep = append(keep, b)
		if i == 31 || i == 39 || i == 45 {
			t0 := time.Now()
			runtime.GC()
			d := time.Since(t0)
			ms := stats("oom: after timed GC")
			harness.Logf("oom: runtime.GC() with %d KB live took %v", ms.HeapInuse/1024, d)
			if i == 45 {
				n := 0
				for k := 0; k < 100000; k++ {
					s := make([]byte, 64)
					s[0] = byte(k)
					sink = s
					n++
				}
				harness.Logf("oom: %d small garbage allocations with %d KB live OK", n, ms.HeapInuse/1024)
				stats("oom: after small allocs")
			}
		}
		if i%8 == 7 {
			ms := stats("oom: progress")
			if ms.HeapIdle < 3*step {
				harness.Logf("oom: near the limit: explicit runtime.GC() with %d live", ms.HeapInuse)
				runtime.GC()
				stats("oom: after GC near limit")
				// Also try a small allocation pattern to see whether small
				// objects still fit when a large one does not.
				for k := 0; k < 100000; k++ {
					s := make([]byte, 64)
					s[0] = 1
					keep[0][k%4096] = s[0]
				}
				harness.Log("oom: 100k small garbage allocations near the limit OK")
			}
		}
	}
}

func main() {
	harness.Run([]harness.Case{
		{Name: "heap-exhaustion", Fn: testExhaust},
	})
}
