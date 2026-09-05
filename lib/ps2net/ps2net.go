// Package ps2net is the PlayStation 2 network driver for TinyGo's net
// package: a netdev.Netdever over the ps2ips RPC to the TCP/IP stack on
// the IOP. Up brings the whole stack up; after Use, net, net/http and
// friends work.
package ps2net

/*
#include <stdlib.h>
int ps2go_socket(int domain, int type, int proto);
int ps2go_bind(int fd, unsigned int ip, unsigned short port);
int ps2go_connect(int fd, unsigned int ip, unsigned short port);
int ps2go_listen(int fd, int backlog);
int ps2go_accept(int fd, unsigned int *ip, unsigned short *port);
int ps2go_send(int fd, const void *buf, int len, int flags);
int ps2go_recv(int fd, void *buf, int len, int flags);
int ps2go_ready(int fd, int want);
int ps2go_close(int fd);
int ps2go_setsockopt(int fd, int level, int opt, int value);
const char *ps2go_strerror(int e);
*/
import "C"

import (
	"fmt"
	"net/netip"
	"time"
	"unsafe"

	"ps2go/lib/ps2ip"
	"tinygo.org/x/drivers/netdev"
)

// Driver implements netdev.Netdever.
type Driver struct{}

// Use installs the driver: net.Dial, net.Listen, net/http use it from
// then on. Call it after Up (or after configuring the stack by hand).
func Use() {
	netdev.UseNetdev(&Driver{})
}

// A Recv/Send/Accept first waits for the socket to be ready, sleeping
// pollInterval between checks so other goroutines run (the RPC calls
// block the whole EE thread), until its deadline.
const pollInterval = 5 * time.Millisecond

// wait polls until the socket is readable (want 0) or writable (want 1),
// or the deadline passes.
func wait(sockfd int, want int, deadline time.Time) error {
	for {
		ret := C.ps2go_ready(C.int(sockfd), C.int(want))
		if ret < 0 {
			return cerr("select", ret)
		}
		if ret > 0 {
			return nil
		}
		if !deadline.IsZero() && !time.Now().Before(deadline) {
			return netdev.ErrTimeout
		}
		time.Sleep(pollInterval)
	}
}

func cerr(op string, ret C.int) error {
	return fmt.Errorf("ps2net: %s: %s (%d)", op, C.GoString(C.ps2go_strerror(-ret)), int(-ret))
}

func toC(a netip.Addr) C.uint {
	if !a.IsValid() || !a.Is4() {
		return 0
	}
	b := a.As4()
	return C.uint(b[0]) | C.uint(b[1])<<8 | C.uint(b[2])<<16 | C.uint(b[3])<<24
}

func fromC(n C.uint) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
}

func (d *Driver) GetHostByName(name string) (netip.Addr, error) {
	if a, err := netip.ParseAddr(name); err == nil {
		return a, nil
	}
	return d.resolve(name)
}

func (d *Driver) Addr() (netip.Addr, error) {
	c, err := ps2ip.GetConfig()
	if err != nil {
		return netip.Addr{}, err
	}
	return c.IP, nil
}

func (d *Driver) Socket(domain int, stype int, protocol int) (int, error) {
	fd := C.ps2go_socket(C.int(domain), C.int(stype), C.int(protocol))
	if fd < 0 {
		return -1, cerr("socket", fd)
	}
	return int(fd), nil
}

func (d *Driver) Bind(sockfd int, ip netip.AddrPort) error {
	if ret := C.ps2go_bind(C.int(sockfd), toC(ip.Addr()), C.ushort(ip.Port())); ret < 0 {
		return cerr("bind", ret)
	}
	return nil
}

func (d *Driver) Connect(sockfd int, host string, ip netip.AddrPort) error {
	if ret := C.ps2go_connect(C.int(sockfd), toC(ip.Addr()), C.ushort(ip.Port())); ret < 0 {
		return cerr("connect", ret)
	}
	return nil
}

func (d *Driver) Listen(sockfd int, backlog int) error {
	if ret := C.ps2go_listen(C.int(sockfd), C.int(backlog)); ret < 0 {
		return cerr("listen", ret)
	}
	return nil
}

func (d *Driver) Accept(sockfd int) (int, netip.AddrPort, error) {
	if err := wait(sockfd, 0, time.Time{}); err != nil {
		return -1, netip.AddrPort{}, err
	}
	var ip C.uint
	var port C.ushort
	fd := C.ps2go_accept(C.int(sockfd), &ip, &port)
	if fd < 0 {
		return -1, netip.AddrPort{}, cerr("accept", fd)
	}
	return int(fd), netip.AddrPortFrom(fromC(ip), uint16(port)), nil
}

// Send writes what the socket takes now (up to the IOP's 1 KB per call,
// looped inside); the net package calls again for the rest.
func (d *Driver) Send(sockfd int, buf []byte, flags int, deadline time.Time) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	if err := wait(sockfd, 1, deadline); err != nil {
		return -1, err
	}
	n := C.ps2go_send(C.int(sockfd), unsafe.Pointer(&buf[0]), C.int(len(buf)), C.int(flags))
	if n < 0 {
		return -1, cerr("send", n)
	}
	return int(n), nil
}

func (d *Driver) Recv(sockfd int, buf []byte, flags int, deadline time.Time) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	if err := wait(sockfd, 0, deadline); err != nil {
		return -1, err
	}
	n := C.ps2go_recv(C.int(sockfd), unsafe.Pointer(&buf[0]), C.int(len(buf)), C.int(flags))
	if n < 0 {
		return -1, cerr("recv", n)
	}
	return int(n), nil
}

func (d *Driver) Close(sockfd int) error {
	if ret := C.ps2go_close(C.int(sockfd)); ret < 0 {
		return cerr("close", ret)
	}
	return nil
}

func (d *Driver) SetSockOpt(sockfd int, level int, opt int, value interface{}) error {
	var v int
	switch x := value.(type) {
	case int:
		v = x
	case bool:
		if x {
			v = 1
		}
	case time.Duration:
		v = int(x / time.Second)
	default:
		return netdev.ErrNotSupported
	}
	if ret := C.ps2go_setsockopt(C.int(sockfd), C.int(level), C.int(opt), C.int(v)); ret < 0 {
		return cerr("setsockopt", ret)
	}
	return nil
}
