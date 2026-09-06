// Package vec is the little 3D math the demos need: vectors and 4x4
// matrices in float32, column vectors, right-handed, OpenGL-style
// perspective.
package vec

import "math"

type Vec3 struct{ X, Y, Z float32 }

type Vec4 struct{ X, Y, Z, W float32 }

// Mat4 is row-major: M[row*4+col].
type Mat4 [16]float32

func Identity() Mat4 {
	return Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

func Translation(x, y, z float32) Mat4 {
	m := Identity()
	m[3], m[7], m[11] = x, y, z
	return m
}

func RotationX(a float32) Mat4 {
	s, c := sincos(a)
	return Mat4{1, 0, 0, 0, 0, c, -s, 0, 0, s, c, 0, 0, 0, 0, 1}
}

func RotationY(a float32) Mat4 {
	s, c := sincos(a)
	return Mat4{c, 0, s, 0, 0, 1, 0, 0, -s, 0, c, 0, 0, 0, 0, 1}
}

func RotationZ(a float32) Mat4 {
	s, c := sincos(a)
	return Mat4{c, -s, 0, 0, s, c, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// Perspective builds a projection with the vertical field of view fovy
// (radians), the aspect ratio and the near/far planes: clip w is the view
// depth, and z maps to -1 (near) .. 1 (far) after division.
func Perspective(fovy, aspect, near, far float32) Mat4 {
	f := 1 / float32(math.Tan(float64(fovy)/2))
	var m Mat4
	m[0] = f / aspect
	m[5] = f
	m[10] = (far + near) / (near - far)
	m[11] = 2 * far * near / (near - far)
	m[14] = -1
	return m
}

// Mul returns a*b (apply b first, then a).
func (a Mat4) Mul(b Mat4) Mat4 {
	var r Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			r[i*4+j] = a[i*4]*b[j] + a[i*4+1]*b[4+j] + a[i*4+2]*b[8+j] + a[i*4+3]*b[12+j]
		}
	}
	return r
}

// Transform applies the matrix to a point (w = 1).
func (m Mat4) Transform(v Vec3) Vec4 {
	return Vec4{
		m[0]*v.X + m[1]*v.Y + m[2]*v.Z + m[3],
		m[4]*v.X + m[5]*v.Y + m[6]*v.Z + m[7],
		m[8]*v.X + m[9]*v.Y + m[10]*v.Z + m[11],
		m[12]*v.X + m[13]*v.Y + m[14]*v.Z + m[15],
	}
}

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float32) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float32   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func sincos(a float32) (s, c float32) {
	sd, cd := math.Sincos(float64(a))
	return float32(sd), float32(cd)
}
