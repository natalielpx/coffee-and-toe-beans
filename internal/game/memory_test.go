package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/drinks"
	"coffee-and-toe-beans/internal/save"
)

// visit starts a game at realNow with a save, keeping whatever it stores,
// and steps inside.
func visit(t *testing.T, d *save.Data, realNow time.Time, seed uint64) (*Game, *[]*save.Data) {
	t.Helper()
	var stored []*save.Data
	store := func(s *save.Data) error {
		raw, err := os.ReadFile(writeTemp(t, s))
		if err != nil {
			return err
		}
		c, err := save.Parse(raw) // a real round trip, as on disk
		if err != nil {
			return err
		}
		stored = append(stored, c)
		return nil
	}
	g, err := New(Config{Seed: seed, Save: d, Store: store}, content.FS, realNow)
	if err != nil {
		t.Fatal(err)
	}
	g.Resize(FrameW, FrameH)
	g.Key(KeyConfirm)
	g.Key(KeyConfirm)
	return g, &stored
}

func writeTemp(t *testing.T, d *save.Data) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "save.json")
	if err := save.Write(p, d); err != nil {
		t.Fatal(err)
	}
	return p
}

// hello waits for the welcome box and returns all its pages.
func hello(t *testing.T, g *Game, now time.Time) ([]string, time.Time) {
	t.Helper()
	now = run(g, now, 3)
	if g.box == nil {
		t.Fatal("no welcome")
	}
	return g.box.Pages(), now
}

// leaveCafe leaves through the Esc menu, as a player does.
func leaveCafe(t *testing.T, g *Game, now time.Time) time.Time {
	t.Helper()
	g.Key(KeyBack)
	for g.pause.items[g.pause.sel] != actLeave {
		g.Key(KeyDown)
	}
	g.Key(KeyConfirm)
	now = run(g, now, 3)
	if !g.Done() {
		t.Fatal("couldn't leave")
	}
	return now
}

func last(s *[]*save.Data) *save.Data {
	if len(*s) == 0 {
		return nil
	}
	return (*s)[len(*s)-1]
}

func TestFirstVisitThenComingBack(t *testing.T) {
	day := time.Date(2026, 11, 7, 15, 0, 0, 0, time.Local)
	g, stored := visit(t, nil, day, 3)
	if !g.FirstVisit() {
		t.Fatal("with no save it's a first visit")
	}
	pages, now := hello(t, g, day)
	if len(pages) != 2 {
		t.Errorf("a first visit is a welcome and the drink offer, got %q", pages)
	}
	// Order a café crème three times and get to know Brie a little.
	for i := 0; i < 3; i++ {
		g.serve(mustDrink(t, g, "cafe_creme"))
	}
	g.Cats().Cat("brie").Bond = 30
	g.cat.seen["brie_sigh"] = g.Now()
	now = run(g, now, 120)
	now = leaveCafe(t, g, now)

	d := last(stored)
	if d == nil {
		t.Fatal("leaving should save")
	}
	if d.Visits != 1 || !d.Returning() || d.Seconds < 120 {
		t.Errorf("visit not remembered: %+v", d)
	}
	if b := d.Bonds["brie"]; b < 30 || b > 31 {
		t.Errorf("Brie's bond should be saved (about 30), got %v", b)
	}
	if d.Favourite(3) != "cafe_creme" {
		t.Errorf("café crème ordered 3 times should be the favourite: %v", d.Orders)
	}
	if _, ok := d.Seen["brie_sigh"]; !ok {
		t.Error("seen moments should be saved")
	}

	// Three hours later.
	back := now.Add(3 * time.Hour)
	g2, _ := visit(t, d, back, 4)
	if g2.FirstVisit() {
		t.Fatal("coming back isn't a first visit")
	}
	if a := g2.Away(); a < 3*time.Hour-time.Minute || a > 3*time.Hour+time.Minute {
		t.Errorf("away for %v, want about 3h", a)
	}
	if b := g2.Cats().Cat("brie").Bond; b < 30 {
		t.Errorf("Brie should remember you: bond %v", b)
	}
	if _, ok := g2.cat.seen["brie_sigh"]; !ok {
		t.Error("seen moments should come back")
	}
	pages, _ = hello(t, g2, back)
	joined := strings.Join(pages, " / ")
	if len(pages) < 3 {
		t.Fatalf("coming back: welcome, away note, offer; got %q", pages)
	}
	opened := false
	for _, o := range g2.mem.narr.Openers {
		if o.Gap == "hours" {
			for _, s := range o.Text {
				opened = opened || strings.Contains(joined, s)
			}
		}
	}
	if !opened {
		t.Errorf("3 hours away should get a few-hours note: %q", joined)
	}
	if g2.mem.data.Visits != 2 {
		t.Errorf("second visit counted as %d", g2.mem.data.Visits)
	}
	g2.openDrinks(false)
	if g2.drinks.Drinks[g2.menu.sel].ID != "cafe_creme" {
		t.Error("the drinks menu should open on your favourite")
	}
}

