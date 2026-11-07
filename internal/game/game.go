// Package game is the café's heart: which screen you're on, what keys do,
// how time passes, and how a whole frame is put together. It uses only the
// Go standard library, so it can be tested anywhere; the thin internal/ui
// package connects it to the terminal.
package game

import (
	"fmt"
	"io/fs"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/drinks"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/save"
	"coffee-and-toe-beans/internal/scene"
	"coffee-and-toe-beans/internal/sprites"
	"coffee-and-toe-beans/internal/text"
	"coffee-and-toe-beans/internal/timeofday"
	"coffee-and-toe-beans/internal/weather"
)

// The café is drawn at this size, in terminal cells (104 x 84 pixels).
const (
	FrameW = 104
	FrameH = 42
)

// titleRow is the row of the title card's words, a little above the middle.
const titleRow = FrameH*7/16 + 1

// Key is a player input, already translated from the terminal's key codes.
type Key int

// The keys the game understands. Only arrows, Enter/Space and Esc are needed
// to play; Ctrl+C is the usual "leave right now" for terminal programs.
const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyConfirm // Enter or Space
	KeyBack    // Esc
	KeyQuit    // Ctrl+C
)

// Config is how the game was started.
type Config struct {
	Interior  string      // interior style, e.g. "french"
	Start     time.Time   // café time to start at (zero: the real clock)
	Fast      bool        // run all timers FastSpeed times faster
	Weather   string      // keep this weather (for previews); "" for ambient
	ColorMode canvas.Mode // true color or 256 colors
	Reduced   bool        // reduced motion
	Quiet     bool        // quiet mode: moments and small talk half as often
	Seed      uint64      // for the little random choices (0: from the clock)
	// Save is what the café remembers from earlier visits (nil: a first
	// visit with nothing remembered).
	Save *save.Data
	// Store is called to save (every minute or so, and when you leave);
	// nil means nothing is saved (previews and tests).
	Store func(*save.Data) error
	// Away pretends your last visit was this long ago (for testing the
	// away notes); 0 uses the save's real last visit.
	Away time.Duration
	// Bonds sets how well cats know you at the start, by cat id (0..100),
	// on top of the save (previews and tests).
	Bonds map[string]float64
}

type phase int

const (
	phaseTitle    phase = iota
	phaseEntering       // the title card fading into the café
	phaseCafe
	phaseLeaving // the café fading out before the game closes
	phaseGone
)

const (
	enterSeconds = 1.6
	leaveSeconds = 1.1
)

var (
	letterbox  = rgb.MustHex("#15110f")
	titleInk   = rgb.MustHex("#f4ead8")
	softInk    = rgb.MustHex("#cdbda4")
	dimInk     = rgb.MustHex("#8c7f6d")
	titleScale = 0.42 // how dim the café is behind the title card
)

// Game is one session at the café.
type Game struct {
	cfg       Config
	ui        *text.UI
	lib       *sprites.Library
	sched     *timeofday.Schedule
	wcfg      *weather.Config
	wstate    *weather.State
	scene     *scene.Scene
	clock     Clock
	talk      *dialogue.Library
	drinks    *drinks.Menu
	cat       catPart
	mem       memory
	lookWords lookFile
	rng       *rand.Rand

	lastReal time.Time
	anim     float64 // seconds since start, for gentle pulses
	phase    phase
	progress float64 // 0..1 through a fade

	pause *pauseMenu
	menu  *drinksMenu
	box   *talkBox   // the dialogue box on screen
	queue []*talkBox // conversations waiting their turn
	look  *lookMode  // Look around (from the Esc menu), or nil

	inCafe  float64   // real seconds since stepping inside
	greeted bool      // the "arrive" conversation has been offered
	order   *cup      // the cup on your table (nil: none yet)
	offerAt time.Time // café time to offer another drink (zero: none due)

	suggestion *drinks.Drink // the drink suggested when the cup is empty
	lastOffer  string        // the last cup_empty conversation, not to repeat

	pinned []scene.CatDraw // cats placed by hand (for previews), or nil

	w, h  int
	frame *canvas.Canvas
}

