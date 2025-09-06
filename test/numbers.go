package main

import (
	"math"
	"ps2go/debug"
)

type numberTest[T comparable] struct {
	name   string
	left   T
	right  T
	result T
	fn     func(T, T) T
}

func isEqual32(a, b float32) bool {
	return float32(math.Abs(float64(a-b))) <= 1e-5
}

func isEqual64(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

var (
	int8Tests = []numberTest[int8]{
		{"add", 42, 58, 100, func(a, b int8) int8 { return a + b }},
		{"sub", 58, 42, 16, func(a, b int8) int8 { return a - b }},
		{"mul", 8, 3, 24, func(a, b int8) int8 { return a * b }},
		{"div", 120, 10, 12, func(a, b int8) int8 { return a / b }},
		{"mod", 120, 13, 3, func(a, b int8) int8 { return a % b }},
	}
	uint8Tests = []numberTest[uint8]{
		{"add", 42, 58, 100, func(a, b uint8) uint8 { return a + b }},
		{"sub", 250, 50, 200, func(a, b uint8) uint8 { return a - b }},
		{"mul", 16, 9, 144, func(a, b uint8) uint8 { return a * b }},
		{"div", 192, 32, 6, func(a, b uint8) uint8 { return a / b }},
		{"mod", 250, 13, 3, func(a, b uint8) uint8 { return a % b }},
	}

	int16Tests = []numberTest[int16]{
		{"add", 1234, 4321, 5555, func(a, b int16) int16 { return a + b }},
		{"sub", 4321, 1234, 3087, func(a, b int16) int16 { return a - b }},
		{"mul", 123, 45, 5535, func(a, b int16) int16 { return a * b }},
		{"div", 5555, 5, 1111, func(a, b int16) int16 { return a / b }},
		{"mod", 5555, 1000, 555, func(a, b int16) int16 { return a % b }},
	}
	uint16Tests = []numberTest[uint16]{
		{"add", 1234, 4321, 5555, func(a, b uint16) uint16 { return a + b }},
		{"sub", 5000, 1234, 3766, func(a, b uint16) uint16 { return a - b }},
		{"mul", 123, 45, 5535, func(a, b uint16) uint16 { return a * b }},
		{"div", 5555, 5, 1111, func(a, b uint16) uint16 { return a / b }},
		{"mod", 5555, 1000, 555, func(a, b uint16) uint16 { return a % b }},
	}

	int32Tests = []numberTest[int32]{
		{"add", 123456, 654321, 777777, func(a, b int32) int32 { return a + b }},
		{"sub", 654321, 123456, 530865, func(a, b int32) int32 { return a - b }},
		{"mul", 12345, 6789, 83810205, func(a, b int32) int32 { return a * b }},
		{"div", 777777, 3, 259259, func(a, b int32) int32 { return a / b }},
		{"mod", 777777, 1000, 777, func(a, b int32) int32 { return a % b }},
	}
	uint32Tests = []numberTest[uint32]{
		{"add", 123456, 654321, 777777, func(a, b uint32) uint32 { return a + b }},
		{"sub", 700000, 123456, 576544, func(a, b uint32) uint32 { return a - b }},
		{"mul", 12345, 6789, 83810205, func(a, b uint32) uint32 { return a * b }},
		{"div", 777777, 3, 259259, func(a, b uint32) uint32 { return a / b }},
		{"mod", 777777, 1000, 777, func(a, b uint32) uint32 { return a % b }},
	}

	int64Tests = []numberTest[int64]{
		{"add", 1234567890, 9876543210, 11111111100, func(a, b int64) int64 { return a + b }},
		{"sub", 9876543210, 1234567890, 8641975320, func(a, b int64) int64 { return a - b }},
		{"mul", 123456, 789012, 97408265472, func(a, b int64) int64 { return a * b }},
		{"div", 11111111100, 3, 3703703700, func(a, b int64) int64 { return a / b }},
		{"mod", 11111111100, 1000, 100, func(a, b int64) int64 { return a % b }},
	}
	uint64Tests = []numberTest[uint64]{
		{"add", 1234567890, 9876543210, 11111111100, func(a, b uint64) uint64 { return a + b }},
		{"sub", 10000000000, 1234567890, 8765432110, func(a, b uint64) uint64 { return a - b }},
		{"mul", 123456, 789012, 97408265472, func(a, b uint64) uint64 { return a * b }},
		{"div", 11111111100, 3, 3703703700, func(a, b uint64) uint64 { return a / b }},
		{"mod", 11111111100, 1000, 100, func(a, b uint64) uint64 { return a % b }},
	}

	float32Tests = []numberTest[float32]{
		{"add", 1.2, 3.4, 4.6, func(a, b float32) float32 { return a + b }},
		{"sub", 3.4, 1.2, 2.2, func(a, b float32) float32 { return a - b }},
		{"mul", 1.2, 3.4, 4.08, func(a, b float32) float32 { return a * b }},
		{"div", 9.9, 3.3, 3, func(a, b float32) float32 { return a / b }},
		{"mod", 9.9, 3, 0.9, func(a, b float32) float32 { return float32(math.Mod(float64(a), float64(b))) }},
	}
	float64Tests = []numberTest[float64]{
		{"add", 16777217, 1, 16777218, func(a, b float64) float64 { return a + b }},
		{"sub", 16777219, 2, 16777217, func(a, b float64) float64 { return a - b }},
		{"mul", 16777217, 2, 33554434, func(a, b float64) float64 { return a * b }},
		{"div", 33554434, 2, 16777217, func(a, b float64) float64 { return a / b }},
		{"mod", 16777218, 2, 0, func(a, b float64) float64 { return math.Mod(a, b) }},
	}
)

func testNumbers() {
	validateAllNumberTests("   int8", int8Tests)
	validateAllNumberTests("  uint8", uint8Tests)
	validateAllNumberTests(" uint16", int16Tests)
	validateAllNumberTests(" uint13", uint16Tests)
	validateAllNumberTests("  int32", int32Tests)
	validateAllNumberTests(" uint32", uint32Tests)
	validateAllNumberTests("  int64", int64Tests)
	validateAllNumberTests(" uint64", uint64Tests)
	validateAllNumberTests("float32", float32Tests)
	validateAllNumberTests("float64", float64Tests)
}

func validateAllNumberTests[T comparable](group string, tests []numberTest[T]) {
	//debug.Clear()

	success := int32(0)
	debug.Printf("%s  ", group)
	for _, test := range tests {
		a, b, expected := test.left, test.right, test.result
		got := test.fn(a, b)

		debug.Printf("%s", test.name)
		equal := false
		switch any(got).(type) {
		case float32:
			const eps = 1e-5
			equal = math.Abs(float64(any(got).(float32))-float64(any(expected).(float32))) <= eps
		case float64:
			const eps = 1e-9
			equal = math.Abs(any(got).(float64)-any(expected).(float64)) <= eps
		default:
			equal = got == expected
		}
		if equal {
			debug.Printf("   ")
			success += 1
		} else {
			debug.Printf("?  ")

			if panicOnFail {
				debug.Printf("\n\n")
				debug.Printf("test %s %s failed\n", group, test.name)
				debug.Printf("a:      %v\n", a)
				debug.Printf("b:      %v\n", b)
				debug.Printf("wanted: %v\n", expected)
				debug.Printf("got:    %v\n", got)

				panic("program halted.")
			}
		}
	}

	debug.Printf("(%02d/%02d)  ", success, len(tests))
	if success == int32(len(tests)) {
		debug.Printf("[PASS]\n")
	} else {
		debug.Printf("[FAIL]\n")
	}
}
