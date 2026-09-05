package main

// Interior pointers: a slice of pointers into the elements of one big backing
// array, a common Go pattern (an index over a table of entities). Each such
// pointer makes the collector search for the head of the big object.

import (
	"fmt"
	"runtime"
	"time"

	"ps2go/lib/harness"
)

type entity struct {
	x, y, vx, vy float32
	hp, kind     int32
	pad          [40]byte
}

var (
	gcTable []entity
	gcIndex []*entity
)

func testGCInteriorPointers() error {
	const n = 16384 // 64 B each: 1 MB
	gcTable = make([]entity, n)
	gcIndex = make([]*entity, 10000)
	seed := uint32(12345)
	for i := range gcIndex {
		seed = seed*1664525 + 1013904223
		gcIndex[i] = &gcTable[seed%n]
	}
	runtime.GC()
	start := time.Now()
	runtime.GC()
	d := time.Since(start)
	harness.Logf("gc: %d KB table, %d interior pointers: collection %.1f ms", n*64/1024, len(gcIndex), float64(d)/1e6)
	if d > 500*time.Millisecond {
		// 8.4 s before the head index and cache, about 90 ms after.
		return fmt.Errorf("collection took %.1f ms", float64(d)/1e6)
	}
	for _, p := range gcIndex {
		if p == nil || p.kind != 0 {
			return fmt.Errorf("index corrupted")
		}
	}
	gcTable, gcIndex = nil, nil
	runtime.GC()
	return nil
}
