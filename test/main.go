package main

import "C"
import (
	"ps2go/debug"
	"ps2go/sifrpc"
)

const (
	panicOnFail = true
)

type (
	TestFunc struct {
		name string
		fn   func()
	}
)

var (
	tests = []func(){
		testNumbers,
		testStrings,
	}
)

func main() {
	//	debug.Init()
	//	debug.Printf("Start\n\n")
	//
	//	for i := 0.12345; i <= 12345; i = i * 10 {
	//		debug.Printf("%.02f\n", i)
	//	}
	//
	//	debug.Printf("\n\nEnd")
	//	for {
	//	}
	//
	debug.Init()
	sifrpc.ResetAndPatchIOP()

	debug.Printf(" \nPlayStation 2 TinyGo test tool\n\n")
	for _, fn := range tests {
		fn()
		debug.Printf(" \n")
	}

	debug.Printf(" \n- end -\n")
	for {
	}
}
