// Package rgb holds the small color type used everywhere in the game, plus
// the math for blending colors and for squeezing them into the 256-color
// palette when a terminal can't show true color.
package rgb

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Color is an sRGB color with each channel in the range 0..1.
// Lighting math happens in this float form; it is only turned into bytes
// at the very end, when a frame is written to the terminal.
type Color struct {
	R, G, B float64
}

// Black and White are handy constants.
var (
	Black = Color{0, 0, 0}
	White = Color{1, 1, 1}
)

// Hex parses "#rrggbb" (the leading # is optional).
func Hex(s string) (Color, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return Color{}, fmt.Errorf("color %q should look like #rrggbb", "#"+s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("color %q should look like #rrggbb", "#"+s)
	}
	return FromBytes(uint8(v>>16), uint8(v>>8), uint8(v)), nil
}

// MustHex is Hex for colors written in Go code; it panics on a typo.
func MustHex(s string) Color {
	c, err := Hex(s)
	if err != nil {
		panic(err)
	}
	return c
}

// FromBytes builds a Color from 0..255 channels.
func FromBytes(r, g, b uint8) Color {
	return Color{float64(r) / 255, float64(g) / 255, float64(b) / 255}
}

// Bytes returns the color as 0..255 channels, clamped and rounded.
func (c Color) Bytes() (r, g, b uint8) {
	return toByte(c.R), toByte(c.G), toByte(c.B)
}

// Pack returns the color as a single 24-bit number (0xRRGGBB).
func (c Color) Pack() uint32 {
	r, g, b := c.Bytes()
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// String returns "#rrggbb".
func (c Color) String() string {
	r, g, b := c.Bytes()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func toByte(v float64) uint8 {
	if v <= 0 || math.IsNaN(v) {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// Lerp blends from c (t=0) to d (t=1).
func (c Color) Lerp(d Color, t float64) Color {
	return Color{
		c.R + (d.R-c.R)*t,
		c.G + (d.G-c.G)*t,
		c.B + (d.B-c.B)*t,
	}
}

// Mul multiplies channel by channel (used to light a surface).
func (c Color) Mul(d Color) Color {
	return Color{c.R * d.R, c.G * d.G, c.B * d.B}
}

// Add adds channel by channel (used to add lamp light).
func (c Color) Add(d Color) Color {
	return Color{c.R + d.R, c.G + d.G, c.B + d.B}
}

// Scale multiplies every channel by f.
func (c Color) Scale(f float64) Color {
	return Color{c.R * f, c.G * f, c.B * f}
}

// Clamp keeps every channel within 0..1.
func (c Color) Clamp() Color {
	return Color{clamp01(c.R), clamp01(c.G), clamp01(c.B)}
}

// Luma is the perceived brightness, 0..1.
func (c Color) Luma() float64 {
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// Desaturate moves the color toward its own grey by t (0..1).
func (c Color) Desaturate(t float64) Color {
	l := c.Luma()
	return c.Lerp(Color{l, l, l}, t)
}

// Screen brightens c by d the way light adds up on film; it never goes past white.
func (c Color) Screen(d Color) Color {
	return Color{
		1 - (1-c.R)*(1-d.R),
		1 - (1-c.G)*(1-d.G),
		1 - (1-c.B)*(1-d.B),
	}
}

// Distance is a perceptual distance between two colors (in Oklab space).
func Distance(a, b Color) float64 {
	la, aa, ba := oklab(a)
	lb, ab, bb := oklab(b)
	dl, da, db := la-lb, aa-ab, ba-bb
	return math.Sqrt(dl*dl + da*da + db*db)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Smoothstep eases t from 0 to 1 between edges e0 and e1.
func Smoothstep(e0, e1, t float64) float64 {
	if e1 == e0 {
		if t < e0 {
			return 0
		}
		return 1
	}
	x := clamp01((t - e0) / (e1 - e0))
	return x * x * (3 - 2*x)
}

// Clamp01 keeps v within 0..1.
func Clamp01(v float64) float64 { return clamp01(v) }
