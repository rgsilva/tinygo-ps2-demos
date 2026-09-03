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
# Demos: each demo is a directory with a Go main package. Extra objects linked
# into a demo are listed in <demo>_OBJS: names of files in resources/ (without
# extension) or of IOP modules from the ps2sdk (see IRX below).
# ---------------------------------------------------------------------------

DEMOS = flappygopher test

flappygopher_OBJS = freesio2 freepad gopher arial bird pipe gameover sky
flappygopher_TINYGO_FLAGS =

test_OBJS =
test_TINYGO_FLAGS = -opt 0
# The ps2sdk archives carry LTO bytecode, so the -O level given at link time
# decides how the SDK code (crt0, libc, libdebug, ...) is compiled into this ELF.
test_LDFLAGS = -O0

# The test suite (tests/) and the harness's negative controls (controls/*)
# are built like demos; `make check` runs them in PCSX2 (harness/ps2test.py).
TESTS = tests
CONTROLS = controls/fail controls/hang controls/crash

# IOP modules taken from the ps2sdk inside the image and embedded via bin2c.
IRX = freesio2 freepad

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
CLANG_FLAGS = -c -fno-pic --target=mips64el -mcpu=r5900 -mabi=n32 -mhard-float \
              -mxgot -mlittle-endian -fno-inline-functions

# C for the EE (loader).
EE_CFLAGS = -D_EE -G0 -O2 -Wall -gdwarf-2 -gz -mxgot \
            -I$(PS2SDK_IMG)/ee/include -I$(PS2SDK_IMG)/common/include \
            -I$(PS2DEV_IMG)/gsKit/include

# Link. The ps2sdk linker script is used as is; TinyGo's runtime needs three
# extra symbols for the heap and stack.
EE_LINKFILE = $(PS2SDK_IMG)/ee/startup/linkfile
EE_LDFLAGS  = -T$(EE_LINKFILE) \
              -Wl,--defsym=_heap_start=_end \
              -Wl,--defsym=_heap_end=0x02000000 \
              -Wl,--defsym=_stack_top=0x02000000-_stack_size \
              -Wl,-zmax-page-size=128 -mhard-float -msingle-float
EE_LIBS     = -L$(PS2SDK_IMG)/ee/lib -L$(PS2SDK_IMG)/ports/lib -L$(PS2DEV_IMG)/gsKit/lib \
              -lpatches -lfileXio -lpad -ldebug -lgskit_toolkit -lgskit -ldmakit -lpng -ljpeg -lz

# ---------------------------------------------------------------------------
# Rules
# ---------------------------------------------------------------------------

.PHONY: all $(DEMOS) $(TESTS) $(CONTROLS) check check-harness shell clean
.SECONDEXPANSION:
.DELETE_ON_ERROR:

all: $(DEMOS) $(TESTS)

$(DEMOS) $(TESTS) $(CONTROLS): %: $(BUILD)/%.elf

$(BUILD):
	$(Q)mkdir -p $@

# Link order: IOP modules, runtime glue, the program, the loader, resources.
objs = $(addprefix $(BUILD)/,$(addsuffix .o,$(1)))
$(BUILD)/%.elf: $$(call objs,$$(filter $(IRX),$$($$*_OBJS))) $(BUILD)/asm_mipsx.o $(BUILD)/%.o $(BUILD)/loader.o $$(call objs,$$(filter-out $(IRX),$$($$*_OBJS))) $(MAKEFILES_)
	@echo "  LINK    $@"
	$(Q)mkdir -p $(@D)
	$(Q)$(DOCKER) $(EE_CC) $(EE_LDFLAGS) $($*_LDFLAGS) -o $@ $(filter %.o,$^) $(EE_LIBS)

# Go -> LLVM IR. Demos import the shared packages, so depend on all Go sources.
GO_SOURCES := $(shell find . -name '*.go' -not -path './$(BUILD)/*')
$(BUILD)/%.ll: $(GO_SOURCES) $(MAKEFILES_) | $(BUILD)
	@echo "  TINYGO  $@"
	$(Q)mkdir -p $(@D)
	$(Q)CGO_CFLAGS="$(CGO_CFLAGS)" $(TINYGO) build $(TINYGO_FLAGS) $($*_TINYGO_FLAGS) -o $@ ./$*

$(BUILD)/%.o: $(BUILD)/%.ll
	@echo "  CLANG   $@"
	$(Q)$(CLANG) $(CLANG_FLAGS) -o $@ $<

$(BUILD)/asm_mipsx.o: loader/asm_mipsx.S | $(BUILD)
	@echo "  CLANG   $@"
	$(Q)$(CLANG) $(CLANG_FLAGS) -o $@ $<

$(BUILD)/loader.o: loader/loader.c | $(BUILD)
	@echo "  EE_CC   $@"
	$(Q)$(DOCKER) $(EE_CC) $(EE_CFLAGS) -c -o $@ $<

# Binary resources -> objects (bin2c from the ps2sdk, symbol = file name).
RESOURCES := $(notdir $(basename $(wildcard resources/*.raw resources/*.fnt)))
define BIN2O
	@echo "  BIN2O   $@"
	$(Q)$(DOCKER) sh -c 'bin2c $(1) $(basename $@).c $* && $(EE_CC) -c -o $@ $(basename $@).c && rm $(basename $@).c'
endef

$(patsubst %,$(BUILD)/%.o,$(RESOURCES)): $(BUILD)/%.o: $$(wildcard resources/%.raw resources/%.fnt) | $(BUILD)
	$(call BIN2O,$<)

$(patsubst %,$(BUILD)/%.o,$(IRX)): $(BUILD)/%.o: | $(BUILD)
	$(call BIN2O,$(PS2SDK_IMG)/iop/irx/$*.irx)

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
	$(PS2TEST) --expect PASS    $(BUILD)/tests.elf

# Run any ELF for TIMEOUT seconds and stream its serial output.
run-%: $(BUILD)/%.elf
	$(PS2TEST) --run $<

shell:
	docker run --rm -it --user=$(shell id -u):$(shell id -g) -v $(CURDIR):/src -w /src $(IMAGE)

clean:
	rm -rf $(BUILD)
