#!/usr/bin/env python3
"""Image server for the imagestream demo: 256x256 RGBA textures (GS alpha
scale) over a line protocol on TCP 9001. PING -> PONG; FETCH -> the next
image as one base64 line; anything else -> ERROR. Images are drawn here
(moving color patterns), so no files are needed.

Usage: imageserver.py [port]
"""
import base64
import math
import socketserver
import sys

SIZE = 256


def image(n):
    """Frame n of a moving pattern, as GS CT32 bytes (R, G, B, A=0x80)."""
    px = bytearray(SIZE * SIZE * 4)
    t = n * 0.15
    for y in range(SIZE):
        for x in range(SIZE):
            dx, dy = x - 128, y - 128
            d = math.sqrt(dx * dx + dy * dy)
            r = int(127 + 127 * math.sin(d / 12 - t))
            g = int(127 + 127 * math.sin(x / 24 + t))
            b = int(127 + 127 * math.sin(y / 24 - t / 2))
            i = (y * SIZE + x) * 4
            px[i], px[i + 1], px[i + 2], px[i + 3] = r, g, b, 0x80
    return bytes(px)


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        n = 0
        while True:
            line = self.rfile.readline()
            if not line:
                return
            cmd = line.strip().upper()
            if cmd == b"PING":
                self.wfile.write(b"PONG\n")
            elif cmd == b"FETCH":
                self.wfile.write(base64.b64encode(image(n)) + b"\n")
                n += 1
            else:
                self.wfile.write(b"ERROR unknown command\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9001
    print(f"image server on port {port}")
    Server(("0.0.0.0", port), Handler).serve_forever()
