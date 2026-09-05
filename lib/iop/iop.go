// Package iop holds the IOP modules (IRX) the programs load, embedded at
// compile time. They are copied from the ps2sdk by make (untracked) and are
// 16-byte aligned because the SIF reads them by DMA.
package iop

import _ "embed"

var (
	// Controller support: the SIO2 driver and the pad driver.

	//go:align 16
	//go:embed freesio2.irx
	Freesio2 []byte
	//go:align 16
	//go:embed freepad.irx
	Freepad []byte
)
