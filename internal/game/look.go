package game

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strings"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/text"
)

// Look around (from the Esc menu): the cats hold still, and ← → move a
// soft marker between the window, each cat in the room and your table. A
// small panel says what you see and, once a cat knows you, what you can
// do. The words are in content/look.json and content/moments.

type lookFile struct {
	Names  map[string]string `json:"names"`
	Spots  map[string]string `json:"spots"`
	Window []noteDef         `json:"window"`
	Table  []noteDef         `json:"table"`
}

type noteDef struct {
	Text    dialogue.Text `json:"text"`
	Weather []string      `json:"weather"`
	When    []string      `json:"when"`
	Cup     string        `json:"cup"`
}

func (n noteDef) conditional() bool { return len(n.Weather) > 0 || len(n.When) > 0 || n.Cup != "" }

var cupStates = []string{"none", "warm", "cooling", "low", "empty"}

// lookLines is the most lines a description can take in the panel.
const lookLines = 4

// The kinds of things you can look at.
const (
	lookWindow = "window"
	lookTable  = "table"
	lookCat    = "cat"
)

type lookTarget struct {
	kind string
	cat  *cats.Cat
	x    int // room pixel under the marker
	top  int // room pixel row just above which the marker sits
}

func (t lookTarget) id() string {
	if t.cat != nil {
		return t.cat.Def.ID
	}
	return t.kind
}

type lookMode struct {
	targets []lookTarget
	sel     int
	choice  int
	reply   *dialogue.Box // what happened after an action, shown in the box
	replyOf *cats.Cat     // ...and with which cat
	desc    []string      // the description, wrapped
	descKey string        // what desc was made for (target, pose, tier)

	// Within one Look around, each thing is described once (coming back
	// to it shows the same words), and each cat offers its actions once:
	// after one, its panel ends with a quiet line about it instead.
	descs map[string][]string
	done  map[string][]string       // cat id -> the closing line, wrapped
	acts  map[string][]*cats.Action // cat id/pose -> what it offers this time
}

// loadLook reads and checks content/look.json.
func (g *Game) loadLook(fsys fs.FS) error {
	const name = "look.json"
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	var f lookFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	for _, k := range []string{lookWindow, lookTable} {
		if strings.TrimSpace(f.Names[k]) == "" {
			return fmt.Errorf("%s: names needs a name for %q", name, k)
		}
	}
	for s := range g.scene.Spots() {
		if strings.TrimSpace(f.Spots[s]) == "" {
			return fmt.Errorf("%s: spots needs words for the cat spot %q (like \"on the windowsill\")", name, s)
		}
	}
	kinds := g.wcfg.Kinds()
	for place, notes := range map[string][]noteDef{lookWindow: f.Window, lookTable: f.Table} {
		plain := false
		for i, n := range notes {
			where := fmt.Sprintf("%s: %s, line %d", name, place, i+1)
			if len(n.Text) == 0 {
				return fmt.Errorf("%s: needs a \"text\"", where)
			}
			plain = plain || !n.conditional()
			for _, w := range n.Weather {
				if !has(kinds, w) {
					return fmt.Errorf("%s: weather %q should be one of: %s", where, w, strings.Join(kinds, ", "))
				}
			}
			for _, w := range n.When {
				if !has(cats.PartsOfDay, w) {
					return fmt.Errorf("%s: when %q should be one of: %s", where, w, strings.Join(cats.PartsOfDay, ", "))
				}
			}
			if n.Cup != "" && (place != lookTable || !has(cupStates, n.Cup)) {
				return fmt.Errorf("%s: cup %q: only table lines have a cup (none, warm, cooling, low, empty)", where, n.Cup)
			}
		}
		if !plain {
			return fmt.Errorf("%s: %s needs at least one line with no conditions, for when nothing else fits", name, place)
		}
	}
	g.lookWords = f
	return nil
}

