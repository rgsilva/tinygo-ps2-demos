package debug

/*
#define _EE
#include <stdlib.h>
#include <stdio.h>
#include <debug.h>
#include <sifrpc.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

var (
	isInDebug = false
)

func Init() {
	isInDebug = true
	C.sceSifInitRpc(0)
	C.init_scr()
}

func Printf(format string, args ...interface{}) {
	formatted := fmt.Sprintf(format, args...)

	str := C.CString(formatted)
	C.scr_printf(str)
	C.free(unsafe.Pointer(str))
}

func Clear() {
	C.scr_clear()
}
