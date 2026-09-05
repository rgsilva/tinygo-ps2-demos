// Blend check: blue squares of increasing alpha across a white bar on
// black, drawn with the usual gsKit setup. tests/visual/blend.steps
// compares the frame.
package main

import (
	"ps2go/lib/dmakit"
	"ps2go/lib/gskit"
)

func main() {
	dmakit.Init(dmakit.D_CTRL_RELE_OFF, dmakit.D_CTRL_MFD_OFF, dmakit.D_CTRL_STS_UNSPEC,
		dmakit.D_CTRL_STD_OFF, dmakit.D_CTRL_RCYC_8, 1<<dmakit.DMA_CHANNEL_GIF)
	dmakit.ChannelInit(dmakit.DMA_CHANNEL_GIF)

	gs := gskit.InitGlobal()
	gs.SetPSM(gskit.GS_PSM_CT24)
	gs.SetPSMZ(gskit.GS_PSMZ_16S)
	gs.SetDoubleBuffering(true)
	gs.SetZBuffering(false)
	gskit.InitScreen(gs)
	gs.SetPrimAlphaEnable(true)
	gskit.SetPrimAlpha(gs, gskit.GS_BLEND_SOURCE_ALPHA, false)
	println("Ready")
	for {
		gskit.SyncFlip(gs)
		gskit.SetActive(gs)
		gskit.Clear(gs, 0, 0, 0, 0x80, 0)
		gskit.PrimSprite(gs, 0, 160, 640, 288, 1, gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0))
		for i := 0; i < 8; i++ {
			x := float32(16 + i*78)
			gskit.PrimSprite(gs, x, 96, x+64, 352, 1, gskit.GS_SETREG_RGBAQ(0x00, 0x00, 0xFF, uint8((i+1)*0x10), 0))
		}
		gskit.QueueExec(gs)
	}
}
