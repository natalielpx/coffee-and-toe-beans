package journal

import (
	"encoding/json"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
)

var at = time.Date(2026, 11, 18, 7, 40, 0, 0, time.UTC)

func load(t *testing.T) *Content {
	t.Helper()
	c, err := Load(content.FS)
	if err != nil {
		t.Fatalf("the journal couldn't load its content: %v", err)
	}
	return c
}

func TestEveryCatHasEntriesAndACard(t *testing.T) {
	c := load(t)
	for _, cat := range []string{"brie", "hunter", "mimi"} {
		if _, ok := c.Card(cat); !ok {
			t.Errorf("%s has no leaflet card", cat)
		}
		if _, ok := c.Cats[cat]; !ok {
			t.Errorf("%s has no entries", cat)
		}
	}
	if len(c.Leaflets) != 3 {
		t.Errorf("want 3 cards, got %d", len(c.Leaflets))
	}
}

func TestEveryJournalMomentHasEntryText(t *testing.T) {
	c := load(t)
	ids := []string{"brie_table", "brie_purr", "hunter_table", "hunter_yourcat", "mimi_table", "mimi_lap"}
	if err := c.CheckMoments(ids); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckMoments([]string{"no_such_moment"}); err == nil {
		t.Error("a moment with no text should be reported")
	}
}

func TestHeader(t *testing.T) {
	if got, want := Header(at, "clear"), "Wednesday 18 November, 07:40. Clear."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := Header(at, "heavy_rain"); !strings.HasSuffix(got, ". Heavy rain.") {
		t.Errorf("heavy rain header: %q", got)
	}
	if got := Header(at, "unknown"); got != "Wednesday 18 November, 07:40." {
		t.Errorf("unknown weather should give no word: %q", got)
	}
}

func TestOpenShowsTheDiscoveryLineOnce(t *testing.T) {
	c := load(t)
	var s State
	if got := s.Open(c); got != c.Text.FirstTime {
		t.Errorf("first open: %q", got)
	}
	if got := s.Open(c); got != c.Text.TookDown {
		t.Errorf("second open: %q", got)
	}
}

func TestEachMilestoneWritesOnce(t *testing.T) {
	c := load(t)
	var s State
	rng := rand.New(rand.NewPCG(1, 1))
	lines := c.Cats["brie"]
	text, _ := lines.Step("curious")
	if _, ok := s.Record(c, StepKey("brie", "curious"), "brie", text, at, "clear", "", rng); !ok {
		t.Fatal("first step should write")
	}
	if _, ok := s.Record(c, StepKey("brie", "curious"), "brie", text, at, "clear", "", rng); ok {
		t.Error("the same step must not write twice (a drop and a climb back up)")
	}
	if len(s.Entries) != 1 {
		t.Errorf("want 1 entry, got %d", len(s.Entries))
	}
}

func TestEmptyTextWritesNothing(t *testing.T) {
	c := load(t)
	var s State
	if _, ok := s.Record(c, "x", "x", "", at, "clear", "", nil); ok {
		t.Error("a missing line should not write an entry")
	}
}

func TestNewestFirstAndAtMostTwoPerPage(t *testing.T) {
	c := load(t)
	var s State
	rng := rand.New(rand.NewPCG(2, 1))
	for _, cat := range []string{"brie", "hunter", "mimi"} {
		for _, step := range []string{"meet", "curious", "familiar", "friend", "your_cat", "bad"} {
			text, _ := c.Cats[cat].Step(step)
			s.Record(c, cat+":"+step, cat, text, at, "rain", "", rng)
		}
	}
	spreads := s.Spreads()
	if len(spreads) == 0 {
		t.Fatal("no spreads")
	}
	first := spreads[0].Right.Entries[0]
	if first.Text != s.Entries[len(s.Entries)-1].Text {
		t.Error("the first entry after the first spread should be the newest")
	}
	seen := 0
	for _, sp := range spreads {
		for _, p := range []Page{sp.Left, sp.Right} {
			if len(p.Entries) > 2 {
				t.Errorf("a page has %d entries", len(p.Entries))
			}
			lines := 0
			for i, e := range p.Entries {
				if i > 0 {
					lines++
				}
				lines += EntryLines(e)
			}
			if lines > PageLines {
				t.Errorf("a page takes %d lines; the limit is %d", lines, PageLines)
			}
			seen += len(p.Entries)
		}
	}
	if seen != len(s.Entries) {
		t.Errorf("laid out %d entries, wrote %d", seen, len(s.Entries))
	}
}

func TestNoSpreadsBeforeAnyEntry(t *testing.T) {
	var s State
	if got := s.Spreads(); len(got) != 0 {
		t.Errorf("want no spreads, got %d", len(got))
	}
}

