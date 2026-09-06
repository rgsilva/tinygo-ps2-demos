package gskit

import "unsafe"

func uintptrOf(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }
