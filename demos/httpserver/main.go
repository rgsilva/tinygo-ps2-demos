// HTTP server: brings the network up and serves a status page. Open
// http://<the console's IP>:8080/ from a browser on the same network.
package main

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"ps2go/lib/debug"
	"ps2go/lib/ps2ip"
	"ps2go/lib/ps2net"
)

const port = 8080

var (
	start    = time.Now()
	requests int
)

func page(w http.ResponseWriter, r *http.Request) {
	requests++
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<html><head><title>PlayStation 2</title></head><body style=\"font-family:sans-serif\">\n")
	fmt.Fprintf(w, "<h1>Hello from a PlayStation 2</h1>\n")
	fmt.Fprintf(w, "<p>Served by TinyGo on the Emotion Engine.</p>\n<ul>\n")
	fmt.Fprintf(w, "<li>Uptime: %s</li>\n", time.Since(start).Round(time.Second))
	fmt.Fprintf(w, "<li>Requests: %d</li>\n", requests)
	fmt.Fprintf(w, "<li>Heap in use: %d KB, idle: %d KB</li>\n", m.HeapInuse/1024, m.HeapIdle/1024)
	fmt.Fprintf(w, "<li>Collections: %d</li>\n", m.NumGC)
	fmt.Fprintf(w, "<li>Your address: %s</li>\n", r.RemoteAddr)
	fmt.Fprintf(w, "</ul></body></html>\n")
	debug.Printf("%s %s from %s\n", r.Method, r.URL.Path, r.RemoteAddr)
}

func main() {
	debug.Init()
	debug.Printf("PS2 TinyGo HTTP server\n\n")
	debug.Printf("Bringing the network up (DHCP)...\n")
	cfg, err := ps2net.Up(ps2ip.Config{DHCP: true}, 30*time.Second)
	if err != nil {
		debug.Printf("fatal: %v\n", err)
		for {
		}
	}
	debug.Printf("Network up: %s\n\n", cfg)

	http.HandleFunc("/", page)
	debug.Printf("Listening on http://%s:%d/\n", cfg.IP, port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		debug.Printf("error: %v\n", err)
	}
	for {
		time.Sleep(time.Second)
	}
}
