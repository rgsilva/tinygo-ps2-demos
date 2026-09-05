// The ps2go test suite. Build with `make tests`, run with `make check`.
package main

import "ps2go/lib/harness"

func main() {
	cases := []harness.Case{
		{Name: "hello", Fn: hello},
		{Name: "gc-floor", Fn: testGCFloor},
		{Name: "strings", Fn: testStrings},
		{Name: "slices-maps", Fn: testSlicesMaps},
		{Name: "interfaces-closures", Fn: testInterfacesClosures},
		{Name: "int64-mul", Fn: testInt64Mul},
		{Name: "int64-div", Fn: testInt64Div},
		{Name: "select-cmov", Fn: testSelect},
		{Name: "float32-compare", Fn: testFloat32Compare},
		{Name: "float64-compare", Fn: testFloat64Compare},
		{Name: "float32-arith", Fn: testFloat32Arith},
		{Name: "float64-arith", Fn: testFloat64Arith},
		{Name: "float-int-conv", Fn: testFloatIntConv},
		{Name: "atomics", Fn: testAtomics},
		{Name: "cgo-abi", Fn: testCgoABI},
		{Name: "cgo-cfile", Fn: testCFile},
		{Name: "embed", Fn: testEmbed},
		{Name: "irx-load", Fn: testIRX},
		{Name: "rtc", Fn: testRTC},
		{Name: "math-rand", Fn: testMathRand},
		{Name: "memory-layout", Fn: testMemoryLayout},
		{Name: "libc-heap-cap", Fn: testLibcHeapCap},
		{Name: "gskit-alloc", Fn: testGsKitAlloc},
		{Name: "gc-stress", Fn: testGCStress},
		{Name: "memstats", Fn: testMemStats},
		{Name: "gc-roots", Fn: testGCRoots},
		{Name: "gc-reclaim", Fn: testGCReclaim},
		{Name: "gc-large", Fn: testGCLarge},
		{Name: "gc-natural", Fn: testGCNatural},
		{Name: "gc-finalizer", Fn: testGCFinalizer},
		{Name: "gc-sweep-time", Fn: testGCSweepTime},
		{Name: "gc-interior-pointers", Fn: testGCInteriorPointers},
	}
	cases = append(cases,
		harness.Case{Name: "timer", Fn: testTimer},
		harness.Case{Name: "interrupts", Fn: testInterrupts},
		harness.Case{Name: "panic-recover", Fn: testRecover},
		// Goroutines (tasks scheduler).
		harness.Case{Name: "goroutines", Fn: testGoroutines},
		harness.Case{Name: "channels", Fn: testChannels},
		harness.Case{Name: "sleep-goroutines", Fn: testSleepGoroutines},
		harness.Case{Name: "timers", Fn: testTimers},
		harness.Case{Name: "gc-goroutine-stacks", Fn: testGCGoroutineStacks},
		harness.Case{Name: "callee-saved", Fn: testCalleeSaved},
		harness.Case{Name: "producer-consumer", Fn: testProducerConsumer},
	)
	harness.Run(cases)
}

func hello() error {
	harness.Log("hello from ps2go")
	return nil
}
