// Negative control: the harness must report FAIL (a failing case and a
// recovered panic).
package main

import (
	"errors"

	"ps2go/harness"
)

func main() {
	harness.Run([]harness.Case{
		{Name: "ok", Fn: func() error { return nil }},
		{Name: "bad", Fn: func() error { return errors.New("expected failure") }},
		// A panic inside a case is recovered by the harness and is a failure.
		{Name: "panics", Fn: func() error { var m map[string]int; m["x"] = 1; return nil }},
	})
}
