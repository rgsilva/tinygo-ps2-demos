package ps2net

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"ps2go/lib/ps2ip"
	"tinygo.org/x/drivers/netdev"
)

// The stack's gethostbyname comes back through the RPC with IOP pointers
// in it, so names are resolved here: one A query over UDP to the
// configured server.

const (
	dnsPort     = 53
	dnsTimeout  = 2 * time.Second
	dnsAttempts = 3
)

var dnsID uint16

func dnsQuery(name string, id uint16) ([]byte, error) {
	q := make([]byte, 12, 12+len(name)+6)
	binary.BigEndian.PutUint16(q[0:], id)
	binary.BigEndian.PutUint16(q[2:], 0x0100) // recursion desired
	binary.BigEndian.PutUint16(q[4:], 1)      // one question
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, netdev.ErrMalAddr
		}
		q = append(q, byte(len(label)))
		q = append(q, label...)
	}
	q = append(q, 0, 0, 1, 0, 1) // end of name, type A, class IN
	return q, nil
}

// skipName steps over a (possibly compressed) name at pos.
func skipName(msg []byte, pos int) (int, error) {
	for {
		if pos >= len(msg) {
			return 0, errors.New("truncated name")
		}
		n := int(msg[pos])
		switch {
		case n == 0:
			return pos + 1, nil
		case n&0xC0 == 0xC0:
			return pos + 2, nil
		default:
			pos += 1 + n
		}
	}
}

// dnsAnswer returns the first A record of a response to the query id.
func dnsAnswer(msg []byte, id uint16) (netip.Addr, error) {
	if len(msg) < 12 || binary.BigEndian.Uint16(msg[0:]) != id {
		return netip.Addr{}, errors.New("bad response")
	}
	if rcode := msg[3] & 0xF; rcode != 0 {
		return netip.Addr{}, netdev.ErrHostUnknown
	}
	questions := int(binary.BigEndian.Uint16(msg[4:]))
	answers := int(binary.BigEndian.Uint16(msg[6:]))
	pos := 12
	var err error
	for i := 0; i < questions; i++ {
		if pos, err = skipName(msg, pos); err != nil {
			return netip.Addr{}, err
		}
		pos += 4
	}
	for i := 0; i < answers; i++ {
		if pos, err = skipName(msg, pos); err != nil {
			return netip.Addr{}, err
		}
		if pos+10 > len(msg) {
			return netip.Addr{}, errors.New("truncated answer")
		}
		typ := binary.BigEndian.Uint16(msg[pos:])
		rdlen := int(binary.BigEndian.Uint16(msg[pos+8:]))
		pos += 10
		if pos+rdlen > len(msg) {
			return netip.Addr{}, errors.New("truncated record")
		}
		if typ == 1 && rdlen == 4 {
			return netip.AddrFrom4([4]byte{msg[pos], msg[pos+1], msg[pos+2], msg[pos+3]}), nil
		}
		pos += rdlen
	}
	return netip.Addr{}, netdev.ErrHostUnknown
}

// resolve looks a name up at the stack's DNS server.
func (d *Driver) resolve(name string) (netip.Addr, error) {
	cfg, err := ps2ip.GetConfig()
	if err != nil {
		return netip.Addr{}, err
	}
	if !cfg.DNS.IsValid() || cfg.DNS.IsUnspecified() {
		return netip.Addr{}, errors.New("ps2net: no DNS server configured")
	}
	fd, err := d.Socket(netdev.AF_INET, netdev.SOCK_DGRAM, netdev.IPPROTO_UDP)
	if err != nil {
		return netip.Addr{}, err
	}
	defer d.Close(fd)
	if err := d.Connect(fd, "", netip.AddrPortFrom(cfg.DNS, dnsPort)); err != nil {
		return netip.Addr{}, err
	}
	buf := make([]byte, 512)
	for attempt := 0; attempt < dnsAttempts; attempt++ {
		dnsID++
		q, err := dnsQuery(name, dnsID)
		if err != nil {
			return netip.Addr{}, err
		}
		if _, err := d.Send(fd, q, 0, time.Now().Add(dnsTimeout)); err != nil {
			return netip.Addr{}, err
		}
		n, err := d.Recv(fd, buf, 0, time.Now().Add(dnsTimeout))
		if err == netdev.ErrTimeout {
			continue
		}
		if err != nil {
			return netip.Addr{}, err
		}
		return dnsAnswer(buf[:n], dnsID)
	}
	return netip.Addr{}, fmt.Errorf("ps2net: no DNS answer from %s", cfg.DNS)
}
