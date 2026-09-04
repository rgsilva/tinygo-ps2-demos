// A C file in the package: TinyGo compiles it with the target's cflags and
// links it (as a real object with the ps2sdk gcc).
#include <stdint.h>

uint32_t ps2go_cfile_mix(uint32_t a, uint32_t b) { return (a * 2654435761u) ^ b; }
