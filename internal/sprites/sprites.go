// Package sprites reads the game's pixel art from plain text files.
//
// A sprite is a little grid of letters, and a palette says which color each
// letter stands for. See content/sprites/README.md for the file format; in
// short:
//
//	palette brass
//	  b #c9a24a  brass
//	  B #e8cc7a  brass highlight
//
//	sprite lamp
//	  use brass
//	  s #efe0c0 @lamp  lampshade (glows when the lamps are on)
//	grid
//	..sss..
//	.sssss.
//	...b...
//	..BbB..
//	end
//
// "." and spaces are transparent. A word starting with "@" after a color
// gives the pixel a special behavior (see Material).
package sprites

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"coffee-and-toe-beans/internal/rgb"
)

// Material tells the renderer how a pixel reacts to light and weather.
type Material uint8

const (
	// Plain surfaces are lit by the room (or the sky, outdoors).
	Plain Material = iota
	// Lamp pixels glow when the lamps are on (lampshades, bulbs).
	Lamp
	// Flame pixels always glow and flicker (candles).
	Flame
	// Roof pixels collect snow.
	Roof
	// Window pixels are distant windows that light up at night.
	Window
	// Eiffel pixels are the tower, golden when its lights are on.
	Eiffel
	// Beacon is the tower's tip light, which sweeps around at night.
	Beacon
	// Drink pixels are the inside of a cup: filled with the drink's colors
	// while there's some left, emptying from the top.
	Drink
)

var materialNames = map[string]Material{
	"lamp":   Lamp,
	"flame":  Flame,
	"roof":   Roof,
	"window": Window,
	"eiffel": Eiffel,
	"beacon": Beacon,
	"drink":  Drink,
}

// Pixel is one square of a sprite.
type Pixel struct {
	Color  rgb.Color
	Mat    Material
	Opaque bool
}

// Sprite is a rectangle of pixels.
type Sprite struct {
	Name string
	W, H int
	Px   []Pixel
}

// At returns the pixel at (x, y); outside the sprite is transparent.
func (s *Sprite) At(x, y int) Pixel {
	if x < 0 || y < 0 || x >= s.W || y >= s.H {
		return Pixel{}
	}
	return s.Px[y*s.W+x]
}

// Library holds every sprite by name.
type Library struct {
	sprites map[string]*Sprite
}

// Get finds a sprite by name.
func (l *Library) Get(name string) (*Sprite, bool) {
	s, ok := l.sprites[name]
	return s, ok
}

