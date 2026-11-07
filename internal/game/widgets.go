package game

import (
	"math"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/text"
	"coffee-and-toe-beans/internal/timeofday"
)

// The dialogue box sits low on the left, like a visual novel, leaving your
// cup on the right in view.
const (
	talkTextW = 46 // letters per line
	talkLines = 3  // lines per page
	talkLeft  = 2  // cells from the left edge of the café
)

// Menu icons are up to iconW x iconH cells (two pixels per cell, upright);
// keep them a pixel shorter than iconH*2 so neighbors don't touch.
const (
	iconW = 8
	iconH = 3
)

var (
	boxBG     = rgb.MustHex("#211a16")
	boxBorder = rgb.MustHex("#7d6c58")
	boxInk    = rgb.MustHex("#efe4d2")
	boxDim    = rgb.MustHex("#a39582")
	boxFaint  = rgb.MustHex("#5e5246")
	highlight = rgb.MustHex("#e8c47a")
)

// panel is a soft rounded box with a thin border, drawn over the scene.
// alpha fades it in and out: 1 solid, 0 invisible.
type panel struct {
	c     *canvas.Canvas
	x, y  int
	w, h  int
	alpha float64
}

func (p panel) bgAt(cx, cy int) rgb.Color {
	behind := p.c.At(cx, cy*2).Lerp(p.c.At(cx, cy*2+1), 0.5)
	return behind.Lerp(boxBG, p.alpha)
}

func (p panel) draw() {
	for row := 0; row < p.h; row++ {
		for col := 0; col < p.w; col++ {
			r := ' '
			switch {
			case row == 0 && col == 0:
				r = '╭'
			case row == 0 && col == p.w-1:
				r = '╮'
			case row == p.h-1 && col == 0:
				r = '╰'
			case row == p.h-1 && col == p.w-1:
				r = '╯'
			case row == 0 || row == p.h-1:
				r = '─'
			case col == 0 || col == p.w-1:
				r = '│'
			}
			cx, cy := p.x+col, p.y+row
			bg := p.bgAt(cx, cy)
			p.c.Text(cx, cy, string(r), bg.Lerp(boxBorder, p.alpha), bg)
		}
	}
}

// write puts words inside the panel at (col, row), counted from its corner.
func (p panel) write(col, row int, s string, ink rgb.Color) {
	p.writeStyled(col, row, s, ink, false)
}

func (p panel) writeStyled(col, row int, s string, ink rgb.Color, italic bool) {
	x := p.x + col
	cy := p.y + row
	for _, r := range s {
		w := canvas.RuneWidth(r)
		if w == 0 {
			continue
		}
		bg := p.bgAt(x, cy)
		fg := bg.Lerp(ink, p.alpha)
		if italic {
			p.c.TextItalic(x, cy, string(r), fg, bg)
		} else {
			p.c.Text(x, cy, string(r), fg, bg)
		}
		x += w
	}
}

// blink is true for the "on" part of a slow blink (always on with reduced
// motion).
func (g *Game) blink() bool {
	if g.cfg.Reduced {
		return true
	}
	return math.Mod(g.anim, 1.1) < 0.75
}

// --- The dialogue box ---------------------------------------------------------

func (g *Game) drawTalk(c *canvas.Canvas, ox, oy int, b *talkBox) {
	if b.Alpha() <= 0 {
		return
	}
	lines := b.MaxPageLines()
	choices := b.Choices()
	h := 2 + lines
	if len(choices) > 0 {
		h += 1 + len(choices)
	}
	w := talkTextW + 4
	p := panel{c: c, x: ox + talkLeft, y: oy + FrameH - 1 - h, w: w, h: h, alpha: b.Alpha()}
	p.draw()
	if s := b.Speaker(); s != "" {
		p.write(2, 0, " "+s+" ", highlight)
	}
	for i, l := range b.Lines() {
		p.write(2, 1+i, l, boxInk)
	}
	if b.MoreToCome() && g.blink() {
		p.write(w-3, h-2, "▼", highlight)
	}
	if b.ShowingChoices() {
		for i, ch := range choices {
			row := 2 + len(b.PageLines()) + i // right under the question
			if i == b.Selected() {
				p.write(2, row, "▸", highlight)
				p.write(4, row, ch.Text, boxInk)
			} else {
				p.write(4, row, ch.Text, boxDim)
			}
		}
	}
}

// --- The pause menu -----------------------------------------------------------

