package main

// The embedded IOP modules load through the SIF straight from the embedded
// data (16-byte aligned by go:align), which is what the demos rely on.

/*
#define _EE
#include <loadfile.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"ps2go/harness"
	"ps2go/resources"
	"ps2go/sifrpc"
)

func testIRX() error {
	for _, m := range []struct {
		name string
		data []byte
	}{{"freesio2", resources.Freesio2}, {"freepad", resources.Freepad}} {
		if len(m.data) == 0 || uintptr(unsafe.Pointer(&m.data[0]))%16 != 0 {
			return fmt.Errorf("%s: %d bytes at %#x", m.name, len(m.data), uintptr(unsafe.Pointer(&m.data[0])))
		}
	}
	sifrpc.ResetAndPatchIOP()
	for _, m := range []struct {
		name string
		data []byte
	}{{"freesio2", resources.Freesio2}, {"freepad", resources.Freepad}} {
		id := int(C.SifExecModuleBuffer(unsafe.Pointer(&m.data[0]), C.uint(len(m.data)), 0, nil, nil))
		harness.Logf("irx: %s (%d bytes) loaded as module %d", m.name, len(m.data), id)
		if id < 0 {
			return fmt.Errorf("%s: SifExecModuleBuffer returned %d", m.name, id)
		}
	}
	return nil
}