func has(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// cupState is the cup on your table, in the look words.
func (g *Game) cupState() string {
	if g.order == nil {
		return "none"
	}
	now := g.Now()
	switch {
	case g.drinks.Empty(g.order.order, now):
		return "empty"
	case g.drinks.Level(g.order.order, now) <= 0.3:
		return "low"
	case g.drinks.Heat(g.order.order, now) > 0.5:
		return "warm"
	}
	return "cooling"
}

// pickNote chooses a line that fits now; lines about right now are
// likelier than plain ones.
func (g *Game) pickNote(notes []noteDef) string {
	ctx := g.catContext()
	var fits []noteDef
	var ws []float64
	for _, n := range notes {
		if len(n.Weather) > 0 && !has(n.Weather, ctx.Weather) {
			continue
		}
		if len(n.When) > 0 && !has(n.When, ctx.Part) {
			continue
		}
		if n.Cup != "" && n.Cup != g.cupState() {
			continue
		}
		w := 1.0
		if n.conditional() {
			w = 4
		}
		fits = append(fits, n)
		ws = append(ws, w)
	}
	total := 0.0
	for _, w := range ws {
		total += w
	}
	r := g.rng.Float64() * total
	for i, w := range ws {
		if r < w || i == len(ws)-1 {
			return text.Fill(fits[i].Text.Pick(g.rng), map[string]string{"drink": g.drinkWord()})
		}
		r -= w
	}
	return ""
}

// lookTargets lists what you can look at, from left to right.
func (g *Game) lookTargets() []lookTarget {
	spots := g.scene.Spots()
	win := spots["sill"]
	stand := g.scene.CupStand()
	out := []lookTarget{
		{kind: lookWindow, x: win.X, top: 6},
		{kind: lookTable, x: stand[0] + 5, top: stand[1] - 15},
	}
	r := roomFor(g)
	for _, c := range g.cat.world.Cats {
		if !c.Settled() {
			continue
		}
		box, ok := r.Box(c.Def, c.Pose(), c.Spot())
		if !ok {
			continue
		}
		out = append(out, lookTarget{kind: lookCat, cat: c, x: spots[c.Spot()].X, top: box.Y0})
	}
	order := map[string]int{lookWindow: 0, lookCat: 1, lookTable: 2}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].x != out[j].x {
			return out[i].x < out[j].x
		}
		return order[out[i].kind] < order[out[j].kind]
	})
	return out
}

// openLook starts looking around: the cats hold still.
func (g *Game) openLook() {
	g.look = &lookMode{descs: map[string][]string{}, done: map[string][]string{}, acts: map[string][]*cats.Action{}}
	g.cat.world.SetStill(true)
	g.holdTalk(true)
	g.look.targets = g.lookTargets()
	// Start on the first cat, if there is one; otherwise the window.
	for i, t := range g.look.targets {
		if t.kind == lookCat {
			g.look.sel = i
			break
		}
	}
	g.lookedAt()
}

func (g *Game) closeLook() {
	g.look = nil
	g.cat.world.SetStill(false)
	g.hush(g.Now()) // let the room settle before anyone speaks
	g.holdTalk(g.busy())
	g.nextTalk()
}

func (g *Game) lookTarget() (lookTarget, bool) {
	l := g.look
	if l == nil || len(l.targets) == 0 {
		return lookTarget{}, false
	}
	if l.sel >= len(l.targets) {
		l.sel = len(l.targets) - 1
	}
	return l.targets[l.sel], true
}

// lookedAt notes that you're looking at the selected thing (cats like
// being looked at quietly, a little).
func (g *Game) lookedAt() {
	if t, ok := g.lookTarget(); ok && t.cat != nil {
		g.cat.world.Looked(t.cat)
	}
	g.look.choice = 0
	g.look.descKey = ""
}

// lookActions are what you can do with the selected cat (none for the
// window, the table, or a cat that doesn't know you yet).
func (g *Game) lookActions() []*cats.Action {
	t, ok := g.lookTarget()
	if !ok || t.cat == nil {
		return nil
	}
	if _, done := g.look.done[t.cat.Def.ID]; done {
		return nil // one thing per cat each time you look around
	}
	all := g.cat.world.Actions(g.cat.content.Looks[t.cat.Def.ID], t.cat)
	key := t.cat.Def.ID + "/" + t.cat.Pose()
	if got, ok := g.look.acts[key]; ok && sameActions(got, all) {
		return got
	}
	got := g.chooseActions(all)
	g.look.acts[key] = got
	return got
}