func (g *Game) drawPause(c *canvas.Canvas, ox, oy int) {
	m := g.pause
	title := g.ui.MenuTitle
	inner := canvas.Width(title)
	for _, a := range m.items {
		if w := canvas.Width(g.label(a)) + 2; w > inner {
			inner = w
		}
	}
	top := 1 // first row of items
	if title != "" {
		top = 3
	}
	w := inner + 6
	h := top + len(m.items) + 1
	// In the middle of the café, clear of the dialogue box low on the left.
	p := panel{c: c, x: ox + (FrameW-w)/2, y: oy + (FrameH-h)/2, w: w, h: h, alpha: 1}
	p.draw()
	if title != "" {
		p.write(3, 1, title, boxInk)
	}
	for i, a := range m.items {
		if i == m.sel {
			p.write(3, top+i, "▸", highlight)
			p.write(5, top+i, g.label(a), boxInk)
		} else {
			p.write(5, top+i, g.label(a), boxDim)
		}
	}
}

// --- The drinks menu ----------------------------------------------------------

func (g *Game) drawDrinks(c *canvas.Canvas, ox, oy int) {
	m := g.menu
	list := g.drinks.Drinks
	now := g.Now()
	const textCol = 4 + iconW + 2 // where names start inside the panel
	inner := canvas.Width(g.ui.DrinksTitle)
	if hw := canvas.Width(g.ui.DrinksHint); hw > inner {
		inner = hw
	}
	for _, d := range list {
		nw := canvas.Width(d.Name) + 3 + canvas.Width(g.laterNote(d.From))
		if tw := canvas.Width(d.Tagline); tw > nw {
			nw = tw
		}
		if nw+textCol-2 > inner {
			inner = nw + textCol - 2
		}
	}
	w := inner + 4
	h := 4 + iconH*len(list) + 1
	if g.ui.DrinksHint == "" {
		h--
	}
	p := panel{c: c, x: ox + (FrameW-w)/2, y: oy + (FrameH-h)/2, w: w, h: h, alpha: 1}
	p.draw()
	p.writeStyled((w-canvas.Width(g.ui.DrinksTitle))/2, 1, g.ui.DrinksTitle, softInk, true)
	for i, d := range list {
		row := 3 + i*iconH
		served := d.Available(now)
		name, tag := boxDim, boxFaint
		if served {
			name, tag = boxInk, boxDim
		}
		if i == m.sel {
			p.write(2, row+1, "▸", highlight)
			name = highlight
		}
		p.write(textCol, row, d.Name, name)
		if !served {
			note := g.laterNote(d.From)
			p.writeStyled(textCol+canvas.Width(d.Name)+3, row, note, boxFaint, true)
		}
		p.write(textCol, row+1, d.Tagline, tag)
		g.drawIcon(p, 4, row, d.Icon, served)
	}
	if g.ui.DrinksHint != "" {
		p.write((w-canvas.Width(g.ui.DrinksHint))/2, h-2, g.ui.DrinksHint, boxFaint)
	}
}

// laterNote is "from 17:00" for a drink served later in the day.
func (g *Game) laterNote(from string) string {
	if from == "" {
		return ""
	}
	if m, err := timeofday.ParseClock(from); err == nil {
		from = clockText(m)
	}
	return text.Fill(g.ui.DrinksLater, map[string]string{"time": from})
}

func clockText(minutes float64) string {
	m := int(minutes)
	return string([]byte{byte('0' + m/600), byte('0' + m/60%10), ':', byte('0' + m%60/10), byte('0' + m%10)})
}

// drawIcon paints a drink's little picture into the panel as pixels,
// centered in its iconW x iconH cells. Drinks not served right now are
// shown faded.
func (g *Game) drawIcon(p panel, col, row int, name string, served bool) {
	s, ok := g.lib.Get(name)
	if !ok {
		return
	}
	x0, y0 := p.x+col, p.y+row
	for cy := y0; cy < y0+iconH; cy++ {
		for cx := x0; cx < x0+iconW; cx++ {
			p.c.ClearText(cx, cy)
			p.c.Set(cx, cy*2, boxBG)
			p.c.Set(cx, cy*2+1, boxBG)
		}
	}
	dx := (iconW - s.W) / 2
	dy := (iconH*2 - s.H) / 2
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			px := s.At(x, y)
			if !px.Opaque {
				continue
			}
			col := px.Color
			if !served {
				col = col.Desaturate(0.7).Lerp(boxBG, 0.55)
			}
			p.c.Set(x0+dx+x, y0*2+dy+y, col)
		}
	}
}
