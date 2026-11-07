package scene

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"math/rand/v2"

	"coffee-and-toe-beans/internal/noise"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
	"coffee-and-toe-beans/internal/timeofday"
)

// The view through the window: sky, the Paris rooftops, the tip of the
// Eiffel Tower, and the weather. Its settings live in content/view.json.

type viewFile struct {
	Seed      int     `json:"seed"`
	SkylineAt float64 `json:"skyline_at"`
	Colors    struct {
		Far             string   `json:"far"`
		Limestone       string   `json:"limestone"`
		LimestoneShadow string   `json:"limestone_shadow"`
		Zinc            string   `json:"zinc"`
		ZincLight       string   `json:"zinc_light"`
		ZincDark        string   `json:"zinc_dark"`
		Chimney         string   `json:"chimney"`
		Pot             string   `json:"pot"`
		Iron            string   `json:"iron"`
		WindowDark      string   `json:"window_dark"`
		WindowLit       []string `json:"window_lit"`
		Moon            string   `json:"moon"`
		Star            string   `json:"star"`
		Cloud           string   `json:"cloud"`
		Snow            string   `json:"snow"`
		Fog             string   `json:"fog"`
		Rain            string   `json:"rain"`
		CityGlow        string   `json:"city_glow"`
	} `json:"colors"`
	Eiffel struct {
		Sprite        string  `json:"sprite"`
		X             float64 `json:"x"`
		Lit           string  `json:"lit"`
		Sparkle       string  `json:"sparkle"`
		LightsOnAbove float64 `json:"lights_on_when_lamps_above"`
		LightsOff     string  `json:"lights_off_at"`
		LightsBack    string  `json:"lights_never_before"`
		SparkleMins   float64 `json:"sparkle_minutes"`
		LastSparkle   string  `json:"last_sparkle"`
	} `json:"eiffel"`
	NearSprite    string `json:"near_sprite"`
	RailingSprite string `json:"railing_sprite"`
}

type viewColors struct {
	far, limestone, limeShadow, zinc, zincLight, zincDark rgb.Color
	chimney, pot, iron, winDark, moon, star, cloud        rgb.Color
	snow, fog, rain, cityGlow                             rgb.Color
	winLit                                                []rgb.Color
}

// scenery is one pixel of the outdoor picture (everything but the sky).
type scenery struct {
	col     rgb.Color
	mat     sprites.Material
	depth   float64 // 0 right outside the glass .. 1 far away
	solid   bool
	snowTop bool // a roof surface facing up, where snow settles first
}

type view struct {
	w, h      int
	seed      int
	skyline   int
	px        []scenery
	c         viewColors
	eiffelOn  float64
	eiffelOff float64 // minutes after midnight
	eiffelSOn float64 // lights never come on between off and this
	sparkle   float64
	lastSpark float64
	eLit      rgb.Color
	eSpark    rgb.Color

	// keepClear is the stretch of the skyline where the tower stands, so no
	// chimney is built right in front of it.
	keepClear [2]int

	// animation state
	drift  float64
	rain   []drop
	flakes []flake
	rng    *rand.Rand
}

type drop struct{ x, y, speed, length float64 }

type flake struct{ x, y, speed, phase, near float64 }

