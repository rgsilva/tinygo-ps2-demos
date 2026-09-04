#!/usr/bin/env python3
"""Run a PS2 ELF headlessly in PCSX2 and report the verdict of the ps2go harness.

Verdicts: PASS, FAIL, TIMEOUT (no result marker before the deadline), CRASH
(EE exception logged), ERROR (emulator or setup problem).

Setup (once): harness/setup-pcsx2.sh. PCSX2's location is taken from
PS2GO_PCSX2_DIR or --pcsx2-dir (default ~/dev/ps2go/tools/pcsx2). That directory
holds squashfs-root/ (the extracted AppImage) and data/ (ini, bios, logs).
"""
import argparse
import os
import re
import signal
import socket
import struct
import subprocess
import sys
import time

DEFAULT_DIR = os.path.expanduser("~/dev/ps2go/tools/pcsx2")
BLOCK_SYMBOL = "ps2go_harness_block"
BLOCK_MAGIC = 0x47325350
BLOCK_FIELDS = ["magic", "version", "state", "cases", "passed", "failed", "current",
                "heap_start", "heap_end", "heap_inuse", "heap_idle", "total_alloc",
                "mallocs", "frees", "xfailed", "num_gc"]
# Log lines that mean the guest died. Refined against controls/crash.
CRASH_PATTERNS = [
    re.compile(r"TLB Miss", re.I),
    re.compile(r"^(panic: |fatal error: |runtime: abort)"),
    re.compile(r"\bException\b.*(Address|Bus|Reserved|Trap|Overflow)", re.I),
    re.compile(r"Unhandled (EE|IOP) exception", re.I),
    re.compile(r"\bEE\b.*\b(Address Error|Bus Error|Reserved Instruction)\b", re.I),
    re.compile(r"# Exception", re.I),
]
EXIT_CODES = {"PASS": 0, "FAIL": 1, "TIMEOUT": 2, "CRASH": 3, "ERROR": 4}


