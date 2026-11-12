package cats

import (
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/scene"
	"coffee-and-toe-beans/internal/sprites"
)

// realRoom answers from the real café (the French interior and its art).
type realRoom struct{ sc *scene.Scene }

func (r realRoom) Spots() []SpotInfo {
	var out []SpotInfo
	for _, s := range r.sc.Spots() {
		out = append(out, SpotInfo{Name: s.Name, Near: s.Near, Place: s.Place, Poses: s.Poses})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r realRoom) Box(d *Def, pose, spot string) (Rect, bool) {
	p := d.Poses[pose]
	x0, y0, x1, y1, err := r.sc.CatBox(p.Frames[FrameStill], spot, p.Anchor)
	w, h := r.sc.Size()
	if err != nil || x0 < 0 || y0 < 0 || x1 > w || y1 > h {
		return Rect{}, false
	}
	return Rect{x0, y0, x1, y1}, true
}

type fixture struct {
	defs  []*Def
	room  realRoom
	ct    *Content
	pace  Pacing
	spots []string
}

func load(t *testing.T) fixture {
	t.Helper()
	defs, err := LoadDefs(content.FS, "cats")
	if err != nil {
		t.Fatal(err)
	}
	lib, err := sprites.Load(content.FS, "sprites")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := scene.New(content.FS, "french", lib, scene.Options{})
	if err != nil {
		t.Fatal(err)
	}
	room := realRoom{sc}
	var spots []string
	for _, s := range room.Spots() {
		spots = append(spots, s.Name)
	}
	ct, err := LoadContent(content.FS, "moments", Checks{Cats: defs, Spots: spots,
		Weathers: []string{"clear", "cloudy", "drizzle", "rain", "heavy_rain", "fog", "snow"},
		Drinks:   []string{"cafe_creme", "chocolat_chaud", "chai_latte", "earl_grey", "matcha_latte", "vin_chaud", "tisane_verveine"}})
	if err != nil {
		t.Fatal(err)
	}
	pace, err := LoadPacing(content.FS, "pacing.json")
	if err != nil {
		t.Fatal(err)
	}
	return fixture{defs, room, ct, pace, spots}
}

var day = time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC)

func ctxAt(now time.Time, part string) Context {
	return Context{Now: now, Part: part, Weather: "rain"}
}

func (f fixture) world(t *testing.T, bonds map[string]float64, seed uint64) *World {
	t.Helper()
	return NewWorld(f.defs, f.room, bonds, ctxAt(day, "afternoon"), rand.New(rand.NewPCG(seed, 99)))
}

