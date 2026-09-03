// The ps2go test suite. Build with `make tests`, run with `make check`.
package main

import "ps2go/harness"

func main() {
	cases := []harness.Case{
		{Name: "hello", Fn: hello},
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
		{Name: "atomics", Fn: testAtomics},
		{Name: "cgo-abi", Fn: testCgoABI},
		{Name: "gc-stress", Fn: testGCStress},
		{Name: "memstats", Fn: testMemStats},
	}
	cases = append(cases, harness.Case{Name: "timer", Fn: testTimer})
	cases = append(cases, harness.Case{Name: "interrupts", Fn: testInterrupts})
	cases = append(cases, harness.Case{Name: "panic-recover", Fn: testRecover})
	// Tag-gated cases (sched_tagged.go with -tags ps2go_sched) go last so a
	// hang does not hide the results above.
	cases = append(cases, extraCases...)
	harness.Run(cases)
}

var extraCases []harness.Case

func hello() error {
	harness.Log("hello from ps2go")
	return nil
}
