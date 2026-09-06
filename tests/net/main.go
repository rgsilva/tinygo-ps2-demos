// Network checks: `make check-net` runs them in PCSX2 with the emulated
// ethernet (harness --net). Not part of the main suite.
package main

import (
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"ps2go/lib/harness"
	"ps2go/lib/iop"
	"ps2go/lib/netman"
	"ps2go/lib/ps2ip"
	"ps2go/lib/ps2net"
	"ps2go/lib/rtc"
	"ps2go/lib/sifrpc"
	"ps2go/lib/tls13"
)

// The harness (--net) resolves this name to the host machine and runs an
// echo server and an HTTP server there.
const (
	host     = "host.ps2go"
	echoPort = 17777
	httpPort = 18080
	tlsPort  = 18443
)

func main() {
	harness.Run([]harness.Case{
		{Name: "net-modules", Fn: testModules},
		{Name: "net-link", Fn: testLink},
		{Name: "net-dhcp", Fn: testDHCP},
		{Name: "net-dns", Fn: testDNS},
		{Name: "net-raw", Fn: testRaw},
		{Name: "net-tcp-echo", Fn: testTCPEcho},
		{Name: "net-tcp-deadline", Fn: testTCPDeadline},
		{Name: "net-tcp-sizes", Fn: testTCPSizes},
		{Name: "net-http-get", Fn: testHTTPGet},
		{Name: "net-http-large", Fn: testHTTPLarge},
		{Name: "net-udp-echo", Fn: testUDPEcho},
		{Name: "tls-selftest", Fn: func() error { return tls13.SelfTest() }},
		{Name: "tls-local-handshake", Fn: testTLSLocal},
		{Name: "tls-bad-cert", Fn: testTLSBadCert},
		{Name: "tls-https-public", Fn: testTLSPublic},
		{Name: "net-tcp-listen", Fn: testTCPListen, XFail: "PCSX2's Sockets backend has no inbound connections"},
	})
}

// testModules loads the network stack onto the IOP.
func testModules() error {
	if err := sifrpc.ResetAndPatchIOP(); err != nil {
		return err
	}
	for _, m := range []struct {
		name string
		data []byte
	}{{"ps2dev9", iop.Ps2dev9}, {"netman", iop.Netman}, {"smap", iop.Smap}, {"ps2ip-nm", iop.Ps2ipNm}, {"ps2ips", iop.Ps2ips}} {
		id, err := sifrpc.LoadModuleBuffer(m.data)
		if err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}
		harness.Logf("%s: id %d", m.name, id)
		if m.name == "netman" {
			if err := netman.Init(); err != nil {
				return err
			}
		}
	}
	return ps2ip.Init()
}

// testLink brings the ethernet link up. The emulated adapter rejects link
// mode changes, so that is only logged.
func testLink() error {
	if err := netman.SetLinkMode(netman.LinkModeAuto); err != nil {
		harness.Logf("note: %v", err)
	}
	t := time.Now()
	if err := netman.WaitLink(3 * time.Second); err != nil {
		harness.Logf("note: %v (the emulated adapter never reports it)", err)
		return nil
	}
	harness.Logf("link up after %s", time.Since(t))
	return nil
}

// testDHCP gets a lease and prints the configuration.
func testDHCP() error {
	t := time.Now()
	if err := ps2ip.SetDHCP(20 * time.Second); err != nil {
		return err
	}
	c, err := ps2ip.GetConfig()
	if err != nil {
		return err
	}
	harness.Logf("lease after %s: %s", time.Since(t), c)
	if !c.IP.IsValid() || c.IP.IsUnspecified() || c.Gateway.IsUnspecified() {
		return fmt.Errorf("incomplete configuration: %s", c)
	}
	return nil
}

// testDNS resolves the host's name through the stack (the emulator answers).
func testDNS() error {
	ps2net.Use()
	t := time.Now()
	addrs, err := net.LookupHost(host)
	if err != nil {
		return err
	}
	harness.Logf("%s: %v in %s", host, addrs, time.Since(t))
	if len(addrs) == 0 || addrs[0] == "0.0.0.0" {
		return fmt.Errorf("no address for %s", host)
	}
	return nil
}

