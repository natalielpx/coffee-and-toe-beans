package game

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/drinks"
	"coffee-and-toe-beans/internal/scene"
)

// Conversations the café needs (in content/dialogue).
const (
	convArrive   = "arrive"    // just after stepping inside
	convCupEmpty = "cup_empty" // a while after the cup is finished
)

// arriveDelay is how long the café lets you look around (real seconds)
// before saying hello.
const arriveDelay = 2.5

// After the cup is empty, another drink is offered once, somewhere between
// these many café minutes later.
const (
	offerMinMinutes = 1.0
	offerMaxMinutes = 3.0
)

type talkKind int

const (
	talkOffer  talkKind = iota // offers a drink; dropped once you order
	talkServed                 // a drink arriving
)

// talkBox is a conversation on screen (or waiting for its turn).
type talkBox struct {
	*dialogue.Box
	kind    talkKind
	handled bool // its answer has been acted on
}

// cup is the drink on your table.
type cup struct {
	order   drinks.Order
	drink   *drinks.Drink
	offered bool // another drink has been offered for this cup
}

func (g *Game) talkOptions() dialogue.Options {
	o := dialogue.DefaultOptions()
	o.Width = talkTextW
	o.Lines = talkLines
	if g.cfg.Reduced {
		o.Fade = 0.12
	}
	return o
}

// say queues a conversation; it shows as soon as nothing else is.
func (g *Game) say(kind talkKind, b *dialogue.Box) {
	g.queue = append(g.queue, &talkBox{Box: b, kind: kind})
	g.nextTalk()
}

func (g *Game) open(id string, kind talkKind, vals map[string]string) {
	c, ok := g.talk.Get(id)
	if !ok {
		return // checked when the game starts, so this shouldn't happen
	}
	g.say(kind, c.Open(vals, g.rng, g.talkOptions()))
}

func (g *Game) busy() bool { return g.pause != nil || g.menu != nil }

func (g *Game) nextTalk() {
	if g.box != nil || g.busy() || g.phase != phaseCafe || len(g.queue) == 0 {
		return
	}
	g.box, g.queue = g.queue[0], g.queue[1:]
}

// updateTalk moves the dialogue box along and decides when the café has
// something to say.
func (g *Game) updateTalk(dt float64, now time.Time) {
	if g.box != nil && !g.busy() {
		g.box.Update(dt)
	}
	g.settle()
	if g.busy() || g.box != nil {
		return
	}
	switch {
	case !g.greeted && g.inCafe >= arriveDelay:
		g.greeted = true
		if g.order == nil { // nobody offers a drink to someone already holding one
			g.open(convArrive, talkOffer, nil)
		}
	case !g.offerAt.IsZero() && !now.Before(g.offerAt):
		g.offerAt = time.Time{}
		g.open(convCupEmpty, talkOffer, map[string]string{"drink": g.drinkWord()})
	}
	g.nextTalk()
}

// settle acts on a conversation's answer and clears it away once it has
// faded.
func (g *Game) settle() {
	b := g.box
	if b == nil {
		return
	}
	if b.Answered() && !b.handled {
		b.handled = true
		r, _ := b.Done()
		if r.Do == "order" {
			g.box = nil
			g.openDrinks(false)
			return
		}
	}
	if _, done := b.Done(); done {
		g.box = nil
		g.nextTalk()
	}
}

// drinkWord is the drink in your cup, as written mid-sentence: "café crème".
func (g *Game) drinkWord() string {
	if g.order == nil {
		return ""
	}
	d, ok := g.drinks.Get(g.order.order.Drink)
	if !ok {
		return ""
	}
	return lowerFirst(d.Name)
}

func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

