package main

import (
	"fmt"
	"runtime"

	"ps2go/lib/harness"
)

type node struct {
	id   int
	data []byte
	next *node
}

func checksum(b []byte) uint32 {
	var s uint32 = 2166136261
	for _, c := range b {
		s = (s ^ uint32(c)) * 16777619
	}
	return s
}

func fill(b []byte, seed int) {
	for i := range b {
		b[i] = byte(seed*31 + i*7)
	}
}

// testGCStress allocates far more than the heap holds, keeping a linked list
// of survivors, and verifies the survivors after every collection.
func testGCStress() error {
	var head *node
	kept := 0
	sums := map[int]uint32{}
	for round := 0; round < 12; round++ {
		for i := 0; i < 400; i++ {
			size := 16 + (i*37)%2048
			b := make([]byte, size)
			fill(b, round*1000+i)
			if i%13 == 0 {
				head = &node{id: round*1000 + i, data: b, next: head}
				sums[head.id] = checksum(b)
				kept++
			}
		}
		runtime.GC()
		n := 0
		for p := head; p != nil; p = p.next {
			if checksum(p.data) != sums[p.id] {
				return fmt.Errorf("round %d: survivor %d corrupted", round, p.id)
			}
			n++
		}
		if n != kept {
			return fmt.Errorf("round %d: %d survivors, want %d", round, n, kept)
		}
		harness.UpdateStats()
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	harness.Logf("gc-stress: kept %d objects, heap in use %d, total allocated %d, frees %d", kept, ms.HeapInuse, ms.TotalAlloc, ms.Frees)
	return nil
}

func testMemStats() error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	if runtime.HeapStart == 0 || runtime.HeapEnd <= runtime.HeapStart {
		return fmt.Errorf("heap bounds %#x-%#x", runtime.HeapStart, runtime.HeapEnd)
	}
	if ms.HeapInuse == 0 || ms.HeapInuse+ms.HeapIdle > uint64(runtime.HeapEnd-runtime.HeapStart) {
		return fmt.Errorf("inuse %d idle %d", ms.HeapInuse, ms.HeapIdle)
	}
	if ms.Mallocs == 0 || ms.TotalAlloc < ms.HeapInuse {
		return fmt.Errorf("mallocs %d total %d", ms.Mallocs, ms.TotalAlloc)
	}
	harness.UpdateStats()
	return nil
}