// testTCPEcho sends lines to the host's echo server and reads them back.
func testTCPEcho() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, echoPort))
	if err != nil {
		return err
	}
	defer conn.Close()
	msg := []byte("hello from the PS2\n")
	buf := make([]byte, 64)
	t := time.Now()
	for i := 0; i < 20; i++ {
		if _, err := conn.Write(msg); err != nil {
			return err
		}
		n, err := io.ReadFull(conn, buf[:len(msg)])
		if err != nil {
			return fmt.Errorf("round trip %d: %w", i, err)
		}
		if string(buf[:n]) != string(msg) {
			return fmt.Errorf("echo %d: got %q", i, buf[:n])
		}
	}
	harness.Logf("20 round trips in %s", time.Since(t))
	return nil
}

// testTCPDeadline: a read with nothing to read must time out, and other
// goroutines must keep running meanwhile.
func testTCPDeadline() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, echoPort))
	if err != nil {
		return err
	}
	defer conn.Close()
	ticks := 0
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				ticks++
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	t := time.Now()
	_, err = conn.Read(make([]byte, 16))
	took := time.Since(t)
	close(done)
	if err == nil {
		return fmt.Errorf("read returned data")
	}
	if ne, ok := err.(interface{ Timeout() bool }); !ok || !ne.Timeout() {
		return fmt.Errorf("not a timeout: %v", err)
	}
	harness.Logf("timed out after %s, %d ticks meanwhile", took, ticks)
	if took < 250*time.Millisecond || took > 2*time.Second || ticks < 5 {
		return fmt.Errorf("deadline off: %s, %d ticks", took, ticks)
	}
	return nil
}

// testHTTPGet fetches a page from the host's HTTP server.
func testHTTPGet() error {
	t := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/hello", host, httpPort))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	harness.Logf("%s, %d bytes in %s: %q", resp.Status, len(body), time.Since(t), body)
	if resp.StatusCode != 200 || string(body) != "hello from the host\n" {
		return fmt.Errorf("unexpected response %s %q", resp.Status, body)
	}
	return nil
}

// testRaw drives the driver by hand, logging each step.
func testRaw() error {
	d := &ps2net.Driver{}
	ip, err := d.GetHostByName(host)
	harness.Logf("resolve: %v %v", ip, err)
	if err != nil {
		return err
	}
	fd, err := d.Socket(2, 1, 6)
	harness.Logf("socket: fd %d %v", fd, err)
	if err != nil {
		return err
	}
	defer d.Close(fd)
	err = d.Connect(fd, "", netip.AddrPortFrom(ip, echoPort))
	harness.Logf("connect: %v", err)
	if err != nil {
		return err
	}
	n, err := d.Send(fd, []byte("ping\n"), 0, time.Time{})
	harness.Logf("send: %d %v", n, err)
	if err != nil {
		return err
	}
	buf := make([]byte, 16)
	n, err = d.Recv(fd, buf, 0, time.Now().Add(5*time.Second))
	harness.Logf("recv: %d %q %v", n, buf[:max(n, 0)], err)
	return err
}

// testHTTPLarge fetches a 200 KB body (many 1 KB RPC pieces, unaligned
// buffers) and checks its content.
func testHTTPLarge() error {
	t := time.Now()
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/large", host, httpPort))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	took := time.Since(t)
	harness.Logf("%s, %d bytes in %s (%.0f KB/s)", resp.Status, len(body), took, float64(len(body))/1024/took.Seconds())
	if len(body) != 200*1024 {
		return fmt.Errorf("got %d bytes", len(body))
	}
	for i, b := range body {
		if b != byte('a'+i%26) {
			return fmt.Errorf("byte %d is %q", i, b)
		}
	}
	return nil
}

// testUDPEcho sends datagrams to the host's UDP echo and reads them back.
func testUDPEcho() error {
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", host, echoPort))
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, 1500)
	for i := 0; i < 5; i++ {
		msg := []byte(fmt.Sprintf("datagram %d", i))
		if _, err := conn.Write(msg); err != nil {
			return err
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		if string(buf[:n]) != string(msg) {
			return fmt.Errorf("got %q", buf[:n])
		}
	}
	return nil
}

// testTCPListen accepts a connection: the guest listens, asks the host's
// echo service to connect back, and reads the host's greeting.
func testTCPListen() error {
	const port = 18081
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	defer ln.Close()
	ctl, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, echoPort))
	if err != nil {
		return err
	}
	defer ctl.Close()
	if _, err := fmt.Fprintf(ctl, "CONNECT %d\n", port); err != nil {
		return err
	}
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		c, err := ln.Accept()
		ch <- result{c, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return r.err
		}
		defer r.conn.Close()
		buf := make([]byte, 64)
		n, err := r.conn.Read(buf)
		if err != nil {
			return err
		}
		harness.Logf("accepted %s, got %q", r.conn.RemoteAddr(), buf[:n])
		if string(buf[:n]) != "knock knock\n" {
			return fmt.Errorf("got %q", buf[:n])
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("no connection within 5 s")
	}
}

