// Package canvas is the game's drawing surface.
//
// The terminal is drawn with half-block characters: each character cell
// shows two square pixels stacked on top of each other, by drawing "▀" with
// the top pixel's color in front and the bottom pixel's color behind. Text
// can be placed on top in whole cells. A canvas turns itself into a plain
// string of terminal escape codes, so everything here can be tested without
// a real terminal.
package canvas

import (
	"strconv"
	"strings"

	"coffee-and-toe-beans/internal/rgb"
)

// Mode is how many colors the terminal can show.
type Mode int

const (
	// TrueColor terminals can show any color (16 million).
	TrueColor Mode = iota
	// Color256 terminals show a fixed palette; colors are matched to it.
	Color256
)

// String names the mode the way it is written in flags and settings.
func (m Mode) String() string {
	if m == Color256 {
		return "256"
	}
	return "truecolor"
}

type textCell struct {
	r      rune
	fg, bg rgb.Color
	set    bool
	cont   bool // right half of a wide character
	italic bool
}

// Canvas is W x H terminal cells, which is W x 2H pixels.
type Canvas struct {
	W, H int
	px   []rgb.Color
	text []textCell
	// vivid marks pixels (the cats) that keep their colors in 256-color
	// mode: 1 a cat, 2 a cat in a dim place (see rgb.To256At).
	vivid []uint8
}

// New makes a blank (black) canvas of w x h cells.
func New(w, h int) *Canvas {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Canvas{
		W:     w,
		H:     h,
		px:    make([]rgb.Color, w*h*2),
		text:  make([]textCell, w*h),
		vivid: make([]uint8, w*h*2),
	}
}

// PixelH is the height in pixels (two per cell).
func (c *Canvas) PixelH() int { return c.H * 2 }

func (c *Canvas) inside(x, y int) bool {
	return x >= 0 && y >= 0 && x < c.W && y < c.H*2
}

// Set paints one pixel. Pixels outside the canvas are ignored.
func (c *Canvas) Set(x, y int, col rgb.Color) {
	if c.inside(x, y) {
		c.px[y*c.W+x] = col
	}
}

// MarkVivid says the pixel belongs to a cat, which keeps its colors in
// 256-color mode; dim says the cat sits in a dim place, where its fur is
// kept a little lighter than the room. Set and Blend leave the mark; Fill
// clears it.
func (c *Canvas) MarkVivid(x, y int, dim bool) {
	if c.inside(x, y) {
		v := uint8(1)
		if dim {
			v = 2
		}
		c.vivid[y*c.W+x] = v
	}
}

// At reads one pixel (black outside the canvas).
func (c *Canvas) At(x, y int) rgb.Color {
	if c.inside(x, y) {
		return c.px[y*c.W+x]
	}
	return rgb.Black
}

// Blend mixes col into a pixel with opacity a (0..1).
func (c *Canvas) Blend(x, y int, col rgb.Color, a float64) {
	if a <= 0 || !c.inside(x, y) {
		return
	}
	if a > 1 {
		a = 1
	}
	i := y*c.W + x
	c.px[i] = c.px[i].Lerp(col, a)
}

// Fill paints every pixel and clears all text.
func (c *Canvas) Fill(col rgb.Color) {
	for i := range c.px {
		c.px[i] = col
		c.vivid[i] = 0
	}
	for i := range c.text {
		c.text[i] = textCell{}
	}
}

// FillRect paints a rectangle of pixels.
func (c *Canvas) FillRect(x, y, w, h int, col rgb.Color) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			c.Set(xx, yy, col)
		}
	}
}

// Text writes s starting at cell (cx, cy) with the given colors and returns
// how many cells it used. Text that runs off the edge is cut.
func (c *Canvas) Text(cx, cy int, s string, fg, bg rgb.Color) int {
	return c.putText(cx, cy, s, fg, bg, false)
}

// TextItalic is Text in italics (terminals without italics show it upright).
func (c *Canvas) TextItalic(cx, cy int, s string, fg, bg rgb.Color) int {
	return c.putText(cx, cy, s, fg, bg, true)
}

func (c *Canvas) putText(cx, cy int, s string, fg, bg rgb.Color, italic bool) int {
	if cy < 0 || cy >= c.H {
		return 0
	}
	x := cx
	for _, r := range s {
		w := RuneWidth(r)
		if w == 0 {
			continue
		}
		if x >= c.W {
			break
		}
		if x >= 0 {
			if w == 2 && x+1 >= c.W {
				r, w = ' ', 1 // no room for a wide character at the edge
			}
			c.text[cy*c.W+x] = textCell{r: r, fg: fg, bg: bg, set: true, italic: italic}
			if w == 2 {
				c.text[cy*c.W+x+1] = textCell{set: true, cont: true}
			}
		}
		x += w
	}
	return x - cx
}

// TextOver writes s over the picture: each cell's background is the average
// of the two pixels it covers, so the text seems to float on the scene.
func (c *Canvas) TextOver(cx, cy int, s string, fg rgb.Color) int {
	if cy < 0 || cy >= c.H {
		return 0
	}
	x := cx
	for _, r := range s {
		w := RuneWidth(r)
		if w == 0 {
			continue
		}
		if x >= c.W {
			break
		}
		if x >= 0 {
			bg := c.At(x, cy*2).Lerp(c.At(x, cy*2+1), 0.5)
			c.Text(x, cy, string(r), fg, bg)
		}
		x += w
	}
	return x - cx
}

