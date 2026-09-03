# TinyGo PlayStation 2 demos.
#
# Go code is compiled to LLVM IR by a custom TinyGo on the host, turned into a
# MIPS N32 object by the matching clang, and linked against the ps2sdk inside
# the ps2dev/ps2dev Docker image (pulled automatically on first use).
#
# Local settings go in config.mk (gitignored) or on the command line:
#   PS2DEV  host copy of the ps2dev tree (headers only, for cgo)
#   TINYGO  custom tinygo binary
#   CLANG   clang from the custom LLVM build
#   IMAGE   docker image with the ps2sdk toolchain (pinned tag; keep PS2DEV in sync)
#   PCSX2_DIR  PCSX2 install for `make check` (see harness/setup-pcsx2.sh)
# Use V=1 for verbose output.

-include config.mk

PS2DEV ?= /usr/local/ps2dev
TINYGO ?= tinygo
CLANG  ?= clang
IMAGE  ?= ps2dev/ps2dev:v2.0.0
PCSX2_DIR ?= $(HOME)/dev/ps2go/tools/pcsx2
PYTHON ?= python3
V      ?= 0

# ---------------------------------------------------------------------------
# Demos: each demo is a directory with a Go main package. Assets and IOP
# modules are embedded from the resources package (go:embed); the IOP modules
# are copied there from the ps2sdk in the image.
# ---------------------------------------------------------------------------

DEMOS = flappygopher test

flappygopher_TINYGO_FLAGS =

test_TINYGO_FLAGS = -opt 0
# The ps2sdk archives carry LTO bytecode, so the -O level given at link time
# decides how the SDK code (crt0, libc, libdebug, ...) is compiled into this ELF.
test_LDFLAGS = -O0

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

# Paths inside the image.
PS2DEV_IMG = /usr/local/ps2dev
PS2SDK_IMG = $(PS2DEV_IMG)/ps2sdk

DOCKER = docker run --rm $(if $(MAKE_TERMOUT),-t) --user=$(shell id -u):$(shell id -g) \
         -v $(CURDIR):/src -w /src $(IMAGE)
EE_CC  = mips64r5900el-ps2-elf-gcc
EE_OBJCOPY = mips64r5900el-ps2-elf-objcopy