func loadView(fsys fs.FS, path string, lib *sprites.Library, w, h int) (*view, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	var f viewFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	v := &view{w: w, h: h, seed: f.Seed, px: make([]scenery, w*h),
		rng: rand.New(rand.NewPCG(uint64(f.Seed), 77))}
	hex := func(field, s string) (rgb.Color, error) {
		c, err := rgb.Hex(s)
		if err != nil {
			return c, fmt.Errorf("%s: colors.%s: %v", path, field, err)
		}
		return c, nil
	}
	cf := f.Colors
	for _, x := range []struct {
		dst  *rgb.Color
		src  string
		name string
	}{
		{&v.c.far, cf.Far, "far"}, {&v.c.limestone, cf.Limestone, "limestone"},
		{&v.c.limeShadow, cf.LimestoneShadow, "limestone_shadow"}, {&v.c.zinc, cf.Zinc, "zinc"},
		{&v.c.zincLight, cf.ZincLight, "zinc_light"}, {&v.c.zincDark, cf.ZincDark, "zinc_dark"},
		{&v.c.chimney, cf.Chimney, "chimney"}, {&v.c.pot, cf.Pot, "pot"}, {&v.c.iron, cf.Iron, "iron"},
		{&v.c.winDark, cf.WindowDark, "window_dark"}, {&v.c.moon, cf.Moon, "moon"},
		{&v.c.star, cf.Star, "star"}, {&v.c.cloud, cf.Cloud, "cloud"}, {&v.c.snow, cf.Snow, "snow"},
		{&v.c.fog, cf.Fog, "fog"}, {&v.c.rain, cf.Rain, "rain"}, {&v.c.cityGlow, cf.CityGlow, "city_glow"},
		{&v.eLit, f.Eiffel.Lit, "eiffel.lit"}, {&v.eSpark, f.Eiffel.Sparkle, "eiffel.sparkle"},
	} {
		if *x.dst, err = hex(x.name, x.src); err != nil {
			return nil, err
		}
	}
	for _, s := range cf.WindowLit {
		c, err := hex("window_lit", s)
		if err != nil {
			return nil, err
		}
		v.c.winLit = append(v.c.winLit, c)
	}
	if len(v.c.winLit) == 0 {
		return nil, fmt.Errorf("%s: colors.window_lit needs at least one color", path)
	}
	v.eiffelOn = f.Eiffel.LightsOnAbove
	if v.eiffelOff, err = timeofday.ParseClock(f.Eiffel.LightsOff); err != nil {
		return nil, fmt.Errorf("%s: eiffel.lights_off_at: %v", path, err)
	}
	if v.eiffelSOn, err = timeofday.ParseClock(f.Eiffel.LightsBack); err != nil {
		return nil, fmt.Errorf("%s: eiffel.lights_never_before: %v", path, err)
	}
	if v.lastSpark, err = timeofday.ParseClock(f.Eiffel.LastSparkle); err != nil {
		return nil, fmt.Errorf("%s: eiffel.last_sparkle: %v", path, err)
	}
	v.sparkle = f.Eiffel.SparkleMins

	v.skyline = int(math.Round(f.SkylineAt * float64(h)))
	if v.skyline < 4 || v.skyline > h-4 {
		return nil, fmt.Errorf("%s: skyline_at puts the rooftops outside the window", path)
	}

	// Back to front: the tower first, so the far roofs hide its feet.
	if f.Eiffel.Sprite != "" {
		s, ok := lib.Get(f.Eiffel.Sprite)
		if !ok {
			return nil, fmt.Errorf("%s: eiffel sprite %q doesn't exist", path, f.Eiffel.Sprite)
		}
		ex := int(math.Round(f.Eiffel.X*float64(w))) - s.W/2
		v.stamp(s, ex, v.skyline+2-s.H, 0.85)
		v.keepClear = [2]int{ex - 1, ex + s.W}
	}
	v.buildFar()
	v.buildMid()
	if f.NearSprite != "" {
		s, ok := lib.Get(f.NearSprite)
		if !ok {
			return nil, fmt.Errorf("%s: near_sprite %q doesn't exist", path, f.NearSprite)
		}
		v.stamp(s, (w-s.W)/2, h-s.H, 0.25)
	}
	if f.RailingSprite != "" {
		s, ok := lib.Get(f.RailingSprite)
		if !ok {
			return nil, fmt.Errorf("%s: railing_sprite %q doesn't exist", path, f.RailingSprite)
		}
		for x := -(s.W - w%s.W) / 2; x < w; x += s.W {
			v.stamp(s, x, h-s.H, 0.04)
		}
	}
	// Mark roof surfaces that face the sky, for snow.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := &v.px[y*w+x]
			if p.mat == sprites.Roof {
				above := v.at(x, y-1)
				p.snowTop = !above.solid || above.mat != sprites.Roof || above.depth != p.depth
			}
		}
	}
	return v, nil
}

