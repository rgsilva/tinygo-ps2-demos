# You need to change these.
PS2DEV = /Users/ricardo/dev/ps2dev
TINYGO = /Users/ricardo/dev/tinygo/build/tinygo
CLANG = /Users/ricardo/dev/tinygo/llvm-build/bin/clang

CGO_CFLAGS = \
	-I$(PS2DEV)/ee/lib/gcc/mips64r5900el-ps2-elf/14.2.0/include/ \
    -I$(PS2DEV)/ee/mips64r5900el-ps2-elf/include \
    -I$(PS2DEV)/gsKit/include \
    -I$(PS2DEV)/ps2sdk/common/include \
    -I$(PS2DEV)/ps2sdk/ee/include \
    -I$(PS2DEV)/ps2sdk/ports/include/freetype2 \
    -I$(PS2DEV)/ps2sdk/ports/include/zlib

.PHONY: loader flappygopher test

ps2dev:
	@docker build -f Dockerfile.ps2dev -t rgsilva/ps2dev .

loader:
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/loader rgsilva/ps2dev make
	$(CLANG) -fno-pic -c -mcpu=mips3 -fno-inline-functions --target=mips64el -mabi=n32 -mhard-float -mxgot -mlittle-endian -o loader/asm_mipsx.o loader/asm_mipsx.S
	@mv -f loader/*.o build/

flappygopher:
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o gopher.raw gopher
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o arial.fnt arial
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o bird.raw bird
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o pipe.raw pipe
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o gameover.raw gameover
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o sky.raw sky
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o ../loader/sio2man.irx sio2man
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o ../loader/padman.irx padman
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o ../loader/freesio2.irx freesio2
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src/resources rgsilva/ps2dev ./bin2o ../loader/freepad.irx freepad
	@mv -f resources/*.o build/
	CGO_CFLAGS="$(CGO_CFLAGS)" $(TINYGO) build -x -gc conservative -target ps2 -o build/flappygopher.ll flappygopher/main.go
	$(CLANG) -fno-pic -c --target=mips64el -mcpu=mips3 -fno-inline-functions -mabi=n32 -mhard-float -mxgot -mlittle-endian -o build/flappygopher.o build/flappygopher.ll
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src rgsilva/ps2dev \
		mips64r5900el-ps2-elf-gcc \
		-Tlinkfile \
		-L/usr/local/ps2dev/ps2sdk/ee/lib \
		-L/usr/local/ps2dev/ps2sdk/ports/lib \
		-L/usr/local/ps2dev/gsKit/lib/ \
		-Lmodules/ds34bt/ee/ \
		-Lmodules/ds34usb/ee/ \
		-Wl,-zmax-page-size=128 \
		-lpatches \
		-lfileXio \
		-lpad \
		-ldebug \
		-lmath3d \
		-ljpeg \
		-lfreetype \
		-lgskit_toolkit \
		-lgskit \
		-ldmakit \
		-lpng \
		-lz \
		-lmc \
		-laudsrv \
		-lelf-loader \
		-laudsrv \
		-lc \
		-ldraw \
		-lgraph \
		-lpacket \
		-ldma \
		-lmath3d \
		-mhard-float \
		-msingle-float \
		-o build/flappygopher.elf \
		build/freesio2.o \
		build/freepad.o \
		build/asm_mipsx.o \
		build/flappygopher.o \
		build/loader.o \
		build/gopher.o \
		build/arial.o \
		build/bird.o \
		build/pipe.o \
		build/gameover.o \
		build/sky.o

test:
	cd test && CGO_CFLAGS="$(CGO_CFLAGS)" $(TINYGO) build -opt 0 -x -gc conservative -target ps2 -o ../build/test.ll
	$(CLANG) -fno-pic -c --target=mips64el -mcpu=mips3 -fno-inline-functions -mabi=n32 -mhard-float -mxgot -mlittle-endian -v -o build/test.o build/test.ll
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src rgsilva/ps2dev \
		mips64r5900el-ps2-elf-gcc \
		-Tlinkfile \
		-L/usr/local/ps2dev/ps2sdk/ee/lib \
		-L/usr/local/ps2dev/ps2sdk/ports/lib \
		-L/usr/local/ps2dev/gsKit/lib/ \
		-Lmodules/ds34bt/ee/ \
		-Lmodules/ds34usb/ee/ \
		-Wl,-zmax-page-size=128 \
		-lpatches \
		-lfileXio \
		-lpad \
		-ldebug \
		-lmath3d \
		-ljpeg \
		-lfreetype \
		-lgskit_toolkit \
		-lgskit \
		-ldmakit \
		-lpng \
		-lz \
		-lmc \
		-laudsrv \
		-lelf-loader \
		-laudsrv \
		-lc \
		-ldraw \
		-lgraph \
		-lpacket \
		-ldma \
		-lmath3d \
		-mhard-float \
		-msingle-float \
		-O0 \
		-o build/test.elf \
		build/asm_mipsx.o \
		build/test.o \
		build/loader.o

shell:
	@docker run --rm -ti --user=$(shell id -u):$(shell id -g) -v ${PWD}:/src -w /src rgsilva/ps2dev

clean:
	rm -rf loader/*.o loader/*.irx loader/*.map
	rm -rf build/*