// ClearText removes any text in one cell, showing the pixels again.
func (c *Canvas) ClearText(cx, cy int) {
	if cx >= 0 && cy >= 0 && cx < c.W && cy < c.H {
		c.text[cy*c.W+cx] = textCell{}
	}
}

// Blit copies src onto c with its top-left cell at (cx, cy).
func (c *Canvas) Blit(src *Canvas, cx, cy int) {
	for y := 0; y < src.H; y++ {
		ty := cy + y
		if ty < 0 || ty >= c.H {
			continue
		}
		for x := 0; x < src.W; x++ {
			tx := cx + x
			if tx < 0 || tx >= c.W {
				continue
			}
			c.px[(ty*2)*c.W+tx] = src.px[(y*2)*src.W+x]
			c.px[(ty*2+1)*c.W+tx] = src.px[(y*2+1)*src.W+x]
			c.text[ty*c.W+tx] = src.text[y*src.W+x]
		}
	}
}

const unset = ^uint32(0)

func key(col rgb.Color, mode Mode) uint32 {
	if mode == Color256 {
		return uint32(rgb.To256(col))
	}
	return col.Pack()
}

// pixelKey is a pixel's color code; in 256-color mode it softens gradient
// steps and keeps vivid pixels colorful (see rgb.To256At).
func (c *Canvas) pixelKey(x, y int, mode Mode) uint32 {
	i := y*c.W + x
	if mode == Color256 {
		return uint32(rgb.To256At(c.px[i], rgb.Vivid(c.vivid[i])))
	}
	return c.px[i].Pack()
}

// Lines renders the canvas as one string per row of cells, using terminal
// color codes. Each line ends by resetting the colors.
func (c *Canvas) Lines(mode Mode) []string {
	lines := make([]string, c.H)
	buf := make([]byte, 0, c.W*24)
	for cy := 0; cy < c.H; cy++ {
		buf = buf[:0]
		curFG, curBG := unset, unset
		curItalic := false
		for cx := 0; cx < c.W; cx++ {
			t := c.text[cy*c.W+cx]
			if t.cont {
				continue
			}
			var glyph rune
			fg, bg := unset, unset
			if t.set {
				glyph = t.r
				fg, bg = key(t.fg, mode), key(t.bg, mode)
			} else {
				top := c.pixelKey(cx, cy*2, mode)
				bot := c.pixelKey(cx, cy*2+1, mode)
				bg = bot
				if top == bot {
					glyph = ' ' // a space only needs the background color
				} else {
					glyph = '▀'
					fg = top
				}
			}
			if t.italic != curItalic {
				if t.italic {
					buf = append(buf, "\x1b[3m"...)
				} else {
					buf = append(buf, "\x1b[23m"...)
				}
				curItalic = t.italic
			}
			needFG := fg != unset && fg != curFG
			needBG := bg != curBG
			if needFG || needBG {
				buf = append(buf, "\x1b["...)
				if needFG {
					buf = appendColor(buf, 38, fg, mode)
					curFG = fg
				}
				if needBG {
					if needFG {
						buf = append(buf, ';')
					}
					buf = appendColor(buf, 48, bg, mode)
					curBG = bg
				}
				buf = append(buf, 'm')
			}
			buf = append(buf, string(glyph)...)
		}
		buf = append(buf, "\x1b[0m"...)
		lines[cy] = string(buf)
	}
	return lines
}

// String renders the whole canvas, rows joined by newlines (no trailing one).
func (c *Canvas) String(mode Mode) string {
	return strings.Join(c.Lines(mode), "\n")
}

func appendColor(buf []byte, layer int, k uint32, mode Mode) []byte {
	buf = strconv.AppendInt(buf, int64(layer), 10)
	if mode == Color256 {
		buf = append(buf, ";5;"...)
		return strconv.AppendInt(buf, int64(k), 10)
	}
	buf = append(buf, ";2;"...)
	buf = strconv.AppendInt(buf, int64(k>>16&0xff), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(k>>8&0xff), 10)
	buf = append(buf, ';')
	return strconv.AppendInt(buf, int64(k&0xff), 10)
}

// RuneWidth is how many terminal cells a character takes: 0 for combining
// marks, 2 for wide characters (Chinese, Japanese, Korean, most emoji),
// otherwise 1.
func RuneWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 32 || (r >= 0x7f && r < 0xa0):
		return 0
	case r >= 0x0300 && r <= 0x036f, r >= 0x200b && r <= 0x200f, r == 0xfe0f:
		return 0
	case r >= 0x1100 && r <= 0x115f,
		r >= 0x2e80 && r <= 0x303e,
		r >= 0x3041 && r <= 0x33ff,
		r >= 0x3400 && r <= 0x4dbf,
		r >= 0x4e00 && r <= 0x9fff,
		r >= 0xa000 && r <= 0xa4cf,
		r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff,
		r >= 0xfe30 && r <= 0xfe4f,
		r >= 0xff00 && r <= 0xff60,
		r >= 0xffe0 && r <= 0xffe6,
		r >= 0x1f300 && r <= 0x1f64f,
		r >= 0x1f900 && r <= 0x1f9ff,
		r >= 0x20000 && r <= 0x3fffd:
		return 2
	}
	return 1
}

// Width is the number of terminal cells s takes.
func Width(s string) int {
	n := 0
	for _, r := range s {
		n += RuneWidth(r)
	}
	return n
}
