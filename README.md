# TinyGo PlayStation 2 Demos

This repository contains a few demo projects showcasing the use of TinyGo to create applications on the PlayStation 2 console.
Everything here is provided _AS IS_ and such should not be used for any actual game and/or software development. You are free
to use this as you wish.

This is a hobby project and I'm not a game developer. This project was created for fun only. The write-up is provided in
[my blog](https://rgsilva.com/blog/), so feel free to check it out there.

## Building

Currently this requires a custom version of the TinyGo compiler. The custom version is provided
[here](https://github.com/rgsilva/tinygo-ps2), and it requires [this](https://github.com/rgsilva/tinygo-ps2-llvm/tree/ps2/tinygo_22.x)
custom version of the LLVM to work.

`tinygo build -target ps2` does it all: the custom TinyGo compiles the Go code to a MIPS N32
object on the host and links it with the ps2sdk gcc, which runs in the amd64 `ps2dev/ps2dev`
Docker image through `tools/mips64r5900el-ps2-elf-gcc` (I'm on an Apple silicon Mac; a native
ps2dev toolchain on `PATH` works too). So you'll need Docker; the image is pulled automatically
on the first build. The target (`targets/ps2.json` in the fork) finds the SDK headers and
libraries through the `PS2DEV` environment variable, which the Makefile sets.

Tell the Makefile where your tools are, either in a `config.mk` file next to it (gitignored):

```make
PS2DEV = /path/to/ps2dev      # a copy of the image's /usr/local/ps2dev (headers and libraries)
TINYGO = /path/to/tinygo-ps2/build/tinygo
```

or on the command line (`make PS2DEV=... TINYGO=...`). After that:

* Everything: `make`
* Flappy Gopher: `make flappygopher`
* Test suite: `make tests` (see Testing below)

ELFs end up in `build/`. Use `V=1` to see the commands. `PCSX2_DIR` in `config.mk` points at the
PCSX2 install used by `make check` (default `~/dev/ps2go/tools/pcsx2`). Adding a demo is a directory with a Go main
package plus one line in the `DEMOS` list in the Makefile.

## Testing

There is a headless test harness that runs ELFs in PCSX2 (real BIOS, Null renderer,
no display needed). One-time setup, with a PS2 BIOS dump you own:

```sh
harness/setup-pcsx2.sh ~/dev/ps2go/tools/pcsx2 /path/to/bios.bin
```

Then:

* `make check` builds `tests/` and runs it: each case prints `PS2GO-CASE <name> PASS|FAIL`
  on the EE serial port, which ends up in PCSX2's log, and the run ends with `PS2GO-RESULT`.
  Cases known to fail on the current runtime are marked `XFail` and reported as `XFAIL`.
* `make check-harness` proves the harness itself with the negative controls in `controls/` (a failing
  case, a hang, a crash, panics, a deadlock, a goroutine stack overflow)
  (a failing case, a hang, an unmapped memory access, an unrecovered panic) which must produce FAIL,
  TIMEOUT, CRASH and CRASH.
* `make run-<demo>` runs any ELF for `TIMEOUT` seconds and streams its serial output.
* `make check-gotest` runs Go's own `testing` package on the PS2: `tinygo test -target ps2 ./gotest`
  builds the test binary and runs it in PCSX2 through the harness, which is the target's emulator
  (`PS2GO_HARNESS` points at `harness/`). A test failure makes `tinygo test` fail.
* `harness/ps2test.py --probe 2 build/tests.elf` also reads the guest's stats block
  (heap in use, allocations, current case) over PCSX2's PINE socket while it runs.

The guest side is the `harness` package: call `harness.Run` with your cases, and use
`harness.Log` (or plain `println`) for output. Goroutines, channels, timers and recover
are all covered by the suite.

## Memory notes

The GC is conservative. It scans the Go program's own globals (the generated linker script
gathers them into one section) and the stacks; pointer-free globals are moved out of the
scan by TinyGo. Large buffers that never hold Go pointers can be put in the `.ps2go.noscan`
section from C. Objects referenced only from C memory (the ps2sdk heap, DMA buffers) are not
kept alive: keep a Go reference too.

Have fun!
