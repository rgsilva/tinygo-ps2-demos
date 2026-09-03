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
		{Name: "float64-soft", Fn: testFloat64},
		{Name: "atomics", Fn: testAtomics},
		{Name: "gc-stress", Fn: testGCStress},
		{Name: "memstats", Fn: testMemStats},
	}
	// Known-incomplete runtime features go last so a hang does not hide the
	// results above: the timer (no clock yet), then the tag-gated cases from
	// sched_tagged.go (-tags ps2go_sched) and recover_tagged.go (-tags ps2go_recover).
	cases = append(cases, harness.Case{Name: "timer", Fn: testTimer, XFail: "runtime ticks/sleep not implemented"})
	cases = append(cases, extraCases...)
	harness.Run(cases)
}

var extraCases []harness.Case

func hello() error {
	harness.Log("hello from ps2go")
	return nil
}
