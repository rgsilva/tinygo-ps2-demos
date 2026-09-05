// The console's real-time clock, through the CD/DVD driver on the IOP
// (libcdvd).
#define _EE
#include <kernel.h>
#include <sifrpc.h>
#include <libcdvd.h>

int ps2go_rtc_init(void) {
	SifInitRpc(0);
	return sceCdInit(SCECdINoD);
}

/* out: stat, second, minute, hour, day, month, year (BCD). */
int ps2go_rtc_read(unsigned char *out) {
	sceCdCLOCK c;
	int r = sceCdReadClock(&c);
	out[0] = c.stat;
	out[1] = c.second;
	out[2] = c.minute;
	out[3] = c.hour;
	out[4] = c.day;
	out[5] = c.month;
	out[6] = c.year;
	return r;
}
