package scene

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"coffee-and-toe-beans/internal/noise"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
)

// painters maps each element "type" in an interior file to the code that
// paints it. New interior styles can reuse all of these.
var painters = map[string]func(p *painter, raw json.RawMessage) error{
	"fill":      paintFill,
	"shade":     paintShade,
	"panel":     paintPanel,
	"floor":     paintFloor,
	"bookshelf": paintBookshelf,
	"window":    paintWindow,
	"curtain":   paintCurtain,
	"table":     paintTable,
	"sprite":    paintSprite,
}

func decode(p *painter, raw json.RawMessage, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %v", p.where, err)
	}
	return nil
}

// fill: a solid rectangle, optionally with a faint grain.
func paintFill(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect    [4]int  `json:"rect"`
		Color   string  `json:"color"`
		Texture float64 `json:"texture"`
		Seed    int     `json:"seed"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	c, err := p.color(e.Color)
	if err != nil {
		return err
	}
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			col := c
			if e.Texture > 0 {
				n := noise.Rand(x, y, e.Seed+17)*2 - 1
				col = c.Scale(1 + n*e.Texture)
			}
			p.set(x, y, col)
		}
	}
	return nil
}

// shade: darkens what's already painted, for soft shadows under things.
func paintShade(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect   [4]int  `json:"rect"`
		Amount float64 `json:"amount"`
		Fade   string  `json:"fade"` // "down", "up", "left", "right" or ""
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if x < 0 || y < 0 || x >= p.r.w || y >= p.r.h {
				continue
			}
			a := e.Amount
			switch e.Fade {
			case "down":
				a *= 1 - float64(y-y0)/float64(h)
			case "up":
				a *= float64(y-y0+1) / float64(h)
			case "right":
				a *= 1 - float64(x-x0)/float64(w)
			case "left":
				a *= float64(x-x0+1) / float64(w)
			}
			i := y*p.r.w + x
			if !p.r.px[i].glass {
				p.r.px[i].col = p.r.px[i].col.Scale(1 - a)
			}
		}
	}
	return nil
}

// panel: a raised molding frame on the wall (Haussmann boiserie).
func paintPanel(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect   [4]int `json:"rect"`
		Light  string `json:"light"`
		Shadow string `json:"shadow"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	lt, err := p.color(e.Light)
	if err != nil {
		return err
	}
	sh, err := p.color(e.Shadow)
	if err != nil {
		return err
	}
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	x1, y1 := x0+w-1, y0+h-1
	// Outer edge catches the light on top and left...
	for x := x0; x <= x1; x++ {
		p.set(x, y0, lt)
		p.set(x, y1, sh)
	}
	for y := y0; y <= y1; y++ {
		p.set(x0, y, lt)
		p.set(x1, y, sh)
	}
	// ...the inner edge is the other way round, so the molding looks raised.
	for x := x0 + 1; x <= x1-1; x++ {
		p.set(x, y0+1, sh)
		p.set(x, y1-1, lt)
	}
	for y := y0 + 1; y <= y1-1; y++ {
		p.set(x0+1, y, sh)
		p.set(x1-1, y, lt)
	}
	return nil
}