func (v *view) at(x, y int) scenery {
	if x < 0 || y < 0 || x >= v.w || y >= v.h {
		return scenery{}
	}
	return v.px[y*v.w+x]
}

func (v *view) put(x, y int, s scenery) {
	if x >= 0 && y >= 0 && x < v.w && y < v.h {
		s.solid = true
		v.px[y*v.w+x] = s
	}
}

func (v *view) stamp(s *sprites.Sprite, ox, oy int, depth float64) {
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			p := s.At(x, y)
			if !p.Opaque {
				continue
			}
			v.put(ox+x, oy+y, scenery{col: p.Color, mat: p.Mat, depth: depth})
		}
	}
}

// buildFar draws the distant skyline: a hazy band of flat-topped roofs,
// a few chimneys, and one small dome.
func (v *view) buildFar() {
	base := v.skyline
	x := 0
	for b := 0; x < v.w; b++ {
		bw := 3 + int(noise.Hash(b, 0, v.seed)%4)
		top := base - int(noise.Hash(b, 1, v.seed)%2)
		for xx := x; xx < x+bw && xx < v.w; xx++ {
			for y := top; y < v.h; y++ {
				v.put(xx, y, scenery{col: v.c.far, depth: 0.9})
			}
			if noise.Rand(xx, 2, v.seed) < 0.15 {
				v.put(xx, top-1, scenery{col: v.c.far, depth: 0.9}) // a chimney
			}
			// A few tiny windows that may light up at night.
			for y := top + 2; y < v.h; y += 2 {
				if noise.Rand(xx, y, v.seed+9) < 0.2 {
					v.put(xx, y, scenery{col: v.c.far.Scale(0.88), mat: sprites.Window, depth: 0.9})
				}
			}
		}
		x += bw
	}
	// A small distant dome, as Paris has so many.
	dx := int(float64(v.w)*0.2) + int(noise.Hash(3, 3, v.seed)%4)
	dome := v.c.far.Scale(1.05)
	for xx := dx - 2; xx <= dx+2; xx++ {
		v.put(xx, base-1, scenery{col: dome, depth: 0.9})
	}
	for xx := dx - 1; xx <= dx+1; xx++ {
		v.put(xx, base-2, scenery{col: dome, depth: 0.9})
	}
	v.put(dx, base-3, scenery{col: dome, depth: 0.9})
}

