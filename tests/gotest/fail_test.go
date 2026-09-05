//go:build ps2fail

package gotest

import "testing"

func TestMustFail(t *testing.T) {
	t.Fatal("expected failure (-tags ps2fail)")
}
