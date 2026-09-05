// HTTP client: brings the network up (DHCP) and fetches a page, printing
// the status and the start of the body on screen and on the serial port.
package main

import (
	"io"
	"net/http"
	"time"

	"ps2go/lib/debug"
	"ps2go/lib/ps2ip"
	"ps2go/lib/ps2net"
)

const url = "http://theoldnet.com/"

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
	debug.Printf("Network up: %s\n\n", cfg)

	debug.Printf("GET %s\n", url)
	t := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		debug.Printf("error: %v\n", err)
		for {
		}
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	debug.Printf("Status: %s (%d bytes in %s)\n", resp.Status, len(body), time.Since(t).Round(time.Millisecond))
	if err != nil {
		debug.Printf("body error: %v\n", err)
	}
	if len(body) > 512 {
		body = body[:512]
	}
	debug.Printf("\n%s\n", body)
	for {
		time.Sleep(time.Second)
	}
}
