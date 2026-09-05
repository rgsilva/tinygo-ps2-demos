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
	GSKIT_FTYPE_FNT              = 0x00
	GSKIT_FTYPE_BMP_DAT          = 0x01
	GSKIT_FTYPE_PNG_DAT          = 0x02
	GS_BLEND_FRONT2BACK          = 0x12
	GS_BLEND_BACK2FRONT          = 0x01
)
