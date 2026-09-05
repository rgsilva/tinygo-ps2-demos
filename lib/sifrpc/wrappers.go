package sifrpc

/*
#cgo LDFLAGS: -lpatches
#define _EE
#include <stdlib.h>
#include <kernel.h>
#include <sifrpc.h>
#include <loadfile.h>
#include <sbv_patches.h>
#include <iopcontrol.h>

static int resetAndPatchIOP()
{
	SifInitRpc(0);
	while(!SifIopReset("", 0)){};
	while(!SifIopSync()){};
	SifInitRpc(0);

	int ret = sbv_patch_enable_lmb();
	if (ret != 0) {
		return ret;
	}

	return sbv_patch_disable_prefix_check();
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// ResetAndPatchIOP resets the IOP and applies the sbv patches so that modules
// can be loaded from EE memory.
func ResetAndPatchIOP() error {
	if ret := int(C.resetAndPatchIOP()); ret < 0 {
		return fmt.Errorf("sifrpc: IOP reset failed: %d", ret)
	}
	return nil
}

// LoadModule loads an IRX module from a path (host:, mc0:, cdrom0:, ...) and
// returns its module id.
func LoadModule(path string) (int, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	id := int(C.SifLoadModule(cPath, 0, nil))
	if id < 0 {
		return id, fmt.Errorf("sifrpc: loading %s failed: %d", path, id)
	}
	return id, nil
}

// LoadModuleBuffer loads an IRX module from EE memory and returns its module
// id. The data is read by DMA, so it must be 16-byte aligned (embedded
// modules are, see the iop package).
func LoadModuleBuffer(data []byte) (int, error) {
	if len(data) == 0 {
		return -1, fmt.Errorf("sifrpc: empty module")
	}
	p := unsafe.Pointer(&data[0])
	if uintptr(p)%16 != 0 {
		return -1, fmt.Errorf("sifrpc: module data at %#x is not 16-byte aligned", uintptr(p))
	}
	id := int(C.SifExecModuleBuffer(p, C.uint(len(data)), 0, nil, nil))
	if id < 0 {
		return id, fmt.Errorf("sifrpc: loading module from memory failed: %d", id)
	}
	return id, nil
}