// maxActions is the most things to do a cat offers at once.
const maxActions = 4

// chooseActions picks what a cat offers this time you look around: at most
// maxActions, at most one sudden one, the newest (highest tier) first, in
// a random order.
func (g *Game) chooseActions(all []*cats.Action) []*cats.Action {
	pool := append([]*cats.Action(nil), all...)
	g.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].MinTier > pool[j].MinTier })
	var out []*cats.Action
	sudden := false
	for _, a := range pool {
		if len(out) == maxActions {
			break
		}
		if a.Effect.Manner == "sudden" {
			if sudden {
				continue
			}
			sudden = true
		}
		out = append(out, a)
	}
	g.rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// sameActions says whether every chosen action is still on offer.
func sameActions(chosen, all []*cats.Action) bool {
	for _, c := range chosen {
		found := false
		for _, a := range all {
			found = found || a == c
		}
		if !found {
			return false
		}
	}
	return len(chosen) > 0 || len(all) == 0
}

// updateLook keeps the targets in step with the room (a cat may leave or
// hop onto your table after an action) and moves a reply along.
func (g *Game) updateLook(dt float64) {
	l := g.look
	if l.reply != nil {
		l.reply.Update(dt)
		if _, done := l.reply.Done(); done {
			if c := l.replyOf; c != nil {
				l.descKey = ""
				if c.Here() {
					g.lookDescription(lookTarget{kind: lookCat, cat: c}) // as it is now (it may have moved)
				}
				l.done[c.Def.ID] = g.closingLine(c)
			}
			l.reply, l.replyOf = nil, nil
			l.descKey = ""
		}
	}
	old, _ := g.lookTarget()
	l.targets = g.lookTargets()
	for i, t := range l.targets {
		if t.id() == old.id() {
			if l.sel != i {
				l.sel = i
			}
			break
		}
	}
	if l.sel >= len(l.targets) {
		l.sel = len(l.targets) - 1
	}
	if n := len(g.lookActions()); l.choice >= n {
		l.choice = 0
	}
}

func (g *Game) lookKey(k Key) {
	l := g.look
	if l.reply != nil {
		switch k {
		case KeyConfirm:
			l.reply.Confirm()
		case KeyBack:
			g.closeLook()
		}
		return
	}
	acts := g.lookActions()
	switch k {
	case KeyBack:
		g.closeLook()
	case KeyLeft:
		if l.sel > 0 {
			l.sel--
			g.lookedAt()
		}
	case KeyRight:
		if l.sel < len(l.targets)-1 {
			l.sel++
			g.lookedAt()
		}
	case KeyUp:
		if l.choice > 0 {
			l.choice--
		}
	case KeyDown:
		if l.choice < len(acts)-1 {
			l.choice++
		}
	case KeyConfirm:
		if len(acts) == 0 {
			return
		}
		t, _ := g.lookTarget()
		a := acts[l.choice]
		g.cat.world.Act(t.cat, a)
		pages := make([]string, len(a.Reply))
		for i, p := range a.Reply {
			pages[i] = text.Fill(p.Pick(g.rng), map[string]string{"drink": g.drinkWord(), "cat": t.cat.Def.Name})
		}
		l.reply = dialogue.Start(t.cat.Def.Name, pages, nil, 0, g.talkOptions())
		l.replyOf = t.cat
	}
}

// lookTitle and lookDescription are what the panel shows.
func (g *Game) lookTitle(t lookTarget) string {
	if t.cat != nil {
		return t.cat.Def.Name
	}
	return g.lookWords.Names[t.kind]
}

