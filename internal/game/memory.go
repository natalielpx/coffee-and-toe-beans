package game

import (
	"io/fs"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/drinks"
	"coffee-and-toe-beans/internal/narrator"
	"coffee-and-toe-beans/internal/save"
	"coffee-and-toe-beans/internal/scene"
)

// Conversations for coming in (in content/dialogue).
const (
	convArriveFav = "arrive_favourite" // the drink offer, mentioning your favourite
	convCupKept   = "cup_kept"         // your drink is still on the table
)

// memory is what the café remembers between visits, and the narrator who
// talks about it.
type memory struct {
	data     *save.Data
	first    bool          // your very first visit
	away     time.Duration // how long since your last visit
	regular  bool          // you've spent a good while here
	baseSecs float64       // time in the café before this visit
	kept     bool          // your drink was still on the table

	narr   *narrator.Content
	pace   narrator.Pacing
	recent []string // small talk said lately, not to repeat

	nextTalk time.Time // café time of the next small talk
	quietTil time.Time // café time before which nobody starts talking
	saveIn   float64   // real seconds until the next autosave
	saveErr  error     // the last saving problem, if any
}

// loadMemory reads the narrator's words and starts from the save.
func (g *Game) loadMemory(fsys fs.FS, realNow time.Time) error {
	narr, err := narrator.Load(fsys, "narrator", narrator.Checks{Parts: cats.PartsOfDay, Weathers: g.wcfg.Kinds()})
	if err != nil {
		return err
	}
	byCat := map[string][]narrator.CatLine{}
	for _, l := range g.cat.content.Away {
		byCat[l.Cat] = append(byCat[l.Cat], narrator.CatLine{Bond: l.Bond, Gone: l.Gone, Text: l.Text})
	}
	for id, ls := range byCat {
		if err := narr.AddCat(id, ls); err != nil {
			return err
		}
	}
	pace, err := narrator.LoadPacing(fsys, "pacing.json")
	if err != nil {
		return err
	}
	d := g.cfg.Save
	if d == nil {
		d = save.New()
	}
	m := &g.mem
	*m = memory{data: d, narr: narr, pace: pace, recent: append([]string(nil), d.RecentTalk...)}
	m.first = !d.Returning()
	if !m.first {
		m.away = realNow.Sub(d.LastVisit)
		if m.away < 0 {
			m.away = 0 // the computer's clock went back
		}
	}
	if g.cfg.Away > 0 {
		m.first, m.away = false, g.cfg.Away
	}
	m.baseSecs = d.Seconds
	m.regular = d.Seconds >= narr.RegularHours*3600
	m.saveIn = pace.AutosaveSeconds
	now := g.Now()
	m.nextTalk = now.Add(narrator.Wait(pace.SmallTalkMinutes, g.slow(), g.rng))
	if d.FirstVisit.IsZero() {
		d.FirstVisit = realNow
	}
	d.Visits++
	g.restoreCup(now)
	return nil
}

// slow is how much quiet mode stretches the waits.
func (g *Game) slow() float64 {
	if g.cfg.Quiet {
		return 2
	}
	return 1
}

// startBonds are how well each cat knows you as the visit begins: from
// the save, then anything the config sets (previews and tests).
func startBonds(cfg Config) map[string]float64 {
	out := map[string]float64{}
	if cfg.Save != nil {
		for id, b := range cfg.Save.Bonds {
			out[id] = b
		}
	}
	for id, b := range cfg.Bonds {
		out[id] = b
	}
	return out
}

// restoreCup puts your drink back on the table if you've only just left;
// it cooled while you were away, but nobody drank from it.
func (g *Game) restoreCup(now time.Time) {
	m := &g.mem
	c := m.data.Cup
	if c == nil || m.first || m.away.Minutes() > m.pace.CupKeptMinutes {
		return
	}
	d, ok := g.drinks.Get(c.Drink)
	if !ok {
		return
	}
	age := time.Duration((c.AgeMinutes) * float64(time.Minute))
	o := drinks.Order{
		Drink:  d.ID,
		At:     now.Add(-age - m.away),
		Seed:   c.Seed,
		Paused: time.Duration(c.PausedMinutes*float64(time.Minute)) + m.away,
	}
	if g.drinks.Empty(o, now) {
		return
	}
	g.putCup(d, o)
	m.kept = true
}

// putCup sets a drink on the table.
func (g *Game) putCup(d *drinks.Drink, o drinks.Order) {
	g.order = &cup{order: o, drink: d}
	g.offerAt = time.Time{}
	shape := g.drinks.Cups[d.Cup]
	body, top := d.Liquid()
	_ = g.scene.PutCup(&scene.Cup{Sprite: shape.Sprite, Steam: shape.Steam, Body: body, Top: top, Level: g.drinks.Level(o, g.Now())})
	g.scene.SetSteam(g.drinks.Heat(o, g.Now()))
}

// favourite is the drink you order most, if there is one yet.
func (g *Game) favourite() *drinks.Drink {
	id := g.mem.data.Favourite(g.mem.pace.FavouriteOrders)
	if id == "" {
		return nil
	}
	d, _ := g.drinks.Get(id)
	return d
}

func (g *Game) narratorContext(now time.Time) narrator.Context {
	return narrator.Context{
		Part:    g.sched.Phase(now),
		Weather: g.wstate.Kind(),
		First:   g.mem.first,
		Regular: g.mem.regular,
	}
}

// awayCats are the cats as the away note needs them.
func (g *Game) awayCats() []narrator.Cat {
	var out []narrator.Cat
	for _, c := range g.cat.world.Cats {
		out = append(out, narrator.Cat{ID: c.Def.ID, Band: narrator.BandOf(int(c.Tier())), Bond: c.Bond})
	}
	return out
}