// New loads all content and starts at the title card.
func New(cfg Config, fsys fs.FS, realNow time.Time) (*Game, error) {
	if cfg.Interior == "" {
		cfg.Interior = "french"
	}
	ui, err := text.LoadUI(fsys)
	if err != nil {
		return nil, err
	}
	lib, err := sprites.Load(fsys, "sprites")
	if err != nil {
		return nil, err
	}
	sched, err := timeofday.Load(fsys, "lighting.json")
	if err != nil {
		return nil, err
	}
	wcfg, err := weather.Load(fsys, "weather.json")
	if err != nil {
		return nil, err
	}
	talk, err := dialogue.Load(fsys, "dialogue")
	if err != nil {
		return nil, err
	}
	if _, ok := talk.Get(convArrive); !ok {
		return nil, fmt.Errorf("content/dialogue: the conversation %q is missing", convArrive)
	}
	if len(talk.WithPrefix(convCupEmpty)) == 0 {
		return nil, fmt.Errorf("content/dialogue: there should be at least one conversation called %q (or starting with it)", convCupEmpty)
	}
	menu, err := drinks.Load(fsys, "drinks.json")
	if err != nil {
		return nil, err
	}
	if err := checkDrinkArt(menu, lib); err != nil {
		return nil, err
	}
	sc, err := scene.New(fsys, cfg.Interior, lib, scene.Options{ReducedMotion: cfg.Reduced})
	if err != nil {
		return nil, err
	}
	_ = sc.PutCup(nil) // the table is clear until you order
	speed := 1.0
	if cfg.Fast {
		speed = FastSpeed
	}
	clock := NewClock(realNow, cfg.Start, speed)
	ws, err := wcfg.NewState(clock.Now(realNow), cfg.Weather)
	if err != nil {
		return nil, err
	}
	seed := cfg.Seed
	if seed == 0 {
		seed = uint64(realNow.UnixNano())
	}
	g := &Game{
		cfg: cfg, ui: ui, lib: lib, sched: sched, wcfg: wcfg, wstate: ws,
		scene: sc, clock: clock, talk: talk, drinks: menu,
		rng:      rand.New(rand.NewPCG(seed, seed>>17|1)),
		lastReal: realNow, w: FrameW, h: FrameH,
	}
	if err := g.loadCats(fsys); err != nil {
		return nil, err
	}
	if err := g.loadLook(fsys); err != nil {
		return nil, err
	}
	if err := g.loadMemory(fsys, realNow); err != nil {
		return nil, err
	}
	g.Tick(realNow)
	return g, nil
}

// checkDrinkArt makes sure every drink's pictures exist.
func checkDrinkArt(m *drinks.Menu, lib *sprites.Library) error {
	for _, d := range m.Drinks {
		if s, ok := lib.Get(d.Icon); !ok {
			return fmt.Errorf("drinks.json: %s's icon %q isn't in content/sprites", d.ID, d.Icon)
		} else if s.W > iconW || s.H > iconH*2 {
			return fmt.Errorf("drinks.json: %s's icon %q is %d x %d pixels; icons can be at most %d x %d", d.ID, d.Icon, s.W, s.H, iconW, iconH*2)
		}
	}
	for id, c := range m.Cups {
		if _, ok := lib.Get(c.Sprite); !ok {
			return fmt.Errorf("drinks.json: cup %q uses sprite %q, which isn't in content/sprites", id, c.Sprite)
		}
	}
	return nil
}

// WeatherKinds lists the weather names --weather accepts.
func WeatherKinds(fsys fs.FS) ([]string, error) {
	c, err := weather.Load(fsys, "weather.json")
	if err != nil {
		return nil, err
	}
	return c.Kinds(), nil
}

// Now is the café's current time.
func (g *Game) Now() time.Time { return g.clock.Now(g.lastReal) }

// Done is true once the player has left the café.
func (g *Game) Done() bool { return g.phase == phaseGone }

// Goodbye is the line printed after the game closes.
func (g *Game) Goodbye() string { return g.ui.Goodbye }

// Resize tells the game the terminal's size in cells.
func (g *Game) Resize(w, h int) {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	g.w, g.h = w, h
}

// Tick moves everything forward to the real time now. Call it ~10 times a
// second.
func (g *Game) Tick(realNow time.Time) {
	dt := realNow.Sub(g.lastReal).Seconds()
	if dt < 0 {
		dt = 0
	}
	g.lastReal = realNow
	g.anim += dt

	now := g.clock.Now(realNow)
	g.wstate.Advance(now)
	g.updateCup(now)
	g.updateCats(dt)
	g.scene.Update(dt, scene.Inputs{
		Now:       now,
		Light:     g.sched.At(now),
		Weather:   g.wstate.Params(),
		SnowCover: g.wstate.SnowCover(),
	})

	switch g.phase {
	case phaseEntering:
		g.progress += dt / g.fadeTime(enterSeconds)
		if g.progress >= 1 {
			g.phase, g.progress = phaseCafe, 0
		}
	case phaseCafe:
		g.inCafe += dt
		g.autosave(dt)
		g.updateTalk(dt, now)
		if g.look != nil {
			g.updateLook(dt)
		}
	case phaseLeaving:
		g.progress += dt / g.fadeTime(leaveSeconds)
		if g.progress >= 1 {
			g.phase, g.progress = phaseGone, 1
		}
	}
}

