#!/usr/bin/env python3
"""Run a PS2 ELF headlessly in PCSX2 and report the verdict of the ps2go harness.

Verdicts: PASS, FAIL, TIMEOUT (no result marker before the deadline), CRASH
(EE exception logged), ERROR (emulator or setup problem).

Setup (once): tools/pcsx2/setup-pcsx2.sh. PCSX2's location is taken from
PS2GO_PCSX2_DIR or --pcsx2-dir (default ~/dev/ps2go/tools/pcsx2). That directory
holds squashfs-root/ (the extracted AppImage) and data/ (ini, bios, logs).

Visual mode (--visual STEPS, --screenshot PATH) renders with PCSX2's software
renderer (see visual.py) and follows a steps file: `until TEXT` waits for a
guest output line, `sleep SECS`, `pad BUTTON [HOLD]` presses a controller
button (start, cross, ...), `key KEYSYM [HOLD]` any key, `shot NAME [TOL]`
takes a screenshot and compares it with the reference NAME.png next to the
steps file (--update writes the references instead). The verdict is FAIL when
a screenshot is blank or differs from its reference by more than TOL percent
of 16x16 cells.
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


class Steps:
    """Runs a visual steps file against the emulator (see visual.py)."""

    def __init__(self, steps, display, snaps, shots_dir, refs, update, say):
        import visual
        self.v = visual
        self.steps = steps
        self.kb = visual.Keyboard(display)
        self.snaps, self.shots_dir, self.refs, self.update, self.say = snaps, shots_dir, refs, update, say
        self.i = 0
        self.deadline = None
        self.failures = []
        self.shots = 0
        self.known = set(os.listdir(snaps))

    def where(self):
        if self.i >= len(self.steps):
            return "end"
        op, args = self.steps[self.i]
        return f"{self.i + 1} ({op} {' '.join(args)})"

    def done(self):
        return self.i >= len(self.steps)

    def verdict(self):
        if self.failures:
            return "FAIL", "; ".join(self.failures)
        return "PASS", f"{self.shots} screenshot(s) {'written' if self.update or self.refs is None else 'match'}"

    def feed(self, text):
        """A guest output line: completes a pending `until`."""
        if not self.done() and self.steps[self.i][0] == "until" and self.steps[self.i][1][0] in text:
            self.say(f"  until {self.steps[self.i][1][0]!r}: {text.strip()}")
            self.i += 1

    def tick(self, now):
        """Runs the steps that do not wait for output."""
        while not self.done():
            op, args = self.steps[self.i]
            if op == "until":
                return
            if op == "sleep":
                if self.deadline is None:
                    self.deadline = now + float(args[0])
                if time.time() < self.deadline:
                    return
                self.deadline = None
            elif op in ("pad", "key"):
                self.press(self.v.PAD_KEYS[args[0]] if op == "pad" else args[0], float(args[1]) if len(args) > 1 else 0.1)
                self.say(f"  {op} {args[0]}")
            elif op == "shot":
                self.shot(args[0], float(args[1]) if len(args) > 1 else self.v.DEFAULT_TOLERANCE)
            self.i += 1

    def press(self, keysym, hold=0.1):
        others = self.kb.focus_display()
        if others is None:
            raise RuntimeError("no PCSX2 window on the display")
        for name in others:
            # A dialog (an emulator error) would take the keys: close it.
            self.say(f"  note: closing PCSX2 dialog {name!r}")
            self.kb.dismiss(name)
            self.kb.focus_display()
        self.kb.press(keysym, hold)

    def shot(self, name, tolerance):
        self.press(self.v.SCREENSHOT_KEY)
        path = None
        deadline = time.time() + 5
        while time.time() < deadline and path is None:
            path = self.v.newest_png(self.snaps, self.known)
            time.sleep(0.1)
        if path is None:
            self.failures.append(f"{name}: PCSX2 saved no screenshot")
            return
        # PCSX2 writes the file after logging it: wait for the size to settle.
        size = -1
        while os.path.getsize(path) != size:
            size = os.path.getsize(path)
            time.sleep(0.2)
        if os.sep in name or name.endswith(".png"):
            dst = name if name.endswith(".png") else name + ".png"
        else:
            dst = os.path.join(self.shots_dir, name + ".png")
        self.v.move(path, dst)
        self.shots += 1
        w, h, rows = self.v.read_png(dst)
        if self.v.blank(w, h, rows):
            self.failures.append(f"{name}: blank screenshot ({dst})")
            return
        if self.refs is None or os.sep in name or name.endswith(".png"):
            self.say(f"  shot {dst} ({w}x{h})")
            return
        ref = os.path.join(self.refs, name + ".png")
        if self.update:
            self.v.write_png(ref, w, h, rows)
            self.say(f"  shot {name}: reference written to {ref}")
            return
        if not os.path.exists(ref):
            self.failures.append(f"{name}: no reference {ref} (run with --update)")
            return
        diff = dst[:-4] + ".diff.png"
        pct, _, detail = self.v.compare((w, h, rows), self.v.read_png(ref), diff)
        if pct > tolerance:
            self.failures.append(f"{name}: {detail}, tolerance {tolerance:g}% (see {dst}, {diff})")
            self.say(f"  shot {name}: DIFFERENT {detail}")
        else:
            os.remove(diff)
            self.say(f"  shot {name}: matches ({detail})")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("elf", help="the ELF to run, or a disc image (.iso) to boot")
    ap.add_argument("--elf", dest="elf_symbols", metavar="ELF",
                    help="with a disc image: the ELF on it, for memory probing")
    ap.add_argument("--slowboot", action="store_true", help="with a disc image: boot through the whole BIOS, browser included")
    ap.add_argument("--timeout", type=float, default=60, help="seconds before TIMEOUT (default 60)")
    ap.add_argument("--expect", choices=list(EXIT_CODES), help="exit 0 only if the verdict equals this")
    ap.add_argument("--detail", help="with --expect: the verdict's detail must also contain this text")
    ap.add_argument("--until", metavar="TEXT",
                    help="stream the guest output and PASS as soon as a line contains TEXT (a boot smoke test)")
    ap.add_argument("--run", action="store_true", help="just run for --timeout seconds and stream the guest output")
    ap.add_argument("--test", action="store_true",
                    help="run a Go test binary: stream its output, verdict from its exit code (PS2GO-EXIT); "
                         "used as the emulator of TinyGo's ps2 target")
    ap.add_argument("ignored", nargs=argparse.REMAINDER, help="extra arguments (test flags) are ignored")
    ap.add_argument("--pcsx2-dir", default=os.environ.get("PS2GO_PCSX2_DIR", DEFAULT_DIR))
    ap.add_argument("--log", help="keep the PCSX2 log at this path")
    ap.add_argument("--probe", type=float, default=0, metavar="SECS", help="print the guest stats block every SECS")
    ap.add_argument("--visual", metavar="STEPS", help="render (software renderer) and follow this steps file, comparing screenshots")
    ap.add_argument("--screenshot", metavar="PNG", help="render and save one screenshot to PNG (after --until TEXT, or 3 seconds)")
    ap.add_argument("--update", action="store_true", help="with --visual: write the reference images instead of comparing")
    ap.add_argument("--refs", metavar="DIR", help="with --visual: reference images directory (default: the steps file's)")
    ap.add_argument("--shots-dir", metavar="DIR", help="with --visual: where the screenshots go (default: <elf dir>/visual)")
    ap.add_argument("--net", action="store_true",
                    help="enable the emulated ethernet (DEV9, PCSX2's Sockets backend: DHCP, TCP/UDP through the host)")
    ap.add_argument("--pine-slot", type=int, default=28011, help="PINE slot of the PCSX2 instance (default 28011)")
    ap.add_argument("-q", "--quiet", action="store_true", help="only print the verdict line")
    args = ap.parse_args()

    elf = os.path.abspath(args.elf)
    disc = elf.lower().endswith(".iso")
    binary = os.path.join(args.pcsx2_dir, "squashfs-root", "usr", "bin", "pcsx2-qt")
    datadir = os.path.join(args.pcsx2_dir, "data")
    log = os.path.abspath(args.log or f"/tmp/ps2test-{os.getpid()}.log")
    steps = None
    if args.visual or args.screenshot:
        import visual
        if args.screenshot:
            steps = [("until", [args.until]) if args.until else ("sleep", ["3"]), ("shot", [os.path.abspath(args.screenshot)])]
            args.until = None
        else:
            steps = visual.parse_steps(args.visual)

    def say(msg):
        if not args.quiet:
            print(msg, flush=True)

    def finish(verdict, detail=""):
        line = f"PS2TEST {verdict} {os.path.basename(elf)}"
        if detail:
            line += f": {detail}"
        print(line, flush=True)
        if args.expect:
            ok = verdict == args.expect and (not args.detail or args.detail in (detail or ""))
            return 0 if ok else 5
        return EXIT_CODES[verdict]

    for path, what in ((elf, "ELF"), (binary, "PCSX2 binary"), (os.path.join(datadir, "PCSX2", "inis", "PCSX2.ini"), "PCSX2 ini")):
        if not os.path.exists(path):
            return finish("ERROR", f"{what} not found: {path}")
    biosdir = os.path.join(datadir, "PCSX2", "bios")
    if not os.path.isdir(biosdir) or not os.listdir(biosdir):
        return finish("ERROR", f"no BIOS in {biosdir}")
    if steps is not None or args.net or args.pine_slot != 28011:
        import visual
        name, overrides = [], {}
        if steps is not None:
            name.append("visual")
            overrides.update(visual.INI_OVERRIDES)
        if args.net:
            name.append("net")
            overrides.update(visual.net_overrides())
        if args.pine_slot != 28011:
            # Another slot means another PCSX2 may run at the same time:
            # its own data directory, its own PINE socket.
            name.append(f"slot{args.pine_slot}")
            overrides[("EmuCore", "PINESlot")] = str(args.pine_slot)
        datadir = visual.prepare_datadir(args.pcsx2_dir, "-".join(name), overrides)
    helpers = None
    if args.net:
        try:
            helpers = visual.NetHelpers()
        except OSError as e:
            return finish("ERROR", f"net helpers: {e}")

    block_addr = None
    symbols = args.elf_symbols or (None if disc else elf)
    if symbols:
        try:
            block_addr = elf_symbol(symbols, BLOCK_SYMBOL)
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
    # A disc image boots through the BIOS (fast boot skips its browser).
    boot = ["-slowboot" if args.slowboot else "-fastboot", elf] if disc else ["-elf", elf]
    proc = subprocess.Popen([binary, "-datapath", datadir, "-batch", "-nogui", "-logfile", log] + boot,
                            env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    pine = None
    verdict, detail = None, ""
    t0 = time.time()
    runner = None
    if steps is not None:
        runner = Steps(steps, display, os.path.join(datadir, "PCSX2", "snaps"),
                       args.shots_dir or os.path.join(os.path.dirname(elf), "visual"),
                       args.refs or (os.path.dirname(os.path.abspath(args.visual)) if args.visual else None),
                       args.update, say)
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
                    elif text.startswith("PS2GO-") or args.run or args.until:
                        say(f"  {text}")
                    if runner:
                        runner.feed(text)
                    # A crash is sticky: PCSX2 logs the faulting access and
                    # carries on, so a later result marker must not hide it.
                    if verdict == "CRASH":
                        continue
                    if text.startswith("PS2GO-RESULT"):
                        verdict = "PASS" if text.split()[1] == "PASS" else "FAIL"
                        detail = " ".join(text.split()[2:])
                    elif args.until and args.until in text:
                        verdict, detail = "PASS", f"reached {args.until!r}"
                    elif text.startswith("PS2GO-EXIT") and (args.test or text.split()[1] != "0"):
                        code = text.split()[1]
                        verdict = "PASS" if code == "0" else "FAIL"
                        detail = f"exit code {code}"
                    elif text.startswith("Startup Error") or "PCSX2 requires a PlayStation 2 BIOS" in text:
                        verdict, detail = "ERROR", text
                    elif any(p.search(text) for p in CRASH_PATTERNS):
                        verdict, detail = "CRASH", text
            if runner and not verdict:
                try:
                    runner.tick(now)
                except (OSError, RuntimeError, ValueError) as e:
                    verdict, detail = "ERROR", str(e)
                if runner.done():
                    verdict, detail = runner.verdict()
            if verdict and not args.run:
                break
            if proc.poll() is not None:
                verdict = verdict or "ERROR"
                detail = detail or f"PCSX2 exited early with code {proc.returncode}"
                break
            if now - t0 > args.timeout:
                if not args.run:
                    verdict, detail = "TIMEOUT", f"after {args.timeout:.0f}s"
                    if runner:
                        detail += f" at step {runner.where()}"
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
        if helpers:
            helpers.stop()
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
