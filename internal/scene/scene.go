// Package scene draws the café: the room, the view through the window, the
// weather, the lamps and the steam from your cup. It paints pixels onto a
// canvas and never talks to the terminal itself, so it can be tested (and
// its frames saved as pictures) anywhere.
package scene

import (
	"fmt"
	"io/fs"
	"math"
	"math/rand/v2"
	"time"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/noise"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
	"coffee-and-toe-beans/internal/timeofday"
	"coffee-and-toe-beans/internal/weather"
)

// Options are comfort settings that change how the scene moves.
type Options struct {
	// ReducedMotion makes everything calmer: no flicker or twinkle, sparse
	// slow rain, raindrops that stay put.
	ReducedMotion bool
}

// Inputs is what the scene needs to know about the world each frame.
type Inputs struct {
	Now       time.Time       // the game clock
	Light     timeofday.Light // from the time of day
	Weather   weather.Params  // from the weather
	SnowCover float64         // snow lying on the roofs, 0..1
}

// Scene is a café you can draw.
type Scene struct {
	room  *room
	view  *view
	lib   *sprites.Library
	opts  Options
	rng   *rand.Rand
	t     float64
	in    Inputs
	env   env
	ready bool

	viewBuf  []rgb.Color
	lightBuf []rgb.Color // light falling on each pixel this frame
	level    []float64   // each light's brightness this frame

	cup      *sprites.Sprite
	cupAt    [2]int // top-left pixel of the cup
	steamAt  [2]int // where steam rises
	liquid   *liquid
	steam    []puff
	steamAmt float64
	sinceP   float64

	drops     []droplet
	sinceDrop float64
}

type puff struct{ x, y, age, life, phase, drift float64 }

type droplet struct {
	pane       int
	x, y       float64
	big        bool
	age, life  float64
	sliding    bool
	speed      float64
	trail      []trailPoint
	nextWobble float64
}

type trailPoint struct{ x, y, age float64 }

// New loads an interior style (for example "french") and the view.
func New(fsys fs.FS, interior string, lib *sprites.Library, opts Options) (*Scene, error) {
	r, err := loadRoom(fsys, "interiors/"+interior+".json", lib)
	if err != nil {
		return nil, err
	}
	if !r.hasGlass {
		return nil, fmt.Errorf("interior %q has no window", interior)
	}
	v, err := loadView(fsys, "view.json", lib, r.glass.w, r.glass.h)
	if err != nil {
		return nil, err
	}
	s := &Scene{
		room:     r,
		view:     v,
		lib:      lib,
		opts:     opts,
		rng:      rand.New(rand.NewPCG(7, 11)),
		viewBuf:  make([]rgb.Color, r.glass.w*r.glass.h),
		lightBuf: make([]rgb.Color, r.w*r.h),
		level:    make([]float64, len(r.lights)),
		steamAmt: 1,
	}
	if r.cup.Sprite != "" {
		s.cup, _ = lib.Get(r.cup.Sprite)
		s.cupAt, s.steamAt = r.cup.At, r.cup.Steam
	}
	return s, nil
}

// Cup is a drink on your table.
type Cup struct {
	Sprite    string    // the cup or glass
	Steam     [2]int    // where steam rises, from the sprite's top-left
	Body, Top rgb.Color // the drink's colors (see the @drink pixels)
	Level     float64   // how full: 1 full, 0 empty
}

type liquid struct {
	body, top rgb.Color
	level     float64
	rows      []int // the rows that have @drink pixels, top to bottom
}

// PutCup sets a drink down on the table (nil clears the table). It stands
// on the interior's cup spot, whatever its size.
func (s *Scene) PutCup(c *Cup) error {
	if c == nil {
		s.cup, s.liquid = nil, nil
		return nil
	}
	sp, ok := s.lib.Get(c.Sprite)
	if !ok {
		return fmt.Errorf("no sprite called %q", c.Sprite)
	}
	stand := s.stand()
	s.cup = sp
	s.cupAt = [2]int{stand[0], stand[1] - sp.H + 1}
	s.steamAt = [2]int{s.cupAt[0] + c.Steam[0], s.cupAt[1] + c.Steam[1]}
	l := &liquid{body: c.Body, top: c.Top, level: rgb.Clamp01(c.Level)}
	for y := 0; y < sp.H; y++ {
		for x := 0; x < sp.W; x++ {
			if p := sp.At(x, y); p.Opaque && p.Mat == sprites.Drink {
				l.rows = append(l.rows, y)
				break
			}
		}
	}
	s.liquid = l
	return nil
}

// SetLevel changes how full the cup on the table is (1 full, 0 empty).
func (s *Scene) SetLevel(level float64) {
	if s.liquid != nil {
		s.liquid.level = rgb.Clamp01(level)
	}
}

