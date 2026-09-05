package main

import _ "embed"

// Textures (raw pixel data), 16-byte aligned because the GS reads them by
// DMA. The PNGs next to them are the sources.
var (
	//go:align 16
	//go:embed assets/gopher.raw
	texGopher []byte
	//go:align 16
	//go:embed assets/bird.raw
	texBird []byte
	//go:align 16
	//go:embed assets/pipe.raw
	texPipe []byte
	//go:align 16
	//go:embed assets/gameover.raw
	texGameover []byte
	//go:align 16
	//go:embed assets/sky.raw
	texSky []byte
)