func (g *Game) lookDescription(t lookTarget) []string {
	l := g.look
	key := t.id()
	if t.cat != nil {
		key += "/" + t.cat.Pose() + "/" + t.cat.Tier().String()
	}
	if key == l.descKey {
		return l.desc
	}
	if d, ok := l.descs[key]; ok {
		l.desc, l.descKey = d, key
		return d
	}
	var s string
	switch t.kind {
	case lookWindow:
		s = g.pickNote(g.lookWords.Window)
	case lookTable:
		s = g.pickNote(g.lookWords.Table)
	default:
		s = g.catDescription(t.cat)
	}
	l.desc, l.descKey = dialogue.Wrap(s, talkTextW), key
	if len(l.desc) > lookLines {
		l.desc = l.desc[:lookLines]
	}
	l.descs[key] = l.desc
	return l.desc
}

// closingLine ends an interaction quietly: an observation about the cat
// that fits now and doesn't involve you (or, failing that, what it's
// doing), and isn't already in its description.
func (g *Game) closingLine(c *cats.Cat) []string {
	if !c.Here() {
		return nil
	}
	vals := map[string]string{"where": g.lookWords.Spots[c.Spot()], "drink": g.drinkWord(), "cat": c.Def.Name}
	shown := g.catDescriptionFor(c)
	var pool []string
	for _, o := range g.cat.world.ObservationsFor(g.cat.content, c) {
		for _, v := range o.Text {
			v = text.Fill(v, vals)
			if !neutral(v) || strings.Contains(shown, v) {
				continue
			}
			pool = append(pool, v)
		}
	}
	if len(pool) == 0 {
		for _, v := range g.cat.content.Looks[c.Def.ID].Doing[c.Pose()] {
			if v = text.Fill(v, vals); !strings.Contains(shown, v) {
				pool = append(pool, v)
			}
		}
	}
	if len(pool) == 0 {
		// Never leave the player hanging after an interaction: fall back
		// to the cat's own words for having had enough.
		if rest := g.cat.content.Looks[c.Def.ID].Rest; len(rest) > 0 {
			pool = append(pool, text.Fill(rest[0], vals))
		}
	}
	if len(pool) == 0 {
		return nil
	}
	lines := dialogue.Wrap(pool[g.rng.IntN(len(pool))], talkTextW)
	if len(lines) > 2 {
		lines = lines[:2]
	}
	return lines
}

// neutral is true for a line that's about the cat alone, not about you.
func neutral(s string) bool {
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '!' || r == '?'
	}) {
		if w == "you" || w == "your" || w == "you're" || w == "yours" {
			return false
		}
	}
	return true
}

// catDescriptionFor is the description shown for a cat in this Look
// around, as one line of text ("" if it hasn't been shown).
func (g *Game) catDescriptionFor(c *cats.Cat) string {
	key := c.Def.ID + "/" + c.Pose() + "/" + c.Tier().String()
	return strings.Join(g.look.descs[key], " ")
}

// catDescription is how a cat looks, then what it's doing: about half the
// time one of its observations that fits (and fits the panel), otherwise
// the words for its pose.
func (g *Game) catDescription(c *cats.Cat) string {
	lk := g.cat.content.Looks[c.Def.ID]
	vals := map[string]string{"where": g.lookWords.Spots[c.Spot()], "drink": g.drinkWord(), "cat": c.Def.Name}
	looks := text.Fill(lk.Description(c.Tier()).Pick(g.rng), vals)
	doing := text.Fill(lk.Doing[c.Pose()].Pick(g.rng), vals)
	if obs := g.cat.world.ObservationsFor(g.cat.content, c); len(obs) > 0 && g.rng.Float64() < 0.5 {
		o := text.Fill(obs[g.rng.IntN(len(obs))].Text.Pick(g.rng), vals)
		if len(dialogue.Wrap(looks+" "+o, talkTextW)) <= lookLines {
			doing = o
		}
	}
	return looks + " " + doing
}

// LookAt opens Look around on something (for previews and tests): the
// window, your table, or a cat's id.
func (g *Game) LookAt(what string) error {
	if g.look == nil {
		g.openLook()
	}
	var names []string
	for i, t := range g.look.targets {
		names = append(names, t.id())
		if t.id() == what {
			g.look.sel = i
			g.lookedAt()
			return nil
		}
	}
	return fmt.Errorf("nothing called %q to look at right now (try %s)", what, strings.Join(names, ", "))
}

