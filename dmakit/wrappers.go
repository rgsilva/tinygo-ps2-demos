package dmakit

/*
#cgo LDFLAGS: -ldmakit
#define _EE
#include <dmaKit.h>
*/
import "C"

func Init(
	rele, mfd, sts, std, rcyc uint32,
	fastWaitChannels uint16,
) {
	C.dmaKit_init(
		C.uint(rele), C.uint(mfd), C.uint(sts), C.uint(std), C.uint(rcyc),
		C.ushort(fastWaitChannels))
}

func ChannelInit(ch uint32) {
	C.dmaKit_chan_init(C.uint(ch))
}
