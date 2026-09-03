package main

import (
	"errors"
	"fmt"

	"ps2go/harness"
)

//go:noinline
func mustPanicIndex(s []int, i int) int { return s[i] }

//go:noinline
func mustPanicNilMap(m map[string]int) { m["x"] = 1 }

//go:noinline
func mustPanicNilPtr(p *int) int { return *p }

//go:noinline
func mustPanicDiv(a, b int) int { return a / b }

func catch(f func()) (r interface{}) {
	defer func() { r = recover() }()
	f()
	return nil
}

func testRecover() error {
	if r := catch(func() {}); r != nil {
		return fmt.Errorf("recover without panic: %v", r)
	}
	if r := catch(func() { panic("boom") }); r != "boom" {
		return fmt.Errorf("panic(string): %v", r)
	}
	if r := catch(func() { panic(errors.New("err")) }); r == nil || r.(error).Error() != "err" {
		return fmt.Errorf("panic(error): %v", r)
	}
	if r := catch(func() { mustPanicIndex(vec32(), 99) }); r == nil {
		return fmt.Errorf("index out of range not recovered")
	}
	if r := catch(func() { mustPanicNilMap(nil) }); r == nil {
		return fmt.Errorf("nil map write not recovered")
	}
	if r := catch(func() { mustPanicNilPtr(nil) }); r == nil {
		return fmt.Errorf("nil dereference not recovered")
	}
	if r := catch(func() { mustPanicDiv(1, len(vec32())-len(vec32())) }); r == nil {
		return fmt.Errorf("division by zero not recovered")
	}
	// Defers run in order on the way out, and a re-panic propagates.
	order := ""
	r := catch(func() {
		defer func() { order += "c" }()
		defer func() {
			order += "b"
			if v := recover(); v != nil {
				panic(fmt.Sprint("again:", v))
			}
		}()
		order += "a"
		panic("first")
	})
	if order != "abc" || r != "again:first" {
		return fmt.Errorf("re-panic: order %q result %v", order, r)
	}
	// Values live across the recover point must survive, floats included
	// (checks the register clobber list at the setjmp point).
	x, y := float32(1.5), 2.25
	sum := 0
	for i := 0; i < 1000; i++ {
		x *= 1.0001
		y += 0.001
		if catch(func() { mustPanicIndex(nil, i) }) == nil {
			return fmt.Errorf("iteration %d not recovered", i)
		}
		sum += i
	}
	if sum != 999*1000/2 || x < 1.5 || x > 1.7 || y < 3.24 || y > 3.26 {
		return fmt.Errorf("state after recover loop: sum %d x %v y %v", sum, x, y)
	}
	harness.Logf("recover: ok, x=%v y=%v", x, y)
	return nil
}

func vec32() []int { return []int{1, 2, 3} }