// testTLSLocal handshakes with the harness's HTTPS server (self-signed, so
// without verification) and fetches a page over it.
func testTLSLocal() error {
	if _, err := rtc.Sync(); err != nil { // certificate dates need the real time
		return err
	}
	t := time.Now()
	c, err := tls13.Dial(fmt.Sprintf("%s:%d", host, tlsPort), &tls13.Config{InsecureSkipVerify: true})
	if err != nil {
		return err
	}
	defer c.Close()
	st := c.ConnectionState()
	harness.Logf("handshake in %s: version %#x suite %#x, %d certs, %s", time.Since(t), st.Version, st.CipherSuite,
		len(st.PeerCertificates), st.PeerCertificates[0].Subject)
	if st.Version != 0x0304 {
		return fmt.Errorf("version %#x", st.Version)
	}
	fmt.Fprintf(c, "GET /hello HTTP/1.0\r\nHost: %s\r\n\r\n", host)
	body, err := io.ReadAll(c)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(body), "HTTP/1.0 200") || !strings.HasSuffix(string(body), "hello from the host\n") {
		return fmt.Errorf("got %q", body)
	}
	return nil
}

// testTLSBadCert: the self-signed certificate must be rejected when
// verifying.
func testTLSBadCert() error {
	c, err := tls13.Dial(fmt.Sprintf("%s:%d", host, tlsPort), nil)
	if err == nil {
		c.Close()
		return fmt.Errorf("self-signed certificate accepted")
	}
	harness.Logf("rejected: %v", err)
	if _, ok := err.(x509.UnknownAuthorityError); !ok && !strings.Contains(err.Error(), "certificate") {
		return fmt.Errorf("unexpected error: %v", err)
	}
	return nil
}

// testTLSPublic fetches https://example.com/ through net/http with the
// chain verified against the embedded roots (the clock from the RTC).
func testTLSPublic() error {
	t := time.Now()
	resp, err := http.Get("https://example.com/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	harness.Logf("%s, %d bytes in %s", resp.Status, len(body), time.Since(t))
	if resp.StatusCode != 200 || !strings.Contains(string(body), "Example") {
		return fmt.Errorf("unexpected response %s", resp.Status)
	}
	return nil
}

// testTCPSizes echoes messages of many sizes (around the RPC chunk and
// cache line boundaries) and checks every byte, reading the echo either
// in one go or as a 5-byte "header" first, like a TLS record.
func testTCPSizes() error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, echoPort))
	if err != nil {
		return err
	}
	defer conn.Close()
	sizes := []int{1, 5, 63, 64, 65, 122, 127, 128, 129, 191, 192, 193, 895, 896, 897, 900, 959, 960, 961, 1023, 1024, 1025, 1500, 2000, 4000}
	buf := make([]byte, 4096)
	var failures []string
	for _, split := range []bool{false, true} {
		for _, n := range sizes {
			msg := make([]byte, n)
			for i := range msg {
				msg[i] = byte(i*7 + n)
			}
			if _, err := conn.Write(msg); err != nil {
				return err
			}
			got := buf[:0]
			for len(got) < n {
				want := n - len(got)
				if split && len(got) < 5 {
					want = 5 - len(got)
				}
				m, err := conn.Read(buf[len(got) : len(got)+want])
				if err != nil {
					return fmt.Errorf("size %d after %d: %w", n, len(got), err)
				}
				got = buf[:len(got)+m]
			}
			for i := range msg {
				if got[i] != msg[i] {
					lo := i - 4
					if lo < 0 {
						lo = 0
					}
					hi := i + 12
					if hi > n {
						hi = n
					}
					failures = append(failures, fmt.Sprintf("size %d split %v: byte %d: got %x want %x", n, split, i, got[lo:hi], msg[lo:hi]))
					break
				}
			}
		}
	}
	for _, f := range failures {
		harness.Logf("%s", f)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d of %d transfers corrupted", len(failures), 2*len(sizes))
	}
	return nil
}