// buildMid draws the nearer rooftops: tall zinc mansard roofs with dormer
// windows, wide chimney stacks crowned with terracotta pots, and the top
// floors of limestone fronts below, with their long iron balconies.
func (v *view) buildMid() {
	x := -int(noise.Hash(0, 5, v.seed) % 3)
	const depth = 0.55
	for i := 0; x < v.w; i++ {
		h := func(k int) int { return int(noise.Hash(i, k, v.seed)) }
		bw := 8 + h(6)%5
		roofTop := v.skyline + 1 + h(7)%2
		roofH := 4 + h(8)%2
		facade := roofTop + roofH

		// Limestone front: cornice, iron balcony, rows of windows.
		for y := facade; y < v.h; y++ {
			for xx := x; xx < x+bw; xx++ {
				c := v.c.limestone
				if xx == x+bw-1 {
					c = v.c.limeShadow
				}
				v.put(xx, y, scenery{col: c, depth: depth})
			}
		}
		for xx := x; xx < x+bw; xx++ {
			v.put(xx, facade, scenery{col: v.c.limestone.Scale(1.05), depth: depth})
			v.put(xx, facade+2, scenery{col: v.c.iron, depth: depth})
		}
		for y := facade + 1; y < v.h; y += 3 {
			for xx := x + 1; xx < x+bw-1; xx += 2 {
				v.put(xx, y, scenery{col: v.c.winDark, mat: sprites.Window, depth: depth})
				if y+1 < v.h && y+1 != facade+2 {
					v.put(xx, y+1, scenery{col: v.c.winDark, mat: sprites.Window, depth: depth})
				}
			}
		}

		// Zinc mansard: lighter ridge, standing seams, darker at the gutter.
		for y := roofTop; y < facade; y++ {
			inset := 0
			if y == roofTop {
				inset = 1
			}
			for xx := x + inset; xx < x+bw-inset; xx++ {
				c := v.c.zinc
				switch {
				case y == roofTop:
					c = v.c.zincLight.Scale(1.08)
				case y == facade-1:
					c = v.c.zincDark
				case (xx-x)%2 == 1:
					c = v.c.zincLight
				}
				v.put(xx, y, scenery{col: c, mat: sprites.Roof, depth: depth})
			}
		}
		// Dormers: a stone cap with a little window beneath.
		for xx := x + 2 + h(13)%2; xx+1 < x+bw-1; xx += 4 {
			v.put(xx, facade-3, scenery{col: v.c.limestone, depth: depth})
			v.put(xx+1, facade-3, scenery{col: v.c.limestone, depth: depth})
			v.put(xx, facade-2, scenery{col: v.c.winDark, mat: sprites.Window, depth: depth})
			v.put(xx+1, facade-2, scenery{col: v.c.limeShadow, depth: depth})
		}
		// A wide chimney stack with a row of pots on most buildings.
		cw := 3 + h(10)%3
		cx := x + 1 + h(11)%max(1, bw-cw-1)
		hidesTower := cx < v.keepClear[1] && cx+cw > v.keepClear[0]
		if h(9)%4 != 0 && !hidesTower {
			for y := roofTop - 2; y < roofTop+1; y++ {
				for xx := cx; xx < cx+cw; xx++ {
					c := v.c.chimney
					if xx == cx+cw-1 {
						c = c.Scale(0.85)
					}
					if y == roofTop-2 {
						c = c.Scale(1.06) // the capping catches the light
					}
					v.put(xx, y, scenery{col: c, depth: depth})
				}
			}
			for xx := cx; xx < cx+cw; xx += 2 {
				v.put(xx, roofTop-3, scenery{col: v.c.pot, depth: depth})
			}
		}
		x += bw
	}
}

// env is everything the view needs to know for one frame.
type env struct {
	light     timeofday.Light
	minute    float64 // minutes after midnight
	moonPhase float64
	cloud     float64
	rain      float64
	snow      float64
	fog       float64
	wind      float64
	snowCover float64
	t         float64 // seconds since the scene started (for animation)
	reduced   bool
}

// refArea is the window size (in pixels of glass) that the amounts of rain
// and snow were tuned for; bigger windows get more, so the weather looks
// just as heavy whatever the window's size.
const refArea = 30 * 34

// density scales a number of raindrops or flakes to this window's size.
func (v *view) density(n float64) int {
	return int(math.Round(n * float64(v.w*v.h) / refArea))
}