func (f fixture) moment(t *testing.T, id string) *Moment {
	t.Helper()
	for _, m := range f.ct.Moments {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("no moment %q", id)
	return nil
}

func TestTiers(t *testing.T) {
	cases := []struct {
		bond float64
		want Tier
	}{{0, Stranger}, {9.9, Stranger}, {10, Curious}, {24, Curious}, {25, Familiar}, {44.9, Familiar}, {45, Friend}, {69, Friend}, {70, YourCat}, {100, YourCat}}
	for _, c := range cases {
		if got := TierOf(c.bond); got != c.want {
			t.Errorf("bond %v: tier %s, want %s", c.bond, got, c.want)
		}
	}
	for tier := Stranger; tier <= YourCat; tier++ {
		if TierOf(BondFor(tier)) != tier {
			t.Errorf("BondFor(%s) doesn't land in %s", tier, tier)
		}
		if p, err := ParseTier(tier.String()); err != nil || p != tier {
			t.Errorf("tier name %q doesn't read back", tier)
		}
	}
	if _, err := ParseTier("best friend"); err == nil {
		t.Error("unknown tier names should be refused")
	}
	if clampBond(-5) != 0 || clampBond(250) != MaxBond {
		t.Error("bonds should stay between 0 and 100")
	}
	// Going up a tier step by step, as moments add to the bond.
	b, last := 0.0, Stranger
	for i := 0; i < 60; i++ {
		b = clampBond(b + 2)
		if TierOf(b) < last {
			t.Fatal("a growing bond should never drop a tier")
		}
		last = TierOf(b)
	}
	if last != YourCat {
		t.Errorf("after many good moments the cat should be yours, got %s", last)
	}
}

func TestEveryCatHasEnoughMoments(t *testing.T) {
	f := load(t)
	count := map[string]int{}
	tiers := map[string]map[Tier]bool{}
	for _, m := range f.ct.Moments {
		count[m.Cat]++
		if tiers[m.Cat] == nil {
			tiers[m.Cat] = map[Tier]bool{}
		}
		tiers[m.Cat][m.MinTier] = true
	}
	for _, d := range f.defs {
		if count[d.ID] < 6 {
			t.Errorf("%s has %d moments; at least 6 are needed", d.Name, count[d.ID])
		}
		if len(tiers[d.ID]) < 3 {
			t.Errorf("%s's moments should unlock across the tiers (only %d different)", d.Name, len(tiers[d.ID]))
		}
	}
}

// Every moment waits for an answer, so every moment has choices; its quiet
// "ignore" choice (used only if you leave the café with it open) is gentle.
func TestEveryMomentHasChoicesAndAGentleQuietAnswer(t *testing.T) {
	f := load(t)
	for _, m := range f.ct.Moments {
		if len(m.Choices) < 2 {
			t.Errorf("%s: a moment needs 2 to 5 choices (it waits for an answer)", m.ID)
			continue
		}
		if m.Conv.Ignore == nil {
			t.Errorf("%s: says nothing about its quiet answer", m.ID)
			continue
		}
		ig := m.Conv.IgnoreChoice() // counted from 0 once read
		e := m.Effect(ig)
		if e.Manner == "sudden" || e.Bond < 0 {
			t.Errorf("%s: its quiet answer is %q, which isn't gentle", m.ID, m.Conv.Choices[ig].Text)
		}
		if e.Then == "leave" {
			t.Errorf("%s: its quiet answer shouldn't send the cat away", m.ID)
		}
	}
}

func TestAMomentWithoutChoicesIsRefused(t *testing.T) {
	f := load(t)
	_, err := ParseContent("x.json", []byte(`{"cat": "brie", "moments": {"x": {"pages": ["Hi."]}}, "look": {}}`), Checks{Cats: f.defs, Spots: f.spots})
	if err == nil || !strings.Contains(err.Error(), "choices") {
		t.Fatalf("a moment without choices should be refused, pointing to observations; got %v", err)
	}
}

func TestEveryCatHasAwayLines(t *testing.T) {
	f := load(t)
	for _, d := range f.defs {
		got := map[string]bool{}
		for _, a := range f.ct.Away {
			if a.Cat == d.ID {
				got[a.Bond+"/"+a.Gone] = true
			}
		}
		for _, b := range []string{"new", "knows", "yours"} {
			for _, g := range []string{"short", "long"} {
				if !got[b+"/"+g] {
					t.Errorf("%s has no away line for bond %s, gone %s", d.Name, b, g)
				}
			}
		}
	}
}

func TestObservationsForLookAround(t *testing.T) {
	f := load(t)
	w := f.world(t, nil, 1)
	brie := w.Cat("brie")
	for _, c := range w.Cats {
		w.Put(c, "", "")
	}
	w.Put(brie, "sill", PoseSleep)
	w.Update(ctxAt(day, "afternoon"), 0.1)
	obs := w.ObservationsFor(f.ct, brie)
	if len(obs) == 0 {
		t.Fatal("asleep on the sill, Brie should have an observation that fits")
	}
	for _, o := range obs {
		if o.Cat != "brie" || (len(o.Poses) > 0 && !has(o.Poses, PoseSleep)) {
			t.Errorf("an observation that doesn't fit: %+v", o)
		}
	}
	if len(w.ObservationsFor(f.ct, w.Cat("hunter"))) != 0 {
		t.Error("a cat that isn't in the room has nothing to observe")
	}
}

func TestMomentEligibility(t *testing.T) {
	f := load(t)
	w := f.world(t, nil, 1)
	brie := w.Cat("brie")
	for _, c := range w.Cats {
		w.Put(c, "", "")
	}
	w.Put(brie, "sill", PoseSit)
	w.Update(ctxAt(day, "afternoon"), 0.1)

	rain := f.moment(t, "brie_rain")   // needs rain and the sill
	creme := f.moment(t, "brie_creme") // needs curious and a café crème
	table := f.moment(t, "brie_table") // needs friend and room on the table
	look := f.moment(t, "brie_look")   // only up to curious

	if !w.CanHave(rain, brie) {
		t.Error("brie_rain should fit: rain, Brie on the sill")
	}
	w.Update(Context{Now: day, Part: "afternoon", Weather: "clear"}, 0.1)
	if w.CanHave(rain, brie) {
		t.Error("brie_rain needs rain")
	}
	w.Update(Context{Now: day, Part: "afternoon", Weather: "rain", Drink: "cafe_creme"}, 0.1)
	if w.CanHave(creme, brie) {
		t.Error("brie_creme needs Brie to be at least curious")
	}
	brie.Bond = BondFor(Curious)
	if !w.CanHave(creme, brie) {
		t.Error("brie_creme should fit: curious, café crème on the table")
	}
	w.Update(Context{Now: day, Part: "afternoon", Weather: "rain", Drink: "earl_grey"}, 0.1)
	if w.CanHave(creme, brie) {
		t.Error("brie_creme needs a café crème, not Earl Grey")
	}
	if w.CanHave(table, brie) {
		t.Error("brie_table needs Brie to be a friend")
	}
	brie.Bond = BondFor(Friend)
	if !w.CanHave(table, brie) {
		t.Error("brie_table should fit for a friend with a free table")
	}
	if w.CanHave(look, brie) {
		t.Error("brie_look is only for strangers and curious guests")
	}
	// Someone else on the table: no room.
	w.Put(w.Cat("hunter"), "table", PoseSit)
	if w.CanHave(table, brie) {
		t.Error("brie_table can't happen while Hunter is on the table")
	}
	// A cat that's still fading in can't start a moment.
	w.Put(w.Cat("hunter"), "", "")
	brie.alpha, brie.fade = 0.5, 1
	if w.CanHave(table, brie) {
		t.Error("a cat must be settled before its moment")
	}
}

func TestPacing(t *testing.T) {
	f := load(t)
	rng := rand.New(rand.NewPCG(3, 4))
	for _, slow := range []float64{1, 2} {
		p := NewPacer(f.pace, day, rng)
		p.Slow = slow
		now := day
		for i := 0; i < 200; i++ {
			p.Moment(now)
			gap := p.NextMoment().Sub(now).Minutes()
			lo, hi := f.pace.MomentMinutes[0]*slow, f.pace.MomentMinutes[1]*slow
			if gap < lo-1e-9 || gap > hi+1e-9 {
				t.Fatalf("slow %v: moment after %.2f min, want %v..%v", slow, gap, lo, hi)
			}
			p.NotBefore(now.Add(time.Duration(hi+5) * time.Minute))
			if p.NextMoment().Before(now.Add(time.Duration(hi+5) * time.Minute)) {
				t.Fatal("NotBefore should push the next moment back")
			}
			now = now.Add(10 * time.Minute)
		}
	}
	if f.pace.MomentMinutes != [2]float64{5, 10} {
		t.Errorf("moments should come every 5-10 min (Nala's change, 8 Nov 2026), got %v", f.pace.MomentMinutes)
	}
	// Not every gap is the same: it shouldn't feel mechanical.
	p := NewPacer(f.pace, day, rng)
	gaps := map[int]bool{}
	for i := 0; i < 20; i++ {
		p.Moment(day)
		gaps[int(p.NextMoment().Sub(day).Seconds())] = true
	}
	if len(gaps) < 10 {
		t.Error("the gaps between moments should vary")
	}
}

// simulate runs the world for hours of café time in 2-second café steps,
// calling check after each step.
func simulate(w *World, from time.Time, hours float64, part func(time.Time) string, check func(time.Time)) {
	now := from
	steps := int(hours * 3600 / 2)
	for i := 0; i < steps; i++ {
		now = now.Add(2 * time.Second)
		w.Update(Context{Now: now, Part: part(now), Weather: "rain"}, 2)
		check(now)
	}
}

func partOf(t time.Time) string {
	switch h := t.Hour(); {
	case h < 5:
		return "late night"
	case h < 8:
		return "dawn"
	case h < 12:
		return "morning"
	case h < 17:
		return "afternoon"
	case h < 21:
		return "evening"
	}
	return "night"
}

func TestMimiKeepsHerDistanceFromStrangers(t *testing.T) {
	f := load(t)
	near := map[string]int{}
	for _, s := range f.room.Spots() {
		near[s.Name] = s.Near
	}
	for seed := uint64(1); seed <= 6; seed++ {
		w := f.world(t, map[string]float64{"brie": 80, "hunter": 80}, seed)
		mimi := w.Cat("mimi")
		visits := 0
		simulate(w, day, 24, partOf, func(now time.Time) {
			if mimi.Tier() != Stranger {
				mimi.Bond = 0 // she stays a stranger for this test
			}
			for _, s := range []string{mimi.Spot(), mimi.next} {
				if s != "" && near[s] > 0 {
					t.Fatalf("seed %d, %s: Mimi went to %s (near %d) as a stranger", seed, now.Format("15:04"), s, near[s])
				}
			}
			if mimi.Settled() {
				visits++
			}
			for _, m := range f.ct.Moments {
				if m.Cat == "mimi" && m.ComesTo != "" && w.CanHave(m, mimi) {
					t.Fatalf("seed %d: %s would bring Mimi to %s while she's a stranger", seed, m.ID, m.ComesTo)
				}
			}
		})
		if visits == 0 {
			t.Errorf("seed %d: in a whole day Mimi never came into the room", seed)
		}
	}
}

func TestMimiLikesCalmAndNotSudden(t *testing.T) {
	f := load(t)
	n := 0
	for _, m := range f.ct.Moments {
		if m.Cat != "mimi" {
			continue
		}
		for i, ch := range m.Choices {
			w := f.world(t, map[string]float64{"mimi": 30}, 5)
			mimi := w.Cat("mimi")
			for _, c := range w.Cats {
				w.Put(c, "", "")
			}
			w.Put(mimi, "shelf_top", PoseLoaf)
			before := mimi.Bond
			e := w.Answer(mimi, m, i)
			switch ch.Manner {
			case "calm":
				n++
				if mimi.Bond <= before {
					t.Errorf("%s choice %d is calm, but Mimi's bond went from %v to %v", m.ID, i+1, before, mimi.Bond)
				}
			case "sudden":
				n++
				if mimi.Bond >= before {
					t.Errorf("%s choice %d is sudden, but Mimi's bond went from %v to %v", m.ID, i+1, before, mimi.Bond)
				}
				if e.Then != "leave" || mimi.fade != -1 {
					t.Errorf("%s choice %d is sudden: Mimi should retreat", m.ID, i+1)
				}
			}
			// Even a moment written carelessly can't make a sudden choice
			// warm her up: the rule is in the code too.
			if ch.Manner == "sudden" && ch.Bond > 0 {
				t.Errorf("%s choice %d is sudden but its bond is +%v in the file", m.ID, i+1, ch.Bond)
			}
		}
	}
	if n < 10 {
		t.Errorf("Mimi's moments should offer calm and sudden choices (found %d)", n)
	}
	// Looking at her quietly helps a little, once every few minutes.
	w := f.world(t, nil, 9)
	mimi := w.Cat("mimi")
	for _, c := range w.Cats {
		w.Put(c, "", "")
	}
	w.Put(mimi, "shelf_top", PoseLoaf)
	w.Looked(mimi)
	if mimi.Bond != 1 {
		t.Errorf("looking at shy Mimi should add 1, got %v", mimi.Bond)
	}
	w.Looked(mimi)
	if mimi.Bond != 1 {
		t.Error("looking again at once shouldn't count")
	}
}

func TestTwoCatsNeverShareASpot(t *testing.T) {
	f := load(t)
	for seed := uint64(1); seed <= 8; seed++ {
		w := f.world(t, map[string]float64{"brie": 90, "hunter": 90, "mimi": 90}, seed)
		simulate(w, day, 24, partOf, func(now time.Time) {
			claimed := map[string]string{}
			var boxes []Rect
			for _, c := range w.Cats {
				s := c.claimed()
				if s == "" {
					continue
				}
				if other, ok := claimed[s]; ok {
					t.Fatalf("seed %d, %s: %s and %s both at %s", seed, now.Format("15:04"), other, c.Def.Name, s)
				}
				claimed[s] = c.Def.Name
				p := c.pose
				if c.next != "" {
					p = c.nextP
				}
				b, ok := f.room.Box(c.Def, p, s)
				if !ok {
					t.Fatalf("seed %d: %s %s at %s hangs off the edge", seed, c.Def.Name, p, s)
				}
				for _, o := range boxes {
					if o.overlaps(b) {
						t.Fatalf("seed %d, %s: %s overlaps another cat at %s", seed, now.Format("15:04"), c.Def.Name, s)
					}
				}
				boxes = append(boxes, b)
				if info, _ := w.spotInfo(s); !info.allows(p) {
					t.Fatalf("seed %d: %s is %s at %s, which doesn't allow it", seed, c.Def.Name, p, s)
				}
			}
		})
	}
}

func TestCatsFollowTheHour(t *testing.T) {
	f := load(t)
	asleep := map[string]int{}
	awake := map[string]int{}
	for seed := uint64(1); seed <= 4; seed++ {
		w := f.world(t, nil, seed)
		simulate(w, day, 48, partOf, func(now time.Time) {
			p := partOf(now)
			for _, c := range w.Cats {
				if !c.Settled() {
					continue
				}
				if c.pose == PoseSleep {
					asleep[p]++
				} else {
					awake[p]++
				}
			}
		})
	}
	share := func(p string) float64 { return float64(asleep[p]) / float64(asleep[p]+awake[p]+1) }
	if share("night") <= share("evening")*1.5 {
		t.Errorf("cats should sleep much more at night (%.2f) than in the evening (%.2f)", share("night"), share("evening"))
	}
	if share("late night") <= share("afternoon") {
		t.Errorf("late night (%.2f) should be sleepier than the afternoon (%.2f)", share("late night"), share("afternoon"))
	}
}

func TestFriendsComeCloserSooner(t *testing.T) {
	f := load(t)
	// How long until each cat first sits somewhere near your seat, as a
	// stranger and as "your cat", over several visits.
	firstNear := func(bond float64) float64 {
		total := 0.0
		for seed := uint64(1); seed <= 12; seed++ {
			w := f.world(t, map[string]float64{"brie": bond, "hunter": bond, "mimi": bond}, seed)
			found := 48.0 * 60
			now := day
			for i := 0; i < 6*1800 && found == 48*60; i++ {
				now = now.Add(2 * time.Second)
				w.Update(Context{Now: now, Part: "afternoon", Weather: "rain"}, 2)
				for _, c := range w.Cats {
					if info, ok := w.spotInfo(c.Spot()); ok && c.Settled() && info.Near >= 1 {
						found = now.Sub(day).Minutes()
					}
				}
			}
			total += found
		}
		return total / 12
	}
	if stranger, yours := firstNear(0), firstNear(85); yours >= stranger {
		t.Errorf("cats who know you should come close sooner (%.1f min) than with a stranger (%.1f min)", yours, stranger)
	}
}

func TestContentErrorsAreClear(t *testing.T) {
	f := load(t)
	ck := Checks{Cats: f.defs, Spots: f.spots, Weathers: []string{"rain"}, Drinks: []string{"cafe_creme"}}
	bad := []struct{ json, want string }{
		{`{"cat": "garfield", "moments": {}}`, "no cat with that id"},
		{`{"cat": "brie", "moments": {"x": {"tier": "bestie", "pages": ["Hi."]}}}`, "unknown tier"},
		{`{"cat": "brie", "moments": {"x": {"spots": ["roof"], "pages": ["Hi."]}}}`, "isn't a cat spot"},
		{`{"cat": "brie", "moments": {"x": {"weather": ["hail"], "pages": ["Hi."]}}}`, "weather"},
		{`{"cat": "brie", "moments": {"x": {"when": ["teatime"], "pages": ["Hi."]}}}`, "when"},
		{`{"cat": "brie", "moments": {"x": {"comes_to": "roof", "pages": ["Hi."]}}}`, "comes_to"},
		{`{"cat": "brie", "moments": {"x": {"pages": ["Hi."], "choices": [{"text": "a", "manner": "loud"}, {"text": "b"}], "ignore": 1}}}`, "manner"},
		{`{"cat": "brie", "observations": [{"poses": ["sit"]}]}`, "needs a \"text\""},
		{`{"cat": "brie", "moments": {"x": {"tier": "friend", "max_tier": "curious", "pages": ["Hi."]}}}`, "below"},
	}
	for _, b := range bad {
		_, err := ParseContent("test.json", []byte(b.json), ck)
		if err == nil || !strings.Contains(err.Error(), b.want) {
			t.Errorf("%s: got error %v, want one mentioning %q", b.json, err, b.want)
		}
	}
}

func TestLittleMovements(t *testing.T) {
	f := load(t)
	w := f.world(t, nil, 2)
	brie := w.Cat("brie")
	for _, c := range w.Cats {
		w.Put(c, "", "")
	}
	w.Put(brie, "sill", PoseSit)
	seen := map[string]bool{}
	now := day
	for i := 0; i < 3000; i++ {
		now = now.Add(100 * time.Millisecond)
		brie.until = now.Add(time.Hour)
		w.Update(Context{Now: now, Part: "evening", Weather: "rain"}, 0.1)
		for _, d := range w.Draw() {
			seen[d.Sprite] = true
		}
	}
	for _, s := range []string{"brie_sit", "brie_sit_blink", "brie_sit_ear", "brie_sit_tail"} {
		if !seen[s] {
			t.Errorf("in five minutes Brie never showed %s", s)
		}
	}
	// Asleep, she breathes; with reduced motion, only blinks remain awake.
	w.Put(brie, "sill", PoseSleep)
	seen = map[string]bool{}
	for i := 0; i < 600; i++ {
		now = now.Add(100 * time.Millisecond)
		brie.until = now.Add(time.Hour)
		w.Update(Context{Now: now, Part: "night", Weather: "rain"}, 0.1)
		for _, d := range w.Draw() {
			seen[d.Sprite] = true
		}
	}
	if !seen["brie_sleep_breathe"] || !seen["brie_sleep"] {
		t.Error("asleep, Brie should breathe slowly")
	}
	w.SetCalm(true)
	w.Put(brie, "sill", PoseSit)
	seen = map[string]bool{}
	for i := 0; i < 3000; i++ {
		now = now.Add(100 * time.Millisecond)
		brie.until = now.Add(time.Hour)
		w.Update(Context{Now: now, Part: "evening", Weather: "rain"}, 0.1)
		for _, d := range w.Draw() {
			seen[d.Sprite] = true
		}
	}
	if seen["brie_sit_tail"] || seen["brie_sit_ear"] {
		t.Error("with reduced motion, no tail flicks or ear twitches")
	}
}

// Every moment must be able to happen: some spot, pose, hour, weather and
// drink where its cat at its bond level can have it.
func TestEveryMomentCanHappen(t *testing.T) {
	f := load(t)
	perTier := map[string]map[Tier]int{}
	for _, m := range f.ct.Moments {
		if perTier[m.Cat] == nil {
			perTier[m.Cat] = map[Tier]int{}
		}
		if m.MinTier == m.MaxTier {
			perTier[m.Cat][m.MinTier]++
		}
		if m.ComesTo == "floor" && len(m.Choices) > 4 {
			t.Errorf("%s: comes to the floor with %d choices; the box would hide the cat", m.ID, len(m.Choices))
		}
		ok := false
		for _, spot := range f.spots {
			for _, pose := range Poses {
				w := f.world(t, nil, 1)
				for _, c := range w.Cats {
					w.Put(c, "", "")
				}
				c := w.Cat(m.Cat)
				c.Bond = BondFor(m.MinTier)
				if info, ok := w.spotInfo(spot); !ok || !info.allows(pose) || !w.free(c, spot, pose) {
					continue
				}
				w.Put(c, spot, pose)
				ctx := Context{Now: day, Part: "afternoon", Weather: "clear", Drink: "cafe_creme"}
				if len(m.When) > 0 {
					ctx.Part = m.When[0]
				}
				if len(m.Weather) > 0 {
					ctx.Weather = m.Weather[0]
				}
				if m.Drink != "" && m.Drink != "any" {
					ctx.Drink = m.Drink
				}
				w.Update(ctx, 0.1)
				if w.CanHave(m, c) {
					ok = true
				}
			}
		}
		if !ok {
			t.Errorf("%s can never happen", m.ID)
		}
	}
	for cat, tiers := range perTier {
		for tr := Stranger; tr <= YourCat; tr++ {
			if tiers[tr] < 10 {
				t.Errorf("%s has %d moments just for %s (10 wanted)", cat, tiers[tr], tr)
			}
		}
	}
}