// stand is the bottom-left pixel where cups stand.
func (s *Scene) stand() [2]int {
	c := s.room.cup
	if c.Stand != nil {
		return *c.Stand
	}
	h := 0
	if sp, ok := s.lib.Get(c.Sprite); ok {
		h = sp.H
	}
	return [2]int{c.At[0], c.At[1] + h - 1}
}

// Size is the scene's size in pixels.
func (s *Scene) Size() (w, h int) { return s.room.w, s.room.h }

// Spots are the named places where cats can sit.
func (s *Scene) Spots() map[string]Spot { return s.room.spots }

// SetOptions changes comfort settings.
func (s *Scene) SetOptions(o Options) { s.opts = o }

// SetSteam sets how much the cup steams: 1 piping hot, 0 cold.
func (s *Scene) SetSteam(amount float64) { s.steamAmt = rgb.Clamp01(amount) }

// Update moves the animation forward by dt seconds of real time.
func (s *Scene) Update(dt float64, in Inputs) {
	if dt < 0 {
		dt = 0
	}
	if dt > 0.5 {
		dt = 0.5 // after a pause, don't let raindrops teleport
	}
	s.t += dt
	s.in = in
	w := in.Weather
	s.env = env{
		light:     in.Light,
		minute:    timeofday.MinuteOfDay(in.Now),
		moonPhase: timeofday.MoonPhase(in.Now),
		cloud:     w.Cloud,
		rain:      w.Rain,
		snow:      w.Snow,
		fog:       w.Fog,
		wind:      w.Wind,
		snowCover: in.SnowCover,
		t:         s.t,
		reduced:   s.opts.ReducedMotion,
	}
	s.view.update(dt, s.env)
	s.updateSteam(dt)
	s.updateDrops(dt)
	s.ready = true
}

// Draw paints the scene onto c with its top-left pixel at (ox, oy).
func (s *Scene) Draw(c *canvas.Canvas, ox, oy int) {
	if !s.ready {
		s.Update(0, s.in)
	}
	r := s.room
	e := s.env
	l := e.light
	s.view.render(s.viewBuf, e)

	// How bright is each lamp right now?
	for i, li := range r.lights {
		f := 1.0
		if !s.opts.ReducedMotion && li.flicker > 0 {
			speed := 1.3
			if li.flicker > 0.06 {
				speed = 6 // candles dance faster
			}
			f = 1 + li.flicker*(noise.Fractal(s.t*speed+float64(i)*13.7, float64(i), 99, 2)*2-1)*1.6
		}
		s.level[i] = l.Lamps * li.strength * f
	}

	bright := brightness(l)
	day := l.Daylight * (1 - 0.45*e.cloud) * (1 - 0.25*e.fog)
	ambient := l.Ambient.Scale(1 - 0.16*e.cloud*bright)

	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			i := y*r.w + x
			dw := r.daylight[i]
			light := ambient.Scale(0.84 + 0.3*dw*day).Add(r.dayTint.Scale(dw * day * 0.08))
			for li, lt := range r.lights {
				if wgt := lt.weight[i]; wgt > 0 {
					light = light.Add(lt.color.Scale(wgt * s.level[li]))
				}
			}
			s.lightBuf[i] = light
		}
	}

	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			i := y*r.w + x
			p := r.px[i]
			var col rgb.Color
			switch {
			case p.glass:
				gx, gy := x-r.glass.x, y-r.glass.y
				col = s.viewBuf[gy*r.glass.w+gx]
				// At night the glass catches a faint reflection of the lamps.
				refl := rgb.Black
				for li, lt := range r.lights {
					if wgt := lt.weight[i]; wgt > 0 {
						refl = refl.Add(lt.color.Scale(wgt * s.level[li]))
					}
				}
				col = col.Add(refl.Scale(0.13 * (1 - bright)))
			case p.mat == sprites.Lamp:
				col = p.col.Mul(s.lightBuf[i])
				if p.light >= 0 {
					lt := r.lights[p.light]
					on := rgb.Clamp01(s.level[p.light] / math.Max(0.01, lt.strength))
					glow := p.col.Scale(0.45).Add(lt.color.Scale(0.7)).Screen(lt.color.Scale(0.25))
					col = col.Lerp(glow, on)
				}
			case p.mat == sprites.Flame:
				on := 0.0
				if p.light >= 0 {
					on = rgb.Clamp01(s.level[p.light] / math.Max(0.01, r.lights[p.light].strength))
				}
				wick := rgb.MustHex("#3a2c22").Mul(s.lightBuf[i])
				flick := 1.0
				if !s.opts.ReducedMotion {
					flick = 0.85 + 0.3*noise.Value(s.t*7+float64(x), float64(y), 5)
				}
				col = wick.Lerp(p.col.Scale(flick), rgb.Smoothstep(0.1, 0.3, on))
			default:
				col = p.col.Mul(s.lightBuf[i])
			}
			c.Set(ox+x, oy+y, col.Clamp())
		}
	}

	s.drawPaneSnow(c, ox, oy)
	s.drawDrops(c, ox, oy)
	s.drawCup(c, ox, oy)
	s.drawSteam(c, ox, oy)
}

