// Control: a goroutine overflows its stack. Expected verdict: CRASH with
// "goroutine stack overflow" in the log (the runtime's message), not a
// silent corruption or a TLB miss.
package main

import (
	"runtime"

	"ps2go/harness"
)

// How often the recursion yields (a yield is where the runtime checks the
// stack canary).
const yieldEvery = 64

//go:noinline
func recurse(depth int) int {
	// A real frame per level: dynamically indexed and used after the call,
	// so the optimizer can neither drop it nor turn the recursion into a loop.
	var frame [256]byte
	frame[(depth*7)%256] = byte(depth)
	if depth%yieldEvery == 0 {
		harness.Logf("depth %d", depth)
		runtime.Gosched()
	}
	r := recurse(depth + 1)
	return r + int(frame[(depth*3)%256])
}

func main() {
	harness.Log("stackoverflow control: recursing in a goroutine")
	done := make(chan int)
	go func() { done <- recurse(1) }()
	<-done
	harness.Log("PS2GO-RESULT FAIL 0/1")
}
