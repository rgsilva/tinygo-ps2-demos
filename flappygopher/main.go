package main

/*
// Modules
extern unsigned char *freesio2;
extern unsigned int size_freesio2;
extern unsigned char *freepad;
extern unsigned int size_freepad;

// Textures
extern unsigned char *gopher;
extern unsigned char *bird;
extern unsigned char *pipe;
extern unsigned char *gameover;
extern unsigned char *sky;

// Fonts
extern unsigned char *arial;
extern unsigned int size_arial;
*/
import "C"
import (
	"fmt"
	"ps2go/clock"
	"ps2go/debug"
	"ps2go/dmakit"
	"ps2go/gskit"
	"ps2go/libpad"
	"ps2go/random"
	"ps2go/sifrpc"
	"unsafe"
)

type (
	GameState int

	Pipe struct {
		X                  int32
		HoleStart, HoleEnd int32
	}
)

const (
	// Game settings
	JumpTargetOffset        = 50
	JumpingSpeed            = 5
	FallingSpeed            = 3
	HorizontalSpeed         = 5
	WaitFramesAfterGameOver = 20
	WaitFramesOnMenu        = 20

	// Pipe settings.
	PipeBodyX            = 99
	PipeBodyWidth        = 75
	PipeHeadX            = 0
	PipeHeadWidth        = 95
	PipeHeadOffset       = 10
	PipeHeight           = 64
	PipeSections         = 448 / PipeHeight
	PipePastScreenOffset = 400
	HowManyPipes         = 6
)

const (
	Menu GameState = iota
	InGame
	GameOver
)

var (
	// Engine stuff
	gsGlobal              gskit.GSGlobal
	gopherTex             gskit.GSTexture
	birdTex               gskit.GSTexture
	pipeTex               gskit.GSTexture
	gameOverTex           gskit.GSTexture
	skyTex                gskit.GSTexture
	textFont              gskit.GSFont
	pad0                  libpad.Pad
	debounceCounter       int32
	debounceCounterTarget int32
	lastFrameTime         int32

	// Game stuff
	gameState    GameState
	isGoingUp    bool
	targetY      uint32
	maxX, maxY   uint32
	birdY        uint32
	pipes        []Pipe
	currentScore int32
	highScore    int32
)

func main() {
	debug.Init()

	sifrpc.ResetAndPatchIOP()

	// Load necessary modules for controller stuff and the init it.
	debug.Printf("Loading freesio2\n")
	sifrpc.LoadModuleBuffer(unsafe.Pointer(&C.freesio2), int(C.size_freesio2))

	debug.Printf("Loading freepad\n")
	sifrpc.LoadModuleBuffer(unsafe.Pointer(&C.freepad), int(C.size_freepad))

	// Initialize the DMA controller
	debug.Printf("Initializing DMA\n")
	initDMA()

	// Initialize random seed.
	debug.Printf("Initializing random seed\n")
	random.Init()

	debug.Printf("Initializing controllers\n")
	initController()

	// Initialize the GS
	debug.Printf("Initializing graphics\n")
	initGraphics()

	// Initialize textures
	debug.Printf("Loading textures\n")
	loadGopherTexture()
	loadBirdTexture()
	loadPipeTexture()
	loadGameOverTexture()
	loadSkyTexture()

	// Initialize fonts
	debug.Printf("Loading fonts\n")
	loadFont()

	gameState = Menu
	isGoingUp = false
	targetY = 0
	birdY = 0
	pipes = []Pipe{}
	currentScore = 0
	highScore = 0
	debounceCounterTarget = 0
	debounceCounter = 0
	maxX = gsGlobal.Width
	maxY = gsGlobal.Height

	debug.Printf("Game start\n")
	for {
		start := clock.Now()

		// Handle all the inputs.
		handleInputs()

		// Draw the frame
		drawFrame()

		// This is a hacky way of holding the user on a screen for a few bits.
		// This also works as a dirty debouncer for the inputs as the CPU
		// runs too fast and the inputs read may remain active between frames.
		if debounceCounterTarget > 0 {
			if debounceCounter < debounceCounterTarget {
				debounceCounter++
				continue
			} else if debounceCounter >= debounceCounterTarget {
				debounceCounter = 0
				debounceCounterTarget = 0
			}
		}

		end := clock.Now()
		lastFrameTime = int32(end - start)
	}
}

