package scene

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"strings"

	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
)

// An interior is described in a JSON file (content/interiors/*.json): its
// named colors, a list of elements painted back to front, its light
// sources, where the cup sits and where cats like to be. Each interior style
// is just another such file.

type interiorFile struct {
	Name     string            `json:"name"`
	Size     [2]int            `json:"size"`
	Colors   map[string]string `json:"colors"`
	Elements []json.RawMessage `json:"elements"`
	Lights   []lightDef        `json:"lights"`
	Cup      cupDef            `json:"cup"`
	Spots    map[string][2]int `json:"cat_spots"`
	Daylight daylightDef       `json:"daylight"`
}

type lightDef struct {
	Name     string  `json:"name"`
	At       [2]int  `json:"at"`
	Radius   float64 `json:"radius"`
	Color    string  `json:"color"`
	Strength float64 `json:"strength"`
	Flicker  float64 `json:"flicker"`
}

type cupDef struct {
	Sprite string `json:"sprite"` // the cup shown at first ("" for none)
	At     [2]int `json:"at"`     // top-left of that first cup
	Steam  [2]int `json:"steam"`  // where its steam rises
	// Stand is where every cup stands: its bottom-left pixel. Drinks come
	// in cups of different sizes, all standing on this spot.
	Stand *[2]int `json:"stand"`
}

type daylightDef struct {
	From   [2]int  `json:"from"`
	Radius float64 `json:"radius"`
	Tint   string  `json:"tint"`
}

// pixel is one pixel of the room's painted, unlit picture.
type pixel struct {
	col   rgb.Color
	mat   sprites.Material
	light int8 // which light a lamp or flame pixel belongs to, -1 if none
	glass bool // part of the window: the view shows through here
}

// light is a lamp or candle that warms the pixels around it.
type light struct {
	name     string
	x, y     float64
	radius   float64
	color    rgb.Color
	strength float64
	flicker  float64
	weight   []float64 // how much this light reaches each pixel
}

// pane is one rectangle of window glass between the bars.
type pane struct{ x, y, w, h int }

// Spot is a place in the room where a cat can be.
type Spot struct{ X, Y int }

// room is a fully painted interior, ready to be lit.
type room struct {
	name     string
	w, h     int
	px       []pixel
	lights   []*light
	daylight []float64
	dayTint  rgb.Color
	glass    struct{ x, y, w, h int }
	panes    []pane
	hasGlass bool
	cup      cupDef
	spots    map[string]Spot
}

type painter struct {
	r      *room
	colors map[string]rgb.Color
	lib    *sprites.Library
	lights map[string]int
	where  string
}

func (p *painter) color(name string) (rgb.Color, error) {
	if strings.HasPrefix(name, "#") {
		c, err := rgb.Hex(name)
		if err != nil {
			return rgb.Color{}, fmt.Errorf("%s: %v", p.where, err)
		}
		return c, nil
	}
	c, ok := p.colors[name]
	if !ok {
		return rgb.Color{}, fmt.Errorf("%s: color %q isn't in this interior's colors (or use #rrggbb)", p.where, name)
	}
	return c, nil
}

func (p *painter) colorList(names []string) ([]rgb.Color, error) {
	var out []rgb.Color
	for _, n := range names {
		c, err := p.color(n)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: needs at least one color", p.where)
	}
	return out, nil
}

func (p *painter) set(x, y int, c rgb.Color) {
	if x >= 0 && y >= 0 && x < p.r.w && y < p.r.h {
		i := y*p.r.w + x
		p.r.px[i] = pixel{col: c, light: -1}
	}
}

func (p *painter) get(x, y int) pixel {
	if x >= 0 && y >= 0 && x < p.r.w && y < p.r.h {
		return p.r.px[y*p.r.w+x]
	}
	return pixel{}
}

// loadRoom reads and paints an interior.
func loadRoom(fsys fs.FS, path string, lib *sprites.Library) (*room, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, err
	}
	var f interiorFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	w, h := f.Size[0], f.Size[1]
	if w <= 0 || h <= 0 || w > 400 || h > 400 {
		return nil, fmt.Errorf("%s: size should be [width, height] in pixels", path)
	}
	r := &room{name: f.Name, w: w, h: h, px: make([]pixel, w*h), cup: f.Cup, spots: map[string]Spot{}}
	p := &painter{r: r, colors: map[string]rgb.Color{}, lib: lib, lights: map[string]int{}, where: path}
	for name, hex := range f.Colors {
		c, err := rgb.Hex(hex)
		if err != nil {
			return nil, fmt.Errorf("%s: colors.%s: %v", path, name, err)
		}
		p.colors[name] = c
	}
	for i := range r.px {
		r.px[i].light = -1
	}

	// Lights first, so lamp sprites can refer to them by name.
	for i, ld := range f.Lights {
		p.where = fmt.Sprintf("%s light %q", path, ld.Name)
		c, err := p.color(ld.Color)
		if err != nil {
			return nil, err
		}
		if ld.Radius <= 0 {
			return nil, fmt.Errorf("%s: radius should be above 0", p.where)
		}
		if _, dup := p.lights[ld.Name]; dup || ld.Name == "" {
			return nil, fmt.Errorf("%s: every light needs its own name", p.where)
		}
		p.lights[ld.Name] = i
		l := &light{name: ld.Name, x: float64(ld.At[0]) + 0.5, y: float64(ld.At[1]) + 0.5,
			radius: ld.Radius, color: c, strength: ld.Strength, flicker: ld.Flicker,
			weight: make([]float64, w*h)}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dx, dy := (float64(x)+0.5-l.x)/l.radius, (float64(y)+0.5-l.y)/l.radius
				d2 := dx*dx + dy*dy
				if d2 < 1 {
					f := 1 - d2
					l.weight[y*w+x] = f * f
				}
			}
		}
		r.lights = append(r.lights, l)
	}

	for i, raw := range f.Elements {
		var head struct {
			Type string `json:"type"`
			Note string `json:"note"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			return nil, fmt.Errorf("%s element %d: %v", path, i+1, err)
		}
		p.where = fmt.Sprintf("%s element %d (%s)", path, i+1, head.Type)
		if head.Note != "" {
			p.where = fmt.Sprintf("%s element %d (%s: %s)", path, i+1, head.Type, head.Note)
		}
		paint, ok := painters[head.Type]
		if !ok {
			return nil, fmt.Errorf("%s: unknown element type %q", p.where, head.Type)
		}
		if err := paint(p, raw); err != nil {
			return nil, err
		}
	}

	// Daylight falls in from the window and fades across the room.
	p.where = path + " daylight"
	r.daylight = make([]float64, w*h)
	tint := rgb.White
	if f.Daylight.Tint != "" {
		if tint, err = p.color(f.Daylight.Tint); err != nil {
			return nil, err
		}
	}
	r.dayTint = tint
	rad := f.Daylight.Radius
	if rad <= 0 {
		rad = float64(w)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := math.Hypot(float64(x-f.Daylight.From[0]), float64(y-f.Daylight.From[1]))
			r.daylight[y*w+x] = math.Max(0, 1-d/rad)
		}
	}

	for name, at := range f.Spots {
		r.spots[name] = Spot{X: at[0], Y: at[1]}
	}
	if f.Cup.Sprite != "" {
		if _, ok := lib.Get(f.Cup.Sprite); !ok {
			return nil, fmt.Errorf("%s: cup sprite %q doesn't exist", path, f.Cup.Sprite)
		}
	}
	return r, nil
}
