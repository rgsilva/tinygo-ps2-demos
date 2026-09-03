// Package resources holds the demos' assets and IOP modules, embedded at
// compile time. Everything is 16-byte aligned because the GS and the SIF read
// it by DMA. The IOP modules are copied from the ps2sdk by make (untracked).
//
// One declaration per file: TinyGo applies //go:align to standalone var
// declarations only, not to specs inside a var ( ... ) block.
package resources

import _ "embed"

// Textures (raw pixel data).

//go:align 16
//go:embed gopher.raw
var Gopher []byte

//go:align 16
//go:embed bird.raw
var Bird []byte

//go:align 16
//go:embed pipe.raw
var Pipe []byte

//go:align 16
//go:embed gameover.raw
var Gameover []byte

//go:align 16
//go:embed sky.raw
var Sky []byte

// Font.

//go:align 16
//go:embed arial.fnt
var Arial []byte

// IOP modules for the controller.

//go:align 16
//go:embed freesio2.irx
var Freesio2 []byte

//go:align 16
//go:embed freepad.irx
var Freepad []byte