func drawFPS() {
	fps := int32(0)
	if lastFrameTime > 0 {
		fps = clock.CLOCKS_PER_SEC / lastFrameTime
	}
	line := fmt.Sprintf("FPS: %d", fps)
	gskit.FontPrint(
		gsGlobal, textFont,
		0, 0, 1, 0.95,
		gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0x00),
		line)
}

func initController() {
	// Initialize the controller
	libpad.Init()

	// Open port
	pad0 = libpad.PortOpen(libpad.PORT_0, libpad.SLOT_0)
}

func initDMA() {
	// Initialize the DMA controller
	dmakit.Init(
		dmakit.D_CTRL_RELE_OFF,
		dmakit.D_CTRL_MFD_OFF,
		dmakit.D_CTRL_STS_UNSPEC,
		dmakit.D_CTRL_STD_OFF,
		dmakit.D_CTRL_RCYC_8,
		1<<dmakit.DMA_CHANNEL_GIF)

	// Initialize DMA channel for GIF (Graphics Interface)
	dmakit.ChannelInit(dmakit.DMA_CHANNEL_GIF)
}

func initGraphics() {
	// Initialize the global graphics state
	gsGlobal = gskit.InitGlobal()
	gsGlobal.PSM = gskit.GS_PSM_CT24
	gsGlobal.PSMZ = gskit.GS_PSMZ_16S
	gsGlobal.DoubleBuffering = true
	gsGlobal.ZBuffering = false
	gsGlobal.PrimAlphaEnable = false

	// Initialize screen
	gskit.InitScreen(gsGlobal)
}

func loadGopherTexture() {
	gopherTex = gskit.NewGSTexture()

	// Set width, height, 32-bit format.
	gopherTex.Width = 256
	gopherTex.Height = 256
	gopherTex.PSM = gskit.GS_PSM_CT32

	// No smoothing
	gopherTex.Filter = gskit.GS_FILTER_NEAREST

	// No Color Lookup Table (CLUT) (EE RAM)
	gopherTex.Clut = nil

	// Point to our texture data (EE RAM)
	gopherTex.Mem = unsafe.Pointer(&C.gopher)

	// Allocate the VRAM on GS (GS RAM)
	textureSize := gskit.TextureSize(gopherTex.Width, gopherTex.Height, gopherTex.PSM)
	gopherTex.VRAM = gskit.VRAMAlloc(gsGlobal, textureSize, gskit.GSKIT_ALLOC_USERBUFFER)

	// Load the texture data into VRAM
	gskit.TextureUpload(gsGlobal, gopherTex)
}

func loadBirdTexture() {
	birdTex = gskit.NewGSTexture()

	// Set width, height, 32-bit format.
	birdTex.Width = 128
	birdTex.Height = 53
	birdTex.PSM = gskit.GS_PSM_CT32

	// No smoothing
	birdTex.Filter = gskit.GS_FILTER_NEAREST

	// No Color Lookup Table (CLUT) (EE RAM)
	birdTex.Clut = nil

	// Point to our texture data (EE RAM)
	birdTex.Mem = unsafe.Pointer(&C.bird)

	// Allocate the VRAM on GS (GS RAM)
	textureSize := gskit.TextureSize(birdTex.Width, birdTex.Height, birdTex.PSM)
	birdTex.VRAM = gskit.VRAMAlloc(gsGlobal, textureSize, gskit.GSKIT_ALLOC_USERBUFFER)

	// Load the texture data into VRAM
	gskit.TextureUpload(gsGlobal, birdTex)
}

