// Package ps2ip configures the IOP's TCP/IP stack (ps2ip-nm.irx) through
// its EE-side RPC (ps2ips.irx): addresses, DHCP and the DNS server. The
// sockets themselves are the libc ones (sys/socket.h), used by lib/netdev.
package ps2ip

/*
#cgo LDFLAGS: -lps2ips

// One interface's configuration; addresses are IPv4 in network byte order.
struct ps2go_ipconfig {
	int dhcp;
	int dhcp_bound;
	unsigned int ip, netmask, gateway, dns;
};
int ps2go_ip_init(void);
int ps2go_ip_getconfig(struct ps2go_ipconfig *c);
int ps2go_ip_setconfig(const struct ps2go_ipconfig *c);
void ps2go_dns_setserver(unsigned int dns);
*/
import "C"

import (
	"fmt"
	"net/netip"
	"time"
)

// Config is the interface's IPv4 configuration.
type Config struct {
	DHCP    bool
	Bound   bool // DHCP: a lease is held
	IP      netip.Addr
	Netmask netip.Addr
	Gateway netip.Addr
	DNS     netip.Addr
}

func (c Config) String() string {
	mode := "static"
	if c.DHCP {
		mode = "dhcp"
		if !c.Bound {
			mode = "dhcp (no lease)"
		}
	}
	return fmt.Sprintf("%s ip %s mask %s gw %s dns %s", mode, c.IP, c.Netmask, c.Gateway, c.DNS)
}

func fromC(n C.uint) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
}

func toC(a netip.Addr) C.uint {
	if !a.IsValid() {
		return 0
	}
	b := a.As4()
	return C.uint(b[0]) | C.uint(b[1])<<8 | C.uint(b[2])<<16 | C.uint(b[3])<<24
}

// Init binds to the stack's RPC. Load the modules (see iop) first.
func Init() error {
	if ret := C.ps2go_ip_init(); ret != 0 {
		return fmt.Errorf("ps2ip: ps2ip_init returned %d", int(ret))
	}
	return nil
}

// GetConfig reads the interface's current configuration.
func GetConfig() (Config, error) {
	var c C.struct_ps2go_ipconfig
	if ret := C.ps2go_ip_getconfig(&c); ret != 0 {
		return Config{}, fmt.Errorf("ps2ip: getconfig returned %d", int(ret))
	}
	return Config{
		DHCP: c.dhcp != 0, Bound: c.dhcp_bound != 0,
		IP: fromC(c.ip), Netmask: fromC(c.netmask), Gateway: fromC(c.gateway), DNS: fromC(c.dns),
	}, nil
}

// SetStatic configures the addresses by hand.
func SetStatic(c Config) error {
	cc := C.struct_ps2go_ipconfig{dhcp: 0, ip: toC(c.IP), netmask: toC(c.Netmask), gateway: toC(c.Gateway)}
	if ret := C.ps2go_ip_setconfig(&cc); ret != 0 {
		return fmt.Errorf("ps2ip: setconfig returned %d", int(ret))
	}
	C.ps2go_dns_setserver(toC(c.DNS))
	return nil
}

// SetDHCP enables DHCP and waits up to timeout for a lease. The DNS
// server comes from the lease.
func SetDHCP(timeout time.Duration) error {
	cc := C.struct_ps2go_ipconfig{dhcp: 1}
	if ret := C.ps2go_ip_setconfig(&cc); ret != 0 {
		return fmt.Errorf("ps2ip: setconfig returned %d", int(ret))
	}
	deadline := time.Now().Add(timeout)
	for {
		c, err := GetConfig()
		if err != nil {
			return err
		}
		if c.Bound {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("ps2ip: no DHCP lease after %s", timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
