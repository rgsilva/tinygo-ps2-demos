//go:build ps2go_recover

// recover does not work on the ps2 target yet (a panic ends in the runtime's
// abort loop); build with -tags ps2go_recover once it does.
package main

import (
	"fmt"

	"ps2go/harness"
)

func init() {
	extraCases = append(extraCases, harness.Case{Name: "panic-recover", Fn: testRecover})
}

func testRecover() error {
	var arr []int
	idx := len(vec) // runtime value
	defer func() { recover() }()
	_ = arr[idx] // index out of range: must panic and be recovered
	return fmt.Errorf("no panic")
}
