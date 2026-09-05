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

	// Networking, in load order: the DEV9 (expansion bay) driver, the
	// network manager, the ethernet driver, and the TCP/IP stack with its
	// netman glue and its EE-side socket RPC server.

	//go:align 16
	//go:embed ps2dev9.irx
	Ps2dev9 []byte
	//go:align 16
	//go:embed netman.irx
	Netman []byte
	//go:align 16
	//go:embed smap.irx
	Smap []byte
	//go:align 16
	//go:embed ps2ip-nm.irx
	Ps2ipNm []byte
	//go:align 16
	//go:embed ps2ip.irx
	Ps2ip []byte
	//go:align 16
	//go:embed ps2ips.irx
	Ps2ips []byte
)