// floor: wooden planks laid in a chevron (point de Hongrie) pattern.
func paintFloor(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect   [4]int   `json:"rect"`
		Colors []string `json:"colors"`
		Plank  int      `json:"plank"`
		Seed   int      `json:"seed"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	cols, err := p.colorList(e.Colors)
	if err != nil {
		return err
	}
	if e.Plank < 2 {
		e.Plank = 4
	}
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			col := (x - x0) / e.Plank
			var along int
			if col%2 == 0 {
				along = (y - (x - x0)) // planks slanting one way...
			} else {
				along = (y + (x - x0)) // ...and the other in the next column
			}
			plank := floorDiv(along, 2)
			p.set(x, y, cols[int(noise.Hash(col, plank, e.Seed)%uint32(len(cols)))])
		}
	}
	return nil
}

// bookshelf: a wooden case whose shelves fill with books (different every
// seed, so no two shelves look alike).
func paintBookshelf(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect    [4]int   `json:"rect"`
		Wood    []string `json:"wood"` // dark, mid, light
		Back    string   `json:"back"`
		Shelves []int    `json:"shelves"`
		Books   []string `json:"books"`
		Gilt    string   `json:"gilt"`
		Seed    int      `json:"seed"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	wood, err := p.colorList(e.Wood)
	if err != nil {
		return err
	}
	if len(wood) != 3 {
		return fmt.Errorf("%s: wood should list 3 colors: dark, mid, light", p.where)
	}
	back, err := p.color(e.Back)
	if err != nil {
		return err
	}
	books, err := p.colorList(e.Books)
	if err != nil {
		return err
	}
	gilt := rgb.MustHex("#b8924a")
	if e.Gilt != "" {
		if gilt, err = p.color(e.Gilt); err != nil {
			return err
		}
	}
	dark, mid, lightW := wood[0], wood[1], wood[2]
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	x1, y1 := x0+w-1, y0+h-1

	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			p.set(x, y, back)
		}
	}
	// Uprights.
	for y := y0; y <= y1; y++ {
		p.set(x0, y, mid)
		p.set(x1, y, dark)
	}
	// Crown on top (overhangs by one pixel) and a plinth at the bottom.
	for x := x0 - 1; x <= x1+1; x++ {
		p.set(x, y0, lightW)
		p.set(x, y0+1, mid)
	}
	for x := x0; x <= x1; x++ {
		p.set(x, y1, dark)
		p.set(x, y1-1, mid)
	}

	boards := append([]int{}, e.Shelves...)
	rng := uint32(e.Seed)*2654435761 + 1
	next := func(n int) int {
		rng ^= rng << 13
		rng ^= rng >> 17
		rng ^= rng << 5
		return int(rng % uint32(n))
	}
	top := y0 + 2
	for i := 0; i <= len(boards); i++ {
		bottom := y1 - 2 // last compartment sits on the plinth
		if i < len(boards) {
			bottom = y0 + boards[i] - 1
		}
		fillShelf(p, x0+1, top, x1-1, bottom, books, gilt, next)
		if i < len(boards) {
			by := y0 + boards[i]
			for x := x0 + 1; x <= x1-1; x++ {
				p.set(x, by, lightW)
				p.set(x, by+1, dark) // shadow under the board
			}
			top = by + 2
		}
	}
	return nil
}

func fillShelf(p *painter, x0, top, x1, bottom int, books []rgb.Color, gilt rgb.Color, next func(int) int) {
	space := bottom - top + 1
	if space < 3 {
		return
	}
	x := x0 + next(2)
	for x <= x1 {
		roll := next(100)
		switch {
		case roll < 8: // a gap
			x += 1 + next(2)
		case roll < 16 && x+3 <= x1: // a little stack lying flat
			wd := 3 + next(2)
			n := 2 + next(2)
			for k := 0; k < n; k++ {
				c := books[next(len(books))]
				for xx := x; xx < x+wd && xx <= x1; xx++ {
					p.set(xx, bottom-k, c)
				}
				p.set(x, bottom-k, c.Scale(1.12))
			}
			x += wd + 1
		default: // a standing book
			bw := 1 + next(2)
			bh := space - next(3)
			if bh < 3 {
				bh = 3
			}
			c := books[next(len(books))].Scale(0.9 + float64(next(20))/100)
			for xx := x; xx < x+bw && xx <= x1; xx++ {
				for yy := bottom - bh + 1; yy <= bottom; yy++ {
					p.set(xx, yy, c)
				}
				p.set(xx, bottom-bh+1, c.Scale(1.15)) // top edge catches light
			}
			if next(3) == 0 && bh >= 5 {
				band := bottom - bh + 2 + next(2)
				for xx := x; xx < x+bw && xx <= x1; xx++ {
					p.set(xx, band, gilt) // gilt lettering on the spine
				}
			}
			x += bw
		}
	}
}

