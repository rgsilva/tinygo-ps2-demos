"""Visual side of the harness: PCSX2 with the software renderer, keyboard input
through the X test extension, PNG screenshots and their comparison.

Used by ps2test.py (--visual, --screenshot). Pure Python: PNGs are decoded and
encoded here (8-bit gray/RGB/RGBA, non-interlaced), keys are sent with libXtst
through ctypes.
"""
import ctypes
import os
import re
import shutil
import struct
import time
import sys
import zlib

# PCSX2 settings for rendering, on top of the harness ini: the software
# renderer, screenshots of the raw GS frame (no aspect correction, no OSD),
# no audio device (its error dialog would steal the keyboard).
INI_OVERRIDES = {
    ("EmuCore/GS", "Renderer"): "13",
    ("EmuCore/GS", "ScreenshotSize"): "2",
    ("EmuCore/GS", "ScreenshotFormat"): "0",
    ("EmuCore/GS", "OsdShowIndicators"): "false",
    ("EmuCore/GS", "OsdShowMessages"): "false",
    ("EmuCore/GS", "OsdShowVideoCapture"): "false",
    ("EmuCore/GS", "OsdShowInputRec"): "false",
    ("SPU2/Output", "Backend"): "Null",
}

# Controller buttons as the keyboard keys PCSX2 binds them to by default
# (the [Pad1] section of the ini): X keysym names.
PAD_KEYS = {
    "start": "Return", "select": "BackSpace",
    "cross": "k", "circle": "l", "triangle": "i", "square": "j",
    "up": "Up", "down": "Down", "left": "Left", "right": "Right",
    "l1": "q", "l2": "1", "r1": "e", "r2": "3",
}
SCREENSHOT_KEY = "F8"  # [Hotkeys] Screenshot


# PCSX2 settings for network runs: DEV9 ethernet through the "Sockets"
# backend, which needs no privileges: PCSX2 runs a DHCP server (the guest
# gets <host subnet>.100, the host adapter's IP as gateway and DNS) and
# forwards TCP/UDP through host sockets.
NET_OVERRIDES = {
    ("DEV9/Eth", "EthEnable"): "true",
    ("DEV9/Eth", "EthApi"): "Sockets",
    ("DEV9/Eth", "EthDevice"): "Auto",
    ("DEV9/Eth", "EthLogDHCP"): "true",
    ("DEV9/Eth", "EthLogDNS"): "true",
    # PCSX2 answers the guest's DNS queries itself through the host's
    # resolver (the host's servers cannot be read behind systemd-resolved).
    ("DEV9/Eth", "ModeDNS1"): "Internal",
}

# The guest reaches this machine by this name (a PCSX2 DNS hosts entry).
HOST_NAME = "host.ps2go"
ECHO_PORT, HTTP_PORT = 17777, 18080


def host_ip():
    """This machine's address on the interface that reaches the outside
    (what PCSX2's "Auto" adapter uses); no packet is sent."""
    import socket
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    try:
        s.connect(("192.0.2.1", 9))
        return s.getsockname()[0]
    finally:
        s.close()


def net_overrides():
    """NET_OVERRIDES plus the hosts entry for this machine."""
    o = dict(NET_OVERRIDES)
    o[("DEV9/Eth/Hosts", "Count")] = "1"
    o[("DEV9/Eth/Hosts/Host0", "Url")] = HOST_NAME
    o[("DEV9/Eth/Hosts/Host0", "Desc")] = "the harness host"
    o[("DEV9/Eth/Hosts/Host0", "Address")] = host_ip()
    o[("DEV9/Eth/Hosts/Host0", "Enabled")] = "true"
    return o