func loadPipeTexture() {
	pipeTex = gskit.NewGSTexture()

	// Set width, height, 32-bit format.
	pipeTex.Width = 174
	pipeTex.Height = 64
	pipeTex.PSM = gskit.GS_PSM_CT32

	// No smoothing
	pipeTex.Filter = gskit.GS_FILTER_NEAREST

	// No Color Lookup Table (CLUT) (EE RAM)
	pipeTex.Clut = nil

	// Point to our texture data (EE RAM)
	pipeTex.Mem = unsafe.Pointer(&C.pipe)

	// Allocate the VRAM on GS (GS RAM)
	textureSize := gskit.TextureSize(pipeTex.Width, pipeTex.Height, pipeTex.PSM)
	pipeTex.VRAM = gskit.VRAMAlloc(gsGlobal, textureSize, gskit.GSKIT_ALLOC_USERBUFFER)

	// Load the texture data into VRAM
	gskit.TextureUpload(gsGlobal, pipeTex)
}

func loadGameOverTexture() {
	gameOverTex = gskit.NewGSTexture()

	// Set width, height, 32-bit format.
	gameOverTex.Width = 400
	gameOverTex.Height = 267
	gameOverTex.PSM = gskit.GS_PSM_CT32

	// No smoothing
	gameOverTex.Filter = gskit.GS_FILTER_NEAREST

	// No Color Lookup Table (CLUT) (EE RAM)
	gameOverTex.Clut = nil

	// Point to our texture data (EE RAM)
	gameOverTex.Mem = unsafe.Pointer(&C.gameover)

	// Allocate the VRAM on GS (GS RAM)
	textureSize := gskit.TextureSize(gameOverTex.Width, gameOverTex.Height, gameOverTex.PSM)
	gameOverTex.VRAM = gskit.VRAMAlloc(gsGlobal, textureSize, gskit.GSKIT_ALLOC_USERBUFFER)

	// Load the texture data into VRAM
	gskit.TextureUpload(gsGlobal, gameOverTex)
}

func loadSkyTexture() {
	skyTex = gskit.NewGSTexture()

	// Set width, height, 24-bit format.
	skyTex.Width = 320
	skyTex.Height = 214
	skyTex.PSM = gskit.GS_PSM_CT32

	// No smoothing
	skyTex.Filter = gskit.GS_FILTER_NEAREST

	// No Color Lookup Table (CLUT) (EE RAM)
	skyTex.Clut = nil

	// Point to our texture data (EE RAM)
	skyTex.Mem = unsafe.Pointer(&C.sky)

	// Allocate the VRAM on GS (GS RAM)
	textureSize := gskit.TextureSize(skyTex.Width, skyTex.Height, skyTex.PSM)
	skyTex.VRAM = gskit.VRAMAlloc(gsGlobal, textureSize, gskit.GSKIT_ALLOC_USERBUFFER)

	// Load the texture data into VRAM
	gskit.TextureUpload(gsGlobal, skyTex)
}

func loadFont() {
	textFont = gskit.InitFontFromMemory(unsafe.Pointer(&C.arial), int(C.size_arial))
	gskit.FontUpload(gsGlobal, textFont)
}

func drawFrame() {
	// Sync & flip (not on first frame though)
	gskit.SyncFlip(gsGlobal)

	// Activate the new buffer
	gskit.SetActive(gsGlobal)

	// Clear the buffer
	gskit.Clear(gsGlobal, 0x00, 0x00, 0x00, 0x80, 0x00)

	if gameState == Menu {
		drawMenu()
	} else if gameState == InGame {
		drawInGame()
	} else {
		drawGameOver()
	}

	// Draw the FPS counter
	drawFPS()

	// Push the draw commands to the GS
	gskit.QueueExec(gsGlobal)
}