// window: a tall French window with painted wooden frame, a center mullion
// with its brass espagnolette, and glazing bars. The glass shows the view.
func paintWindow(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect     [4]int `json:"rect"`
		Frame    string `json:"frame"`
		Light    string `json:"frame_light"`
		Shadow   string `json:"frame_shadow"`
		Border   int    `json:"border"`
		Mullion  int    `json:"mullion"`
		Transoms []int  `json:"transoms"`
		Handle   string `json:"handle"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	if p.r.hasGlass {
		return fmt.Errorf("%s: an interior can only have one window for now", p.where)
	}
	frame, err := p.color(e.Frame)
	if err != nil {
		return err
	}
	lt, err := p.color(e.Light)
	if err != nil {
		return err
	}
	sh, err := p.color(e.Shadow)
	if err != nil {
		return err
	}
	if e.Border < 1 {
		e.Border = 2
	}
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	gx, gy, gw, gh := x0+e.Border, y0+e.Border, w-2*e.Border, h-2*e.Border
	if gw < 4 || gh < 4 {
		return fmt.Errorf("%s: window is too small for its frame", p.where)
	}
	p.r.glass.x, p.r.glass.y, p.r.glass.w, p.r.glass.h = gx, gy, gw, gh
	p.r.hasGlass = true

	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			if x >= gx && x < gx+gw && y >= gy && y < gy+gh {
				if x >= 0 && y >= 0 && x < p.r.w && y < p.r.h {
					p.r.px[y*p.r.w+x] = pixel{glass: true, light: -1}
				}
				continue
			}
			c := frame
			switch {
			case y == y0 || x == x0:
				c = lt
			case y == y0+h-1 || x == x0+w-1:
				c = sh
			case y == gy+gh && x >= gx-1 && x <= gx+gw:
				c = lt // the bottom rail's top edge catches the light
			}
			p.set(x, y, c)
		}
	}

	// Center mullion.
	mx := gx + (gw-e.Mullion)/2
	for y := gy; y < gy+gh; y++ {
		for k := 0; k < e.Mullion; k++ {
			c := frame
			if k == 0 {
				c = lt
			} else if k == e.Mullion-1 {
				c = sh
			}
			p.set(mx+k, y, c)
		}
	}
	// Glazing bars.
	sort.Ints(e.Transoms)
	tops := []int{gy}
	var bars []int
	for _, t := range e.Transoms {
		ty := gy + t
		if ty <= gy || ty >= gy+gh-1 {
			return fmt.Errorf("%s: transom %d is outside the glass", p.where, t)
		}
		for x := gx; x < gx+gw; x++ {
			p.set(x, ty, frame)
		}
		bars = append(bars, ty)
		tops = append(tops, ty+1)
	}
	// The brass espagnolette rod and its handle, on the mullion.
	if e.Handle != "" && e.Mullion >= 1 {
		brass, err := p.color(e.Handle)
		if err != nil {
			return err
		}
		rx := mx + e.Mullion/2
		for y := gy + 1; y < gy+gh-1; y++ {
			p.set(rx, y, brass.Scale(0.85))
		}
		hy := gy + gh/2
		p.set(rx, hy-1, brass.Scale(1.15))
		p.set(rx, hy, brass.Scale(1.15))
		p.set(rx-1, hy, brass)
	}
	// Remember the panes for raindrops and snow.
	for i, top := range tops {
		bottom := gy + gh - 1
		if i < len(bars) {
			bottom = bars[i] - 1
		}
		ph := bottom - top + 1
		p.r.panes = append(p.r.panes,
			pane{x: gx, y: top, w: mx - gx, h: ph},
			pane{x: mx + e.Mullion, y: top, w: gx + gw - mx - e.Mullion, h: ph})
	}
	return nil
}

// curtain: a linen curtain gathered by a tie-back, like an hourglass.
func paintCurtain(p *painter, raw json.RawMessage) error {
	var e struct {
		Rect   [4]int   `json:"rect"`
		Side   string   `json:"side"`
		Tie    int      `json:"tie"`
		Colors []string `json:"colors"` // light, mid, dark
		TieCol string   `json:"tie_color"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	cols, err := p.colorList(e.Colors)
	if err != nil {
		return err
	}
	if len(cols) != 3 {
		return fmt.Errorf("%s: colors should list 3 colors: light, mid, dark", p.where)
	}
	tieCol, err := p.color(e.TieCol)
	if err != nil {
		return err
	}
	if e.Side != "left" && e.Side != "right" {
		return fmt.Errorf("%s: side should be \"left\" or \"right\"", p.where)
	}
	x0, y0, w, h := e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3]
	tieY := y0 + e.Tie
	waist := math.Max(2, float64(w)*0.45)
	for y := y0; y < y0+h; y++ {
		var width float64
		if y <= tieY {
			t := float64(y-y0) / math.Max(1, float64(tieY-y0))
			width = float64(w) - (float64(w)-waist)*t*t
		} else {
			t := float64(y-tieY) / math.Max(1, float64(y0+h-1-tieY))
			width = waist + (float64(w)*0.9-waist)*math.Sqrt(t)
		}
		n := int(math.Round(width))
		for k := 0; k < n; k++ {
			u := (float64(k) + 0.5) / width
			fold := math.Sin(u * float64(w) * 1.6)
			c := cols[1]
			if fold > 0.45 {
				c = cols[0]
			} else if fold < -0.45 {
				c = cols[2]
			}
			if k == n-1 {
				c = cols[2] // the inner edge
			}
			if y == y0+h-1 {
				c = c.Scale(0.9) // hem
			}
			x := x0 + k
			if e.Side == "right" {
				x = x0 + w - 1 - k
			}
			p.set(x, y, c)
		}
		if y == tieY || y == tieY+1 {
			for k := 0; k < n; k++ {
				x := x0 + k
				if e.Side == "right" {
					x = x0 + w - 1 - k
				}
				c := tieCol
				if y == tieY+1 {
					c = tieCol.Scale(0.8)
				}
				p.set(x, y, c)
			}
		}
	}
	return nil
}

