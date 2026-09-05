// Package fonts holds gsKit fonts (FNT format), embedded at compile time and
// 16-byte aligned because the GS reads them by DMA.
package fonts

import _ "embed"

//go:align 16
//go:embed arial.fnt
var Arial []byte