# Headers for cgo (host side). gcc's own headers (stddef.h, ...) live in a
# versioned directory, so detect the version.
EE_GCC_VER = $(notdir $(wildcard $(PS2DEV)/ee/lib/gcc/mips64r5900el-ps2-elf/*))
CGO_CFLAGS = \
	-I$(PS2DEV)/ee/lib/gcc/mips64r5900el-ps2-elf/$(EE_GCC_VER)/include \
	-I$(PS2DEV)/ee/mips64r5900el-ps2-elf/include \
	-I$(PS2DEV)/gsKit/include \
	-I$(PS2DEV)/ps2sdk/common/include \
	-I$(PS2DEV)/ps2sdk/ee/include \
	-I$(PS2DEV)/ps2sdk/ports/include/freetype2 \
	-I$(PS2DEV)/ps2sdk/ports/include/zlib

TINYGO_FLAGS = -gc conservative -target ps2 $(if $(filter 1,$(V)),-x)

# LLVM IR / assembly -> MIPS N32 object, for the EE.
CLANG_FLAGS = -c -fno-pic --target=mips64el-unknown-none-gnuabin32 -mcpu=r5900 -mabi=n32 -mhard-float -msingle-float \
              -mxgot -mlittle-endian -fno-inline-functions

# Link. The linker script is the ps2sdk one with one section added in front
# of .data: the Go program's own .data and .bss, bracketed by
# _globals_start/_globals_end, which is the range the GC scans for roots.
# Scanning the whole SDK .data..bss instead pinned megabytes of heap: the
# kernel patch blobs in libkernel are full of words that look like pointers.
# Data that must not be scanned (big buffers, counters) can be placed in the
# .ps2go.noscan section. The script is generated per program (build/%.ld).
# TinyGo's runtime also needs the heap bounds (replaced at runtime by a libc
# allocation) and the stack top: crt0 asks the kernel for a stack of
# _stack_size bytes at the top of RAM, [0x02000000-_stack_size, 0x02000000).
SDK_LINKFILE = $(PS2DEV)/ps2sdk/ee/startup/linkfile
EE_LDFLAGS  = -Wl,--defsym=_heap_start=_end \
              -Wl,--defsym=_heap_end=0x02000000 \
              -Wl,--defsym=_stack_top=0x02000000 \
              -Wl,-zmax-page-size=128 -mhard-float -msingle-float
EE_LIBS     = -L$(PS2SDK_IMG)/ee/lib -L$(PS2SDK_IMG)/ports/lib -L$(PS2DEV_IMG)/gsKit/lib \
              -lpatches -lfileXio -lpad -ldebug -lgskit_toolkit -lgskit -ldmakit -lpng -ljpeg -lz

# ---------------------------------------------------------------------------
# Rules
# ---------------------------------------------------------------------------

.PHONY: all $(DEMOS) $(TESTS) $(CONTROLS) check check-harness shell clean
.DELETE_ON_ERROR:

all: $(DEMOS) $(TESTS)

$(DEMOS) $(TESTS) $(CONTROLS): %: $(BUILD)/%.elf

$(BUILD):
	$(Q)mkdir -p $@

# Link: runtime glue (assembly) and the program.
$(BUILD)/%.elf: $(BUILD)/asm_mipsx.o $(BUILD)/%.o $(BUILD)/%.ld $(MAKEFILES_)
	@echo "  LINK    $@"
	$(Q)mkdir -p $(@D)
	$(Q)$(DOCKER) sh -c '$(EE_OBJCOPY) --set-section-flags .bss=alloc,load,contents,data $(BUILD)/$*.o && \
	  $(EE_CC) -T$(BUILD)/$*.ld $(EE_LDFLAGS) $($*_LDFLAGS) -o $@ $(filter %.o,$^) $(EE_LIBS)'

# Linker script for one program: the SDK script plus the Go globals section.
$(BUILD)/%.ld: $(SDK_LINKFILE) $(MAKEFILES_)
	@echo "  LDSCRIPT $@"
	$(Q)mkdir -p $(@D)
	$(Q)awk -v go='$(BUILD)/$*.o' '\
	  /^\t\.data ALIGN\(128\): \{/ { \
	    print "\t/* Go globals: the range the GC scans for roots (ps2go). The"; \
	    print "\t   program has a single load segment, so the Go .bss is turned"; \
	    print "\t   into file-backed zeros (objcopy, see the link rule). */"; \
	    print "\t.go.data ALIGN(16): {"; \
	    print "\t\t_globals_start = . ;"; \
	    print "\t\t" go "(.data .data.* .bss .bss.* COMMON)"; \
	    print "\t\t. = ALIGN(16);"; \
	    print "\t\t_globals_end = . ;"; \
	    print "\t}"; \
	    print; print "\t\t*(.ps2go.noscan)"; next } \
	  { print }' $< > $@
	$(Q)grep -q '_globals_end' $@ || { echo "failed to patch $(SDK_LINKFILE)"; rm -f $@; exit 1; }

# Go -> LLVM IR. Demos import the shared packages, so depend on all Go sources
# (and on the embedded IOP modules).
GO_SOURCES := $(shell find . -name '*.go' -not -path './$(BUILD)/*')
$(BUILD)/%.ll: $(GO_SOURCES) $(IRX_FILES) $(MAKEFILES_) | $(BUILD)
	@echo "  TINYGO  $@"
	$(Q)mkdir -p $(@D)
	$(Q)CGO_CFLAGS="$(CGO_CFLAGS)" $(TINYGO) build $(TINYGO_FLAGS) $($*_TINYGO_FLAGS) -o $@ ./$*

$(BUILD)/%.o: $(BUILD)/%.ll
	@echo "  CLANG   $@"
	$(Q)$(CLANG) $(CLANG_FLAGS) -o $@ $<

$(BUILD)/asm_mipsx.o: loader/asm_mipsx.S | $(BUILD)
	@echo "  CLANG   $@"
	$(Q)$(CLANG) $(CLANG_FLAGS) -o $@ $<

# IOP modules from the ps2sdk in the image, for the resources package.
resources/%.irx:
	@echo "  IRX     $@"
	$(Q)$(DOCKER) cp $(PS2SDK_IMG)/iop/irx/$*.irx $@

# Keep intermediate files (.ll, .o) instead of deleting them after the link.
.SECONDARY:

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

# Run any ELF for TIMEOUT seconds and stream its serial output.
run-%: $(BUILD)/%.elf
	$(PS2TEST) --run $<

shell:
	docker run --rm -it --user=$(shell id -u):$(shell id -g) -v $(CURDIR):/src -w /src $(IMAGE)

clean:
	rm -rf $(BUILD)