def elf_symbol(path, name):
    """Address of a symbol in a 32-bit little-endian ELF, or None."""
    with open(path, "rb") as f:
        data = f.read()
    if data[:4] != b"\x7fELF" or data[4] != 1 or data[5] != 1:
        raise ValueError("not a 32-bit little-endian ELF")
    shoff, = struct.unpack_from("<I", data, 0x20)
    shentsize, shnum = struct.unpack_from("<HH", data, 0x2E)
    sections = []
    for i in range(shnum):
        off = shoff + i * shentsize
        sh_type, = struct.unpack_from("<I", data, off + 4)
        sh_offset, sh_size, sh_link = struct.unpack_from("<III", data, off + 0x10)
        sh_entsize, = struct.unpack_from("<I", data, off + 0x24)
        sections.append((sh_type, sh_offset, sh_size, sh_link, sh_entsize))
    for sh_type, off, size, link, entsize in sections:
        if sh_type != 2:  # SHT_SYMTAB
            continue
        _, stroff, strsize, _, _ = sections[link]
        strtab = data[stroff:stroff + strsize]
        for j in range(size // entsize):
            st_name, st_value = struct.unpack_from("<II", data, off + j * entsize)
            end = strtab.index(b"\0", st_name)
            if strtab[st_name:end].decode("utf-8", "replace") == name:
                return st_value
    return None


class Pine:
    """Minimal PCSX2 PINE client (read memory, status)."""

    def __init__(self, slot=28011):
        base = os.environ.get("XDG_RUNTIME_DIR") or "/tmp"
        path = os.path.join(base, "pcsx2.sock")
        if slot != 28011:
            path += f".{slot}"
        self.s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.s.settimeout(2)
        self.s.connect(path)

    def _cmd(self, op, payload=b""):
        self.s.sendall(struct.pack("<IB", 5 + len(payload), op) + payload)
        size, = struct.unpack("<I", self._recv(4))
        body = self._recv(size - 4)
        if body[0] != 0:
            raise RuntimeError(f"PINE op {op} failed")
        return body[1:]

    def _recv(self, n):
        buf = b""
        while len(buf) < n:
            chunk = self.s.recv(n - len(buf))
            if not chunk:
                raise ConnectionError("PINE socket closed")
            buf += chunk
        return buf

    def read32(self, addr):
        return struct.unpack("<I", self._cmd(2, struct.pack("<I", addr)))[0]

    def status(self):
        return {0: "Running", 1: "Paused", 2: "Shutdown"}.get(struct.unpack("<I", self._cmd(0xF))[0])

    def close(self):
        self.s.close()


def read_block(pine, addr):
    vals = [pine.read32(addr + 4 * i) for i in range(len(BLOCK_FIELDS))]
    if vals[0] != BLOCK_MAGIC:
        return None
    return dict(zip(BLOCK_FIELDS, vals))


def start_xvfb():
    for n in range(90, 200):
        if os.path.exists(f"/tmp/.X{n}-lock"):
            continue
        p = subprocess.Popen(["Xvfb", f":{n}", "-screen", "0", "640x480x24", "-nolisten", "tcp"],
                             stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for _ in range(50):
            if os.path.exists(f"/tmp/.X11-unix/X{n}"):
                return p, f":{n}"
            if p.poll() is not None:
                break
            time.sleep(0.1)
        p.kill()
    raise RuntimeError("could not start Xvfb")


def stop(proc, grace=10.0):
    if proc.poll() is not None:
        return
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(grace)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait()


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("elf")
    ap.add_argument("--timeout", type=float, default=60, help="seconds before TIMEOUT (default 60)")
    ap.add_argument("--expect", choices=list(EXIT_CODES), help="exit 0 only if the verdict equals this")
    ap.add_argument("--run", action="store_true", help="just run for --timeout seconds and stream the guest output")
    ap.add_argument("--test", action="store_true",
                    help="run a Go test binary: stream its output, verdict from its exit code (PS2GO-EXIT); "
                         "used as the emulator of TinyGo's ps2 target")
    ap.add_argument("ignored", nargs=argparse.REMAINDER, help="extra arguments (test flags) are ignored")
    ap.add_argument("--pcsx2-dir", default=os.environ.get("PS2GO_PCSX2_DIR", DEFAULT_DIR))
    ap.add_argument("--log", help="keep the PCSX2 log at this path")
    ap.add_argument("--probe", type=float, default=0, metavar="SECS", help="print the guest stats block every SECS")
    ap.add_argument("--pine-slot", type=int, default=28011, help="PINE slot of the PCSX2 instance (default 28011)")
    ap.add_argument("-q", "--quiet", action="store_true", help="only print the verdict line")
    args = ap.parse_args()

    elf = os.path.abspath(args.elf)
    binary = os.path.join(args.pcsx2_dir, "squashfs-root", "usr", "bin", "pcsx2-qt")
    datadir = os.path.join(args.pcsx2_dir, "data")
    log = os.path.abspath(args.log or f"/tmp/ps2test-{os.getpid()}.log")

    def say(msg):
        if not args.quiet:
            print(msg, flush=True)

    def finish(verdict, detail=""):
        line = f"PS2TEST {verdict} {os.path.basename(elf)}"
        if detail:
            line += f": {detail}"
        print(line, flush=True)
        if args.expect:
            return 0 if verdict == args.expect else 5
        return EXIT_CODES[verdict]

    for path, what in ((elf, "ELF"), (binary, "PCSX2 binary"), (os.path.join(datadir, "PCSX2", "inis", "PCSX2.ini"), "PCSX2 ini")):
        if not os.path.exists(path):
            return finish("ERROR", f"{what} not found: {path}")
    biosdir = os.path.join(datadir, "PCSX2", "bios")
    if not os.path.isdir(biosdir) or not os.listdir(biosdir):
        return finish("ERROR", f"no BIOS in {biosdir}")

    try:
        block_addr = elf_symbol(elf, BLOCK_SYMBOL)
    except ValueError as e:
        return finish("ERROR", str(e))
    if block_addr is None:
        say(f"note: no {BLOCK_SYMBOL} symbol, memory probing disabled")

    try:
        xvfb, display = start_xvfb()
    except (RuntimeError, FileNotFoundError) as e:
        return finish("ERROR", f"Xvfb: {e}")
    env = dict(os.environ, DISPLAY=display, LIBGL_ALWAYS_SOFTWARE="1", QT_QPA_PLATFORM="xcb")
    if os.path.exists(log):
        os.remove(log)
    proc = subprocess.Popen([binary, "-datapath", datadir, "-batch", "-nogui", "-logfile", log, "-elf", elf],
                            env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    pine = None
    verdict, detail = None, ""
    t0 = time.time()
    next_probe = t0 + args.probe if args.probe else None
    pos = 0
    block = None
    started = False  # --test: echo only what the program prints
    try:
        while True:
            now = time.time()
            if os.path.exists(log):
                with open(log, "r", errors="replace") as f:
                    f.seek(pos)
                    chunk = f.read()
                    pos = f.tell()
                for line in chunk.splitlines():
                    text = re.sub(r"^\[\s*[\d.]+\]\s*", "", line)
                    if args.test:
                        if started and not text.startswith("PS2GO-EXIT"):
                            print(text, flush=True)
                        elif "is executing" in text:
                            started = True
                    elif text.startswith("PS2GO-") or args.run:
                        say(f"  {text}")
                    # A crash is sticky: PCSX2 logs the faulting access and
                    # carries on, so a later result marker must not hide it.
                    if verdict == "CRASH":
                        continue
                    if text.startswith("PS2GO-RESULT"):
                        verdict = "PASS" if text.split()[1] == "PASS" else "FAIL"
                        detail = " ".join(text.split()[2:])
                    elif text.startswith("PS2GO-EXIT") and (args.test or text.split()[1] != "0"):
                        code = text.split()[1]
                        verdict = "PASS" if code == "0" else "FAIL"
                        detail = f"exit code {code}"
                    elif text.startswith("Startup Error") or "PCSX2 requires a PlayStation 2 BIOS" in text:
                        verdict, detail = "ERROR", text
                    elif any(p.search(text) for p in CRASH_PATTERNS):
                        verdict, detail = "CRASH", text
            if verdict and not args.run:
                break
            if proc.poll() is not None:
                verdict = verdict or "ERROR"
                detail = detail or f"PCSX2 exited early with code {proc.returncode}"
                break
            if now - t0 > args.timeout:
                if not args.run:
                    verdict, detail = "TIMEOUT", f"after {args.timeout:.0f}s"
                break
            if pine is None and block_addr is not None:
                try:
                    pine = Pine(args.pine_slot)
                except OSError:
                    pass
            if pine is not None and next_probe and now >= next_probe:
                next_probe = now + args.probe
                try:
                    block = read_block(pine, block_addr)
                    if block:
                        say("  stats: " + " ".join(f"{k}={v:#x}" if k.startswith("heap_") else f"{k}={v}" for k, v in block.items() if k not in ("magic", "version")))
                except (OSError, RuntimeError):
                    pine = None
            time.sleep(0.1)
        if pine is not None and block_addr is not None:
            try:
                block = read_block(pine, block_addr) or block
            except (OSError, RuntimeError):
                pass
    finally:
        # Close PINE first: PCSX2's graceful shutdown waits for the client.
        if pine:
            pine.close()
        stop(proc)
        stop(xvfb, 2)
        if not args.log and os.path.exists(log) and verdict in ("PASS", None):
            os.remove(log)
    elapsed = time.time() - t0
    if block:
        say(f"  final stats: state={block['state']} passed={block['passed']} failed={block['failed']} xfailed={block['xfailed']} "
            f"heap_inuse={block['heap_inuse']} heap_idle={block['heap_idle']} mallocs={block['mallocs']} frees={block['frees']} num_gc={block['num_gc']}")
    if args.run:
        return 0
    if verdict != "PASS" and os.path.exists(log):
        detail += f" (log: {log})"
    return finish(verdict, f"{detail} in {elapsed:.1f}s")


if __name__ == "__main__":
    sys.exit(main())
