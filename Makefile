# TinyGo PlayStation 2 demos.
#
# `tinygo build -target ps2` does it all: the custom TinyGo compiles the Go
# code to a MIPS N32 object and links it with the ps2sdk gcc (targets/ps2.json
# in the fork names the linker, the SDK paths through $PS2DEV, and the linker
# script targets/ps2.ld). The gcc runs in the ps2dev/ps2dev Docker image
# through tools/mips64r5900el-ps2-elf-gcc unless a native toolchain is on PATH.
#
# Local settings go in config.mk (gitignored) or on the command line:
#   PS2DEV  ps2dev tree: headers and libraries (a copy extracted from the image)
#   TINYGO  custom tinygo binary
#   IMAGE   docker image with the ps2sdk toolchain (pinned tag; keep PS2DEV in sync)
#   PCSX2_DIR  PCSX2 install for `make check` (see harness/setup-pcsx2.sh)
# Use V=1 for verbose output.

-include config.mk

PS2DEV ?= /usr/local/ps2dev
TINYGO ?= tinygo
IMAGE  ?= ps2dev/ps2dev:v2.0.0
PCSX2_DIR ?= $(HOME)/dev/ps2go/tools/pcsx2
PYTHON ?= python3
V      ?= 0

# ---------------------------------------------------------------------------
# Demos: each demo is a directory with a Go main package. Assets and IOP
# modules are embedded from the resources package (go:embed); the IOP modules
# are copied there from the ps2sdk in the image.
# ---------------------------------------------------------------------------

DEMOS = flappygopher

# Per-program knobs: <name>_TINYGO_FLAGS and <name>_LIBC_HEAP. Libraries are
# declared by the packages that use them (#cgo LDFLAGS lines).
flappygopher_TINYGO_FLAGS =

# EE memory map, decided here at link time (32 MB of RAM, the first 1 MB is
# the kernel's): program | libc heap | Go heap | stack. The libc heap (gsKit
# structures, C mallocs) is LIBC_HEAP bytes after the program, capped there by
# crt0/SetupHeap; the Go heap is everything up to the stack. Override per
# program with <name>_LIBC_HEAP.
LIBC_HEAP ?= 4*1024*1024

# The test suite (tests/) and the harness's negative controls (controls/*)
# are built like demos; `make check` runs them in PCSX2 (harness/ps2test.py).
TESTS = tests
CONTROLS = controls/fail controls/hang controls/crash controls/panic controls/deadlock controls/gopanic

# IOP modules embedded by the resources package, taken from the ps2sdk.
IRX = freesio2 freepad
IRX_FILES = $(addprefix resources/,$(addsuffix .irx,$(IRX)))

# ---------------------------------------------------------------------------
# Tools and flags
# ---------------------------------------------------------------------------

BUILD = build
Q = $(if $(filter 1,$(V)),,@)
# Flags live here, so rebuild when the Makefile (or config.mk) changes.
MAKEFILES_ = Makefile $(wildcard config.mk)

# Paths inside the image (IOP modules are copied from there).
PS2DEV_IMG = /usr/local/ps2dev
PS2SDK_IMG = $(PS2DEV_IMG)/ps2sdk

DOCKER = docker run --rm $(if $(MAKE_TERMOUT),-t) --user=$(shell id -u):$(shell id -g) \
         -v $(CURDIR):/src -w /src $(IMAGE)

# Precise GC (pointer-free objects are not scanned: faster collections with
# buffers and tables, slower with many small pointer-heavy objects) and speed
# over size. Measured on the suite, 2026-09-04: idle collection 7.7 ms vs
# 14.4 ms, 10k interior pointers 10 ms vs 40 ms, suite 6.4 s vs 7.9 s, code
# +15%.
TINYGO_FLAGS = -gc precise -opt 2 -target ps2 $(if $(filter 1,$(V)),-x)
# What tinygo needs from the environment: the SDK root for targets/ps2.json
# and the gcc (wrapper) on PATH.
TINYGO_ENV = PS2DEV=$(PS2DEV) PS2DEV_IMAGE=$(IMAGE) PATH=$(CURDIR)/tools:$$PATH \
             TINYGOROOT=$$($(TINYGO) env TINYGOROOT) \
             PS2GO_HARNESS=$(CURDIR)/harness PS2GO_PCSX2_DIR=$(PCSX2_DIR)

# ---------------------------------------------------------------------------
# Rules
# ---------------------------------------------------------------------------

.PHONY: all $(DEMOS) $(TESTS) $(CONTROLS) check check-harness check-gotest shell clean
.DELETE_ON_ERROR:

all: $(DEMOS) $(TESTS)

$(DEMOS) $(TESTS) $(CONTROLS): %: $(BUILD)/%.elf

$(BUILD):
	$(Q)mkdir -p $@

# Demos import the shared packages, so depend on all Go sources (and on the
# embedded IOP modules).
GO_SOURCES := $(shell find . -name '*.go' -not -path './$(BUILD)/*')
$(BUILD)/%.elf: $(GO_SOURCES) $(IRX_FILES) $(MAKEFILES_) | $(BUILD)
	@echo "  TINYGO  $@"
	$(Q)mkdir -p $(@D)
	$(Q)$(TINYGO_ENV) $(TINYGO) build $(TINYGO_FLAGS) $($*_TINYGO_FLAGS) \
	  -ldflags '-extldflags "-Wl,--defsym=_heap_size=$(or $($*_LIBC_HEAP),$(LIBC_HEAP))"' -o $@ ./$*

# IOP modules from the ps2sdk in the image, for the resources package.
resources/%.irx:
	@echo "  IRX     $@"
	$(Q)$(DOCKER) cp $(PS2SDK_IMG)/iop/irx/$*.irx $@

# Run the test suite in PCSX2 (headless). TIMEOUT is in seconds.
TIMEOUT ?= 120
PS2TEST = PS2GO_PCSX2_DIR=$(PCSX2_DIR) $(PYTHON) harness/ps2test.py --timeout $(TIMEOUT)
check: $(BUILD)/tests.elf
	$(PS2TEST) $<

# Prove the harness itself: each control must produce its verdict.
check-harness: $(patsubst %,$(BUILD)/%.elf,$(CONTROLS)) $(BUILD)/tests.elf
	$(PS2TEST) --expect FAIL    $(BUILD)/controls/fail.elf
	$(PS2TEST) --expect TIMEOUT --timeout 20 $(BUILD)/controls/hang.elf
	$(PS2TEST) --expect CRASH   $(BUILD)/controls/crash.elf
	$(PS2TEST) --expect CRASH   $(BUILD)/controls/panic.elf
	$(PS2TEST) --expect CRASH   $(BUILD)/controls/deadlock.elf
	$(PS2TEST) --expect CRASH   $(BUILD)/controls/gopanic.elf
	$(PS2TEST) --expect PASS    $(BUILD)/tests.elf

# Go's own testing package on the PS2: `tinygo test` builds the test binary
# and runs it in PCSX2 through the harness (the target's emulator). The
# second run must fail (it includes a failing test).
check-gotest:
	$(TINYGO_ENV) $(TINYGO) test $(TINYGO_FLAGS) ./gotest
	! $(TINYGO_ENV) $(TINYGO) test $(TINYGO_FLAGS) -tags ps2fail ./gotest

# Run any ELF for TIMEOUT seconds and stream its serial output.
run-%: $(BUILD)/%.elf
	$(PS2TEST) --run $<

shell:
	docker run --rm -it --user=$(shell id -u):$(shell id -g) -v $(CURDIR):/src -w /src $(IMAGE)

clean:
	rm -rf $(BUILD)