// table: the round marble top of your bistro table, with a brass rim.
func paintTable(p *painter, raw json.RawMessage) error {
	var e struct {
		Center [2]float64 `json:"center"`
		Radius [2]float64 `json:"radius"`
		Top    string     `json:"top"`
		Vein   string     `json:"vein"`
		Shade  string     `json:"shade"`
		Rim    string     `json:"rim"`
		Seed   int        `json:"seed"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	top, err := p.color(e.Top)
	if err != nil {
		return err
	}
	vein, err := p.color(e.Vein)
	if err != nil {
		return err
	}
	shade, err := p.color(e.Shade)
	if err != nil {
		return err
	}
	rim, err := p.color(e.Rim)
	if err != nil {
		return err
	}
	inside := func(x, y int) bool {
		dx := (float64(x) + 0.5 - e.Center[0]) / e.Radius[0]
		dy := (float64(y) + 0.5 - e.Center[1]) / e.Radius[1]
		return dx*dx+dy*dy <= 1
	}
	for y := 0; y < p.r.h; y++ {
		for x := 0; x < p.r.w; x++ {
			if !inside(x, y) {
				continue
			}
			if !inside(x, y-1) || !inside(x-1, y) || !inside(x+1, y) {
				p.set(x, y, rim)
				continue
			}
			c := top
			// Veins: thin wandering lines where the noise crosses a level.
			v := noise.Fractal(float64(x)*0.09, float64(y)*0.22, e.Seed, 3)
			if math.Abs(v-0.5) < 0.008 {
				c = top.Lerp(vein, 0.7)
			}
			// The far edge of the top is a touch darker (it tilts away).
			if !inside(x, y-2) {
				c = c.Lerp(shade, 0.6)
			}
			p.set(x, y, c)
		}
	}
	return nil
}

// sprite: a piece of pixel art from content/sprites.
func paintSprite(p *painter, raw json.RawMessage) error {
	var e struct {
		Sprite string `json:"sprite"`
		At     [2]int `json:"at"`
		Light  string `json:"light"`
		Flip   bool   `json:"flip"`
	}
	if err := decode(p, raw, &e); err != nil {
		return err
	}
	s, ok := p.lib.Get(e.Sprite)
	if !ok {
		return fmt.Errorf("%s: sprite %q doesn't exist in content/sprites", p.where, e.Sprite)
	}
	lightIdx := -1
	if e.Light != "" {
		i, ok := p.lights[e.Light]
		if !ok {
			return fmt.Errorf("%s: light %q isn't in this interior's lights", p.where, e.Light)
		}
		lightIdx = i
	}
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			sx := x
			if e.Flip {
				sx = s.W - 1 - x
			}
			px := s.At(sx, y)
			if !px.Opaque {
				continue
			}
			tx, ty := e.At[0]+x, e.At[1]+y
			if tx < 0 || ty < 0 || tx >= p.r.w || ty >= p.r.h {
				continue
			}
			np := pixel{col: px.Color, mat: px.Mat, light: -1}
			if px.Mat == sprites.Lamp || px.Mat == sprites.Flame {
				if lightIdx < 0 {
					return fmt.Errorf("%s: sprite %q has glowing pixels, so it needs a \"light\"", p.where, e.Sprite)
				}
				np.light = int8(lightIdx)
			}
			p.r.px[ty*p.r.w+tx] = np
		}
	}
	return nil
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func mod(a, b int) int {
	m := a % b
	if m < 0 {
		m += b
	}
	return m
}