// serve brings a drink to the table, replacing whatever was there.
func (g *Game) serve(d *drinks.Drink) {
	now := g.Now()
	g.order = &cup{order: drinks.Order{Drink: d.ID, At: now, Seed: g.rng.Uint64()}, drink: d}
	g.offerAt = time.Time{}
	shape := g.drinks.Cups[d.Cup]
	body, top := d.Liquid()
	_ = g.scene.PutCup(&scene.Cup{Sprite: shape.Sprite, Steam: shape.Steam, Body: body, Top: top, Level: 1})
	g.scene.SetSteam(1)

	// Any offer of a drink is now beside the point, and so is the line
	// about a drink that has just been replaced.
	stale := func(k talkKind) bool { return k == talkOffer || k == talkServed }
	if g.box != nil && stale(g.box.kind) {
		g.box = nil
	}
	kept := g.queue[:0]
	for _, q := range g.queue {
		if !stale(q.kind) {
			kept = append(kept, q)
		}
	}
	g.queue = kept

	line := d.Served[g.rng.IntN(len(d.Served))]
	g.say(talkServed, dialogue.Start("", []string{line}, nil, 0, g.talkOptions()))
}

// updateCup cools and empties the cup on the table.
func (g *Game) updateCup(now time.Time) {
	c := g.order
	if c == nil {
		return
	}
	g.scene.SetLevel(g.drinks.Level(c.order, now))
	g.scene.SetSteam(g.drinks.Heat(c.order, now))
	if !c.offered && g.drinks.Empty(c.order, now) {
		c.offered = true
		wait := offerMinMinutes + g.rng.Float64()*(offerMaxMinutes-offerMinMinutes)
		g.offerAt = g.drinks.EmptySince(c.order).Add(time.Duration(wait * float64(time.Minute)))
	}
}

// --- The drinks menu ----------------------------------------------------------

type drinksMenu struct {
	sel       int
	fromPause bool // Esc goes back to the pause menu
}

func (g *Game) openDrinks(fromPause bool) {
	m := &drinksMenu{sel: g.lastDrink(), fromPause: fromPause}
	g.menu = m
	if !g.servedNow(m.sel) {
		g.moveDrink(1)
		if !g.servedNow(m.sel) {
			g.moveDrink(-1)
		}
	}
	g.holdTalk(true)
}

// lastDrink is where the menu opens: on the drink in your cup, if any.
func (g *Game) lastDrink() int {
	if g.order != nil {
		for i, d := range g.drinks.Drinks {
			if d.ID == g.order.order.Drink {
				return i
			}
		}
	}
	return 0
}

func (g *Game) servedNow(i int) bool {
	return i >= 0 && i < len(g.drinks.Drinks) && g.drinks.Drinks[i].Available(g.Now())
}

// moveDrink moves the selection to the next drink being served, skipping
// the ones that aren't (it stays put if there are none that way).
func (g *Game) moveDrink(dir int) {
	m := g.menu
	for i := m.sel + dir; i >= 0 && i < len(g.drinks.Drinks); i += dir {
		if g.servedNow(i) {
			m.sel = i
			return
		}
	}
}

func (g *Game) closeDrinks() {
	g.menu = nil
	g.holdTalk(g.pause != nil)
	g.nextTalk()
}

func (g *Game) drinksKey(k Key) {
	m := g.menu
	switch k {
	case KeyUp:
		g.moveDrink(-1)
	case KeyDown:
		g.moveDrink(1)
	case KeyBack:
		g.closeDrinks()
		if m.fromPause {
			g.openPause()
			for i, a := range g.pause.items {
				if a == actOrder {
					g.pause.sel = i
				}
			}
		}
	case KeyConfirm:
		if !g.servedNow(m.sel) {
			return // it's not that time of day (the time may have just changed)
		}
		d := g.drinks.Drinks[m.sel]
		g.closeDrinks()
		g.serve(d)
	}
}

// Serve puts a drink on the table straight away, as if just ordered (for
// previews; it will also restore your cup when saving arrives).
func (g *Game) Serve(id string) error {
	d, ok := g.drinks.Get(id)
	if !ok {
		var ids []string
		for _, d := range g.drinks.Drinks {
			ids = append(ids, d.ID)
		}
		return fmt.Errorf("no drink called %q (try %s)", id, strings.Join(ids, ", "))
	}
	g.greeted = true
	g.serve(d)
	return nil
}
