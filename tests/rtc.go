package main

import (
	"fmt"
	"time"

	"ps2go/harness"
	"ps2go/rtc"
)

// testRTC syncs the wall clock from the console's RTC (JST on the hardware
// and in PCSX2, converted to UTC by the package).
func testRTC() error {
	before := time.Now()
	t, err := rtc.Sync()
	if err != nil {
		return err
	}
	now := time.Now()
	harness.Logf("rtc: %s (time.Now was %s, now %s)", t.Format(time.RFC3339), before.Format(time.RFC3339), now.Format(time.RFC3339))
	if t.Year() < 2026 || t.Year() > 2100 {
		return fmt.Errorf("implausible RTC time %s", t)
	}
	if d := now.Sub(t); d < 0 || d > 5*time.Second {
		return fmt.Errorf("time.Now() %s is not the synced time %s", now, t)
	}
	time.Sleep(20 * time.Millisecond)
	if !time.Now().After(now) {
		return fmt.Errorf("clock did not advance")
	}
	return nil
}
