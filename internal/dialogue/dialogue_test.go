package dialogue

import (
	"math/rand/v2"
	"strings"
	"testing"

	"coffee-and-toe-beans/internal/canvas"
)

func opts() Options {
	o := DefaultOptions()
	o.Fade = 0.2
	return o
}

// step runs the box for some seconds at 10 frames a second.
func step(b *Box, seconds float64) {
	for i := 0; i < int(seconds*10+0.5); i++ {
		b.Update(0.1)
	}
}

func TestWrap(t *testing.T) {
	got := Wrap("Rain gathers on the zinc roofs, and the lamps hum.", 20)
	want := []string{"Rain gathers on the", "zinc roofs, and the", "lamps hum."}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q", got)
	}
	for _, l := range Wrap("Café crème — très bien. Ça va ? Un chocolat chaud, s'il vous plaît.", 12) {
		if canvas.Width(l) > 12 {
			t.Errorf("line %q is too wide", l)
		}
	}
	if got := Wrap("one\ntwo", 40); len(got) != 2 {
		t.Errorf("a newline should start a new line: %q", got)
	}
	if got := Wrap("abcdefghij", 4); strings.Join(got, "|") != "abcd|efgh|ij" {
		t.Errorf("a very long word should be cut: %q", got)
	}
}

func TestTypesLetterByLetter(t *testing.T) {
	b := Start("", []string{"Hello there, cat."}, nil, 0, opts())
	step(b, 0.2) // fade in
	if got := strings.Join(b.Lines(), ""); got != "" {
		t.Errorf("nothing should be typed during the fade in, got %q", got)
	}
	step(b, 0.2)
	first := strings.Join(b.Lines(), "")
	if first == "" || first == "Hello there, cat." {
		t.Fatalf("after a moment some, not all, letters should show: %q", first)
	}
	if b.MoreToCome() {
		t.Error("▼ only shows once the page has finished")
	}
	step(b, 2)
	if got := strings.Join(b.Lines(), ""); got != "Hello there, cat." {
		t.Errorf("should finish typing, got %q", got)
	}
	if !b.MoreToCome() {
		t.Error("▼ should show when the page is finished")
	}
}

func TestPunctuationTakesABreath(t *testing.T) {
	o := opts()
	o.Fade = 0
	plain := Start("", []string{"aaaaaaaaaaaaaaaaaaaa"}, nil, 0, o)
	comma := Start("", []string{"aaaa. aaaa, aaaa. aa"}, nil, 0, o)
	tPlain, tComma := 0, 0
	for !plain.Finished() {
		plain.Update(0.01)
		tPlain++
	}
	for !comma.Finished() {
		comma.Update(0.01)
		tComma++
	}
	if tComma <= tPlain+50 {
		t.Errorf("punctuation should add little pauses: %d vs %d", tComma, tPlain)
	}
}

func TestEnterSkipsThenAdvances(t *testing.T) {
	b := Start("", []string{"First page, quite long so it takes a while.", "Second page."}, nil, 0, opts())
	step(b, 0.3)
	b.Confirm()
	if got := strings.Join(b.Lines(), " "); got != "First page, quite long so it takes a while." {
		t.Fatalf("Enter while typing should show the whole page, got %q", got)
	}
	if b.PageLines()[0] != b.Lines()[0] {
		t.Error("page lines and shown lines should match once finished")
	}
	b.Confirm()
	if strings.Join(b.Lines(), "") != "" {
		t.Error("Enter on a finished page should turn to the next, starting empty")
	}
	b.Confirm() // skip
	b.Confirm() // close
	if !b.Answered() {
		t.Fatal("Enter on the last page should close the box")
	}
	step(b, 1)
	r, done := b.Done()
	if !done || r.Ignored || r.Choice != -1 {
		t.Errorf("got %+v done=%v", r, done)
	}
}

func TestChoices(t *testing.T) {
	ch := []Choice{{Text: "Order a drink", Do: "order"}, {Text: "Not just now"}}
	b := Start("", []string{"Something warm?"}, ch, 1, opts())
	if b.ShowingChoices() {
		t.Error("choices wait for the question to finish typing")
	}
	b.Down() // ignored while typing
	step(b, 2)
	if !b.ShowingChoices() || b.MoreToCome() {
		t.Fatal("choices should show (and no ▼) once typed")
	}
	if b.Selected() != 0 {
		t.Error("arrows pressed before the choices appeared should not count")
	}
	b.Up()
	if b.Selected() != 0 {
		t.Error("Up at the top stays at the top")
	}
	b.Down()
	b.Down()
	if b.Selected() != 1 {
		t.Error("Down at the bottom stays at the bottom")
	}
	b.Up()
	b.Confirm()
	step(b, 1)
	r, done := b.Done()
	if !done || r.Choice != 0 || r.Do != "order" || r.Ignored {
		t.Errorf("got %+v", r)
	}
}

