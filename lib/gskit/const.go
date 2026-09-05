package gskit

const (
	GS_RENDER_QUEUE_PER_POOLSIZE = 1024 * 256
	GS_RENDER_QUEUE_OS_POOLSIZE  = 1024 * 1024
	GS_PSM_CT32                  = 0x00
	GS_PSM_CT24                  = 0x01
	GS_PSMZ_16S                  = 0x0A
	GS_FILTER_NEAREST            = 0x00
	GS_FILTER_LINEAR             = 0x01
	GSKIT_ALLOC_SYSBUFFER        = 0x00
	GSKIT_ALLOC_USERBUFFER       = 0x01
	GSKIT_ALLOC_ERROR            = 0x00
	GSKIT_FTYPE_FNT              = 0x00
	GS_BLEND_FRONT2BACK          = 0x12
	GS_BLEND_BACK2FRONT          = 0x01 // gsKit's default: (Cd - Cs) * As + Cs, alpha 0 is opaque

	// SetTest presets.
	GS_ZTEST_OFF   = 0x01
	GS_ZTEST_ON    = 0x02
	GS_ATEST_OFF   = 0x03
	GS_ATEST_ON    = 0x04
	GS_D_ATEST_OFF = 0x05
	GS_D_ATEST_ON  = 0x06

	// Alpha test methods (TEST.ATST): the pixel passes if its alpha
	// compares so against the reference.
	GS_ATEST_NEVER    = 0
	GS_ATEST_ALWAYS   = 1
	GS_ATEST_LESS     = 2
	GS_ATEST_LEQUAL   = 3
	GS_ATEST_EQUAL    = 4
	GS_ATEST_GEQUAL   = 5
	GS_ATEST_GREATER  = 6
	GS_ATEST_NOTEQUAL = 7
)