// update moves the weather particles forward by dt seconds.
func (v *view) update(dt float64, e env) {
	v.drift += dt * (0.12 + 0.25*math.Abs(e.wind))

	// Rain streaks.
	want := v.density(e.rain * 34)
	if e.reduced {
		want = v.density(e.rain * 12)
	}
	for len(v.rain) < want {
		d := v.newDrop(e, true)
		v.rain = append(v.rain, d)
	}
	if len(v.rain) > want {
		v.rain = v.rain[:want]
	}
	slant := e.wind * 0.35
	for i := range v.rain {
		d := &v.rain[i]
		d.y += d.speed * dt
		d.x += d.speed * slant * dt
		if d.y-d.length > float64(v.h) || d.x < -4 || d.x > float64(v.w)+4 {
			*d = v.newDrop(e, false)
		}
	}

	// Snowflakes.
	want = v.density(e.snow * 46)
	if e.reduced {
		want = v.density(e.snow * 20)
	}
	for len(v.flakes) < want {
		v.flakes = append(v.flakes, flake{
			x: v.rng.Float64() * float64(v.w), y: v.rng.Float64() * float64(v.h),
			speed: 3 + v.rng.Float64()*4, phase: v.rng.Float64() * 6.28, near: v.rng.Float64(),
		})
	}
	if len(v.flakes) > want {
		v.flakes = v.flakes[:want]
	}
	for i := range v.flakes {
		f := &v.flakes[i]
		sp := f.speed * (0.7 + f.near*0.8)
		if e.reduced {
			sp *= 0.6
		}
		f.y += sp * dt
		f.x += (e.wind*2 + math.Sin(e.t*1.1+f.phase)*0.9) * dt
		if f.y > float64(v.h) {
			f.y = -1
			f.x = v.rng.Float64() * float64(v.w)
		}
		if f.x < -1 {
			f.x += float64(v.w) + 1
		} else if f.x > float64(v.w) {
			f.x -= float64(v.w) + 1
		}
	}
}

func (v *view) newDrop(e env, anywhere bool) drop {
	heavy := e.rain
	d := drop{
		x:      v.rng.Float64()*float64(v.w+8) - 4,
		speed:  38 + heavy*42 + v.rng.Float64()*14,
		length: 2 + math.Round(heavy*2+v.rng.Float64()),
	}
	if e.reduced {
		d.speed *= 0.45
	}
	if anywhere {
		d.y = v.rng.Float64() * float64(v.h)
	} else {
		d.y = -v.rng.Float64() * 10
	}
	return d
}

// brightness is a rough 0..1 measure of how much daylight is outside.
func brightness(l timeofday.Light) float64 {
	return rgb.Clamp01(l.Exterior.Luma())
}

