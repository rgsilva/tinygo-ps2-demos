// GC experiments for the PS2 target. Build: make DEMOS=gcexp gcexp BUILD=build-gc
package main

import "ps2go/harness"

func main() {
	harness.Run([]harness.Case{
		{Name: "layout", Fn: testLayout},
		{Name: "false-roots", Fn: testFalseRoots},
		{Name: "false-retention", Fn: testFalseRetention},
		{Name: "drop-patterns", Fn: testDropPatterns},
		{Name: "false-roots-2", Fn: testFalseRoots},
		{Name: "gc-cost", Fn: testGCCost},
		{Name: "finalizers", Fn: testFinalizers},
		{Name: "c-memory-pointer", Fn: testCMemoryPointer},
		{Name: "small-objects", Fn: testSmallObjects},
		{Name: "maps-strings", Fn: testMapsStrings},
		{Name: "slice-of-pointers", Fn: testSliceOfPointers},
		{Name: "interior-pointers", Fn: testInteriorPointers},
		{Name: "globals-only", Fn: testGlobalsOnly},
		{Name: "stack-deep", Fn: testStackDeep},
		{Name: "closures", Fn: testClosures},
		{Name: "natural-gc", Fn: testNaturalGC},
		{Name: "leak-cycles", Fn: testLeakCycles},
		{Name: "leak-cycles-via-call", Fn: testLeakCyclesViaCall},
		{Name: "large-objects", Fn: testLargeObjects},
	})
}