// Names lists all sprite names, sorted.
func (l *Library) Names() []string {
	var out []string
	for n := range l.sprites {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

type entry struct {
	px   Pixel
	line int
}

type rawSprite struct {
	name    string
	file    string
	line    int
	uses    []string
	entries map[rune]entry
	grid    []string
}

type parser struct {
	palettes map[string]map[rune]entry
	sprites  []*rawSprite
}

// Load reads every .txt file in dir as sprite definitions.
func Load(fsys fs.FS, dir string) (*Library, error) {
	files, err := fs.Glob(fsys, path.Join(dir, "*.txt"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	p := &parser{palettes: map[string]map[rune]entry{}}
	for _, f := range files {
		r, err := fsys.Open(f)
		if err != nil {
			return nil, err
		}
		err = p.parse(f, r)
		r.Close()
		if err != nil {
			return nil, err
		}
	}
	return p.resolve()
}

// Parse reads sprite definitions from a single source (handy for tests).
func Parse(name string, r io.Reader) (*Library, error) {
	p := &parser{palettes: map[string]map[rune]entry{}}
	if err := p.parse(name, r); err != nil {
		return nil, err
	}
	return p.resolve()
}

func (p *parser) parse(file string, r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	var curPalette map[rune]entry
	var cur *rawSprite
	inGrid := false

	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s line %d: %s", file, lineNo, fmt.Sprintf(format, args...))
	}

	for sc.Scan() {
		lineNo++
		raw := strings.TrimRight(sc.Text(), " \t\r")
		if inGrid {
			if strings.TrimSpace(raw) == "end" {
				inGrid = false
				cur = nil
				continue
			}
			cur.grid = append(cur.grid, raw)
			continue
		}
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "palette":
			if len(fields) != 2 {
				return fail("write it as: palette NAME")
			}
			if _, dup := p.palettes[fields[1]]; dup {
				return fail("palette %q is defined twice", fields[1])
			}
			curPalette = map[rune]entry{}
			p.palettes[fields[1]] = curPalette
			cur = nil
			continue
		case "sprite":
			if len(fields) != 2 {
				return fail("write it as: sprite NAME")
			}
			cur = &rawSprite{name: fields[1], file: file, line: lineNo, entries: map[rune]entry{}}
			p.sprites = append(p.sprites, cur)
			curPalette = nil
			continue
		case "use":
			if cur == nil {
				return fail("\"use\" must come inside a sprite, before its grid")
			}
			cur.uses = append(cur.uses, fields[1:]...)
			continue
		case "grid":
			if cur == nil {
				return fail("\"grid\" must come after \"sprite NAME\"")
			}
			inGrid = true
			continue
		case "end":
			return fail("\"end\" without a grid")
		}

		// Otherwise it's a color entry: KEY COLOR [@material] [note...]
		target := curPalette
		if cur != nil {
			target = cur.entries
		}
		if target == nil {
			return fail("I don't know what %q belongs to (it should be under a palette or sprite)", line)
		}
		keyRunes := []rune(fields[0])
		if len(keyRunes) != 1 {
			return fail("color keys must be a single character, got %q", fields[0])
		}
		if len(fields) < 2 {
			return fail("key %q needs a color like #aabbcc", fields[0])
		}
		var px Pixel
		if fields[1] != "none" {
			c, err := rgb.Hex(fields[1])
			if err != nil {
				return fail("%v", err)
			}
			px = Pixel{Color: c, Opaque: true}
		}
		if len(fields) >= 3 && strings.HasPrefix(fields[2], "@") {
			m, ok := materialNames[strings.TrimPrefix(fields[2], "@")]
			if !ok {
				return fail("unknown material %s (known: @lamp @flame @roof @window @eiffel @beacon @drink)", fields[2])
			}
			px.Mat = m
		}
		target[keyRunes[0]] = entry{px: px, line: lineNo}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if inGrid {
		return fmt.Errorf("%s: sprite %q has a grid with no \"end\"", file, cur.name)
	}
	return nil
}

func (p *parser) resolve() (*Library, error) {
	lib := &Library{sprites: map[string]*Sprite{}}
	for _, rs := range p.sprites {
		where := fmt.Sprintf("%s line %d (sprite %s)", rs.file, rs.line, rs.name)
		if _, dup := lib.sprites[rs.name]; dup {
			return nil, fmt.Errorf("%s: a sprite with this name already exists", where)
		}
		keys := map[rune]Pixel{'.': {}, ' ': {}}
		for _, u := range rs.uses {
			pal, ok := p.palettes[u]
			if !ok {
				return nil, fmt.Errorf("%s: uses palette %q, which doesn't exist", where, u)
			}
			for k, e := range pal {
				keys[k] = e.px
			}
		}
		for k, e := range rs.entries {
			keys[k] = e.px
		}
		if len(rs.grid) == 0 {
			return nil, fmt.Errorf("%s: has no grid", where)
		}
		w := 0
		for _, row := range rs.grid {
			if n := len([]rune(row)); n > w {
				w = n
			}
		}
		s := &Sprite{Name: rs.name, W: w, H: len(rs.grid), Px: make([]Pixel, w*len(rs.grid))}
		for y, row := range rs.grid {
			for x, r := range []rune(row) {
				px, ok := keys[r]
				if !ok {
					return nil, fmt.Errorf("%s: grid row %d uses %q, which isn't in its palette", where, y+1, string(r))
				}
				s.Px[y*w+x] = px
			}
		}
		lib.sprites[rs.name] = s
	}
	return lib, nil
}