// render paints the view into dst (w x h, row by row).
func (v *view) render(dst []rgb.Color, e env) {
	l := e.light
	bright := brightness(l)
	night := 1 - rgb.Smoothstep(0.25, 0.75, bright)
	haze := math.Max(e.fog, e.rain*0.35)

	// Colors that depend on the light.
	overcastTop := l.SkyTop.Desaturate(0.75).Lerp(v.c.cloud.Mul(l.Exterior), 0.5).Scale(1 - 0.25*e.rain)
	overcastLow := l.SkyHorizon.Desaturate(0.7).Lerp(v.c.cloud.Mul(l.Exterior), 0.5).Scale(1 - 0.2*e.rain)
	glow := v.c.cityGlow.Scale(night * (0.35 + 0.65*e.cloud))
	cloudCol := v.c.cloud.Mul(l.Exterior).Lerp(l.SkyHorizon, 0.25).Scale(1 - 0.3*e.rain)
	fogCol := v.c.fog.Mul(l.Exterior).Lerp(l.SkyHorizon, 0.3).Add(glow.Scale(0.6))
	overcast := rgb.Smoothstep(0.8, 1.0, e.cloud)
	threshold := 0.5 + (0.5-e.cloud)*0.45

	// Moon position: rises in the east at dusk, sets at dawn.
	nightProgress := math.Mod(e.minute-18*60+1440, 1440) / (12 * 60)
	moonX := 0.12*float64(v.w) + nightProgress*0.76*float64(v.w)
	moonY := 2 + 5*(1-math.Sin(math.Min(1, nightProgress)*math.Pi))
	moonVis := l.Stars * (1 - e.fog) * (1 - overcast)
	if nightProgress > 1 {
		moonVis = 0
	}

	for y := 0; y < v.h; y++ {
		ty := math.Pow(math.Min(1, float64(y)/float64(v.skyline)), 1.3)
		clear := l.SkyTop.Lerp(l.SkyHorizon, ty)
		grey := overcastTop.Lerp(overcastLow, ty).Add(glow.Scale(ty * 0.8))
		base := clear.Lerp(grey, overcast)
		for x := 0; x < v.w; x++ {
			i := y*v.w + x
			s := v.px[i]
			if s.solid {
				dst[i] = v.lightScenery(x, y, s, e, bright, haze, fogCol)
				continue
			}
			// Sky with clouds.
			n := noise.Fractal((float64(x)+v.drift)*0.085, float64(y)*0.21, v.seed+40, 3)
			cov := rgb.Smoothstep(threshold-0.07, threshold+0.07, n)
			c := base
			if cov > 0 && overcast < 1 {
				shade := 1 - 0.18*rgb.Smoothstep(threshold, threshold+0.2, n)
				c = base.Lerp(cloudCol.Scale(shade), cov*(1-overcast)*0.9)
			}
			if overcast > 0 {
				// texture in the overcast: soft darker billows
				c = c.Scale(1 - overcast*0.08*(n-0.5))
			}
			// Stars.
			if l.Stars > 0.01 && y < v.skyline-1 {
				h := noise.Rand(x, y, v.seed+3)
				if h < 0.035 {
					tw := 1.0
					if !e.reduced {
						tw = 0.55 + 0.45*noise.Value(e.t*0.6+h*500, 0, v.seed)
					}
					b := l.Stars * (1 - cov) * (1 - overcast) * (1 - haze) * tw * (0.5 + h*14)
					c = c.Lerp(v.c.star, rgb.Clamp01(b))
				}
			}
			// The moon.
			if moonVis > 0.01 {
				dx, dy := float64(x)+0.5-moonX, float64(y)+0.5-moonY
				d := math.Hypot(dx, dy)
				if d < 1.9 {
					xr := dx / 1.9
					tt := math.Cos(2 * math.Pi * e.moonPhase)
					lit := (e.moonPhase < 0.5 && xr > tt) || (e.moonPhase >= 0.5 && xr < -tt)
					vis := moonVis * (1 - cov*0.8)
					if lit {
						c = c.Lerp(v.c.moon, vis)
					} else {
						c = c.Lerp(v.c.moon.Scale(0.35), vis*0.35) // earthshine
					}
				} else if d < 5 {
					c = c.Lerp(v.c.moon, moonVis*0.12*(1-d/5)*(0.5+haze)*math.Max(0.2, 1-e.cloud))
				}
			}
			// Fog over the sky too.
			if haze > 0 {
				c = c.Lerp(fogCol, haze*0.75)
			}
			dst[i] = c
		}
	}
	// Fog drifts in slow banks in front of everything far away.
	if e.fog > 0.05 {
		for y := 0; y < v.h; y++ {
			for x := 0; x < v.w; x++ {
				i := y*v.w + x
				d := 1.0
				if v.px[i].solid {
					d = v.px[i].depth
				}
				bank := noise.Fractal((float64(x)+v.drift*3)*0.06, float64(y)*0.12+7, v.seed+60, 2)
				a := e.fog * d * (0.25 + 0.6*bank)
				dst[i] = dst[i].Lerp(fogCol, rgb.Clamp01(a))
			}
		}
	}
	v.drawRain(dst, e, l, fogCol)
	v.drawSnow(dst, e, l)
}