func drawMenu() {
	// Draw the Gopher logo
	gopherCenterX := (maxX - gopherTex.Width) / 2
	gskit.PrimSpriteTexture3D(
		gsGlobal,
		gopherTex,
		int32(gopherCenterX), 0, 1,
		0, 0,
		int32(gopherCenterX+gopherTex.Width), int32(gopherTex.Height), 1,
		int32(gopherTex.Width), int32(gopherTex.Height),
		gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
	)

	// Draw the text
	lines := []string{
		"Flappy Gopher",
		"", "",
		"Developed by Ricardo",
		"using TinyGo @ PlayStation 2",
		"", "",
		"Press [Start] to start",
		"", "",
		fmt.Sprintf("High score: %d", highScore),
	}
	for i, line := range lines {
		y := int32(gopherTex.Height + uint32((i+1)*14))
		gskit.FontPrint(
			gsGlobal, textFont,
			0, y, 1, 0.95,
			gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0x00),
			line)
	}
}

func drawInGame() {
	if isGoingUp {
		// If we're going up, decrease Y.
		if birdY > JumpingSpeed {
			birdY = birdY - JumpingSpeed
		} else {
			birdY = 0
		}
	} else {
		// If we're going down, increase Y.
		birdY = birdY + FallingSpeed
	}
	// If we reached/crossed our jumping target, let's go down.
	if birdY <= targetY {
		isGoingUp = false
	}

	// Move the pipes a bit.
	for i := range pipes {
		if pipes[i].X < -PipeBodyWidth {
			// Pipe is already offscreen to the left, move it offscreen to the right.
			x := int32(maxY) + (PipePastScreenOffset * (HowManyPipes - 1))
			pipes[i] = randomPipe(x)

			// This is also where we increase our score!
			currentScore++
		} else {
			pipes[i].X = pipes[i].X - HorizontalSpeed
		}
	}

	// If we're over the screen limit, it's game over.
	if birdY > maxY {
		endGame()
		return
	}

	// Detect collision against each pipe.
	for _, pipe := range pipes {
		// Check if the pipe above/under bird. If not, carry on.
		if pipe.X > int32(birdTex.Width/2) {
			continue
		}

		// Check if the bird is inside the hole.
		pipeMinY := pipe.HoleStart*PipeHeight - 10
		pipeMaxY := pipe.HoleEnd*PipeHeight + 10

		if int32(birdY) >= pipeMinY && int32(birdY) <= pipeMaxY {
			continue
		}

		endGame()
		return
	}

	// Draw the background
	// TODO: disable as alpha is broken.
	//gskit.PrimSpriteTexture3D(
	//	gsGlobal,
	//	skyTex,
	//	0, 0, 10,
	//	0, 0,
	//	int(maxX), int(maxY), 1,
	//	int(skyTex.Width), int(skyTex.Height),
	//	gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
	//)

	// Draw the Gopher. Width 0 to 64 is it going down, width 65 to 128 is it going up.
	textureX := uint32(0)
	if isGoingUp {
		textureX = birdTex.Width/2 + 1
	}
	gskit.PrimSpriteTexture3D(
		gsGlobal,
		birdTex,
		0, int32(birdY), 1,
		int32(textureX), 0,
		int32(birdTex.Width/2), int32(birdY+birdTex.Height), 1,
		int32(textureX+birdTex.Width/2), int32(birdTex.Height),
		gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
	)

	// Draw the pipes
	for _, pipe := range pipes {
		// Do not draw pipes outside the screen.
		if pipe.X < -PipeBodyWidth || pipe.X > int32(maxX) {
			continue
		}

		for s := int32(0); s < PipeSections; s++ {
			// Do not draw the pipe where we need a hole.
			if s >= pipe.HoleStart && s <= pipe.HoleEnd {
				continue
			}

			// Everywhere else, draw it.
			posY := s * int32(pipeTex.Height)
			posYEnd := s*int32(pipeTex.Height) + int32(pipeTex.Height)

			if s == pipe.HoleEnd+1 && pipe.HoleEnd < PipeSections-1 {
				// Bottom pipe head
				gskit.PrimSpriteTexture3D(
					gsGlobal,
					pipeTex,
					pipe.X-PipeHeadOffset, posY, 1,
					int32(PipeHeadX), 0,
					pipe.X+PipeHeadWidth-PipeHeadOffset, posYEnd, 1,
					int32(PipeHeadX+PipeHeadWidth), int32(pipeTex.Height),
					gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
				)
			} else if pipe.HoleStart > 0 && s == pipe.HoleStart-1 {
				// Top pipe head. We just flip the Y of the texture for this.
				gskit.PrimSpriteTexture3D(
					gsGlobal,
					pipeTex,
					pipe.X-PipeHeadOffset, posY, 1,
					int32(PipeHeadX), int32(pipeTex.Height),
					pipe.X+PipeHeadWidth-PipeHeadOffset, posYEnd, 1,
					int32(PipeHeadX+PipeHeadWidth), 0,
					gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
				)
			} else {
				// Pipe body
				gskit.PrimSpriteTexture3D(
					gsGlobal,
					pipeTex,
					pipe.X, posY, 1,
					int32(PipeBodyX), 0,
					pipe.X+PipeBodyWidth, posYEnd, 1,
					int32(PipeBodyX+PipeBodyWidth), int32(pipeTex.Height),
					gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
				)
			}
		}
	}

	// Draw the score
	line := fmt.Sprintf("Score: %d", currentScore)
	gskit.FontPrint(
		gsGlobal, textFont,
		0, 16, 1, 1,
		gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0x00),
		line)
}

