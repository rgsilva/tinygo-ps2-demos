// Negative control (scheduler): main blocking forever with no other
// goroutine must be reported as CRASH ("fatal error: ... deadlock").
package main

import "ps2go/lib/harness"

func main() {
	harness.Init()
	harness.Log("about to deadlock")
	c := make(chan int)
	<-c
	harness.Log("not reached")
}
