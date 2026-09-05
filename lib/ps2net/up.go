package ps2net

import (
	"fmt"
	"time"

	"ps2go/lib/iop"
	"ps2go/lib/netman"
	"ps2go/lib/ps2ip"
	"ps2go/lib/sifrpc"
)

// Up loads the network modules onto the IOP (after an IOP reset, so do it
// before loading anything else), waits for the link, configures the
// addresses (DHCP when cfg.DHCP, else cfg's addresses) and installs the
// driver. It returns the configuration in effect.
func Up(cfg ps2ip.Config, timeout time.Duration) (ps2ip.Config, error) {
	if err := sifrpc.ResetAndPatchIOP(); err != nil {
		return ps2ip.Config{}, err
	}
	for _, m := range []struct {
		name string
		data []byte
	}{{"ps2dev9", iop.Ps2dev9}, {"netman", iop.Netman}, {"smap", iop.Smap}, {"ps2ip-nm", iop.Ps2ipNm}, {"ps2ips", iop.Ps2ips}} {
		if _, err := sifrpc.LoadModuleBuffer(m.data); err != nil {
			return ps2ip.Config{}, fmt.Errorf("ps2net: %s: %w", m.name, err)
		}
		if m.name == "netman" {
			if err := netman.Init(); err != nil {
				return ps2ip.Config{}, err
			}
		}
	}
	if err := ps2ip.Init(); err != nil {
		return ps2ip.Config{}, err
	}
	// The emulated adapter rejects link modes and never reports the link
	// up; DHCP (or the first connection) is the real test.
	netman.SetLinkMode(netman.LinkModeAuto)
	netman.WaitLink(3 * time.Second)
	var err error
	if cfg.DHCP {
		err = ps2ip.SetDHCP(timeout)
	} else {
		err = ps2ip.SetStatic(cfg)
	}
	if err != nil {
		return ps2ip.Config{}, err
	}
	Use()
	return ps2ip.GetConfig()
}