func (g *Game) fadeTime(normal float64) float64 {
	if g.cfg.Reduced {
		return normal / 3 // still soft, but quick
	}
	return normal
}

// Key handles one key press.
func (g *Game) Key(k Key) {
	if k == KeyQuit {
		if g.phase != phaseGone {
			g.leave()
		}
		g.phase, g.progress = phaseGone, 1
		return
	}
	if g.pause != nil {
		g.pauseKey(k)
		return
	}
	if g.menu != nil {
		g.drinksKey(k)
		return
	}
	if g.look != nil {
		g.lookKey(k)
		return
	}
	switch g.phase {
	case phaseTitle:
		switch k {
		case KeyConfirm:
			g.phase, g.progress = phaseEntering, 0
		case KeyBack:
			g.openPause()
		}
	case phaseEntering:
		switch k {
		case KeyConfirm: // Enter skips the fade, as in a visual novel
			g.phase, g.progress = phaseCafe, 0
		case KeyBack:
			g.phase, g.progress = phaseCafe, 0
			g.openPause()
		}
	case phaseCafe:
		g.cafeKey(k)
	case phaseLeaving:
		if k == KeyConfirm {
			g.phase, g.progress = phaseGone, 1
		}
	}
}

func (g *Game) cafeKey(k Key) {
	switch k {
	case KeyBack:
		g.openPause()
		return
	}
	b := g.box
	if b == nil || b.Answered() {
		return
	}
	switch k {
	case KeyUp:
		b.Up()
	case KeyDown:
		b.Down()
	case KeyConfirm:
		b.Confirm()
	}
	g.settle()
}

// --- The pause menu -----------------------------------------------------------

type menuAction int

const (
	actOrder menuAction = iota
	actLook
	actLeave
)

type pauseMenu struct {
	items []menuAction
	sel   int
}

func (g *Game) openPause() {
	// Order a drink first, Leave the café last (Nala's choice after the M2
	// review). Esc closes the menu, so no "stay" item is needed. On the
	// title card there's nothing to order yet. Settings join in milestone 5.
	p := &pauseMenu{items: []menuAction{actLeave}}
	if g.phase == phaseCafe {
		p.items = []menuAction{actOrder, actLook, actLeave}
	}
	g.pause = p
	g.holdTalk(true)
}

func (g *Game) closePause() {
	g.pause = nil
	g.holdTalk(g.menu != nil)
}

func (g *Game) label(a menuAction) string {
	switch a {
	case actLeave:
		return g.ui.MenuLeave
	case actLook:
		return g.ui.MenuLook
	}
	return g.ui.MenuOrder
}

func (g *Game) pauseKey(k Key) {
	p := g.pause
	switch k {
	case KeyUp:
		if p.sel > 0 {
			p.sel--
		}
	case KeyDown:
		if p.sel < len(p.items)-1 {
			p.sel++
		}
	case KeyBack:
		g.closePause() // Esc again just closes the menu: never leave by accident
	case KeyConfirm:
		switch p.items[p.sel] {
		case actLeave:
			g.closePause()
			g.leave()
			g.phase, g.progress = phaseLeaving, 0
		case actOrder:
			g.pause = nil
			g.openDrinks(true)
		case actLook:
			g.pause = nil
			g.openLook()
		}
	}
}

// holdTalk pauses the dialogue box while a menu covers it.
func (g *Game) holdTalk(hold bool) {
	if g.box != nil {
		g.box.SetPaused(hold)
	}
}

// --- Frames -------------------------------------------------------------------

// View draws the whole terminal.
func (g *Game) View() string {
	if g.w <= 0 || g.h <= 0 {
		return ""
	}
	if g.frame == nil || g.frame.W != g.w || g.frame.H != g.h {
		g.frame = canvas.New(g.w, g.h)
	}
	c := g.frame
	c.Fill(letterbox)
	if g.w < FrameW || g.h < FrameH {
		g.drawTooSmall(c)
		return c.String(g.cfg.ColorMode)
	}
	ox, oy := (g.w-FrameW)/2, (g.h-FrameH)/2
	_ = g.scene.SetCats(g.catsToDraw())
	g.scene.Draw(c, ox, oy*2)

	switch g.phase {
	case phaseTitle:
		g.dim(c, ox, oy, 1)
		g.drawTitle(c, ox, oy, 1)
	case phaseEntering:
		t := ease(g.progress)
		g.dim(c, ox, oy, 1-t)
		g.drawTitle(c, ox, oy, math.Max(0, 1-g.progress*2.2))
	case phaseCafe:
		if g.look != nil {
			g.drawLook(c, ox, oy)
		} else if g.box != nil {
			g.drawTalk(c, ox, oy, g.box)
		}
	case phaseLeaving, phaseGone:
		g.fadeOut(c, ox, oy, ease(g.progress))
	}
	if g.menu != nil {
		g.drawDrinks(c, ox, oy)
	}
	if g.pause != nil {
		g.drawPause(c, ox, oy)
	}
	return c.String(g.cfg.ColorMode)
}

