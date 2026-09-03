// Negative control (scheduler): an unrecovered panic in a goroutine must be
// reported as CRASH ("panic: ..." and "runtime: abort").
package main

import (
	"time"

	"ps2go/harness"
)

func main() {
	harness.Init()
	harness.Log("about to panic in a goroutine")
	go func() {
		panic("in goroutine")
	}()
	time.Sleep(time.Second)
	harness.Log("not reached")
}