class NetHelpers:
    """A TCP echo server and an HTTP server on this machine for the guest's
    network checks; both run in daemon threads."""

    def __init__(self):
        import http.server
        import socketserver
        import threading

        import socket

        class Echo(socketserver.BaseRequestHandler):
            """Echoes; a first line "CONNECT <port>" instead makes it connect
            back to the client's address at that port and say hello."""
            def handle(self):
                first = True
                while True:
                    data = self.request.recv(4096)
                    if not data:
                        return
                    if first and data.startswith(b"CONNECT "):
                        port = int(data.split()[1])
                        try:
                            back = socket.create_connection((self.client_address[0], port), timeout=5)
                            back.sendall(b"knock knock\n")
                            back.close()
                        except OSError:
                            pass  # expected under PCSX2: no inbound connections
                        return
                    first = False
                    self.request.sendall(data)

        class UDPEcho(socketserver.BaseRequestHandler):
            def handle(self):
                data, sock = self.request
                sock.sendto(data, self.client_address)

        class Page(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path == "/hello":
                    body, code = b"hello from the host\n", 200
                elif self.path == "/large":
                    body, code = bytes(ord("a") + i % 26 for i in range(200 * 1024)), 200
                else:
                    body, code = b"not found\n", 404
                self.send_response(code)
                self.send_header("Content-Type", "text/plain")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        socketserver.TCPServer.allow_reuse_address = True
        self.servers = [
            socketserver.ThreadingTCPServer(("0.0.0.0", ECHO_PORT), Echo),
            socketserver.ThreadingUDPServer(("0.0.0.0", ECHO_PORT), UDPEcho),
            http.server.ThreadingHTTPServer(("0.0.0.0", HTTP_PORT), Page),
        ]
        # The imagestream demo's server (tools/imageserver.py), on its port.
        sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
        import imageserver
        self.servers.append(imageserver.Server(("0.0.0.0", 9001), imageserver.Handler))
        for srv in self.servers:
            threading.Thread(target=srv.serve_forever, daemon=True).start()

    def stop(self):
        for srv in self.servers:
            srv.shutdown()
            srv.server_close()


def prepare_datadir(pcsx2_dir, name, overrides):
    """A second PCSX2 data directory (data-<name>): the BIOS and memory
    cards of the harness one (linked, never copied), its ini with the
    overrides {(section, key): value}. Refreshed on every run. Returns its
    path."""
    src = os.path.join(pcsx2_dir, "data")
    dst = os.path.join(pcsx2_dir, "data-" + name)
    for sub in ("inis", "snaps", "logs", "cache"):
        os.makedirs(os.path.join(dst, "PCSX2", sub), exist_ok=True)
    for sub in ("bios", "memcards", "resources"):
        link = os.path.join(dst, "PCSX2", sub)
        if not os.path.lexists(link):
            os.symlink(os.path.join("..", "..", "data", "PCSX2", sub), link)
    with open(os.path.join(src, "PCSX2", "inis", "PCSX2.ini")) as f:
        lines = f.read().splitlines()
    out, section, done = [], None, set()

    def flush(sec):
        for (s, k), v in overrides.items():
            if s == sec and (s, k) not in done:
                out.append(f"{k} = {v}")
                done.add((s, k))

    for line in lines:
        m = re.match(r"\s*\[(.*)\]\s*$", line)
        if m:
            flush(section)
            section = m.group(1)
            out.append(line)
            continue
        m = re.match(r"\s*([\w/]+)\s*=", line)
        if m and (section, m.group(1)) in overrides:
            out.append(f"{m.group(1)} = {overrides[(section, m.group(1))]}")
            done.add((section, m.group(1)))
        else:
            out.append(line)
    flush(section)
    for s in {s for s, _ in overrides} - {s for s, _ in done}:
        out.append(f"\n[{s}]")
        flush(s)
    with open(os.path.join(dst, "PCSX2", "inis", "PCSX2.ini"), "w") as f:
        f.write("\n".join(out) + "\n")
    return dst


class _XWindowAttributes(ctypes.Structure):
    _fields_ = [("x", ctypes.c_int), ("y", ctypes.c_int), ("width", ctypes.c_int), ("height", ctypes.c_int),
                ("border_width", ctypes.c_int), ("depth", ctypes.c_int), ("visual", ctypes.c_void_p),
                ("root", ctypes.c_ulong), ("c_class", ctypes.c_int), ("bit_gravity", ctypes.c_int),
                ("win_gravity", ctypes.c_int), ("backing_store", ctypes.c_int), ("backing_planes", ctypes.c_ulong),
                ("backing_pixel", ctypes.c_ulong), ("save_under", ctypes.c_int), ("colormap", ctypes.c_ulong),
                ("map_installed", ctypes.c_int), ("map_state", ctypes.c_int), ("all_event_masks", ctypes.c_long),
                ("your_event_mask", ctypes.c_long), ("do_not_propagate_mask", ctypes.c_long),
                ("override_redirect", ctypes.c_int), ("screen", ctypes.c_void_p)]


class Keyboard:
    """Key presses on an X display (XTEST), aimed at PCSX2's display window."""

    def __init__(self, display):
        self.x11 = x11 = ctypes.CDLL("libX11.so.6")
        self.xtst = xtst = ctypes.CDLL("libXtst.so.6")
        x11.XOpenDisplay.restype = ctypes.c_void_p
        x11.XOpenDisplay.argtypes = [ctypes.c_char_p]
        x11.XDefaultRootWindow.restype = ctypes.c_ulong
        x11.XDefaultRootWindow.argtypes = [ctypes.c_void_p]
        x11.XQueryTree.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_ulong),
                                   ctypes.POINTER(ctypes.c_ulong), ctypes.POINTER(ctypes.POINTER(ctypes.c_ulong)),
                                   ctypes.POINTER(ctypes.c_uint)]
        x11.XGetWindowAttributes.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(_XWindowAttributes)]
        x11.XFetchName.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.POINTER(ctypes.c_char_p)]
        x11.XSetInputFocus.argtypes = [ctypes.c_void_p, ctypes.c_ulong, ctypes.c_int, ctypes.c_ulong]
        x11.XStringToKeysym.restype = ctypes.c_ulong
        x11.XStringToKeysym.argtypes = [ctypes.c_char_p]
        x11.XKeysymToKeycode.restype = ctypes.c_ubyte
        x11.XKeysymToKeycode.argtypes = [ctypes.c_void_p, ctypes.c_ulong]
        x11.XFlush.argtypes = [ctypes.c_void_p]
        x11.XFree.argtypes = [ctypes.c_void_p]
        xtst.XTestFakeKeyEvent.argtypes = [ctypes.c_void_p, ctypes.c_uint, ctypes.c_int, ctypes.c_ulong]
        xtst.XTestFakeMotionEvent.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_ulong]
        self.dpy = x11.XOpenDisplay(display.encode())
        if not self.dpy:
            raise RuntimeError(f"cannot open display {display}")

    def windows(self):
        """Mapped top-level windows as (id, name, x, y, w, h)."""
        root = self.x11.XDefaultRootWindow(self.dpy)
        r, p = ctypes.c_ulong(), ctypes.c_ulong()
        kids, n = ctypes.POINTER(ctypes.c_ulong)(), ctypes.c_uint()
        if not self.x11.XQueryTree(self.dpy, root, ctypes.byref(r), ctypes.byref(p), ctypes.byref(kids), ctypes.byref(n)):
            return []
        out = []
        for i in range(n.value):
            a = _XWindowAttributes()
            if self.x11.XGetWindowAttributes(self.dpy, kids[i], ctypes.byref(a)) and a.map_state == 2:
                name = ctypes.c_char_p()
                self.x11.XFetchName(self.dpy, kids[i], ctypes.byref(name))
                out.append((kids[i], (name.value or b"").decode("utf-8", "replace"), a.x, a.y, a.width, a.height))
        self.x11.XFree(kids)
        return out

    def focus(self, window):
        wid, _, x, y, w, h = window
        self.x11.XSetInputFocus(self.dpy, wid, 1, 0)  # RevertToParent, CurrentTime
        self.xtst.XTestFakeMotionEvent(self.dpy, 0, x + w // 2, y + h // 2, 0)
        self.x11.XFlush(self.dpy)

    def focus_display(self):
        """Focus PCSX2's display window (the largest one). Returns the names
        of other windows (dialogs), which would take the keys instead."""
        ws = self.windows()
        if not ws:
            return None
        main = max(ws, key=lambda t: t[4] * t[5])
        self.focus(main)
        return [w[1] for w in ws if w is not main]

    def dismiss(self, name):
        for w in self.windows():
            if w[1] == name:
                self.focus(w)
                self.press("Escape")

    def press(self, keysym, hold=0.1):
        code = self.x11.XKeysymToKeycode(self.dpy, self.x11.XStringToKeysym(keysym.encode()))
        if code == 0:
            raise ValueError(f"unknown key {keysym!r}")
        self.xtst.XTestFakeKeyEvent(self.dpy, code, 1, 0)
        self.x11.XFlush(self.dpy)
        time.sleep(hold)
        self.xtst.XTestFakeKeyEvent(self.dpy, code, 0, 0)
        self.x11.XFlush(self.dpy)


# --- PNG ---------------------------------------------------------------------

def read_png(path):
    """(width, height, rows) with rows as lists of (r, g, b) tuples."""
    with open(path, "rb") as f:
        data = f.read()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError(f"{path}: not a PNG")
    pos, idat, w = 8, b"", None
    while pos < len(data):
        length, ctype = struct.unpack_from(">I4s", data, pos)
        body = data[pos + 8:pos + 8 + length]
        pos += 12 + length
        if ctype == b"IHDR":
            w, h, depth, color, _, _, interlace = struct.unpack(">IIBBBBB", body)
            if depth != 8 or interlace != 0 or color not in (0, 2, 6):
                raise ValueError(f"{path}: unsupported PNG (8-bit gray/RGB/RGBA, non-interlaced only)")
            bpp = {0: 1, 2: 3, 6: 4}[color]
        elif ctype == b"IDAT":
            idat += body
    raw = zlib.decompress(idat)
    stride = w * bpp
    rows, prev = [], bytearray(stride)
    for y in range(h):
        off = y * (stride + 1)
        ftype = raw[off]
        line = bytearray(raw[off + 1:off + 1 + stride])
        for i in range(stride):
            a = line[i - bpp] if i >= bpp else 0
            b = prev[i]
            c = prev[i - bpp] if i >= bpp else 0
            if ftype == 1:
                line[i] = (line[i] + a) & 255
            elif ftype == 2:
                line[i] = (line[i] + b) & 255
            elif ftype == 3:
                line[i] = (line[i] + (a + b) // 2) & 255
            elif ftype == 4:
                p = a + b - c
                pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                line[i] = (line[i] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 255
        prev = line
        if bpp == 1:
            rows.append([(v, v, v) for v in line])
        else:
            rows.append([tuple(line[i:i + 3]) for i in range(0, stride, bpp)])
    return w, h, rows


def write_png(path, w, h, rows):
    raw = b"".join(b"\0" + bytes(v for px in row for v in px) for row in rows)

    def chunk(ctype, body):
        return struct.pack(">I", len(body)) + ctype + body + struct.pack(">I", zlib.crc32(ctype + body) & 0xFFFFFFFF)

    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0))
                + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b""))


