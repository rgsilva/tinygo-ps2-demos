package main

import (
	"fmt"
	"time"

	"ps2go/harness"
)

// testTimer checks the runtime clock: time.Sleep must actually wait and
// time.Now must advance monotonically.
func testTimer() error {
	t0 := time.Now()
	time.Sleep(10 * time.Millisecond)
	d10 := time.Since(t0)
	if d10 < 10*time.Millisecond || d10 > 100*time.Millisecond {
		return fmt.Errorf("slept %v for 10ms", d10)
	}
	t1 := time.Now()
	time.Sleep(250 * time.Millisecond)
	d250 := time.Since(t1)
	if d250 < 250*time.Millisecond || d250 > 400*time.Millisecond {
		return fmt.Errorf("slept %v for 250ms", d250)
	}
	prev := time.Now()
	for i := 0; i < 1000; i++ {
		now := time.Now()
		if now.Before(prev) {
			return fmt.Errorf("clock went backwards at %d", i)
		}
		prev = now
	}
	harness.Logf("timer: 10ms slept %v, 250ms slept %v", d10, d250)
	return nil
}