// lightScenery colors one pixel of rooftops for the current light.
func (v *view) lightScenery(x, y int, s scenery, e env, bright, haze float64, fogCol rgb.Color) rgb.Color {
	l := e.light
	c := s.col.Mul(l.Exterior)
	switch s.mat {
	case sprites.Roof:
		if e.snowCover > 0 {
			a := e.snowCover * 0.85
			if !s.snowTop {
				a *= 0.55
			}
			c = c.Lerp(v.c.snow.Mul(l.Exterior).Scale(1.08), a)
		}
	case sprites.Window:
		// Each window has its own "bedtime", so lights come and go one by
		// one rather than all at once.
		h := noise.Rand(x, y, v.seed+21)
		slot := math.Floor((e.minute + h*600) / (25 + h*50))
		chance := noise.Rand(x, y+int(slot)*97, v.seed+22)
		if chance < l.Windows {
			lit := v.c.winLit[int(noise.Hash(x, y, v.seed+23)%uint32(len(v.c.winLit)))]
			strength := 1 - 0.45*s.depth
			c = c.Lerp(lit, strength*(1-0.6*bright))
		}
	case sprites.Eiffel, sprites.Beacon:
		on := v.eiffelLit(e, l)
		if on > 0 {
			c = c.Lerp(v.eLit, on*0.9)
			if s.mat == sprites.Beacon {
				pulse := 0.5 + 0.5*math.Max(0, math.Cos(e.t*2*math.Pi/5.5))
				if e.reduced {
					pulse = 0.8
				}
				c = c.Lerp(rgb.MustHex("#fff4d8"), on*pulse)
			} else if v.sparkling(e) && !e.reduced && noise.Rand(x, y+int(e.t*7)*31, v.seed+5) < 0.3 {
				c = c.Lerp(v.eSpark, on)
			}
		}
	}
	// Distance softens everything toward the sky, a little even on clear
	// days, much more in fog or heavy rain.
	air := s.depth*0.32 + haze*s.depth*0.8
	c = c.Lerp(l.SkyHorizon.Lerp(fogCol, haze), rgb.Clamp01(air))
	return c
}

// between reports whether minute m falls in [from, to), wrapping past midnight.
func between(m, from, to float64) bool {
	if from <= to {
		return m >= from && m < to
	}
	return m >= from || m < to
}

// eiffelLit says how lit the tower is (0..1) at this moment.
func (v *view) eiffelLit(e env, l timeofday.Light) float64 {
	if between(e.minute, v.eiffelOff, v.eiffelSOn) {
		return 0
	}
	return rgb.Smoothstep(v.eiffelOn-0.05, v.eiffelOn+0.05, l.Lamps)
}

// sparkling: the tower glitters for the first few minutes of each hour in
// the evening, up to the last sparkle of the night.
func (v *view) sparkling(e env) bool {
	m := e.minute
	hour := math.Floor(m/60) * 60
	if m < 12*60 || hour > v.lastSpark {
		return false
	}
	return m-hour < v.sparkle
}

func (v *view) drawRain(dst []rgb.Color, e env, l timeofday.Light, fogCol rgb.Color) {
	if len(v.rain) == 0 {
		return
	}
	col := v.c.rain.Mul(l.Exterior.Lerp(rgb.White, 0.35)).Lerp(fogCol, 0.2)
	slant := e.wind * 0.35
	for _, d := range v.rain {
		for k := 0; k < int(d.length); k++ {
			yy := d.y - float64(k)
			xx := d.x - slant*float64(k)
			ix, iy := int(math.Floor(xx)), int(math.Floor(yy))
			if ix < 0 || iy < 0 || ix >= v.w || iy >= v.h {
				continue
			}
			a := (0.22 + 0.3*e.rain) * (1 - float64(k)/d.length*0.6)
			i := iy*v.w + ix
			dst[i] = dst[i].Lerp(col, a)
		}
	}
}

func (v *view) drawSnow(dst []rgb.Color, e env, l timeofday.Light) {
	if len(v.flakes) == 0 {
		return
	}
	col := v.c.snow.Mul(l.Exterior.Lerp(rgb.White, 0.45))
	for _, f := range v.flakes {
		ix, iy := int(math.Floor(f.x)), int(math.Floor(f.y))
		if ix < 0 || iy < 0 || ix >= v.w || iy >= v.h {
			continue
		}
		a := 0.55 + 0.4*f.near
		i := iy*v.w + ix
		dst[i] = dst[i].Lerp(col, a)
	}
}
