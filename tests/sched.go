package main

import (
	"fmt"
	"time"
)

// testTimer needs a working runtime clock (ticks and sleepTicks); it is
// expected to fail until the scheduler work lands.
func testTimer() error {
	t0 := time.Now()
	time.Sleep(10 * time.Millisecond)
	d := time.Since(t0)
	if d < 10*time.Millisecond || d > 2*time.Second {
		return fmt.Errorf("slept %v", d)
	}
	return nil
}