func ease(t float64) float64 {
	t = rgb.Clamp01(t)
	return t * t * (3 - 2*t)
}

// dim darkens the café behind the title card; amount 1 is fully dimmed,
// 0 is normal. The middle band, where the words sit, is a little darker.
func (g *Game) dim(c *canvas.Canvas, ox, oy int, amount float64) {
	if amount <= 0 {
		return
	}
	for y := 0; y < FrameH*2; y++ {
		band := math.Exp(-math.Pow((float64(y)-float64(titleRow*2+1))/16, 2)) // centered on the title
		f := titleScale * (1 - 0.35*band)
		f = 1 - (1-f)*amount
		for x := 0; x < FrameW; x++ {
			px, py := ox+x, oy*2+y
			c.Set(px, py, c.At(px, py).Scale(f))
		}
	}
}

func (g *Game) fadeOut(c *canvas.Canvas, ox, oy int, t float64) {
	for y := 0; y < FrameH*2; y++ {
		for x := 0; x < FrameW; x++ {
			px, py := ox+x, oy*2+y
			c.Set(px, py, c.At(px, py).Lerp(letterbox, t))
		}
	}
}

// writeText puts words on the picture, blending into it by alpha (1 solid,
// 0 invisible).
func writeText(c *canvas.Canvas, cx, cy int, s string, fg rgb.Color, alpha float64) {
	writeStyled(c, cx, cy, s, fg, alpha, false)
}

func writeStyled(c *canvas.Canvas, cx, cy int, s string, fg rgb.Color, alpha float64, italic bool) {
	x := cx
	for _, r := range s {
		w := canvas.RuneWidth(r)
		if w == 0 {
			continue
		}
		bg := c.At(x, cy*2).Lerp(c.At(x, cy*2+1), 0.5)
		if r != ' ' {
			ink := bg.Lerp(fg, rgb.Clamp01(alpha))
			if italic {
				c.TextItalic(x, cy, string(r), ink, bg)
			} else {
				c.Text(x, cy, string(r), ink, bg)
			}
		}
		x += w
	}
}

func centered(s string) int { return (FrameW - canvas.Width(s)) / 2 }

// spaced lays a title out with air between the letters: "C o f f e e".
func spaced(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.Join(strings.Split(w, ""), " ")
	}
	out := strings.Join(words, "   ")
	if canvas.Width(out) > FrameW-4 {
		return s // too long to space out nicely
	}
	return out
}

func (g *Game) drawTitle(c *canvas.Canvas, ox, oy int, alpha float64) {
	if alpha <= 0 {
		return
	}
	title := spaced(g.ui.Title)
	rule := strings.Repeat("─", 21) + "  ·  " + strings.Repeat("─", 21)
	writeText(c, ox+centered(rule), oy+titleRow-2, rule, softInk, alpha*0.8)
	writeStyled(c, ox+centered(title), oy+titleRow, title, titleInk, alpha, true)
	writeText(c, ox+centered(rule), oy+titleRow+2, rule, softInk, alpha*0.8)
	if g.ui.Subtitle != "" {
		writeText(c, ox+centered(g.ui.Subtitle), oy+titleRow+3, g.ui.Subtitle, softInk, alpha)
	}

	pulse := 1.0
	if !g.cfg.Reduced {
		pulse = 0.6 + 0.4*(0.5+0.5*math.Sin(g.anim*2*math.Pi/3.6))
	}
	writeText(c, ox+centered(g.ui.Prompt), oy+titleRow+5, g.ui.Prompt, softInk, alpha*pulse)
	if g.ui.Controls != "" {
		writeText(c, ox+centered(g.ui.Controls), oy+FrameH-2, g.ui.Controls, dimInk, alpha)
	}
}

func (g *Game) drawTooSmall(c *canvas.Canvas) {
	vals := map[string]string{
		"need": fmt.Sprintf("%d×%d", FrameW, FrameH),
		"have": fmt.Sprintf("%d×%d", g.w, g.h),
	}
	lines := g.ui.TooSmall
	top := (g.h - len(lines)) / 2
	for i, l := range lines {
		s := text.Fill(l, vals)
		x := (g.w - canvas.Width(s)) / 2
		if x < 0 {
			x = 0
		}
		ink := softInk
		if i == 0 {
			ink = titleInk
		}
		c.Text(x, top+i, s, ink, letterbox)
	}
}
