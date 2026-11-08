package game

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/scene"
	"coffee-and-toe-beans/internal/sprites"
)

// catPart is the cats' share of the game: the cats themselves, their
// moments and observations, and when to share them.
type catPart struct {
	defs    []*cats.Def
	world   *cats.World
	content *cats.Content
	pacer   *cats.Pacer

	seen    map[string]time.Time // moment id -> when it was last shown (café time)
	lastCat string               // the cat of the last moment
}

// loadCats reads the cats, their moments and the pacing, and checks them
// against the art, the room, the weather and the drinks.
func (g *Game) loadCats(fsys fs.FS) error {
	defs, err := cats.LoadDefs(fsys, "cats")
	if err != nil {
		return err
	}
	if err := checkCatArt(defs, g.lib); err != nil {
		return err
	}
	var spots, drinkIDs []string
	for name := range g.scene.Spots() {
		spots = append(spots, name)
	}
	sort.Strings(spots)
	for _, d := range g.drinks.Drinks {
		drinkIDs = append(drinkIDs, d.ID)
	}
	ct, err := cats.LoadContent(fsys, "moments", cats.Checks{Cats: defs, Spots: spots, Weathers: g.wcfg.Kinds(), Drinks: drinkIDs})
	if err != nil {
		return err
	}
	pacing, err := cats.LoadPacing(fsys, "pacing.json")
	if err != nil {
		return err
	}
	g.cat = catPart{defs: defs, content: ct, seen: map[string]time.Time{}}
	if g.cfg.Save != nil {
		for id, t := range g.cfg.Save.Seen {
			g.cat.seen[id] = t
		}
	}
	g.cat.world = cats.NewWorld(defs, roomFor(g), startBonds(g.cfg), g.catContext(), g.rng)
	g.cat.world.SetCalm(g.cfg.Reduced)
	g.cat.pacer = cats.NewPacer(pacing, g.Now(), g.rng) // the settings stretch it in applySettings
	return nil
}

// checkCatArt makes sure every cat's sprites exist and their anchors are
// inside them.
func checkCatArt(defs []*cats.Def, lib *sprites.Library) error {
	for _, d := range defs {
		for _, p := range cats.Poses {
			pose := d.Poses[p]
			for f, name := range pose.Frames {
				s, ok := lib.Get(name)
				if !ok {
					return fmt.Errorf("%s: %s's %s/%s frame uses sprite %q, which isn't in content/sprites", d.File, d.Name, p, f, name)
				}
				if a := pose.Anchor; a[0] < 0 || a[1] < 0 || a[0] >= s.W || a[1] >= s.H {
					return fmt.Errorf("%s: the %s anchor %v is outside sprite %q (%d x %d)", d.File, p, a, name, s.W, s.H)
				}
			}
		}
	}
	return nil
}

// room answers the cats' questions about the room from the scene.
type room struct{ g *Game }

func roomFor(g *Game) room { return room{g} }