func TestIgnoredChoicesPassGently(t *testing.T) {
	ch := []Choice{{Text: "Yes", Do: "order"}, {Text: "No"}}
	o := opts()
	b := Start("", []string{"Another?"}, ch, 1, o)
	step(b, o.Patience-5)
	if b.Answered() {
		t.Fatal("too early to give up")
	}
	b.Down() // a key press resets the patience
	step(b, o.Patience-5)
	if b.Answered() {
		t.Fatal("a key press should give the player more time")
	}
	step(b, 10)
	r, done := b.Done()
	if !done || !r.Ignored || r.Choice != 1 || r.Do != "" {
		t.Errorf("ignoring should take the ignore choice: %+v done=%v", r, done)
	}
}

func TestIgnoredPagesMoveOn(t *testing.T) {
	b := Start("", []string{"One.", "Two."}, nil, 0, opts())
	step(b, 40)
	r, done := b.Done()
	if !done || !r.Ignored {
		t.Errorf("unread pages should move on by themselves: %+v done=%v", r, done)
	}
}

func TestPausedBoxWaits(t *testing.T) {
	b := Start("", []string{"Hold on."}, nil, 0, opts())
	b.SetPaused(true)
	step(b, 100)
	if b.Answered() || b.Alpha() > 0 {
		t.Error("a paused box should not move")
	}
	b.SetPaused(false)
	step(b, 30)
	if _, done := b.Done(); !done {
		t.Error("should carry on once unpaused")
	}
}

func TestLongTextBecomesPages(t *testing.T) {
	o := opts()
	o.Width, o.Lines = 10, 2
	b := Start("", []string{"one two three four five six seven"}, nil, 0, o)
	if len(b.pages) != 2 || b.MaxPageLines() != 2 {
		t.Errorf("long text should split into pages of 2 lines: %q", b.pages)
	}
}

func TestInstantText(t *testing.T) {
	o := opts()
	o.Speed, o.Fade = 0, 0
	b := Start("", []string{"All at once."}, nil, 0, o)
	b.Update(0.01)
	if !b.Finished() {
		t.Error("speed 0 shows the page at once")
	}
}

const sample = `{
  "conversations": {
    "offer": {
      "pages": ["Hello, {who}.", ["Something warm?", "A drink, perhaps?"]],
      "choices": [{"text": "Yes please", "do": "order"}, {"text": "Not now"}],
      "ignore": 2
    },
    "note": {"pages": ["Rain."]}
  }
}`

func TestParse(t *testing.T) {
	lib, err := Parse("test.json", []byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(lib.IDs(), ","); got != "note,offer" {
		t.Errorf("ids: %s", got)
	}
	c, _ := lib.Get("offer")
	if c.IgnoreChoice() != 1 {
		t.Errorf("ignore is written from 1; got index %d", c.IgnoreChoice())
	}
	seen := map[string]bool{}
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 50; i++ {
		seen[c.Lines(rng)[1]] = true
	}
	if len(seen) != 2 {
		t.Errorf("both versions should get picked sometimes: %v", seen)
	}
	b := c.Open(map[string]string{"who": "friend"}, rng, opts())
	if b.PageLines()[0] != "Hello, friend." {
		t.Errorf("placeholders should be filled: %q", b.PageLines())
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		`{"conversations": {"a": {"pages": []}}}`:                                                            "at least one page",
		`{"conversations": {"a": {"pages": [""]}}}`:                                                          "empty text",
		`{"conversations": {"a": {"pages": ["x"], "choices": [{"text": "only"}], "ignore": 1}}}`:             "2 to 4",
		`{"conversations": {"a": {"pages": ["x"], "choices": [{"text": "a"}, {"text": "b"}]}}}`:              "needs \"ignore\"",
		`{"conversations": {"a": {"pages": ["x"], "choices": [{"text": "a"}, {"text": "b"}], "ignore": 3}}}`: "only 2 choices",
		`{"conversations": {"a": {"pages": [42]}}}`:                                                          "text in quotes",
	}
	for src, want := range cases {
		_, err := Parse("bad.json", []byte(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want an error mentioning %q", src, err, want)
		}
	}
}
