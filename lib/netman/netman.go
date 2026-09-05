// Package netman drives the IOP's network manager (netman.irx): the link
// between the ethernet driver (smap.irx) and the TCP/IP stack.
package netman

/*
#cgo LDFLAGS: -lnetman
int ps2go_netman_init(void);
int ps2go_netman_set_link_mode(int mode);
int ps2go_netman_link_up(void);
*/
import "C"

import (
	"fmt"
	"time"
)

// Ethernet link modes (NETMAN_NETIF_ETH_LINK_MODE_*).
const (
	LinkModeAuto     = 0
	LinkMode10HDX    = 1
	LinkMode10FDX    = 2
	LinkMode100HDX   = 3
	LinkMode100FDX   = 4
	LinkDisablePause = 0x40 // or it into a mode
)

// Init binds to netman.irx. Load ps2dev9.irx and netman.irx first.
func Init() error {
	if ret := C.ps2go_netman_init(); ret != 0 {
		return fmt.Errorf("netman: NetManInit returned %d", int(ret))
	}
	return nil
}

// SetLinkMode sets the ethernet link mode (LinkMode*).
func SetLinkMode(mode int) error {
	if ret := C.ps2go_netman_set_link_mode(C.int(mode)); ret != 0 {
		return fmt.Errorf("netman: NetManSetLinkMode(%d) returned %d", mode, int(ret))
	}
	return nil
}

// LinkUp reports whether the ethernet link is up.
func LinkUp() bool {
	return C.ps2go_netman_link_up() != 0
}

// WaitLink waits for the link to come up.
func WaitLink(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for !LinkUp() {
		if time.Now().After(deadline) {
			return fmt.Errorf("netman: link not up after %s", timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