func TestAsidesFitTheContext(t *testing.T) {
	c := load(t)
	rng := rand.New(rand.NewPCG(3, 1))
	var s State
	ctx := Context{Weather: "rain", When: "morning"}
	for i := 0; i < 200; i++ {
		text := s.pickAside(c, ctx, rng)
		if text == "" {
			continue
		}
		if text != "It's raining enough to justify using an umbrella, but somehow Parisians don't." {
			t.Fatalf("an aside for another weather was picked: %q", text)
		}
	}
}

func TestAsidesAreNotRepeatedUntilTheyRunOut(t *testing.T) {
	c := load(t)
	rng := rand.New(rand.NewPCG(4, 1))
	var s State
	ctx := Context{Weather: "clear", When: "evening"} // fits clear_night and lamps
	var picks []string
	for len(picks) < 30 {
		if text := s.pickAside(c, ctx, rng); text != "" {
			picks = append(picks, text)
		}
	}
	fitting := map[string]bool{}
	for _, a := range c.Asides {
		if a.fits(ctx) {
			fitting[a.Text] = true
		}
	}
	if len(fitting) != 2 {
		t.Fatalf("test expects two fitting asides, got %d", len(fitting))
	}
	// Between two picks of the same aside, the other one must have come up.
	for i, p := range picks {
		for j := i - 1; j >= 0; j-- {
			if picks[j] == p {
				between := map[string]bool{}
				for _, q := range picks[j+1 : i] {
					between[q] = true
				}
				for other := range fitting {
					if other != p && !between[other] {
						t.Fatalf("%q repeated before %q was used", p, other)
					}
				}
				break
			}
		}
	}
}

func TestAsideFromAnotherDrinkDoesNotShow(t *testing.T) {
	c := load(t)
	rng := rand.New(rand.NewPCG(5, 1))
	var s State
	for i := 0; i < 200; i++ {
		text := s.pickAside(c, Context{Weather: "cloudy", When: "morning", Drink: "cafe_creme"}, rng)
		if strings.Contains(text, "Earl Grey") || strings.Contains(text, "matcha") {
			t.Fatalf("a drink aside showed for another drink: %q", text)
		}
	}
}

func TestEntryTextHasNoNumbersOrTiers(t *testing.T) {
	c := load(t)
	digits := regexp.MustCompile(`[0-9]`)
	bond := regexp.MustCompile(`(?i)\b(bond|stranger|tier)\b`)
	check := func(where, s string) {
		if digits.MatchString(s) || bond.MatchString(s) {
			t.Errorf("%s has a number or a tier name: %q", where, s)
		}
	}
	for cat, lines := range c.Cats {
		for _, step := range []string{"meet", "curious", "familiar", "friend", "your_cat", "bad"} {
			text, _ := lines.Step(step)
			check(cat+" "+step, text)
		}
	}
	for id, text := range c.Moments {
		check(id, text)
	}
	for _, a := range c.Asides {
		check(a.ID, a.Text)
	}
}

func TestListsHaveNoFullStops(t *testing.T) {
	c := load(t)
	for _, l := range c.Leaflets {
		for _, item := range append(append([]string{}, l.Likes...), l.Dislikes...) {
			if strings.HasSuffix(item, ".") {
				t.Errorf("%s: list item %q ends with a full stop", l.Cat, item)
			}
		}
	}
}

func TestCardLinesFitTheWidth(t *testing.T) {
	c := load(t)
	for _, l := range c.Leaflets {
		card := l.Lines(Width)
		check := func(line string) {
			if n := len([]rune(line)); n > Width {
				t.Errorf("%s: line of %d letters: %q", l.Cat, n, line)
			}
		}
		check(card.Name + card.Age)
		for _, r := range card.Body {
			check(r.Text)
		}
		for _, line := range card.Comment {
			check(line)
		}
		if len(card.Comment) == 0 {
			t.Errorf("%s: the card has no comment", l.Cat)
		}
	}
}

func TestSaveRoundTrip(t *testing.T) {
	c := load(t)
	var s State
	rng := rand.New(rand.NewPCG(6, 1))
	text, _ := c.Cats["mimi"].Step("bad")
	s.Record(c, BadKey("mimi"), "mimi", text, at, "fog", "", rng)
	s.Open(c)
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var back State
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Discovered || len(back.Entries) != 1 || back.Entries[0].Text != text {
		t.Errorf("the journal didn't survive a save: %+v", back)
	}
	if !back.Written(BadKey("mimi")) {
		t.Error("the first bad moment should still count as written after a save")
	}
}

func TestWrap(t *testing.T) {
	got := Wrap("one two three four", 9)
	want := []string{"one two", "three", "four"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
