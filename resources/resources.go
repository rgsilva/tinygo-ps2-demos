// Package resources holds the demos' assets and IOP modules, embedded at
// compile time. Everything is 16-byte aligned because the GS and the SIF read
// it by DMA. The IOP modules are copied from the ps2sdk by make (untracked).
package resources

import _ "embed"

var (
	// Textures (raw pixel data).

	//go:align 16
	//go:embed gopher.raw
	Gopher []byte
	//go:align 16
	//go:embed bird.raw
	Bird []byte
	//go:align 16
	//go:embed pipe.raw
	Pipe []byte
	//go:align 16
	//go:embed gameover.raw
	Gameover []byte
	//go:align 16
	//go:embed sky.raw
	Sky []byte

	// Font.

	//go:align 16
	//go:embed arial.fnt
	Arial []byte

	// IOP modules for the controller.

	//go:align 16
	//go:embed freesio2.irx
	Freesio2 []byte
	//go:align 16
	//go:embed freepad.irx
	Freepad []byte
)
