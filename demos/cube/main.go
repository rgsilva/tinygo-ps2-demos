// A rotating, textured, semi-transparent cube over the Flappy Gopher sky:
// perspective-correct texturing through the GS's STQ coordinates, the
// faces sorted back to front so the far ones show through.
package main

import (
	"fmt"
	"sort"

	"ps2go/lib/debug"
	"ps2go/lib/dmakit"
	"ps2go/lib/fonts"
	"ps2go/lib/gskit"
	"ps2go/lib/vec"
)

const (
	alpha      = 0x66 // 80% of the GS's 0x80
	zFar       = 0    // depth of the background (larger is nearer)
	zScale     = 60000
	cubeSize   = 1.0
	holdFrames = 180 // the pose is held this long first (the visual check shoots it)
	poseY      = 0.6
	poseX      = 0.45
)

var (
	gs        gskit.GSGlobal
	gopherTex *gskit.GSTexture
	skyTex    *gskit.GSTexture
	font      *gskit.GSFont
	frame     int
	fps       int
)

// The cube: 8 corners, 6 faces of 4 corner indices (counter-clockwise
// seen from outside) with the texture corners.
var corners = [8]vec.Vec3{
	{-1, -1, -1}, {1, -1, -1}, {1, 1, -1}, {-1, 1, -1},
	{-1, -1, 1}, {1, -1, 1}, {1, 1, 1}, {-1, 1, 1},
}

var faces = [6][4]int{
	{4, 5, 6, 7}, // front  (z = 1)
	{1, 0, 3, 2}, // back   (z = -1)
	{0, 4, 7, 3}, // left   (x = -1)
	{5, 1, 2, 6}, // right  (x = 1)
	{7, 6, 2, 3}, // top    (y = 1)
	{0, 1, 5, 4}, // bottom (y = -1)
}

var uvs = [4][2]float32{{0, 1}, {1, 1}, {1, 0}, {0, 0}}

type projected struct {
	x, y float32 // screen pixels
	z    uint32  // GS depth
	q    float32 // 1/w
}

var (
	proj     [8]projected
	viewZ    [8]float32
	vertices = make([]gskit.STQVertex, 0, 36)
)

func must(err error) {
	if err != nil {
		debug.Printf("fatal: %v\n", err)
		for {
		}
	}
}

func initGraphics() {
	dmakit.Init(dmakit.D_CTRL_RELE_OFF, dmakit.D_CTRL_MFD_OFF, dmakit.D_CTRL_STS_UNSPEC,
		dmakit.D_CTRL_STD_OFF, dmakit.D_CTRL_RCYC_8, 1<<dmakit.DMA_CHANNEL_GIF)
	dmakit.ChannelInit(dmakit.DMA_CHANNEL_GIF)

	gs = gskit.InitGlobal()
	gs.SetPSM(gskit.GS_PSM_CT24)
	gs.SetPSMZ(gskit.GS_PSMZ_16S)
	gs.SetDoubleBuffering(true)
	gs.SetZBuffering(true)
	gskit.InitScreen(gs)
	gs.SetPrimAlphaEnable(true)
	gskit.SetPrimAlpha(gs, gskit.GS_BLEND_SOURCE_ALPHA, false)
	gskit.SetTest(gs, gskit.GS_ZTEST_ON)

	gopherTex = gskit.NewTexture(256, 256, gskit.GS_PSM_CT32, gskit.GS_FILTER_LINEAR)
	must(gopherTex.Upload(gs, texGopher))
	skyTex = gskit.NewTexture(320, 214, gskit.GS_PSM_CT32, gskit.GS_FILTER_LINEAR)
	must(skyTex.Upload(gs, texSky))
	font = gskit.InitFontFromMemory(fonts.Arial)
	must(gskit.FontUpload(gs, font))
	font.AddSpacing(-2)
}

// project transforms the cube's corners for this frame.
func project() {
	w, h := float32(gs.Width()), float32(gs.Height())
	spin := float32(frame - holdFrames)
	if spin < 0 {
		spin = 0
	}
	model := vec.RotationY(poseY + spin*0.021).Mul(vec.RotationX(poseX + spin*0.013))
	view := vec.Translation(0, 0, -6)
	mv := view.Mul(model)
	p := vec.Perspective(0.9, w/h*0.7, 1, 10) // 0.7: the NTSC frame is 640x448 on a 4:3 screen
	for i, c := range corners {
		v := mv.Transform(c.Scale(cubeSize))
		viewZ[i] = v.Z
		clip := p.Transform(vec.Vec3{X: v.X, Y: v.Y, Z: v.Z})
		q := 1 / clip.W
		nx, ny, nz := clip.X*q, clip.Y*q, clip.Z*q
		proj[i] = projected{
			x: (nx + 1) / 2 * w,
			y: (1 - ny) / 2 * h,
			z: uint32((1 - nz) / 2 * zScale), // nearer = larger
			q: q,
		}
	}
}

// drawCube emits the faces back to front, two triangles each.
func drawCube() {
	order := [6]int{0, 1, 2, 3, 4, 5}
	depth := func(f int) float32 {
		return viewZ[faces[f][0]] + viewZ[faces[f][1]] + viewZ[faces[f][2]] + viewZ[faces[f][3]]
	}
	sort.Slice(order[:], func(a, b int) bool { return depth(order[a]) < depth(order[b]) }) // most negative z = farthest
	vertices = vertices[:0]
	for _, f := range order {
		fc := faces[f]
		for _, tri := range [2][3]int{{0, 1, 2}, {0, 2, 3}} {
			for _, k := range tri {
				p := proj[fc[k]]
				vertices = append(vertices, gskit.NewSTQVertex(
					gskit.VertexColor(0x80, 0x80, 0x80, alpha, p.q),
					gskit.VertexSTQ(uvs[k][0]*p.q, uvs[k][1]*p.q),
					gs.VertexXYZ(p.x, p.y, p.z),
				))
			}
		}
	}
	gskit.PrimListTriangleTextureSTQ(gs, gopherTex, vertices)
}

func drawFrame() {
	gskit.SyncFlip(gs)
	gskit.SetActive(gs)
	gskit.Clear(gs, 0, 0, 0, 0x80, 0)
	w, h := float32(gs.Width()), float32(gs.Height())
	gskit.PrimSpriteTexture3D(gs, skyTex, 0, 0, zFar, 0, 0, w, h, zFar, float32(skyTex.Width()), float32(skyTex.Height()),
		gskit.GS_SETREG_RGBAQ(0x80, 0x80, 0x80, 0x80, 0))
	project()
	drawCube()
	gskit.FontPrint(gs, font, 8, 8, 65535, 0.8, gskit.GS_SETREG_RGBAQ(0xFF, 0xFF, 0xFF, 0x80, 0),
		fmt.Sprintf("FPS %d  frame %d", fps, frame))
	gskit.QueueExec(gs)
}

func main() {
	debug.Init()
	debug.Printf("PS2 TinyGo cube\n")
	initGraphics()
	debug.Printf("Cube start\n")
	frames, last := 0, ticks()
	for {
		drawFrame()
		frame++
		frames++
		if now := ticks(); now-last >= 1e9 {
			fps, frames, last = frames, 0, now
		}
	}
}