func mustDrink(t *testing.T, g *Game, id string) *drinks.Drink {
	t.Helper()
	d, ok := g.drinks.Get(id)
	if !ok {
		t.Fatal("no drink", id)
	}
	return d
}

func TestAwayNotesScaleWithTheAbsence(t *testing.T) {
	gaps := map[time.Duration]string{
		10 * time.Minute:    "minutes",
		5 * time.Hour:       "hours",
		30 * time.Hour:      "day",
		4 * 24 * time.Hour:  "days",
		10 * 24 * time.Hour: "week",
	}
	day := time.Date(2026, 11, 7, 18, 0, 0, 0, time.Local)
	for away, gap := range gaps {
		d := save.New()
		d.LastVisit = day.Add(-away)
		d.Bonds = map[string]float64{"brie": 50, "hunter": 20, "mimi": 75}
		g, _ := visit(t, d, day, uint64(away))
		pages, _ := hello(t, g, day)
		joined := strings.Join(pages, " / ")
		ok := false
		for _, o := range g.mem.narr.Openers {
			if o.Gap == gap {
				for _, s := range o.Text {
					ok = ok || strings.Contains(joined, s)
				}
			}
		}
		if !ok {
			t.Errorf("%v away should get a %s opener: %q", away, gap, joined)
		}
		if !strings.Contains(joined, "Brie") && !strings.Contains(joined, "Hunter") && !strings.Contains(joined, "Mimi") {
			t.Errorf("an away note names a cat: %q", joined)
		}
		t.Logf("%v: %s", away, joined)
	}
}

func TestYourCupIsStillThereIfYouComeBackSoon(t *testing.T) {
	day := time.Date(2026, 11, 7, 15, 0, 0, 0, time.Local)
	g, stored := visit(t, nil, day, 5)
	now := run(g, day, 4)
	g.serve(mustDrink(t, g, "chai_latte"))
	now = run(g, now, 5*60) // five minutes with it
	level := g.drinks.Level(g.order.order, g.Now())
	heat := g.drinks.Heat(g.order.order, g.Now())
	now = leaveCafe(t, g, now)
	d := last(stored)
	if d.Cup == nil || d.Cup.Drink != "chai_latte" {
		t.Fatalf("the cup on the table should be saved: %+v", d.Cup)
	}

	// Back 10 minutes later: the same cup, as full as you left it, cooler.
	g2, _ := visit(t, d, now.Add(10*time.Minute), 6)
	if g2.order == nil || g2.order.drink.ID != "chai_latte" {
		t.Fatal("back within 30 minutes, your drink should still be on the table")
	}
	if l := g2.drinks.Level(g2.order.order, g2.Now()); l != level {
		t.Errorf("nobody drank it while you were away: level %v, was %v", l, level)
	}
	if h := g2.drinks.Heat(g2.order.order, g2.Now()); h >= heat {
		t.Errorf("it should have cooled: heat %v, was %v", h, heat)
	}
	pages, _ := hello(t, g2, now.Add(10*time.Minute))
	if g2.box.kind != talkNarrator || len(g2.box.Choices()) != 0 {
		t.Error("with your drink still there, nobody offers another")
	}
	kept, _ := g2.talk.Get(convCupKept)
	found := false
	for _, s := range kept.Pages[0] {
		found = found || strings.Contains(strings.Join(pages, " "), strings.ReplaceAll(s, "{drink}", "chai latte"))
	}
	if !found {
		t.Errorf("the café should mention the drink is still there: %q", pages)
	}

	// Back an hour later: the table starts empty.
	g3, _ := visit(t, d, now.Add(time.Hour), 7)
	if g3.order != nil {
		t.Error("after an hour away, the table starts empty")
	}
	hello(t, g3, now.Add(time.Hour))
	if len(g3.box.Choices()) == 0 {
		t.Error("with an empty table, the café offers a drink")
	}
}

func TestAnEmptyCupIsNotKept(t *testing.T) {
	day := time.Date(2026, 11, 7, 15, 0, 0, 0, time.Local)
	g, stored := visit(t, nil, day, 5)
	g.serve(mustDrink(t, g, "cafe_creme"))
	now := run(g, day, 16*60)
	now = leaveCafe(t, g, now)
	if last(stored).Cup != nil {
		t.Error("an empty cup isn't saved")
	}
}

func TestAutosave(t *testing.T) {
	g, stored := visit(t, nil, t0, 2)
	run(g, t0, 125)
	if n := len(*stored); n != 2 {
		t.Errorf("about 2 minutes inside should autosave twice, got %d", n)
	}
	if last(stored).Seconds < 59 {
		t.Error("autosaves keep the time spent")
	}
}

func TestCtrlCStillSaves(t *testing.T) {
	g, stored := visit(t, nil, t0, 2)
	run(g, t0, 10)
	g.Key(KeyQuit)
	if len(*stored) != 1 || !g.Done() {
		t.Error("Ctrl+C should save and close")
	}
}

func TestABrokenSaveStartsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.json")
	good := writeTemp(t, func() *save.Data {
		d := save.New()
		d.LastVisit = t0
		d.Bonds["brie"] = 60
		return d
	}())
	raw, _ := os.ReadFile(good)
	if err := os.WriteFile(path, raw[:len(raw)/3], 0o644); err != nil { // cut off mid-write
		t.Fatal(err)
	}
	d, st, backup, err := save.Load(path, t0)
	if st != save.Broken || err != nil || backup == "" {
		t.Fatalf("a cut-off save should be found broken and backed up: %v %v %q", st, err, backup)
	}
	g, stored := visit(t, d, t0, 1)
	if !g.FirstVisit() || g.Cats().Cat("brie").Bond != 0 {
		t.Error("after a broken save the café starts fresh")
	}
	run(g, t0, 5)
	g.Key(KeyQuit)
	if len(*stored) != 1 {
		t.Error("and it can save again")
	}
}

func TestPretendingToBeAway(t *testing.T) {
	g, err := New(Config{Away: 8 * 24 * time.Hour, Seed: 1}, content.FS, t0)
	if err != nil {
		t.Fatal(err)
	}
	if g.FirstVisit() || g.Away() != 8*24*time.Hour {
		t.Errorf("--away should make it a return after that long: first %v, away %v", g.FirstVisit(), g.Away())
	}
}

func TestTheFavouriteIsMentioned(t *testing.T) {
	day := time.Date(2026, 11, 7, 15, 0, 0, 0, time.Local)
	mentioned := 0
	for seed := uint64(1); seed <= 30; seed++ {
		d := save.New()
		d.LastVisit = day.Add(-5 * time.Hour)
		d.Orders["earl_grey"] = 4
		g, _ := visit(t, d, day, seed)
		pages, _ := hello(t, g, day)
		if strings.Contains(pages[len(pages)-1], "Earl Grey") {
			mentioned++
		}
		if len(g.box.Choices()) != 2 {
			t.Fatal("the favourite is only mentioned: the choices stay the same")
		}
	}
	if mentioned < 5 || mentioned > 25 {
		t.Errorf("the favourite was mentioned %d times in 30 visits; about half expected", mentioned)
	}
}

func TestNoSmallTalkDuringAMoment(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Bonds: map[string]float64{"brie": 50}})
	now = answerHello(t, g, now, 1)
	if err := g.PinCat("brie", "sit", "sill", ""); err != nil {
		t.Fatal(err)
	}
	g.pinned = nil
	if err := g.ShowMoment("brie_sigh"); err != nil {
		t.Fatal(err)
	}
	box := g.box
	run(g, now, 120) // an hour of café time, at --fast
	if g.box != box || len(g.queue) != 0 {
		t.Error("nothing else is said, or queued, while a cat's moment waits")
	}
}

// TestTwoHourFastSession is the acceptance run: two real hours at --fast
// (two and a half café days), saving to a real file every minute, leaving
// and coming back half-way, pressing keys all along (Look around too).
// Nothing may panic; every frame keeps its size.
func TestTwoHourFastSession(t *testing.T) {
	if testing.Short() {
		t.Skip("long session")
	}
	path := filepath.Join(t.TempDir(), "coffee-and-toe-beans", "save.json")
	store := func(d *save.Data) error { return save.Write(path, d) }
	start := func(now time.Time) *Game {
		d, _, _, err := save.Load(path, now)
		if err != nil {
			t.Fatal(err)
		}
		g, err := New(Config{Fast: true, Save: d, Store: store, Seed: uint64(now.Unix())}, content.FS, now)
		if err != nil {
			t.Fatal(err)
		}
		g.Resize(FrameW, FrameH)
		g.Key(KeyConfirm)
		return g
	}
	now := t0
	g := start(now)
	for i := 0; i < 2*60*60; i++ {
		now = now.Add(time.Second)
		g.Tick(now)
		switch {
		case i == 60*60: // leave, and come back 20 minutes later
			g.Key(KeyQuit)
			now = now.Add(20 * time.Minute)
			g = start(now)
			if g.FirstVisit() {
				t.Fatal("coming back after an hour shouldn't be a first visit")
			}
		case i%600 == 300: // look around for a bit
			g.Key(KeyBack)
			g.Key(KeyDown)
			g.Key(KeyConfirm)
			g.Key(KeyRight)
			g.Key(KeyDown)
			g.Key(KeyConfirm)
		case i%600 == 320:
			g.Key(KeyBack)
		case i%37 == 0: // answer whatever is on screen
			g.Key(KeyConfirm)
		case i%53 == 0:
			g.Key(KeyDown)
		}
		if i%97 == 0 {
			checkSize(t, g.View(), FrameW, FrameH)
		}
	}
	if g.Done() {
		t.Fatal("the game closed on its own")
	}
	g.Key(KeyQuit)
	d, st, _, _ := save.Load(path, now)
	if st != save.Loaded || d.Visits != 2 || len(d.Orders) == 0 || len(d.Seen) == 0 {
		t.Errorf("after two visits the save should hold them: status %v, %+v", st, d)
	}
	t.Logf("after 2 hours: bonds %v, orders %v, %d moments seen, %.0f min inside", d.Bonds, d.Orders, len(d.Seen), d.Seconds/60)
}