// arrive says hello: the welcome, what you missed if you've been away,
// then either the drink offer or a word about the drink still on your
// table, all in one box.
func (g *Game) arrive(now time.Time) {
	m := &g.mem
	ctx := g.narratorContext(now)
	pages := []string{m.narr.Welcome(ctx, g.rng)}
	if !m.first {
		pages = append(pages, m.narr.AwayNote(m.away, g.awayCats(), g.rng, pageFits)...)
	}
	vals := map[string]string{"drink": g.drinkWord()}
	var choices []dialogue.Choice
	ignore := 0
	kind := talkNarrator
	switch {
	case m.kept && g.order != nil:
		if c, ok := g.talk.Get(convCupKept); ok {
			pages = append(pages, fill(c.Lines(g.rng), vals)...)
		}
	case g.order == nil: // nobody offers a drink to someone already holding one
		conv, _ := g.talk.Get(convArrive)
		if fav := g.favourite(); fav != nil && fav.Available(now) && g.rng.Float64() < 0.5 {
			if c, ok := g.talk.Get(convArriveFav); ok {
				conv = c
			}
			vals["favourite"] = fav.Word()
		}
		pages = append(pages, fill(conv.Lines(g.rng), vals)...)
		for _, ch := range conv.Choices {
			choices = append(choices, dialogue.Choice{Text: fillOne(ch.Text, vals), Do: ch.Do})
		}
		ignore = conv.IgnoreChoice()
		kind = talkOffer
	}
	var kept []string
	for _, p := range pages {
		if p != "" {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return
	}
	g.say(kind, dialogue.Start("", kept, choices, ignore, g.talkOptions()))
}

func fill(lines []string, vals map[string]string) []string {
	for i := range lines {
		lines[i] = fillOne(lines[i], vals)
	}
	return lines
}

func fillOne(s string, vals map[string]string) string {
	for k, v := range vals {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// smallTalk says something about the café now and then, when nothing else
// is going on.
func (g *Game) smallTalk(now time.Time) bool {
	m := &g.mem
	if now.Before(m.nextTalk) || now.Before(m.quietTil) {
		return false
	}
	m.nextTalk = now.Add(narrator.Wait(m.pace.SmallTalkMinutes, g.slow(), g.rng))
	line := m.narr.SmallTalk(g.narratorContext(now), m.recent, g.rng)
	if line == "" {
		return false
	}
	m.recent = append(m.recent, line)
	if keep := len(m.narr.Talk) * 7 / 10; len(m.recent) > keep {
		m.recent = m.recent[len(m.recent)-keep:]
	}
	g.say(talkNarrator, dialogue.Start("", []string{line}, nil, 0, g.talkOptions()))
	return true
}

// hush is called when something has just been said: nothing new starts
// for a little while after.
func (g *Game) hush(now time.Time) {
	q := now.Add(time.Duration(g.mem.pace.QuietAfterMinutes * g.slow() * float64(time.Minute)))
	if g.mem.quietTil.Before(q) {
		g.mem.quietTil = q
	}
	g.cat.pacer.NotBefore(q)
}

// Remember is the save as it stands now.
func (g *Game) Remember() *save.Data {
	m := &g.mem
	d := m.data
	d.LastVisit = g.lastReal
	d.Seconds = m.baseSecs + g.inCafe
	if d.Bonds == nil {
		d.Bonds = map[string]float64{}
	}
	for _, c := range g.cat.world.Cats {
		d.Bonds[c.Def.ID] = c.Bond
	}
	d.Seen = map[string]time.Time{}
	for id, t := range g.cat.seen {
		d.Seen[id] = t
	}
	d.RecentTalk = append([]string(nil), m.recent...)
	d.Cup = nil
	if c := g.order; c != nil && !g.drinks.Empty(c.order, g.Now()) {
		d.Cup = &save.Cup{
			Drink:         c.order.Drink,
			Seed:          c.order.Seed,
			AgeMinutes:    g.Now().Sub(c.order.At).Minutes(),
			PausedMinutes: c.order.Paused.Minutes(),
		}
	}
	return d
}

// store saves now, if the game was given somewhere to save.
func (g *Game) store() {
	if g.cfg.Store == nil {
		return
	}
	g.mem.saveErr = g.cfg.Store(g.Remember())
}

// SaveError is the last problem saving, if any (shown after you leave).
func (g *Game) SaveError() error { return g.mem.saveErr }

// autosave saves every so often while you're in the café.
func (g *Game) autosave(dt float64) {
	g.mem.saveIn -= dt
	if g.mem.saveIn <= 0 {
		g.mem.saveIn = g.mem.pace.AutosaveSeconds
		g.store()
	}
}

// leave wraps up the visit: a moment still open passes quietly, and the
// café remembers everything.
func (g *Game) leave() {
	if g.inCafe <= 0 {
		return // never stepped inside: nothing to remember
	}
	if b := g.box; b != nil && !b.Answered() {
		b.Abandon()
		g.settle()
	}
	for _, q := range g.queue {
		if q.catOf != nil {
			g.cat.world.Hold(q.catOf, false)
		}
	}
	g.queue = nil
	g.store()
}

// Away is how long you'd been away when this visit began (0 on a first
// visit).
func (g *Game) Away() time.Duration { return g.mem.away }

// FirstVisit is true on your very first visit.
func (g *Game) FirstVisit() bool { return g.mem.first }

// countOrder remembers that you ordered a drink.
func (g *Game) countOrder(id string) {
	if g.mem.data.Orders == nil {
		g.mem.data.Orders = map[string]int{}
	}
	g.mem.data.Orders[id]++
}

// pageFits is true when text fits on one page of the dialogue box.
func pageFits(s string) bool { return len(dialogue.Wrap(s, talkTextW)) <= talkLines }
