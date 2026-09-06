package ps2net

import (
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"

	"ps2go/lib/tls13"
	"tinygo.org/x/drivers/netdev"
)

// TLS sockets: the net package asks the driver for protocol IPPROTO_TLS
// (0xFE) sockets and connects them by host name (net.DialTLS, and https
// in net/http). Each one is a TCP connection through this driver wrapped
// in a tls13 client; the handshake runs in Connect with SNI and chain
// verification against the embedded roots.

const ipprotoTLS = 0xFE

// tlsSocket is one TLS socket; its fd is an index into tlsSockets plus
// tlsFdBase, well above any IOP socket number.
type tlsSocket struct {
	conn *tls13.Conn
	raw  net.Conn
}

const tlsFdBase = 0x10000

var (
	tlsMu      sync.Mutex
	tlsSockets = map[int]*tlsSocket{}
	tlsNext    = tlsFdBase
)

// TLSConfig is applied to every TLS socket the net package opens (for
// InsecureSkipVerify, RootCAs, Time); ServerName is set per connection.
var TLSConfig tls13.Config

func tlsGet(fd int) *tlsSocket {
	tlsMu.Lock()
	defer tlsMu.Unlock()
	return tlsSockets[fd]
}

func tlsNewSocket() int {
	tlsMu.Lock()
	defer tlsMu.Unlock()
	fd := tlsNext
	tlsNext++
	tlsSockets[fd] = &tlsSocket{}
	return fd
}

func tlsConnect(s *tlsSocket, host string, ip netip.AddrPort) error {
	if host == "" {
		return errors.New("ps2net: TLS needs a host name")
	}
	if !ip.Addr().IsValid() || ip.Addr().IsUnspecified() {
		a, err := (&Driver{}).GetHostByName(host)
		if err != nil {
			return err
		}
		ip = netip.AddrPortFrom(a, ip.Port())
	}
	raw, err := net.Dial("tcp", ip.String())
	if err != nil {
		return err
	}
	cfg := TLSConfig
	cfg.ServerName = host
	c := tls13.Client(raw, &cfg)
	if err := c.Handshake(); err != nil {
		raw.Close()
		return err
	}
	s.raw, s.conn = raw, c
	return nil
}

func tlsSend(s *tlsSocket, buf []byte, deadline time.Time) (int, error) {
	if s.conn == nil {
		return -1, netdev.ErrInvalidSocketFd
	}
	s.raw.SetWriteDeadline(deadline)
	return s.conn.Write(buf)
}

func tlsRecv(s *tlsSocket, buf []byte, deadline time.Time) (int, error) {
	if s.conn == nil {
		return -1, netdev.ErrInvalidSocketFd
	}
	s.raw.SetReadDeadline(deadline)
	return s.conn.Read(buf)
}

func tlsClose(fd int, s *tlsSocket) error {
	tlsMu.Lock()
	delete(tlsSockets, fd)
	tlsMu.Unlock()
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
