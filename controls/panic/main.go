// Negative control: an unrecovered panic must be reported as CRASH.
package main

import "ps2go/harness"

var m map[string]int

func main() {
	harness.Run([]harness.Case{
		{Name: "ok", Fn: func() error { return nil }},
		{Name: "panic", Fn: func() error {
			m["boom"] = 1 // assignment to entry in nil map
			return nil
		}},
	})
}
