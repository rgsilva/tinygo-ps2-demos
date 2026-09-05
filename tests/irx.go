package main

// The embedded IOP modules load through the SIF straight from the embedded
// data (16-byte aligned by go:align), which is what the demos rely on.

import (
	"fmt"
	"unsafe"

	"ps2go/harness"
	"ps2go/lib/sifrpc"
	"ps2go/resources"
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
	if err := sifrpc.ResetAndPatchIOP(); err != nil {
		return err
	}
	for _, m := range []struct {
		name string
		data []byte
	}{{"freesio2", resources.Freesio2}, {"freepad", resources.Freepad}} {
		id, err := sifrpc.LoadModuleBuffer(m.data)
		if err != nil {
			return err
		}
		harness.Logf("irx: %s (%d bytes) loaded as module %d", m.name, len(m.data), id)
	}
	// An unaligned buffer must be refused before it reaches the DMA.
	if _, err := sifrpc.LoadModuleBuffer(resources.Freepad[1:]); err == nil {
		return fmt.Errorf("unaligned module data was accepted")
	}
	return nil
}
