// HTTP client: brings the network up (DHCP) and fetches a page over plain
// HTTP and one over HTTPS (TLS 1.3, certificate checked against the
// embedded roots), printing the status and the start of the bodies on
// screen and on the serial port.
package main

import (
	"io"
	"net/http"
	"time"

	"ps2go/lib/debug"
	"ps2go/lib/ps2ip"
	"ps2go/lib/ps2net"
	"ps2go/lib/rtc"
	"ps2go/lib/tls13"
)

const (
	url      = "http://theoldnet.com/"
	httpsURL = "https://example.com/"
	tlsHost  = "example.com"
)

func main() {
	debug.Init()
	debug.Printf("PS2 TinyGo HTTP client\n\n")
	debug.Printf("Bringing the network up (DHCP)...\n")
	cfg, err := ps2net.Up(ps2ip.Config{DHCP: true}, 30*time.Second)
	if err != nil {
		debug.Printf("fatal: %v\n", err)
		for {
		}
	}
	debug.Printf("Network up: %s\n", cfg)
	if _, err := rtc.Sync(); err != nil {
		debug.Printf("rtc: %v (certificate dates cannot be checked)\n", err)
	}
	debug.Printf("\n")

	get(url, 512)

	debug.Printf("\nTLS handshake with %s... ", tlsHost)
	t := time.Now()
	c, err := tls13.Dial(tlsHost+":443", nil)
	if err != nil {
		debug.Printf("error: %v\n", err)
	} else {
		st := c.ConnectionState()
		debug.Printf("%s: TLS %#x, cipher suite %#x, %s\n", time.Since(t).Round(time.Millisecond), st.Version, st.CipherSuite,
			st.PeerCertificates[0].Subject.CommonName)
		c.Close()
	}
	get(httpsURL, 256)
	for {
		time.Sleep(time.Second)
	}
}

func get(url string, show int) {
	debug.Printf("GET %s\n", url)
	t := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		debug.Printf("error: %v\n", err)
		return
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	debug.Printf("Status: %s (%d bytes in %s)\n", resp.Status, len(body), time.Since(t).Round(time.Millisecond))
	if err != nil {
		debug.Printf("body error: %v\n", err)
	}
	if len(body) > show {
		body = body[:show]
	}
	debug.Printf("%s\n", body)
}
