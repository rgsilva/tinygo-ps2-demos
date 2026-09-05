// Package fonts holds gsKit fonts (FNT format), embedded at compile time.
package fonts

import _ "embed"

//go:align 16
//go:embed arial.fnt
var Arial []byte
