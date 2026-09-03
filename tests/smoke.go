package main

import (
	"fmt"
	"sort"
	"strings"
)

func testStrings() error {
	s := fmt.Sprintf("%d-%s-%x-%.2f", 42, "go", 255, 3.14159)
	if s != "42-go-ff-3.14" {
		return fmt.Errorf("Sprintf: %q", s)
	}
	if strings.ToUpper("ps2") != "PS2" || !strings.Contains(s, "-go-") {
		return fmt.Errorf("strings")
	}
	parts := strings.Split("a,b,c", ",")
	if len(parts) != 3 || strings.Join(parts, "") != "abc" {
		return fmt.Errorf("split/join")
	}
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteByte(byte('a' + i%26))
	}
	if b.Len() != 100 || b.String()[26] != 'a' {
		return fmt.Errorf("builder")
	}
	return nil
}

func testSlicesMaps() error {
	xs := make([]int, 0)
	for i := 0; i < 1000; i++ {
		xs = append(xs, (i*7919)%1000)
	}
	sort.Ints(xs)
	for i := 1; i < len(xs); i++ {
		if xs[i-1] > xs[i] {
			return fmt.Errorf("sort at %d", i)
		}
	}
	m := map[string]int{}
	for i := 0; i < 500; i++ {
		m[fmt.Sprintf("k%d", i)] = i
	}
	sum := 0
	for _, v := range m {
		sum += v
	}
	if len(m) != 500 || sum != 500*499/2 {
		return fmt.Errorf("map len=%d sum=%d", len(m), sum)
	}
	delete(m, "k7")
	if _, ok := m["k7"]; ok {
		return fmt.Errorf("delete")
	}
	return nil
}

type shape interface{ area() int }
type rect struct{ w, h int }
type square struct{ s int }

func (r rect) area() int   { return r.w * r.h }
func (s square) area() int { return s.s * s.s }

func testInterfacesClosures() error {
	shapes := []shape{rect{2, 3}, square{4}, rect{1, 1}}
	total := 0
	for _, s := range shapes {
		total += s.area()
	}
	if total != 23 {
		return fmt.Errorf("area total %d", total)
	}
	if _, ok := shapes[1].(square); !ok {
		return fmt.Errorf("type assertion")
	}
	counter := 0
	inc := func(n int) { counter += n }
	for i := 1; i <= 10; i++ {
		inc(i)
	}
	if counter != 55 {
		return fmt.Errorf("closure %d", counter)
	}
	order := ""
	func() {
		defer func() { order += "3" }()
		defer func() { order += "2" }()
		order += "1"
	}()
	if order != "123" {
		return fmt.Errorf("defer order %q", order)
	}
	return nil
}