func drawGameOver() {
	centerX := (maxX - gameOverTex.Width) / 2

	// Draw the Gopher logo
	gskit.PrimSpriteTexture3D(
		gsGlobal,
		gameOverTex,
		int32(centerX), 0, 1,
		0, 0,
		int32(centerX+gameOverTex.Width), int32(gameOverTex.Height), 1,
		int32(gameOverTex.Width), int32(gameOverTex.Height),
		gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0x00),
	)

	// Draw the text
	lines := []string{
		"Game over",
		"Thanks for playing!",
		"", "",
		"Press [Start] to restart",
		"", "",
		fmt.Sprintf("Your score: %d, high score: %d", currentScore, highScore),
	}
	for i, line := range lines {
		y := int32(gameOverTex.Height + uint32((i+1)*14))
		gskit.FontPrint(
			gsGlobal, textFont,
			0, y, 1, 0.95,
			gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0x00),
			line)
	}
}

func handleInputs() {
	input := pad0.Read()

	if gameState == Menu {
		if input.Start {
			initGame()
		}
	} else if gameState == InGame {
		if input.Cross {
			isGoingUp = true
			if birdY-JumpTargetOffset < 0 {
				targetY = 0
			} else {
				targetY = birdY - JumpTargetOffset
			}
		}
	} else if gameState == GameOver {
		if input.Start {
			goToMenu()
		}
	}
}

func initGame() {
	gameState = InGame
	birdY = 100
	currentScore = 0

	pipes = make([]Pipe, HowManyPipes)

	// First pipe is pre-defined.
	pipes[0] = Pipe{
		X:         int32(maxY) - PipeHeadX,
		HoleStart: 3,
		HoleEnd:   5,
	}

	// All other pipes are random.
	for i := 1; i < HowManyPipes; i++ {
		x := int32(maxY) + int32(PipePastScreenOffset*i)
		pipes[i] = randomPipe(x)
	}
}

func randomPipe(x int32) Pipe {
	holePos := random.Between(1, PipeSections-1)
	return Pipe{
		X:         x,
		HoleStart: int32(holePos - 1),
		HoleEnd:   int32(holePos + 1),
	}
}

func endGame() {
	gameState = GameOver
	debounceCounter = 0
	debounceCounterTarget = WaitFramesAfterGameOver

	if currentScore > highScore {
		highScore = currentScore
	}
}

func goToMenu() {
	gameState = Menu
	debounceCounter = 0
	debounceCounterTarget = WaitFramesOnMenu
}
