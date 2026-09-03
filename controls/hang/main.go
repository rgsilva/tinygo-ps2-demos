// Negative control: the harness must report TIMEOUT, never PASS.
package main

import "ps2go/harness"

func main() {
	harness.Run([]harness.Case{
		{Name: "ok", Fn: func() error { return nil }},
		{Name: "hang", Fn: func() error {
			for {
			}
		}},
	})
}
