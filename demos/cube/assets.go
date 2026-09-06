package main

import _ "embed"

// Textures (CT32, 16-byte aligned for the GS DMA): the gopher on the cube's
// faces, the sky behind it (sky.raw is made from sky.png by make).
var (
	//go:align 16
	//go:embed assets/gopher.raw
	texGopher []byte
	//go:align 16
	//go:embed assets/sky.raw
	texSky []byte
)
