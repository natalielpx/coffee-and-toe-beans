package game

import (
	"fmt"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/drinks"
)

// Conversations the café needs (in content/dialogue).
const (
	convArrive   = "arrive"    // just after stepping inside
	convCupEmpty = "cup_empty" // a while after the cup is finished; every
	// conversation whose name starts with this is one way of saying it
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
	talkOffer    talkKind = iota // offers a drink; dropped once you order
	talkServed                   // a drink arriving
	talkMoment                   // a moment with a cat (waits for you)
	talkNarrator                 // the narrator: welcome, small talk
	talkCatReply                 // the cat's answer after a moment (so nothing ends in silence)
)

// talkBox is a conversation on screen (or waiting for its turn).
type talkBox struct {
	*dialogue.Box
	kind    talkKind
	handled bool // its answer has been acted on

	catOf  *cats.Cat    // for a moment: whose it is
	moment *cats.Moment // ...and which
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
	o.Speed = g.letters()
	o.Fade = g.fade()
	if q := g.mem.pace.QuestionSeconds; q > 0 {
		o.Patience = q
	}
	return o
}

// momentOptions are for the cats' moments: they wait for you on every
// page, and the cat stays put.
func (g *Game) momentOptions() dialogue.Options {
	o := g.talkOptions()
	o.Wait = true
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

func (g *Game) busy() bool { return g.pause != nil || g.menu != nil || g.look != nil }

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
		g.arrive(now)
	case !g.offerAt.IsZero() && !now.Before(g.offerAt):
		g.offerAt = time.Time{}
		g.offerAnother(now)
	case g.greeted && len(g.queue) == 0:
		g.smallTalk(now)
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
	if b.Answered() && !b.handled && b.kind == talkMoment {
		b.handled = true
		if b.moment != nil {
			g.catReplyAfter(b)
			g.endMoment(b)
		}
	}
	if b.Answered() && !b.handled {
		b.handled = true
		r, _ := b.Done()
		switch r.Do {
		case "order":
			g.box = nil
			g.hush(g.Now())
			g.openDrinks(false)
			return
		case "order_suggestion":
			g.box = nil
			g.hush(g.Now())
			if d := g.suggestion; d != nil && d.Available(g.dayNow(g.Now())) {
				g.serve(d)
			} else {
				g.openDrinks(false) // its hour just passed: choose another
			}
			return
		}
	}
	if _, done := b.Done(); done {
		g.box = nil
		g.hush(g.Now())
		g.nextTalk()
	}
}

// offerAnother says something once the cup is empty, picked from the
// cup_empty conversations: never the same one twice in a row, and only one
// that suggests a drink if there's something else being served.
func (g *Game) offerAnother(now time.Time) {
	g.suggestion = g.suggest(now)
	var pool []string
	for _, id := range g.talk.WithPrefix(convCupEmpty) {
		c, _ := g.talk.Get(id)
		if c.Uses("suggestion") && g.suggestion == nil {
			continue
		}
		pool = append(pool, id)
	}
	if len(pool) > 1 {
		kept := pool[:0]
		for _, id := range pool {
			if id != g.lastOffer {
				kept = append(kept, id)
			}
		}
		pool = kept
	}
	if len(pool) == 0 {
		return
	}
	id := pool[g.rng.IntN(len(pool))]
	g.lastOffer = id
	vals := map[string]string{"drink": g.drinkWord()}
	if g.suggestion != nil {
		vals["suggestion"] = g.suggestion.Word()
	}
	g.open(id, talkOffer, vals)
}

// suggest picks another drink being served right now (nil if none).
func (g *Game) suggest(now time.Time) *drinks.Drink {
	var pool []*drinks.Drink
	for _, d := range g.drinks.Drinks {
		if d.Available(g.dayNow(now)) && (g.order == nil || d.ID != g.order.order.Drink) {
			pool = append(pool, d)
		}
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[g.rng.IntN(len(pool))]
}

// drinkWord is the drink in your cup, as written mid-sentence: "café crème".
func (g *Game) drinkWord() string {
	if g.order == nil {
		return ""
	}
	return g.order.drink.Word()
}

// serve brings a drink to the table, replacing whatever was there.
func (g *Game) serve(d *drinks.Drink) {
	now := g.Now()
	g.putCup(d, drinks.Order{Drink: d.ID, At: now, Seed: g.rng.Uint64()})
	g.mem.kept = false
	g.countOrder(d.ID)

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

// lastDrink is where the menu opens: on the drink in your cup, or else
// your favourite.
func (g *Game) lastDrink() int {
	id := ""
	if g.order != nil {
		id = g.order.order.Drink
	} else if f := g.favourite(); f != nil {
		id = f.ID
	}
	for i, d := range g.drinks.Drinks {
		if d.ID == id {
			return i
		}
	}
	return 0
}

func (g *Game) servedNow(i int) bool {
	return i >= 0 && i < len(g.drinks.Drinks) && g.drinks.Drinks[i].Available(g.dayNow(g.Now()))
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