// lightAt is the light falling on a room pixel this frame.
func (s *Scene) lightAt(x, y int) rgb.Color {
	if x < 0 || y < 0 || x >= s.room.w || y >= s.room.h {
		return rgb.Black
	}
	return s.lightBuf[y*s.room.w+x]
}

func (s *Scene) drawCup(c *canvas.Canvas, ox, oy int) {
	if s.cup == nil {
		return
	}
	at := s.cupAt
	for y := 0; y < s.cup.H; y++ {
		for x := 0; x < s.cup.W; x++ {
			p := s.cup.At(x, y)
			if !p.Opaque {
				continue
			}
			col := p.Color
			if p.Mat == sprites.Drink {
				col = s.drinkColor(y, col)
			}
			px, py := at[0]+x, at[1]+y
			c.Set(ox+px, oy+py, col.Mul(s.lightAt(px, py)).Clamp())
		}
	}
}

// drinkColor is what an @drink pixel on row y shows: the drink's top on the
// highest row still filled, the drink below it, and the empty cup above. An
// empty cup keeps a faint ring of the drink at the bottom.
func (s *Scene) drinkColor(y int, empty rgb.Color) rgb.Color {
	l := s.liquid
	if l == nil || len(l.rows) == 0 {
		return empty
	}
	n := len(l.rows)
	full := int(math.Ceil(l.level*float64(n) - 1e-9))
	first := n - full // index of the highest filled row
	ri := 0
	for i, r := range l.rows {
		if r == y {
			ri = i
		}
	}
	switch {
	case full == 0 && ri == n-1:
		return empty.Lerp(l.body, 0.3)
	case ri < first:
		return empty
	case ri == first:
		return l.top
	}
	return l.body
}

func (s *Scene) updateSteam(dt float64) {
	if s.cup == nil {
		s.steam = s.steam[:0]
		return
	}
	every := 0.24
	if s.opts.ReducedMotion {
		every = 0.6
	}
	s.sinceP += dt
	for s.sinceP >= every {
		s.sinceP -= every
		if s.rng.Float64() < s.steamAmt {
			o := s.steamAt
			s.steam = append(s.steam, puff{
				x:     float64(o[0]) + s.rng.Float64()*2 - 1,
				y:     float64(o[1]),
				life:  2.2 + s.rng.Float64()*1.6,
				phase: s.rng.Float64() * 6.28,
				drift: (s.rng.Float64() - 0.4) * 0.5,
			})
		}
	}
	rise := 2.6
	if s.opts.ReducedMotion {
		rise = 1.4
	}
	kept := s.steam[:0]
	for _, p := range s.steam {
		p.age += dt
		p.y -= rise * dt
		if p.age < p.life {
			kept = append(kept, p)
		}
	}
	s.steam = kept
}

func (s *Scene) drawSteam(c *canvas.Canvas, ox, oy int) {
	white := rgb.MustHex("#f4f1ec")
	for _, p := range s.steam {
		x := p.x + math.Sin(p.age*1.6+p.phase)*0.7 + p.drift*p.age
		ix, iy := int(math.Floor(x)), int(math.Floor(p.y))
		fade := 1 - p.age/p.life
		in := math.Min(1, p.age/0.35)
		a := 0.42 * fade * in * (0.4 + 0.6*s.steamAmt)
		col := white.Mul(s.lightAt(ix, iy).Add(rgb.Color{R: 0.12, G: 0.12, B: 0.12})).Clamp()
		c.Blend(ox+ix, oy+iy, col, a)
		c.Blend(ox+ix, oy+iy+1, col, a*0.35) // a faint wisp trailing below
	}
}

