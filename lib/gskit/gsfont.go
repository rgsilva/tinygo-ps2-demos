package gskit

import (
	"encoding/binary"
	"fmt"
)

// GSFont is a gsKit FNT font drawn by this package rather than by gsKit:
// gsKit prints with additive blending (black text vanishes over a
// background) and advances by an unreliable width table. The atlas is
// converted to white glyphs with the ink as alpha, so any color blends
// correctly, and each glyph's advance is measured from its pixels.
type GSFont struct {
	tex    *GSTexture
	pixels []byte // the converted atlas, kept alive for the texture
	texW   int
	cellW  int
	cellH  int
	widths [256]int16
}

// The FNT file: a 32-byte header (magic, texture width and height, PSM,
// glyphs per row and column, cell width and height), a 256-byte width
// table, then a CT32 atlas of 16x16 cells where the ink is in the blue
// channel and the background is the color of the first pixel.
const (
	fntHeader = 32
	fntPixels = fntHeader + 256
	fntCells  = 16
)

// InitFontFromMemory prepares a FNT font from its file contents. Upload it
// with FontUpload before use.
func InitFontFromMemory(data []byte) *GSFont {
	f := &GSFont{
		texW:  int(binary.LittleEndian.Uint32(data[4:])),
		cellW: int(binary.LittleEndian.Uint32(data[24:])),
		cellH: int(binary.LittleEndian.Uint32(data[28:])),
	}
	texH := int(binary.LittleEndian.Uint32(data[8:]))
	src := data[fntPixels:]
	key := [4]byte{src[0], src[1], src[2], src[3]}
	// White glyphs, ink as alpha on the GS scale, in a 16-byte aligned copy.
	buf := make([]byte, f.texW*texH*4+16)
	off := (16 - alignOf(buf)) % 16
	f.pixels = buf[off : off+f.texW*texH*4]
	for i := 0; i+4 <= len(src) && i+4 <= len(f.pixels); i += 4 {
		a := byte(0)
		if [4]byte{src[i], src[i+1], src[i+2], src[i+3]} != key {
			a = src[i+2] // the blue channel is the ink, 0x7f at full strength
			if a > 0x80 {
				a = 0x80
			}
			if a == 0 {
				a = 1
			}
		}
		f.pixels[i], f.pixels[i+1], f.pixels[i+2], f.pixels[i+3] = 0xFF, 0xFF, 0xFF, a
	}
	f.measure(data[fntHeader:fntPixels])
	f.tex = NewTexture(f.texW, texH, GS_PSM_CT32, GS_FILTER_LINEAR)
	return f
}

// measure sets each glyph's advance: the ink's leftmost run of columns
// plus a pixel of bearing (a stray pixel further right does not count),
// or the file's width for a blank glyph like the space.
func (f *GSFont) measure(table []byte) {
	for c := 0; c < 256; c++ {
		cx, cy := (c%fntCells)*f.cellW, (c/fntCells)*f.cellH
		last, started := -1, false
		for x := 0; x < f.cellW; x++ {
			ink := false
			for y := 0; y < f.cellH && !ink; y++ {
				i := ((cy+y)*f.texW + cx + x) * 4
				ink = i+3 < len(f.pixels) && f.pixels[i+3] != 0
			}
			if ink {
				last, started = x, true
			} else if started {
				break
			}
		}
		if last < 0 {
			f.widths[c] = int16(table[c])
		} else {
			f.widths[c] = int16(last + 2)
		}
	}
}

// FontUpload sends the font's atlas to VRAM.
func FontUpload(g GSGlobal, font *GSFont) error {
	return font.tex.Upload(g, font.pixels)
}

func (f *GSFont) CharWidth() int  { return f.cellW }
func (f *GSFont) CharHeight() int { return f.cellH }

// AddSpacing changes the advance of every glyph by delta pixels.
func (f *GSFont) AddSpacing(delta int) {
	for i := range f.widths {
		f.widths[i] += int16(delta)
	}
}

// Free releases the font's C memory (its VRAM stays allocated).
func (f *GSFont) Free() {
	f.tex.Free()
	f.pixels = nil
}

// FontPrint draws text at (x, y), scaled, in color (GS_SETREG_RGBAQ; the
// alpha modulates the glyphs). Newlines start a new line.
func FontPrint(g GSGlobal, font *GSFont, x, y float32, z int, scale float32, color uint64, text string) {
	cx, cy := x, y
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\n' {
			cx, cy = x, cy+float32(font.cellH)*scale+1
			continue
		}
		w := float32(font.widths[c])
		u := float32((int(c) % fntCells) * font.cellW)
		v := float32((int(c) / fntCells) * font.cellH)
		PrimSpriteTexture3D(g, font.tex,
			cx, cy, z, u, v,
			cx+w*scale, cy+float32(font.cellH)*scale, z, u+w, v+float32(font.cellH),
			color)
		cx += w*scale + 1
	}
}

// alignOf is the address of b's first byte modulo 16.
func alignOf(b []byte) int {
	return int(uintptrOf(b) % 16)
}

func (f *GSFont) String() string {
	return fmt.Sprintf("font %dx%d cells", f.cellW, f.cellH)
}
