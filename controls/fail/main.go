// Negative control: the harness must report FAIL.
package main

import (
	"errors"

	"ps2go/harness"
)

func main() {
	harness.Run([]harness.Case{
		{Name: "ok", Fn: func() error { return nil }},
		{Name: "bad", Fn: func() error { return errors.New("expected failure") }},
	})
}
