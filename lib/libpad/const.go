package libpad

const (
	PORT_0             = 0x00 // Port 0
	SLOT_0             = 0x00 // Slot 0
	PAD_LEFT           = 0x0080
	PAD_DOWN           = 0x0040
	PAD_RIGHT          = 0x0020
	PAD_UP             = 0x0010
	PAD_START          = 0x0008
	PAD_R3             = 0x0004
	PAD_L3             = 0x0002
	PAD_SELECT         = 0x0001
	PAD_SQUARE         = 0x8000
	PAD_CROSS          = 0x4000
	PAD_CIRCLE         = 0x2000
	PAD_TRIANGLE       = 0x1000
	PAD_R1             = 0x0800
	PAD_L1             = 0x0400
	PAD_R2             = 0x0200
	PAD_L2             = 0x0100
	PAD_STATE_DISCONN  = 0x00
	PAD_STATE_FINDPAD  = 0x01
	PAD_STATE_FINDCTP1 = 0x02
	PAD_STATE_EXECCMD  = 0x05
	PAD_STATE_STABLE   = 0x06
	PAD_STATE_ERROR    = 0x07
)
