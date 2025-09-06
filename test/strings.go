package main

import (
	"fmt"
	"ps2go/debug"
)

type genericTest[T comparable] struct {
	name     string
	fn       func() T
	expected T
}

var (
	stringTests = []genericTest[string]{
		{"emp", func() string { return "" }, ""},
		{"cst", func() string { return "Hello, World!" }, "Hello, World!"},
		{"app", func() string { return "Hello, " + "World!" }, "Hello, World!"},
	}

	formatStringTests = []genericTest[string]{
		{" %%s", func() string { return fmt.Sprintf("%s!", "abc") }, "abc!"},
		{" %%v", func() string { return fmt.Sprintf("%v!", "abc") }, "abc!"},
	}

	formatIntegerTests = []genericTest[string]{
		{" s8", func() string { return fmt.Sprintf("%d", int8(42)) }, "42"},
		{" u8", func() string { return fmt.Sprintf("%d", uint8(42)) }, "42"},
		{"s16", func() string { return fmt.Sprintf("%d", int16(1234)) }, "1234"},
		{"u16", func() string { return fmt.Sprintf("%d", uint16(1234)) }, "1234"},
		{"s32", func() string { return fmt.Sprintf("%d", int32(123456)) }, "123456"},
		{"u32", func() string { return fmt.Sprintf("%d", uint32(123456)) }, "123456"},
		{"s64", func() string { return fmt.Sprintf("%d", int64(1234567890)) }, "1234567890"},
		{"u64", func() string { return fmt.Sprintf("%d", uint64(1234567890)) }, "1234567890"},
	}

	formatFloatTests = []genericTest[string]{
		{"32f", func() string { return fmt.Sprintf("%.5f", float32(1.234)) }, "1.23400"},
		{"32v", func() string { return fmt.Sprintf("%v", float32(1.234)) }, "1.234"},
		{"64f", func() string { return fmt.Sprintf("%.5f", float64(123456789.123456789)) }, "123456789.12346"},
		{"64v", func() string { return fmt.Sprintf("%v", float64(123456789.123456789)) }, "1.2345678912345679e+08"},
	}
)

func testStrings() {
	validateAllGeneric(" simple", stringTests)
	validateAllGeneric("fmt.str", formatStringTests)
	validateAllGeneric("fmt.int", formatIntegerTests)
	validateAllGeneric("fmt.flt", formatFloatTests)
}

func validateAllGeneric[T comparable](group string, tests []genericTest[T]) {
	//debug.Clear()

	success := int32(0)
	debug.Printf("%s  ", group)
	for _, test := range tests {
		got := test.fn()

		debug.Printf("%s", test.name)
		if got == test.expected {
			debug.Printf("   ")
			success += 1
		} else {
			debug.Printf("?  ")

			if panicOnFail {
				debug.Printf("\n\n")
				debug.Printf("test %s %s failed\n", group, test.name)
				debug.Printf("wanted: %v\n", test.expected)
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
