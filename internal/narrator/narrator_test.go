package narrator

import (
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/dialogue"
)

// The dialogue box: 3 lines of 46 letters.
const boxW, boxLines = 46, 3

var weathers = []string{"clear", "cloudy", "drizzle", "rain", "heavy_rain", "fog", "snow"}

func load(t *testing.T) *Content {
	t.Helper()
	c, err := Load(content.FS, "narrator", Checks{Parts: cats.PartsOfDay, Weathers: weathers})
	if err != nil {
		t.Fatal(err)
	}
	defs, err := cats.LoadDefs(content.FS, "cats")
	if err != nil {
		t.Fatal(err)
	}
	ct, err := cats.LoadContent(content.FS, "moments", cats.Checks{Cats: defs, Spots: []string{"sill", "shelf_top", "sideboard", "cushion", "table", "floor"}, Drinks: []string{"cafe_creme", "chocolat_chaud", "chai_latte", "earl_grey", "matcha_latte", "vin_chaud", "tisane_verveine"}})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string][]CatLine{}
	for _, l := range ct.Away {
		by[l.Cat] = append(by[l.Cat], CatLine{Bond: l.Bond, Gone: l.Gone, Text: l.Text})
	}
	for id, ls := range by {
		if err := c.AddCat(id, ls); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func fits(s string) bool { return len(dialogue.Wrap(s, boxW)) <= boxLines }

func TestEveryLineFitsTheBox(t *testing.T) {
	c := load(t)
	check := func(what string, ts dialogue.Text) {
		for _, s := range ts {
			if !fits(s) {
				t.Errorf("%s is %d lines (most is %d): %q", what, len(dialogue.Wrap(s, boxW)), boxLines, s)
			}
		}
	}
	for _, l := range c.Welcomes {
		check("welcome", l.Text)
	}
	for _, l := range c.Talk {
		check("small talk", l.Text)
	}
	for _, ch := range c.Changes {
		check("change", ch.Text)
	}
	// An opener and a cat line share one page when they fit together (if
	// not, they take a page each). Most should fit.
	total, split := 0, 0
	for _, gap := range c.Gaps {
		long := "long"
		if gap.UnderHours > 0 && gap.UnderHours <= c.LongFromHours {
			long = "short"
		}
		for _, o := range c.Openers {
			if o.Gap != gap.Name {
				continue
			}
			for _, id := range []string{"brie", "hunter", "mimi"} {
				for _, l := range c.CatLines(id) {
					if l.Gone != long {
						continue
					}
					for _, ot := range o.Text {
						for _, lt := range l.Text {
							total++
							if !fits(ot + " " + lt) {
								split++
							}
						}
					}
				}
			}
		}
	}
	if split*10 > total {
		t.Errorf("%d of %d away notes need two pages; most should fit on one", split, total)
	}
}

func TestEnoughToSay(t *testing.T) {
	c := load(t)
	count := func(ls []Line) int {
		n := 0
		for _, l := range ls {
			n += len(l.Text)
		}
		return n
	}
	if n := count(c.Welcomes); n < 25 {
		t.Errorf("only %d welcome lines", n)
	}
	if n := count(c.Talk); n < 30 {
		t.Errorf("only %d small talk lines", n)
	}
	nala := 0
	for _, l := range c.Talk {
		for _, s := range l.Text {
			if strings.Contains(s, "Nala") {
				nala++
			}
		}
	}
	if nala < 4 || nala > count(c.Talk)/3 {
		t.Errorf("Nala should come up lightly in small talk: %d of %d lines", nala, count(c.Talk))
	}
	for _, id := range []string{"brie", "hunter", "mimi"} {
		for _, b := range Bands {
			for _, g := range Lengths {
				ok := false
				for _, l := range c.CatLines(id) {
					ok = ok || (l.Bond == b && l.Gone == g)
				}
				if !ok {
					t.Errorf("%s has no away line for %s/%s", id, b, g)
				}
			}
		}
	}
}

func TestWelcomeFitsTheVisit(t *testing.T) {
	c := load(t)
	firsts, returns, regulars := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, l := range c.Welcomes {
		for _, s := range l.Text {
			switch l.Visit {
			case "first":
				firsts[s] = true
			case "return":
				returns[s] = true
			case "regular":
				regulars[s] = true
			}
		}
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for _, part := range cats.PartsOfDay {
		for _, w := range weathers {
			for i := 0; i < 20; i++ {
				if s := c.Welcome(Context{Part: part, Weather: w, First: true}, rng); !firsts[s] {
					t.Fatalf("first visit got a returning line: %q", s)
				}
				if s := c.Welcome(Context{Part: part, Weather: w}, rng); !returns[s] {
					t.Fatalf("a returning (not yet regular) guest got %q", s)
				}
				s := c.Welcome(Context{Part: part, Weather: w, Regular: true}, rng)
				if !returns[s] && !regulars[s] {
					t.Fatalf("a regular got %q", s)
				}
			}
		}
	}
	// Conditions are kept: no snow lines in the rain, no morning lines at night.
	for i := 0; i < 200; i++ {
		s := c.Welcome(Context{Part: "night", Weather: "rain"}, rng)
		if strings.Contains(s, "snow") || strings.Contains(s, "morning") || strings.Contains(s, "sun") {
			t.Fatalf("a rainy night welcome shouldn't be %q", s)
		}
	}
	// Regulars sometimes hear they're regulars.
	seen := false
	for i := 0; i < 300 && !seen; i++ {
		seen = regulars[c.Welcome(Context{Part: "afternoon", Weather: "cloudy", Regular: true}, rng)]
	}
	if !seen {
		t.Error("a regular should sometimes be welcomed as one")
	}
}

func TestGaps(t *testing.T) {
	c := load(t)
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{time.Minute, "minutes"},
		{59 * time.Minute, "minutes"},
		{3 * time.Hour, "hours"},
		{9 * time.Hour, "hours"},
		{20 * time.Hour, "day"},
		{47 * time.Hour, "day"},
		{3 * 24 * time.Hour, "days"},
		{6 * 24 * time.Hour, "days"},
		{8 * 24 * time.Hour, "week"},
		{400 * 24 * time.Hour, "week"},
	} {
		if got := c.GapOf(tc.d); got != tc.want {
			t.Errorf("%v away: %s, want %s", tc.d, got, tc.want)
		}
	}
}

func TestAwayNotes(t *testing.T) {
	c := load(t)
	rng := rand.New(rand.NewPCG(5, 6))
	cats := []Cat{{ID: "brie", Band: "knows", Bond: 40}, {ID: "hunter", Band: "new", Bond: 5}, {ID: "mimi", Band: "yours", Bond: 80}}
	for _, d := range []time.Duration{20 * time.Minute, 3 * time.Hour, 26 * time.Hour, 4 * 24 * time.Hour, 12 * 24 * time.Hour} {
		gap := c.GapOf(d)
		var openers []string
		for _, o := range c.Openers {
			if o.Gap == gap {
				openers = append(openers, o.Text...)
			}
		}
		for i := 0; i < 50; i++ {
			pages := c.AwayNote(d, cats, rng, fits)
			if len(pages) == 0 {
				t.Fatalf("%v: no away note", d)
			}
			ok := false
			for _, o := range openers {
				ok = ok || strings.HasPrefix(pages[0], o)
			}
			if !ok {
				t.Fatalf("%v: should open with a %s line: %q", d, gap, pages[0])
			}
			if i == 0 {
				t.Logf("%v away: %s", d, strings.Join(pages, " / "))
			}
		}
	}
	// Mostly about the cat who knows you best, sometimes another.
	mimi, two, short := 0, 0, 0
	for i := 0; i < 1000; i++ {
		p := c.AwayNote(4*24*time.Hour, cats, rng, fits)
		if strings.Contains(p[0], "Mimi") {
			mimi++
		}
		if len(p) == 2 {
			two++
		}
		if len(c.AwayNote(3*time.Hour, cats, rng, fits)) > 1 {
			short++
		}
	}
	if mimi < 550 || mimi > 900 {
		t.Errorf("Mimi (your cat) was in %d of 1000 notes; she should be in most, not all", mimi)
	}
	if two < 300 || two > 700 {
		t.Errorf("after days away, a second page %d times in 1000; expected about half", two)
	}
	if short != 0 {
		t.Error("a few hours away never gets a second page")
	}
	// The cat line fits how well the cat knows you and how long you were gone.
	for i := 0; i < 300; i++ {
		p := c.AwayNote(2*time.Hour, []Cat{{ID: "hunter", Band: "yours", Bond: 90}}, rng, fits)
		found := false
		for _, l := range c.CatLines("hunter") {
			for _, s := range l.Text {
				if strings.HasSuffix(p[0], s) {
					found = true
					if l.Bond != "yours" || l.Gone != "short" {
						t.Fatalf("2 hours away from your cat Hunter got a %s/%s line: %q", l.Bond, l.Gone, s)
					}
				}
			}
		}
		if !found {
			t.Fatalf("no Hunter line in %q", p[0])
		}
	}
}

func TestSmallTalkFollowsTheDayAndAvoidsRepeats(t *testing.T) {
	c := load(t)
	rng := rand.New(rand.NewPCG(7, 8))
	var recent []string
	keep := len(c.Talk) * 7 / 10
	for i := 0; i < 400; i++ {
		ctx := Context{Part: "night", Weather: "rain"}
		s := c.SmallTalk(ctx, recent, rng)
		for _, l := range c.Talk {
			for _, v := range l.Text {
				if v == s && !l.fits(ctx) {
					t.Fatalf("%q doesn't fit a rainy night", s)
				}
			}
		}
		for _, r := range recent {
			if r == s {
				eligible := 0
				for _, l := range c.Talk {
					if l.fits(ctx) {
						eligible += len(l.Text)
					}
				}
				if eligible > len(recent) {
					t.Fatalf("%q came back while other lines were unused", s)
				}
			}
		}
		recent = append(recent, s)
		if len(recent) > keep {
			recent = recent[len(recent)-keep:]
		}
	}
}

func TestPacing(t *testing.T) {
	p, err := LoadPacing(content.FS, "pacing.json")
	if err != nil {
		t.Fatal(err)
	}
	if p.SmallTalkMinutes != [2]float64{8, 15} || p.QuestionSeconds != 90 || p.CupKeptMinutes != 30 || p.AutosaveSeconds != 60 {
		t.Errorf("pacing changed: %+v", p)
	}
	rng := rand.New(rand.NewPCG(1, 1))
	for i := 0; i < 500; i++ {
		w := Wait(p.SmallTalkMinutes, 1, rng).Minutes()
		q := Wait(p.SmallTalkMinutes, 2, rng).Minutes()
		if w < 8 || w > 15 || q < 16 || q > 30 {
			t.Fatalf("waits out of range: %v, quiet %v", w, q)
		}
	}
}

func TestContentErrorsAreClear(t *testing.T) {
	c := &Content{}
	c.Welcomes = []Line{{Text: dialogue.Text{"Hi."}, Visit: "sometimes"}}
	c.Talk = []Line{{Text: dialogue.Text{"Hm."}}}
	if err := c.check("narrator", Checks{}); err == nil || !strings.Contains(err.Error(), "visit") {
		t.Errorf("a bad visit should be named: %v", err)
	}
	c.Welcomes = []Line{{Text: dialogue.Text{"Hi."}, Visit: "first"}}
	if err := c.check("narrator", Checks{}); err == nil || !strings.Contains(err.Error(), "\"return\"") {
		t.Errorf("a missing plain return welcome should be named: %v", err)
	}
	if err := c.AddCat("x", []CatLine{{Bond: "bestie", Gone: "short", Text: dialogue.Text{"x"}}}); err == nil {
		t.Error("a bad bond on an away line should be refused")
	}
}
