// Package rtc reads the PlayStation 2's real-time clock and sets the Go
// wall clock from it. Without it time.Now() is 1970 plus the uptime.
//
// The RTC is read through the CD/DVD driver on the IOP, which the boot ROM
// provides, so nothing has to be loaded; the SIF RPC is initialized here if
// the program has not done so. The hardware keeps the clock in Japan
// Standard Time (UTC+9) regardless of the console's region, and PCSX2 does
// the same, so the value is converted to UTC.
package rtc

/*
#cgo LDFLAGS: -lcdvd
int ps2go_rtc_init(void);
int ps2go_rtc_read(unsigned char *out);
*/
import "C"

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

var initOnce sync.Once
var initErr error

func bcd(b byte) int { return int(b>>4)*10 + int(b&15) }

// Read returns the current RTC time in UTC.
func Read() (time.Time, error) {
	initOnce.Do(func() {
		if C.ps2go_rtc_init() != 1 {
			initErr = errors.New("rtc: sceCdInit failed")
		}
	})
	if initErr != nil {
		return time.Time{}, initErr
	}
	var raw [7]C.uchar
	if C.ps2go_rtc_read(&raw[0]) != 1 || raw[0] != 0 {
		return time.Time{}, errors.New("rtc: sceCdReadClock failed")
	}
	jst := time.Date(2000+bcd(byte(raw[6])), time.Month(bcd(byte(raw[5]))), bcd(byte(raw[4])),
		bcd(byte(raw[3])), bcd(byte(raw[2])), bcd(byte(raw[1])), 0, time.UTC)
	return jst.Add(-9 * time.Hour), nil
}

// Sync reads the RTC and adjusts the runtime's clock so that time.Now()
// returns real (UTC) time from now on. It returns the time read.
func Sync() (time.Time, error) {
	t, err := Read()
	if err != nil {
		return t, err
	}
	runtime.AdjustTimeOffset(int64(t.Sub(time.Now())))
	return t, nil
}