func (r room) Spots() []cats.SpotInfo {
	var out []cats.SpotInfo
	for _, s := range r.g.scene.Spots() {
		out = append(out, cats.SpotInfo{Name: s.Name, Near: s.Near, Place: s.Place, Poses: s.Poses})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r room) Box(d *cats.Def, pose, spot string) (cats.Rect, bool) {
	p, ok := d.Poses[pose]
	if !ok {
		return cats.Rect{}, false
	}
	x0, y0, x1, y1, err := r.g.scene.CatBox(p.Frames[cats.FrameStill], spot, p.Anchor)
	if err != nil {
		return cats.Rect{}, false
	}
	w, h := r.g.scene.Size()
	if x0 < 0 || y0 < 0 || x1 > w || y1 > h {
		return cats.Rect{}, false // it would hang off the edge of the café
	}
	return cats.Rect{X0: x0, Y0: y0, X1: x1, Y1: y1}, true
}

// catContext is what the cats know about the café right now.
func (g *Game) catContext() cats.Context {
	now := g.Now()
	drink := ""
	if g.order != nil && !g.drinks.Empty(g.order.order, now) {
		drink = g.order.drink.ID
	}
	return cats.Context{
		Now:     now,
		Part:    g.sched.Phase(g.dayNow(now)),
		Weather: g.wstate.Kind(),
		Drink:   drink,
	}
}

// updateCats moves the cats along and, in the café, lets one come to you
// with a moment when it's time and nothing else is being said. (Their
// little observations are in Look around.)
func (g *Game) updateCats(dt float64) {
	ctx := g.catContext()
	g.cat.world.Update(ctx, dt)
	if g.phase != phaseCafe || g.pinned != nil {
		return
	}
	if g.busy() || g.box != nil || len(g.queue) > 0 || !g.greeted {
		return
	}
	now := ctx.Now
	p := g.cat.pacer
	if !p.MomentDue(now) {
		return
	}
	c, m := g.cat.world.PickMoment(g.cat.content, g.cat.seen, g.cat.lastCat)
	if m == nil {
		p.RetryMoment(now)
		return
	}
	g.startMoment(c, m)
	p.Moment(now)
}

// startMoment opens a cat's moment in the dialogue box. The cat holds
// still meanwhile (after hopping over first, if the moment says so).
func (g *Game) startMoment(c *cats.Cat, m *cats.Moment) {
	if m.ComesTo != "" && !g.cat.world.Bring(c, m.ComesTo) {
		return
	}
	g.cat.world.Hold(c, true)
	g.cat.seen[m.ID] = g.Now()
	g.cat.lastCat = c.Def.ID
	vals := map[string]string{"drink": g.drinkWord(), "cat": c.Def.Name}
	b := m.Conv.Open(vals, g.rng, g.momentOptions())
	b.Shuffle(g.rng) // choices in a different order each time
	g.queue = append(g.queue, &talkBox{Box: b, kind: talkMoment, catOf: c, moment: m})
	g.nextTalk()
}

// endMoment applies a moment's answer to its cat.
func (g *Game) endMoment(b *talkBox) {
	r, _ := b.Done()
	g.cat.world.Answer(b.catOf, b.moment, r.Choice)
}

// catsToDraw is what the scene should show this frame.
func (g *Game) catsToDraw() []scene.CatDraw {
	if g.pinned != nil {
		return g.pinned
	}
	var out []scene.CatDraw
	for _, d := range g.cat.world.Draw() {
		out = append(out, scene.CatDraw{Sprite: d.Sprite, Spot: d.Spot, Anchor: d.Anchor, Alpha: d.Alpha})
	}
	return out
}

// Cats lets tests and previews look at the cats.
func (g *Game) Cats() *cats.World { return g.cat.world }

// PinCat puts a cat at a spot in a pose and keeps it there (for previews
// and the snapshot tool); the cats stop wandering. frame is "still",
// "blink", "ear", "tail" or "breathe".
func (g *Game) PinCat(id, pose, spot, frame string) error {
	var d *cats.Def
	var ids []string
	for _, x := range g.cat.defs {
		ids = append(ids, x.ID)
		if x.ID == id {
			d = x
		}
	}
	if d == nil {
		return fmt.Errorf("no cat called %q (try %s)", id, strings.Join(ids, ", "))
	}
	p, ok := d.Poses[pose]
	if !ok {
		return fmt.Errorf("no pose called %q (try %s)", pose, strings.Join(cats.Poses, ", "))
	}
	if frame == "" {
		frame = cats.FrameStill
	}
	if !p.Has(frame) {
		return fmt.Errorf("%s's %s pose has no %q frame", d.Name, pose, frame)
	}
	if _, ok := g.scene.Spots()[spot]; !ok {
		var names []string
		for n := range g.scene.Spots() {
			names = append(names, n)
		}
		sort.Strings(names)
		return fmt.Errorf("no spot called %q (try %s)", spot, strings.Join(names, ", "))
	}
	if g.pinned == nil {
		// The first cat pinned: the others leave, so only pinned cats show.
		g.pinned = []scene.CatDraw{}
		for _, c := range g.cat.world.Cats {
			g.cat.world.Put(c, "", "")
		}
	}
	g.pinned = append(g.pinned, scene.CatDraw{Sprite: p.Sprite(frame), Spot: spot, Anchor: p.Anchor, Alpha: 1})
	// Keep the wandering cats in step, so labels and moments agree.
	for _, c := range g.cat.world.Cats {
		if c.Def.ID == id {
			g.cat.world.Put(c, spot, pose)
		}
	}
	return g.scene.SetCats(g.pinned)
}

// ShowMoment opens a cat's moment at once (for previews): the cat is put
// at its spot first if it isn't in the room.
func (g *Game) ShowMoment(id string) error {
	for _, m := range g.cat.content.Moments {
		if m.ID != id {
			continue
		}
		c := g.cat.world.Cat(m.Cat)
		g.greeted = true
		g.box, g.queue = nil, nil
		vals := map[string]string{"drink": g.drinkWord(), "cat": c.Def.Name}
		b := m.Conv.Open(vals, g.rng, g.momentOptions())
		b.Shuffle(g.rng)
		g.say(talkMoment, b)
		g.box.catOf, g.box.moment = c, m
		return nil
	}
	var ids []string
	for _, m := range g.cat.content.Moments {
		ids = append(ids, m.ID)
	}
	return fmt.Errorf("no moment called %q (try %s)", id, strings.Join(ids, ", "))
}

// SetBond sets how well a cat knows you (previews and tests).
func (g *Game) SetBond(id string, bond float64) error {
	c := g.cat.world.Cat(id)
	if c == nil {
		return fmt.Errorf("no cat called %q", id)
	}
	c.Bond = bond
	return nil
}
