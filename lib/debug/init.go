// Package debug prints text on the screen (libdebug) and on the serial port.
package debug

/*
#define _EE
#include <stdlib.h>
#include <debug.h>
#include <sio.h>

// scr_printf is printf-formatted; print the text as an argument so that a
// '%' in it is not a directive.
static void scr_puts(const char *s) {
	scr_printf("%s", s);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func Init() {
	C.init_scr()
}

// Printf formats and prints on the screen and on the serial port (emulator
// log, harness).
func Printf(format string, args ...interface{}) {
	str := C.CString(fmt.Sprintf(format, args...))
	C.scr_puts(str)
	C.sio_putsn(str)
	C.free(unsafe.Pointer(str))
}

func Clear() {
	C.scr_clear()
}
