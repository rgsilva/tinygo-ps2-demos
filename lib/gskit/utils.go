package gskit

import "C"

func boolToCuchar(value bool) C.uchar {
	if value {
		return C.uchar(0x01)
	}
	return C.uchar(0x00)
}

func boolToCint(value bool) C.int {
	if value {
		return C.int(0x01)
	}
	return C.int(0x00)
}

func GS_SETREG_RGBAQ(r, g, b, a, q uint8) uint64 {
	return uint64(r) |
		(uint64(g) << 8) |
		(uint64(b) << 16) |
		(uint64(a) << 24) |
		(uint64(q) << 32)
}

func GS_SETREG_ALPHA(A, B, C, D uint8, FIX uint16) uint64 {
	return uint64(A)<<0 |
		uint64(B)<<2 |
		uint64(C)<<4 |
		uint64(D)<<6 |
		uint64(FIX)<<32
}

// GS_BLEND_SOURCE_ALPHA is the usual blend: (Cs - Cd) * As + Cd, so alpha
// 0 is transparent and 0x80 opaque. gsKit's default (GS_BLEND_BACK2FRONT)
// is the reverse; set this with SetPrimAlpha after InitScreen.
var GS_BLEND_SOURCE_ALPHA = GS_SETREG_ALPHA(0, 1, 0, 1, 0)
