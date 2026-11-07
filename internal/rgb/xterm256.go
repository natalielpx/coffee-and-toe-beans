package rgb

import (
	"math"
	"sync"
)

// The 256-color fallback.
//
// Terminals in 256-color mode (macOS Terminal, for example) can only show a
// fixed palette. Indexes 0-15 are avoided on purpose: every terminal theme
// redefines them, so they would make the café look different everywhere.
// We use the 6x6x6 color cube (16-231) and the grey ramp (232-255) and pick
// the closest entry by how different the colors look to a human eye
// (the Oklab color space), not by raw numbers.
//
// Hand-tuning, found by comparing frames side by side:
//   - The pure, single-channel colors of the cube (deep reds, greens, blues
//     such as #870000 or #00005f) are left out. In a soft, lamp-lit room
//     they show up as harsh blotches; without them, warm wood falls onto the
//     palette's browns and night falls onto its greys.
//   - Hue counts a little more than brightness (chromaWeight), and colors are
//     made a touch more colorful before matching (chromaBoost), so that the
//     warm and cool moods survive.
//   - No dithering: patterns of two colors looked like noise on the walls.

var cubeLevels = [6]float64{0, 95, 135, 175, 215, 255}

type paletteEntry struct {
	index   uint8
	l, a, b float64
	color   Color
}

var (
	paletteOnce sync.Once
	palette     []paletteEntry

	cacheMu sync.Mutex
	cache   = map[uint32]uint8{}
)

// chromaWeight makes hue count a little more than brightness when choosing a
// 256-color match. Without it, dusky blues and warm lamp light both collapse
// into flat greys and the room loses its mood.
const chromaWeight = 2.0

// chromaBoost nudges colors to be a little more colorful before matching, so
// that in the sparse dark corner of the palette a dusky color still lands on
// a tinted entry rather than plain grey. It is the main hand-tuned knob.
const chromaBoost = 1.1

func buildPalette() {
	for i := 16; i < 256; i++ {
		if harsh(uint8(i)) {
			continue
		}
		c := Xterm(uint8(i))
		l, a, b := oklab(c)
		palette = append(palette, paletteEntry{index: uint8(i), l: l, a: a, b: b, color: c})
	}
}

// harsh reports whether a cube color has only one channel lit, like #870000.
func harsh(i uint8) bool {
	if i < 16 || i >= 232 || i == 16 {
		return false
	}
	n := int(i) - 16
	r, g, b := n/36, (n/6)%6, n%6
	lit := 0
	for _, v := range []int{r, g, b} {
		if v > 0 {
			lit++
		}
	}
	return lit == 1
}

// Xterm returns the color an xterm-compatible terminal shows for a palette
// index from 16 to 255.
func Xterm(i uint8) Color {
	if i >= 232 {
		v := (8 + 10*float64(i-232)) / 255
		return Color{v, v, v}
	}
	if i < 16 {
		return Black
	}
	n := int(i) - 16
	r, g, b := n/36, (n/6)%6, n%6
	return Color{cubeLevels[r] / 255, cubeLevels[g] / 255, cubeLevels[b] / 255}
}

// To256 returns the closest 256-color palette index (always 16..255).
func To256(c Color) uint8 {
	key := c.Pack()
	cacheMu.Lock()
	if idx, ok := cache[key]; ok {
		cacheMu.Unlock()
		return idx
	}
	cacheMu.Unlock()

	paletteOnce.Do(buildPalette)
	l, a, b := oklab(FromBytes(uint8(key>>16), uint8(key>>8), uint8(key)))
	a, b = a*chromaBoost, b*chromaBoost
	best := palette[0].index
	bestD := math.MaxFloat64
	for _, p := range palette {
		dl, da, db := l-p.l, (a-p.a)*chromaWeight, (b-p.b)*chromaWeight
		d := dl*dl + da*da + db*db
		if d < bestD {
			bestD = d
			best = p.index
		}
	}

	cacheMu.Lock()
	if len(cache) > 1<<16 {
		cache = map[uint32]uint8{}
	}
	cache[key] = best
	cacheMu.Unlock()
	return best
}

func linear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// oklab converts an sRGB color to the Oklab perceptual color space.
func oklab(c Color) (L, A, B float64) {
	r, g, b := linear(clamp01(c.R)), linear(clamp01(c.G)), linear(clamp01(c.B))
	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b
	l, m, s = math.Cbrt(l), math.Cbrt(m), math.Cbrt(s)
	L = 0.2104542553*l + 0.7936177850*m - 0.0040720468*s
	A = 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	B = 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	return
}
