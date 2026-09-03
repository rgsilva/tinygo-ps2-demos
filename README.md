# TinyGo PlayStation 2 Demos

This repository contains a few demo projects showcasing the use of TinyGo to create applications on the PlayStation 2 console.
Everything here is provided _AS IS_ and such should not be used for any actual game and/or software development. You are free
to use this as you wish.

This is a hobby project and I'm not a game developer. This project was created for fun only. The write-up is provided in
[my blog](https://rgsilva.com/blog/), so feel free to check it out there.

## Building

Currently this requires a custom version of the TinyGo compiler. The custom version is provided
[here](https://github.com/rgsilva/tinygo-ps2), and it requires [this](https://github.com/rgsilva/tinygo-ps2-llvm/tree/xtensa_release_19.1.2)
custom version of the LLVM to work.

The Go code is compiled to LLVM IR by TinyGo and turned into MIPS objects by clang, both on the host.
The final link against the ps2sdk happens in Docker, using the amd64 `ps2dev/ps2dev` image (I'm on an
Apple silicon Mac). So you'll need Docker; the image is pulled automatically on the first build.

Then tell the Makefile where your tools are, either in a `config.mk` file next to it (gitignored):

```make
PS2DEV = /path/to/ps2dev      # a copy of the image's /usr/local/ps2dev, headers are needed on the host
TINYGO = /path/to/tinygo-ps2/build/tinygo
CLANG  = /path/to/tinygo-ps2/llvm-build/bin/clang
```

or on the command line (`make PS2DEV=... TINYGO=... CLANG=...`). After that:

* Everything: `make`
* Flappy Gopher: `make flappygopher`
* Test application: `make test`

ELFs end up in `build/`. Use `V=1` to see the commands. Adding a demo is a directory with a Go main
package plus one line in the `DEMOS` list in the Makefile.

Have fun!
