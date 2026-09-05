package main

import _ "embed"

// Textures (raw pixel data), 16-byte aligned because the GS reads them by
// DMA. Made from the PNGs next to them by tools/png2raw.py (make).
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
