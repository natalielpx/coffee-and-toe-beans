package scene

import (
	"fmt"
	"sort"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
)

// CatDraw is one cat in the room this frame: which picture, at which spot,
// and how visible it is.
type CatDraw struct {
	Sprite string // the sprite to draw (one frame of one pose)
	Spot   string // the spot it is at (see the interior's cat_spots)
	// Anchor is the sprite's pixel that sits on the spot: the bottom row,
	// under the middle of the cat's body.
	Anchor [2]int
	Alpha  float64 // 1 fully there, 0 invisible (cats fade in and out)
}

type catDraw struct {
	CatDraw
	sp   *sprites.Sprite
	spot Spot
}

// SetCats says which cats to draw this frame (nil for none). Cats further
// back in the room are drawn first, and the room's nearer things, such as
// the table, stay in front of them.
func (s *Scene) SetCats(cats []CatDraw) error {
	s.cats = s.cats[:0]
	for _, c := range cats {
		sp, ok := s.lib.Get(c.Sprite)
		if !ok {
			return fmt.Errorf("no sprite called %q", c.Sprite)
		}
		spot, ok := s.room.spots[c.Spot]
		if !ok {
			return fmt.Errorf("this interior has no cat spot called %q", c.Spot)
		}
		s.cats = append(s.cats, catDraw{CatDraw: c, sp: sp, spot: spot})
	}
	sort.SliceStable(s.cats, func(i, j int) bool { return s.cats[i].spot.Y < s.cats[j].spot.Y })
	return nil
}

// CatBox is the rectangle of room pixels a sprite covers when it sits at a
// spot with the given anchor: x0, y0 inclusive, x1, y1 exclusive.
func (s *Scene) CatBox(sprite, spot string, anchor [2]int) (x0, y0, x1, y1 int, err error) {
	sp, ok := s.lib.Get(sprite)
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("no sprite called %q", sprite)
	}
	st, ok := s.room.spots[spot]
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("this interior has no cat spot called %q", spot)
	}
	x0, y0 = place(sp, st, anchor)
	return x0, y0, x0 + sp.W, y0 + sp.H, nil
}

// place is where a sprite's top-left pixel goes so that its anchor sits on
// the spot. Mirrored spots mirror the anchor too.
func place(sp *sprites.Sprite, st Spot, anchor [2]int) (x, y int) {
	ax := anchor[0]
	if st.Flip {
		ax = sp.W - 1 - ax
	}
	return st.X - ax, st.Y - anchor[1]
}

// catGlow is a little warm light every cat catches after dark (as if from
// the lamps and the candle), so a cat in a dim corner is still easy to see.
var catGlow = rgb.Color{R: 1, G: 0.84, B: 0.66}

// dimCat is how dim the light on a cat must be (0..1) for its fur to be
// kept lighter in 256-color mode.
const dimCat = 0.6

// catGlowMax is how much of it at the darkest hour of the night.
const catGlowMax = 0.2

func (s *Scene) drawCats(c *canvas.Canvas, ox, oy int) {
	r := s.room
	night := 1 - rgb.Smoothstep(0.25, 0.75, brightness(s.env.light))
	glow := catGlow.Scale(catGlowMax * night)
	for _, cd := range s.cats {
		if cd.Alpha <= 0 {
			continue
		}
		a := cd.Alpha
		if a > 1 {
			a = 1
		}
		x0, y0 := place(cd.sp, cd.spot, cd.Anchor)
		// A cat in a dim place is marked, so that in 256 colors its fur is
		// kept lighter than the room around it (see rgb.To256At).
		mid := s.lightAt(x0+cd.sp.W/2, y0+cd.sp.H/2).Add(glow)
		dim := mid.Luma() < dimCat
		for y := 0; y < cd.sp.H; y++ {
			for x := 0; x < cd.sp.W; x++ {
				sx := x
				if cd.spot.Flip {
					sx = cd.sp.W - 1 - x
				}
				p := cd.sp.At(sx, y)
				if !p.Opaque {
					continue
				}
				px, py := x0+x, y0+y
				if px < 0 || py < 0 || px >= r.w || py >= r.h {
					continue
				}
				if b := cd.spot.Behind; b >= 0 && int(r.layer[py*r.w+px]) >= b {
					continue // the table (or whatever is nearer) hides this part
				}
				col := p.Color.Mul(s.lightAt(px, py).Add(glow)).Clamp()
				if a < 1 {
					col = c.At(ox+px, oy+py).Lerp(col, a)
				}
				c.Set(ox+px, oy+py, col)
				if a >= 0.5 {
					c.MarkVivid(ox+px, oy+py, dim)
				}
			}
		}
	}
}