# --- Comparison --------------------------------------------------------------

BLOCK = 16          # comparison cell, in pixels: a shift of a few pixels stays within the tolerance
BLOCK_DIFF = 24     # a cell differs when its mean gray level moved by more than this (0-255)
DEFAULT_TOLERANCE = 5.0  # percent of differing cells allowed


def gray_blocks(w, h, rows, block=BLOCK):
    """Mean gray level of each block x block cell, row-major."""
    out = []
    for by in range(0, h, block):
        for bx in range(0, w, block):
            total = n = 0
            for y in range(by, min(by + block, h)):
                row = rows[y]
                for x in range(bx, min(bx + block, w)):
                    r, g, b = row[x]
                    total += (r * 299 + g * 587 + b * 114) // 1000
                    n += 1
            out.append(total / n)
    return out


def blank(w, h, rows):
    """A screenshot with no picture: fewer than 16 distinct colors in a sample."""
    colors = set()
    for y in range(0, h, 4):
        row = rows[y]
        for x in range(0, w, 4):
            colors.add(row[x])
            if len(colors) >= 16:
                return False
    return True


def compare(got, ref, diff_path=None):
    """(differing cells in percent, mean cell difference, detail). Writes a
    diff image (reference dimmed, differing pixels in red) when a path is
    given."""
    gw, gh, grows = got
    rw, rh, rrows = ref
    if (gw, gh) != (rw, rh):
        return 100.0, 255.0, f"size {gw}x{gh}, reference {rw}x{rh}"
    a, b = gray_blocks(gw, gh, grows), gray_blocks(rw, rh, rrows)
    diffs = [abs(x - y) for x, y in zip(a, b)]
    over = sum(1 for d in diffs if d > BLOCK_DIFF)
    pct = 100.0 * over / len(diffs)
    mean = sum(diffs) / len(diffs)
    if diff_path:
        out = []
        for gy, ry in zip(grows, rrows):
            line = []
            for (r1, g1, b1), (r2, g2, b2) in zip(gy, ry):
                if max(abs(r1 - r2), abs(g1 - g2), abs(b1 - b2)) > 40:
                    line.append((255, 0, 0))
                else:
                    line.append((r2 // 3, g2 // 3, b2 // 3))
            out.append(line)
        write_png(diff_path, gw, gh, out)
    return pct, mean, f"{over}/{len(diffs)} cells differ ({pct:.1f}%), mean {mean:.1f}"


def newest_png(directory, known):
    """A PNG in directory that is not in known, or None."""
    for name in os.listdir(directory):
        if name.endswith(".png") and name not in known:
            return os.path.join(directory, name)
    return None


def parse_steps(path):
    """Steps file: one `op args` per line, # comments. Returns [(op, args)]."""
    steps = []
    with open(path) as f:
        for n, line in enumerate(f, 1):
            line = line.split("#", 1)[0].strip()
            if not line:
                continue
            op, *args = line.split()
            if op not in ("until", "sleep", "pad", "key", "shot"):
                raise ValueError(f"{path}:{n}: unknown step {op!r}")
            if op == "until":
                args = [" ".join(args)]
            if op == "pad" and args and args[0] not in PAD_KEYS:
                raise ValueError(f"{path}:{n}: unknown button {args[0]!r} (one of {', '.join(PAD_KEYS)})")
            if not args:
                raise ValueError(f"{path}:{n}: {op} needs an argument")
            steps.append((op, args))
    return steps


def move(src, dst):
    os.makedirs(os.path.dirname(os.path.abspath(dst)), exist_ok=True)
    shutil.move(src, dst)