// drawLook draws the marker over what you're looking at and the panel.
func (g *Game) drawLook(c *canvas.Canvas, ox, oy int) {
	l := g.look
	t, ok := g.lookTarget()
	if !ok {
		return
	}
	pulse := 1.0
	if !g.cfg.Reduced {
		pulse = 0.55 + 0.45*(0.5+0.5*math.Sin(g.anim*2*math.Pi/2.4))
	}
	g.drawMarker(c, ox, oy, t, pulse)

	if l.reply != nil {
		g.drawTalk(c, ox, oy, &talkBox{Box: l.reply})
		return
	}
	desc := g.lookDescription(t)
	acts := g.lookActions()
	var closing []string
	if t.cat != nil {
		closing = l.done[t.cat.Def.ID]
	}
	_, done := l.done[t.id()]
	resting := t.cat != nil && !done && g.cat.world.Resting(t.cat)
	h := 2 + len(desc) + 2 // borders, description, a gap, the hint
	extra := 0
	switch {
	case len(acts) > 0:
		extra = len(acts) + 1
	case len(closing) > 0:
		extra = len(closing) + 1
	case resting:
		extra = 2
	}
	h += extra
	w := talkTextW + 4
	// Always low on the left, where the dialogue box is (Nala's choice).
	x, y := talkLeft, FrameH-1-h
	p := panel{c: c, x: ox + x, y: oy + y, w: w, h: h, alpha: 1}
	p.draw()
	p.write(2, 0, " "+g.lookTitle(t)+" ", highlight)
	row := 0
	for i, s := range desc {
		p.write(2, 1+i, s, boxInk)
	}
	row = 1 + len(desc) + 1
	switch {
	case len(acts) > 0:
		for i, a := range acts {
			if i == l.choice {
				p.write(2, row+i, "▸", highlight)
				p.write(4, row+i, a.Text, boxInk)
			} else {
				p.write(4, row+i, a.Text, boxDim)
			}
		}
	case len(closing) > 0:
		for i, s := range closing {
			p.write(2, row+i, s, boxDim)
		}
	case resting:
		lk := g.cat.content.Looks[t.cat.Def.ID]
		p.write(2, row, lk.Rest[0], boxDim)
	}
	hint := g.ui.LookHint
	if len(acts) > 0 {
		hint = g.ui.LookHintChoose
	}
	p.write(2, h-2, hint, boxFaint)
}

// markerShape is the little arrow over what you're looking at.
var markerShape = []string{
	"#####",
	".###.",
	"..#..",
}

// drawMarker draws the arrow a couple of pixels above the target, in its
// color (look_marker in content/text/ui.json), pulsing gently (steady with
// reduced motion). A light arrow gets a soft shadow under it.
func (g *Game) drawMarker(c *canvas.Canvas, ox, oy int, t lookTarget, alpha float64) {
	ink := highlight
	if v, err := rgb.Hex(g.ui.LookMarker); err == nil {
		ink = v
	}
	light := ink.R+ink.G+ink.B > 1.5
	h := len(markerShape)
	top := t.top - h - 3
	if top < 1 {
		top = 1
	}
	x0 := t.x - len(markerShape[0])/2
	on := func(x, y int) bool {
		return y >= 0 && y < h && x >= 0 && x < len(markerShape[y]) && markerShape[y][x] == '#'
	}
	outline, hasOutline := rgb.Color{}, false
	if v, err := rgb.Hex(g.ui.LookOutline); err == nil {
		outline, hasOutline = v, true
	}
	for y := -1; y <= h; y++ {
		for x := -1; x <= len(markerShape[0]); x++ {
			px, py := ox+x0+x, oy*2+top+y
			switch {
			case on(x, y):
				c.Set(px, py, c.At(px, py).Lerp(ink, alpha))
			case hasOutline && (on(x-1, y) || on(x+1, y) || on(x, y-1) || on(x, y+1)):
				c.Set(px, py, c.At(px, py).Lerp(outline, alpha))
			case !hasOutline && light && on(x, y-1):
				c.Set(px, py, c.At(px, py).Lerp(boxBG, 0.45*alpha))
			}
		}
	}
}