// Raindrops on the glass: they gather, sometimes grow heavy and run down,
// leaving a short wet trail.
func (s *Scene) updateDrops(dt float64) {
	r := s.room
	if len(r.panes) == 0 {
		return
	}
	rain := s.env.rain
	limit := int(rain * 70)
	if s.opts.ReducedMotion {
		limit = int(rain * 35)
	}
	s.sinceDrop += dt
	rate := rain * 9 // new drops per second
	for rate > 0 && s.sinceDrop > 1/rate {
		s.sinceDrop -= 1 / rate
		if len(s.drops) >= limit {
			continue
		}
		pi := s.rng.IntN(len(r.panes))
		pn := r.panes[pi]
		s.drops = append(s.drops, droplet{
			pane: pi,
			x:    float64(pn.x) + s.rng.Float64()*float64(pn.w),
			y:    float64(pn.y) + s.rng.Float64()*float64(pn.h),
			big:  s.rng.Float64() < 0.25,
			life: 8 + s.rng.Float64()*16,
		})
	}
	if rate <= 0 {
		s.sinceDrop = 0
	}
	kept := s.drops[:0]
	for _, d := range s.drops {
		d.age += dt
		pn := r.panes[d.pane]
		bottom := float64(pn.y + pn.h)
		if !d.sliding && d.big && !s.opts.ReducedMotion && s.rng.Float64() < dt*0.06 {
			d.sliding = true
			d.speed = 2 + s.rng.Float64()*4
		}
		if d.sliding && d.y < bottom {
			d.trail = append(d.trail, trailPoint{x: d.x, y: d.y})
			d.y += d.speed * dt
			d.nextWobble -= dt
			if d.nextWobble <= 0 {
				d.nextWobble = 0.6 + s.rng.Float64()
				if s.rng.Float64() < 0.3 {
					d.x += float64(s.rng.IntN(3) - 1)
				}
			}
			d.x = math.Max(float64(pn.x), math.Min(float64(pn.x+pn.w)-0.01, d.x))
		}
		t := d.trail[:0]
		for _, tp := range d.trail {
			tp.age += dt
			if tp.age < 2.5 {
				t = append(t, tp)
			}
		}
		d.trail = t
		done := false
		if d.sliding && d.y >= bottom {
			d.y = bottom // gone past the bar; wait for its trail to dry
			done = len(d.trail) == 0
		}
		if !d.sliding && d.age > d.life {
			done = true
		}
		if !done {
			kept = append(kept, d)
		}
	}
	s.drops = kept
}

func (s *Scene) drawDrops(c *canvas.Canvas, ox, oy int) {
	r := s.room
	l := s.env.light
	sheen := rgb.White.Lerp(l.SkyHorizon, 0.3).Mul(l.Exterior.Lerp(rgb.White, 0.3))
	for _, d := range s.drops {
		pn := r.panes[d.pane]
		for _, tp := range d.trail {
			ix, iy := int(tp.x), int(tp.y)
			if iy >= pn.y+pn.h {
				continue
			}
			bg := c.At(ox+ix, oy+iy)
			c.Set(ox+ix, oy+iy, bg.Lerp(sheen.Add(s.lightAt(ix, iy).Scale(0.1)).Clamp(), 0.14*(1-tp.age/2.5)))
		}
		ix, iy := int(d.x), int(d.y)
		if iy >= pn.y+pn.h || iy < pn.y {
			continue
		}
		fade := 1.0
		if !d.sliding {
			fade = math.Min(1, math.Min(d.age/0.8, (d.life-d.age)/2))
		}
		a := 0.3
		if d.big {
			a = 0.46
		}
		hi := sheen.Add(s.lightAt(ix, iy).Scale(0.25)).Clamp()
		bg := c.At(ox+ix, oy+iy)
		c.Set(ox+ix, oy+iy, bg.Lerp(hi, a*fade))
		if d.big && iy+1 < pn.y+pn.h {
			// the lower edge of a fat drop is darker, like a little lens
			b2 := c.At(ox+ix, oy+iy+1)
			c.Set(ox+ix, oy+iy+1, b2.Scale(1-0.18*fade))
		}
	}
}

// Snow collecting in the bottom corners of each pane, outside the glass.
func (s *Scene) drawPaneSnow(c *canvas.Canvas, ox, oy int) {
	cover := s.env.snowCover
	if cover < 0.05 {
		return
	}
	l := s.env.light
	snow := s.view.c.snow.Mul(l.Exterior.Lerp(rgb.White, 0.35))
	for pi, pn := range s.room.panes {
		for x := pn.x; x < pn.x+pn.w; x++ {
			// Deeper in the corners, a little uneven along the bar.
			edge := math.Min(float64(x-pn.x), float64(pn.x+pn.w-1-x))
			hgt := cover*1.7 - edge*0.3 + noise.Value(float64(x)*0.7, float64(pi), 31)*1.1 - 0.3
			n := int(math.Round(hgt))
			for k := 0; k < n && k < pn.h/4; k++ {
				y := pn.y + pn.h - 1 - k
				shade := 1.0 - 0.06*float64(k)
				c.Set(ox+x, oy+y, snow.Scale(shade).Clamp())
			}
		}
	}
}
