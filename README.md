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

Since I'm working on an Apple silicon Mac, I'm using Docker as a way to use the amd64 binaries of the ps2dev/ps2sdk. As such,
you'll need to Docker to build the project. First step is to build our special custom image:

```sh
make ps2dev
```

Then you'll also some very basic code that I call "loader". To build that, run:

```sh
make loader
```

After this, you can build the demos as follows:

* Flappy Gopher: `make flappygopher`
* Test application: `make test`

Have fun!
