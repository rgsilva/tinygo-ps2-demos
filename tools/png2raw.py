#!/usr/bin/env python3
"""Convert a PNG to a raw GS texture (CT32: R8 G8 B8 A8 per pixel).

The GS alpha scale is 0..0x80 (0x80 = opaque), so the PNG's 0..255 alpha is
halved; an RGB PNG gets 0x80 everywhere. 8-bit RGB/RGBA, non-interlaced.

Usage: png2raw.py in.png out.raw
"""
import struct, sys, zlib


def decode(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise SystemExit("not a PNG")
    pos, idat, hdr = 8, [], None
    while pos < len(data):
        length, kind = struct.unpack(">I4s", data[pos:pos + 8])
        body = data[pos + 8:pos + 8 + length]
        pos += 12 + length
        if kind == b"IHDR":
            hdr = struct.unpack(">IIBBBBB", body)
        elif kind == b"IDAT":
            idat.append(body)
        elif kind == b"IEND":
            break
    w, h, depth, ctype, _, _, interlace = hdr
    if depth != 8 or ctype not in (2, 6) or interlace:
        raise SystemExit("only 8-bit RGB/RGBA non-interlaced PNGs (got depth %d type %d interlace %d)" % (depth, ctype, interlace))
    bpp = 3 if ctype == 2 else 4
    raw = zlib.decompress(b"".join(idat))
    stride = w * bpp
    prev = bytearray(stride)
    out = bytearray()
    p = 0
    for _ in range(h):
        f = raw[p]
        line = bytearray(raw[p + 1:p + 1 + stride])
        p += 1 + stride
        for i in range(stride):
            a = line[i - bpp] if i >= bpp else 0
            b = prev[i]
            c = prev[i - bpp] if i >= bpp else 0
            if f == 1:
                line[i] = (line[i] + a) & 255
            elif f == 2:
                line[i] = (line[i] + b) & 255
            elif f == 3:
                line[i] = (line[i] + (a + b) // 2) & 255
            elif f == 4:
                q = a + b - c
                pa, pb, pc = abs(q - a), abs(q - b), abs(q - c)
                pred = a if pa <= pb and pa <= pc else (b if pb <= pc else c)
                line[i] = (line[i] + pred) & 255
        out += line
        prev = line
    return w, h, bpp, bytes(out)


def main():
    src, dst = sys.argv[1], sys.argv[2]
    w, h, bpp, px = decode(open(src, "rb").read())
    out = bytearray(w * h * 4)
    for i in range(w * h):
        r, g, b = px[i * bpp], px[i * bpp + 1], px[i * bpp + 2]
        a = (px[i * bpp + 3] + 1) // 2 if bpp == 4 else 0x80
        out[i * 4:i * 4 + 4] = bytes((r, g, b, a))
    open(dst, "wb").write(out)


if __name__ == "__main__":
    main()
