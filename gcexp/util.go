package main

import (
	"runtime"
	"time"
	"unsafe"

	"ps2go/harness"
)

// sink defeats escape analysis / dead-store elimination.
var sink []byte
var sinkInt int

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

// fillSafe writes a pattern whose 32-bit words can never look like a heap
// pointer (every byte has the high bit set, so every word is >= 0x80000000).
func fillSafe(b []byte, seed int) {
	for i := range b {
		b[i] = 0x80 | byte(seed*31+i*7)
	}
}

// churn allocates about total bytes of garbage in objects of varying size.
// Nothing is kept.
//
//go:noinline
func churn(total int) {
	n := 0
	i := 0
	for n < total {
		size := 16 + (i*37)%3000
		b := make([]byte, size)
		b[0] = byte(i)
		b[len(b)-1] = byte(i)
		sink = b
		n += size
		i++
	}
	sink = nil
}

// scrubStack overwrites a large part of the free stack below the caller so
// that stale pointer values from earlier frames are not seen by the
// conservative scanner.
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

func logStats(tag string) runtime.MemStats {
	ms := memstats()
	harness.Logf("%s: inuse=%d idle=%d objects=%d numgc=%d mallocs=%d frees=%d totalalloc=%d",
		tag, ms.HeapInuse, ms.HeapIdle, ms.HeapObjects, ms.NumGC, ms.Mallocs, ms.Frees, ms.TotalAlloc)
	return ms
}

// timeGC runs runtime.GC() n times and returns min and average duration.
func timeGC(n int) (min, avg time.Duration) {
	var total time.Duration
	for i := 0; i < n; i++ {
		t0 := time.Now()
		runtime.GC()
		d := time.Since(t0)
		if i == 0 || d < min {
			min = d
		}
		total += d
	}
	return min, total / time.Duration(n)
}

// Heap metadata inspection (mirrors gc_blocks.go's layout: 2 state bits per
// 16-byte block, 4 blocks per state byte, metadata at the end of the heap).
type freeRuns struct {
	largest, total, runs, runs64K, runs1M uintptr
}

func heapFreeRuns() freeRuns {
	const bytesPerBlock = 16
	const blocksPerStateByte = 4
	totalSize := runtime.HeapEnd - runtime.HeapStart
	metadataSize := (totalSize + blocksPerStateByte*bytesPerBlock) / (1 + blocksPerStateByte*bytesPerBlock)
	metadataStart := runtime.HeapEnd - metadataSize
	numBlocks := (metadataStart - runtime.HeapStart) / bytesPerBlock
	meta := unsafe.Slice((*byte)(unsafe.Pointer(metadataStart)), metadataSize)
	var r freeRuns
	var run uintptr
	flush := func() {
		if run == 0 {
			return
		}
		r.runs++
		r.total += run
		if run > r.largest {
			r.largest = run
		}
		if run*bytesPerBlock >= 64*1024 {
			r.runs64K++
		}
		if run*bytesPerBlock >= 1024*1024+16 {
			r.runs1M++
		}
		run = 0
	}
	for b := uintptr(0); b < numBlocks; b++ {
		st := (meta[b/blocksPerStateByte] >> (b % blocksPerStateByte)) & 0x11
		if st == 0 {
			run++
		} else {
			flush()
		}
	}
	flush()
	r.largest *= bytesPerBlock
	r.total *= bytesPerBlock
	return r
}

func logFrag(tag string) freeRuns {
	r := heapFreeRuns()
	harness.Logf("%s: free=%d KB in %d runs, largest run=%d KB, runs>=64K: %d, runs>=1M: %d", tag, r.total/1024, r.runs, r.largest/1024, r.runs64K, r.runs1M)
	return r
}
