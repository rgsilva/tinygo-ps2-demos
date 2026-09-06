package main

import "time"

var start = time.Now()

// ticks is the monotonic clock in nanoseconds.
func ticks() int64 { return int64(time.Since(start)) }
