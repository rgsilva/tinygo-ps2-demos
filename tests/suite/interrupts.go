package main

import (
	"fmt"
	"runtime/interrupt"
	"time"
)

// testInterrupts checks that interrupt.Disable/Restore really touch the EE
// interrupt enable: while disabled, the system timer cannot service its
// overflow interrupt, so a sleep afterwards only completes if Restore turned
// interrupts back on. Nesting must not re-enable early.
func testInterrupts() error {
	outer := interrupt.Disable()
	inner := interrupt.Disable()
	if inner != 0 {
		return fmt.Errorf("nested Disable reported interrupts enabled")
	}
	interrupt.Restore(inner) // must not enable: outer section still open
	// Spin well past the 16-bit timer overflow period (0.44 ms) with
	// interrupts off, then restore. If interrupts came back too early or not
	// at all, the sleep below misbehaves.
	t0 := time.Now()
	for time.Since(t0) < 2*time.Millisecond {
	}
	interrupt.Restore(outer)
	if outer == 0 {
		return fmt.Errorf("outer Disable reported interrupts already disabled")
	}
	t1 := time.Now()
	time.Sleep(20 * time.Millisecond)
	if d := time.Since(t1); d < 20*time.Millisecond || d > 200*time.Millisecond {
		return fmt.Errorf("sleep after Restore took %v", d)
	}
	return nil
}
