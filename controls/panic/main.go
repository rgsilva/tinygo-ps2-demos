// Negative control: an unrecovered panic (outside any case) must be reported
// as CRASH. The runtime prints "panic: ..." and "runtime: abort".
package main

import "ps2go/harness"

var m map[string]int

func main() {
	harness.Init()
	harness.Log("about to panic outside a case")
	m["boom"] = 1 // assignment to entry in nil map
	harness.Log("not reached")
}
