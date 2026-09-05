// Image stream: fetches 256x256 textures from a server over a small TCP
// protocol (tools/imageserver.py) and draws them as they arrive, with the
// frame rate and memory stats. Made for a conference talk.
package main

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"ps2go/lib/debug"
	"ps2go/lib/dmakit"
	"ps2go/lib/fonts"
	"ps2go/lib/gskit"
	"ps2go/lib/ps2ip"
	"ps2go/lib/ps2net"
)

// The server: tools/imageserver.py on the host (host.ps2go is what the
// harness calls the host machine; on a real network put the address here).
const server = "host.ps2go:9001"

const (
	texSize   = 256
	texBytes  = texSize * texSize * 4
	fetchEach = 30 // frames
)

var (
	gs      gskit.GSGlobal
	tex     *gskit.GSTexture
	font    *gskit.GSFont
	pixels  = make([]byte, texBytes) // the current image
	decoded = make([]byte, texBytes) // base64-decoded reply
	line    = make([]byte, 0, 4*texBytes/3+64)
	fetches int
	lastErr error
)

// protocol: a line per command; PING answers PONG, FETCH answers the
// next image as one base64 line.
type protocol struct {
	conn net.Conn
	r    *bufio.Reader
}

func dial() (*protocol, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return nil, err
	}
	p := &protocol{conn: conn, r: bufio.NewReaderSize(conn, 8192)}
	if reply, err := p.command("PING"); err != nil || reply != "PONG" {
		conn.Close()
		return nil, fmt.Errorf("ping: %q %v", reply, err)
	}
	return p, nil
}

func (p *protocol) command(cmd string) (string, error) {
	if _, err := p.conn.Write([]byte(cmd + "\n")); err != nil {
		return "", err
	}
	line = line[:0]
	for {
		part, isPrefix, err := p.r.ReadLine()
		if err != nil {
			return "", err
		}
		line = append(line, part...)
		if !isPrefix {
			break
		}
	}
	reply := string(line)
	if strings.HasPrefix(reply, "ERROR") {
		return "", fmt.Errorf("%s", reply)
	}
	return reply, nil
}

// fetch gets the next image into pixels.
func (p *protocol) fetch() error {
	reply, err := p.command("FETCH")
	if err != nil {
		return err
	}
	n, err := base64.StdEncoding.Decode(decoded, []byte(reply))
	if err != nil {
		return err
	}
	if n != texBytes {
		return fmt.Errorf("image is %d bytes, want %d", n, texBytes)
	}
	copy(pixels, decoded[:n])
	return nil
}

func initGraphics() {
	dmakit.Init(dmakit.D_CTRL_RELE_OFF, dmakit.D_CTRL_MFD_OFF, dmakit.D_CTRL_STS_UNSPEC,
		dmakit.D_CTRL_STD_OFF, dmakit.D_CTRL_RCYC_8, 1<<dmakit.DMA_CHANNEL_GIF)
	dmakit.ChannelInit(dmakit.DMA_CHANNEL_GIF)
	gs = gskit.InitGlobal()
	gs.SetPSM(gskit.GS_PSM_CT24)
	gs.SetPSMZ(gskit.GS_PSMZ_16S)
	gs.SetDoubleBuffering(true)
	gs.SetZBuffering(false)
	gskit.InitScreen(gs)
	tex = gskit.NewTexture(texSize, texSize, gskit.GS_PSM_CT32, gskit.GS_FILTER_NEAREST)
	font = gskit.InitFontFromMemory(fonts.Arial)
	must(gskit.FontUpload(gs, font))
	font.AddSpacing(-2)
}

func must(err error) {
	if err != nil {
		debug.Printf("fatal: %v\n", err)
		for {
		}
	}
}

func drawFrame(fps int) {
	gskit.SyncFlip(gs)
	gskit.SetActive(gs)
	gskit.Clear(gs, 0x10, 0x10, 0x30, 0x80, 0)
	x := float32((gs.Width() - texSize) / 2)
	gskit.PrimSpriteTexture3D(gs, tex, x, 40, 1, 0, 0, x+texSize, 40+texSize, 1, texSize, texSize,
		gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	white := gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0)
	gskit.FontPrint(gs, font, 8, 8, 2, 0.8, white, fmt.Sprintf("FPS %d   images %d", fps, fetches))
	gskit.FontPrint(gs, font, 8, 320, 2, 0.8, white, fmt.Sprintf("heap %d KB in use, %d KB idle, %d collections", m.HeapInuse/1024, m.HeapIdle/1024, m.NumGC))
	if lastErr != nil {
		gskit.FontPrint(gs, font, 8, 350, 2, 0.8, white, fmt.Sprintf("error: %v", lastErr))
	}
	gskit.QueueExec(gs)
}

func main() {
	debug.Init()
	debug.Printf("PS2 TinyGo image stream\n\n")
	debug.Printf("Bringing the network up (DHCP)...\n")
	cfg, err := ps2net.Up(ps2ip.Config{DHCP: true}, 30*time.Second)
	must(err)
	debug.Printf("Network up: %s\n", cfg)
	debug.Printf("Connecting to %s...\n", server)
	p, err := dial()
	must(err)
	debug.Printf("Connected\n")
	initGraphics()

	frames, fps := 0, 0
	second := time.Now()
	for frame := 0; ; frame++ {
		if frame%fetchEach == 0 {
			t := time.Now()
			if lastErr = p.fetch(); lastErr == nil {
				must(tex.Upload(gs, pixels))
				fetches++
				debug.Printf("image %d in %s\n", fetches, time.Since(t).Round(time.Millisecond))
			} else {
				debug.Printf("fetch: %v\n", lastErr)
			}
		}
		drawFrame(fps)
		frames++
		if time.Since(second) >= time.Second {
			fps, frames, second = frames, 0, time.Now()
		}
	}
}
