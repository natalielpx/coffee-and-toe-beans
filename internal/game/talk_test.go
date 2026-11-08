package game

import (
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/dialogue"
)

// inside starts a game and steps into the café (skipping the fade).
func inside(t *testing.T, cfg Config) (*Game, time.Time) {
	t.Helper()
	if cfg.Seed == 0 {
		cfg.Seed = 7
	}
	g := newGame(t, cfg)
	g.Key(KeyConfirm)
	g.Key(KeyConfirm)
	if g.phase != phaseCafe {
		t.Fatal("couldn't step inside")
	}
	return g, t0
}

// screen is the visible text of the frame, with the pixels blanked out.
func screen(g *Game) string { return strings.ReplaceAll(visible(g.View()), "▀", " ") }

func items(g *Game) []menuAction { return g.pause.items }

func TestPauseMenuOrder(t *testing.T) {
	g := newGame(t, Config{})
	g.Key(KeyBack)
	if got := items(g); len(got) != 2 || got[0] != actSettings || got[1] != actLeave {
		t.Errorf("on the title card the menu is Settings, Leave; got %v", got)
	}
	g.Key(KeyBack)

	g, _ = inside(t, Config{})
	g.Key(KeyBack)
	want := []menuAction{actOrder, actLook, actSettings, actLeave}
	got := items(g)
	if len(got) != len(want) {
		t.Fatalf("café menu: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("café menu: got %v, want %v", got, want)
		}
	}
	s := screen(g)
	if strings.Contains(s, "Stay a little longer") {
		t.Error("there's no Stay item any more: Esc closes the menu")
	}
	for _, w := range []string{"Leave the café", "Order a drink", "Look around", "Settings"} {
		if !strings.Contains(s, w) {
			t.Errorf("pause menu should show %q", w)
		}
	}
}

func TestTheCafeSaysHello(t *testing.T) {
	g, now := inside(t, Config{})
	now = run(g, now, 1)
	if g.box != nil {
		t.Fatal("the café lets you look around for a moment first")
	}
	now = run(g, now, 2)
	if g.box == nil {
		t.Fatal("after a moment the café should say hello")
	}
	now = run(g, now, 15)
	if !g.box.ShowingChoices() {
		g.Key(KeyConfirm) // the first page may be waiting for Enter
		now = run(g, now, 5)
	}
	if !g.box.ShowingChoices() {
		t.Fatal("the hello should end with a choice")
	}
	s := screen(g)
	if !strings.Contains(s, "Order a drink") || !strings.Contains(s, "Not yet") {
		t.Errorf("choices missing from the screen:\n%s", s)
	}

	// Ignored, it passes gently after 90 seconds and nothing is ordered.
	now = run(g, now, 60)
	if g.box == nil {
		t.Fatal("a narrator question should wait 90 seconds, not 40")
	}
	run(g, now, 40)
	if g.box != nil || g.menu != nil || g.order != nil {
		t.Error("an ignored offer should pass, ordering nothing")
	}
}

// answerCats reads on through a cat's moment (they wait for you) and
// takes its first choice.
func answerCats(g *Game) {
	if g.box != nil && g.box.kind == talkMoment && !g.box.Answered() {
		g.Key(KeyConfirm)
	}
}

// answerHello waits for the hello and takes choice i.
func answerHello(t *testing.T, g *Game, now time.Time, i int) time.Time {
	t.Helper()
	now = run(g, now, 3)
	for k := 0; k < 4 && (g.box == nil || !g.box.ShowingChoices()); k++ {
		g.Key(KeyConfirm)
		now = run(g, now, 1)
	}
	if g.box == nil || !g.box.ShowingChoices() {
		t.Fatal("no choices")
	}
	for k := 0; k < i; k++ {
		g.Key(KeyDown)
	}
	g.Key(KeyConfirm)
	return run(g, now, 1)
}

func TestOrderingADrink(t *testing.T) {
	g, now := inside(t, Config{})
	now = answerHello(t, g, now, 0)
	if g.menu == nil {
		t.Fatal("choosing Order a drink should open the drinks menu")
	}
	s := screen(g)
	for _, d := range g.drinks.Drinks {
		if !strings.Contains(s, d.Name) {
			t.Errorf("the menu should list %s", d.Name)
		}
	}
	g.Key(KeyDown) // chocolat chaud
	g.Key(KeyConfirm)
	if g.menu != nil {
		t.Fatal("ordering closes the menu")
	}
	if g.order == nil || g.order.drink.ID != "chocolat_chaud" {
		t.Fatalf("the chocolat chaud should be on the table, got %+v", g.order)
	}
	if g.box == nil || g.box.kind != talkServed {
		t.Fatal("the narrator should mention the drink arriving")
	}
	run(g, now, 2)
	if !strings.Contains(screen(g), "hocolat chaud") {
		t.Errorf("the served line should be about the chocolat chaud:\n%s", screen(g))
	}
}

func TestJustSitting(t *testing.T) {
	g, now := inside(t, Config{})
	now = answerHello(t, g, now, 1)
	if g.menu != nil || g.order != nil {
		t.Fatal("just sitting orders nothing")
	}
	run(g, now, 120)
	if g.box != nil {
		t.Error("nobody nags you to order")
	}
}

func TestDrinksMenuFromThePauseMenu(t *testing.T) {
	g, _ := inside(t, Config{})
	g.Key(KeyBack)
	g.Key(KeyConfirm) // Order a drink is first
	if g.menu == nil || g.pause != nil {
		t.Fatal("Order a drink should open the drinks menu")
	}
	g.Key(KeyBack)
	if g.menu != nil || g.pause == nil {
		t.Fatal("Esc in the drinks menu goes back to the pause menu")
	}
	if g.pause.items[g.pause.sel] != actOrder {
		t.Error("back in the pause menu, Order a drink should still be selected")
	}
	g.Key(KeyBack)
	if g.pause != nil || g.Done() {
		t.Fatal("and Esc again closes it")
	}
}

func TestEveningDrinks(t *testing.T) {
	ids := func(hh, mm int) []string {
		start := time.Date(2026, 11, 7, hh, mm, 0, 0, time.Local)
		g, _ := inside(t, Config{Start: start})
		g.Key(KeyBack)
		g.Key(KeyConfirm)
		var seen []string
		for i := 0; i < 10; i++ {
			seen = append(seen, g.drinks.Drinks[g.menu.sel].ID)
			g.Key(KeyDown)
		}
		return seen
	}
	has := func(list []string, id string) bool {
		for _, s := range list {
			if s == id {
				return true
			}
		}
		return false
	}
	noon := ids(12, 0)
	if has(noon, "vin_chaud") || has(noon, "tisane_verveine") {
		t.Errorf("at noon you can't select vin chaud or tisane: %v", noon)
	}
	if !has(ids(18, 0), "vin_chaud") || has(ids(18, 0), "tisane_verveine") {
		t.Error("at 18:00 vin chaud is served, tisane not yet")
	}
	late := ids(23, 0)
	if !has(late, "vin_chaud") || !has(late, "tisane_verveine") {
		t.Errorf("at 23:00 both are served: %v", late)
	}

	start := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	g, _ := inside(t, Config{Start: start})
	g.openDrinks(false)
	if s := screen(g); !strings.Contains(s, "from 17:00") || !strings.Contains(s, "from 21:00") {
		t.Errorf("drinks served later should say when:\n%s", s)
	}
}

func TestTheCupCoolsEmptiesAndStays(t *testing.T) {
	g, now := inside(t, Config{Fast: true}) // 15 café minutes = 30 real seconds
	now = answerHello(t, g, now, 0)
	g.Key(KeyConfirm) // café crème
	o := g.order.order
	now = run(g, now, 5)
	if g.drinks.Heat(o, g.Now()) >= 1 || g.drinks.Level(o, g.Now()) >= 1 {
		t.Error("after a few café minutes the cup should be cooler and a little lower")
	}
	now = run(g, now, 26)
	if !g.drinks.Empty(o, g.Now()) {
		t.Fatal("after 15 café minutes the cup is empty")
	}
	if g.order == nil {
		t.Fatal("an empty cup stays on the table")
	}

	// A little later, one gentle offer of another.
	sawOffer := false
	for i := 0; i < 100 && !sawOffer; i++ {
		now = run(g, now, 0.5)
		sawOffer = g.box != nil && g.box.kind == talkOffer
	}
	if !sawOffer {
		t.Fatal("a few minutes after the cup is empty, another should be offered")
	}
	now = run(g, now, 120) // ignored
	if g.order == nil || g.order.drink.ID != "cafe_creme" {
		t.Fatal("ignoring the offer leaves the empty cup where it is")
	}
	run(g, now, 600)
	if g.box != nil && g.box.kind == talkOffer {
		t.Error("the offer is made only once per cup")
	}
}

func TestANewDrinkReplacesTheOld(t *testing.T) {
	g, now := inside(t, Config{})
	now = answerHello(t, g, now, 0)
	g.Key(KeyConfirm) // café crème
	now = run(g, now, 60)
	g.Key(KeyBack)
	g.Key(KeyConfirm)
	if g.drinks.Drinks[g.menu.sel].ID != "cafe_creme" {
		t.Error("the menu opens on the drink you're having")
	}
	g.Key(KeyDown)
	g.Key(KeyDown) // chai latte
	g.Key(KeyConfirm)
	if g.order.drink.ID != "chai_latte" || g.drinks.Level(g.order.order, g.Now()) != 1 {
		t.Error("the new drink replaces the half-finished one, full")
	}
	run(g, now, 1)
}

func TestTalkPausesUnderMenus(t *testing.T) {
	g, now := inside(t, Config{})
	now = run(g, now, 3)
	if g.box == nil {
		t.Fatal("expected the hello")
	}
	g.Key(KeyBack)
	now = run(g, now, 300)
	if g.box == nil || g.box.Answered() {
		t.Fatal("the hello waits while the menu is open")
	}
	g.Key(KeyBack)
	run(g, now, 1)
	if g.box == nil {
		t.Fatal("and carries on after")
	}
}

func TestArrowsInsideTheCafe(t *testing.T) {
	// Outside Look around, ← → do nothing at all.
	g, _ := inside(t, Config{})
	before := screen(g)
	for _, k := range []Key{KeyRight, KeyRight, KeyLeft, KeyRight} {
		g.Key(k)
	}
	if g.box != nil || g.pause != nil || g.menu != nil || g.look != nil {
		t.Error("← → don't open anything in the café")
	}
	if screen(g) != before {
		t.Error("← → shouldn't change anything on screen")
	}
}

func TestBoxesFitTheFrame(t *testing.T) {
	for _, size := range [][2]int{{104, 42}, {140, 50}} {
		g, now := inside(t, Config{})
		g.Resize(size[0], size[1])
		now = answerHello(t, g, now, 0)
		checkSize(t, g.View(), size[0], size[1])
		g.Key(KeyConfirm)
		run(g, now, 3)
		checkSize(t, g.View(), size[0], size[1])
		g.Key(KeyBack)
		checkSize(t, g.View(), size[0], size[1])
	}
}

func TestDialogueBoxLeavesTheCupInView(t *testing.T) {
	stand := 55 // where cups stand in the French interior, in cells
	if talkLeft+talkTextW+4 > stand-1 {
		t.Errorf("the dialogue box (to column %d) would cover the cup (from %d)", talkLeft+talkTextW+4, stand)
	}
}

// Every line the café can say must fit its box without being cut into
// extra pages, and every choice must fit on one line.
func TestAllWordsFit(t *testing.T) {
	g := newGame(t, Config{})
	lib, err := dialogue.Load(content.FS, "dialogue")
	if err != nil {
		t.Fatal(err)
	}
	long := map[string]string{"drink": "tisane verveine", "suggestion": "cup of Earl Grey", "favourite": "cup of Earl Grey"} // the longest
	for _, id := range lib.IDs() {
		c, _ := lib.Get(id)
		for pi, p := range c.Pages {
			for _, v := range p {
				for k, w := range long {
					v = strings.ReplaceAll(v, "{"+k+"}", w)
				}
				if n := len(dialogue.Wrap(v, talkTextW)); n > talkLines {
					t.Errorf("%s page %d is %d lines (most is %d): %q", id, pi+1, n, talkLines, v)
				}
			}
		}
		for _, ch := range c.Choices {
			if canvas.Width(ch.Text) > talkTextW-2 {
				t.Errorf("%s: choice %q is too long for one line", id, ch.Text)
			}
		}
	}
	for _, d := range g.drinks.Drinks {
		for _, s := range d.Served {
			if n := len(dialogue.Wrap(s, talkTextW)); n > talkLines {
				t.Errorf("%s served line is %d lines (most is %d): %q", d.ID, n, talkLines, s)
			}
		}
	}
	g.Resize(FrameW, FrameH)
	g.Key(KeyConfirm)
	g.Key(KeyConfirm)
	g.openDrinks(false)
	checkSize(t, g.View(), FrameW, FrameH)
}

// emptyCup orders a café crème with --fast and waits until the café says
// something about the empty cup; it returns that conversation's name.
func emptyCup(t *testing.T, seed uint64, start time.Time) (*Game, time.Time) {
	t.Helper()
	g, now := inside(t, Config{Fast: true, Seed: seed, Start: start})
	now = answerHello(t, g, now, 0)
	g.Key(KeyConfirm) // the first drink being served
	for i := 0; i < 400; i++ {
		now = run(g, now, 0.5)
		answerCats(g)
		if g.box != nil && g.box.kind == talkOffer {
			return g, now
		}
	}
	t.Fatal("no offer after the cup was empty")
	return nil, now
}

func TestOffersVaryAndSuggest(t *testing.T) {
	seen := map[string]bool{}
	for seed := uint64(1); seed <= 40; seed++ {
		g, now := emptyCup(t, seed, time.Time{})
		seen[g.lastOffer] = true
		if g.lastOffer == "cup_empty_suggest" {
			if g.suggestion == nil || g.suggestion.ID == "cafe_creme" {
				t.Fatalf("a suggestion should be another drink, got %+v", g.suggestion)
			}
			if !strings.Contains(screen(g)+strings.Join(g.box.PageLines(), " "), g.suggestion.Word()) {
				run(g, now, 3)
				if !strings.Contains(screen(g), g.suggestion.Word()) {
					t.Errorf("the suggestion should name the %s:\n%s", g.suggestion.Word(), screen(g))
				}
			}
		}
	}
	for _, id := range []string{"cup_empty", "cup_empty_suggest", "cup_empty_quiet"} {
		if !seen[id] {
			t.Errorf("over many cups, %s should come up sometimes (saw %v)", id, seen)
		}
	}
}

func TestSuggestionCanBeOrdered(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		g, now := emptyCup(t, seed, time.Time{})
		if g.lastOffer != "cup_empty_suggest" {
			continue
		}
		want := g.suggestion.ID
		for k := 0; k < 4 && !g.box.ShowingChoices(); k++ {
			g.Key(KeyConfirm)
			now = run(g, now, 1)
		}
		g.Key(KeyConfirm) // "A ..., please"
		if g.menu != nil || g.order.drink.ID != want {
			t.Fatalf("choosing the suggestion should serve the %s, got %s", want, g.order.drink.ID)
		}
		return
	}
	t.Fatal("never saw a suggestion")
}

func TestOffersDontRepeatBackToBack(t *testing.T) {
	g, now := emptyCup(t, 3, time.Time{})
	first := g.lastOffer
	g.Key(KeyBack)
	g.Key(KeyConfirm) // Order a drink
	g.Key(KeyConfirm) // whatever is selected
	for i := 0; i < 400; i++ {
		now = run(g, now, 0.5)
		answerCats(g)
		if g.box != nil && g.box.kind == talkOffer {
			break
		}
	}
	if g.lastOffer == first {
		t.Errorf("the same offer twice in a row: %s", first)
	}
}

func TestDrinkWords(t *testing.T) {
	g := newGame(t, Config{})
	want := map[string]string{"cafe_creme": "café crème", "earl_grey": "cup of Earl Grey", "tisane_verveine": "tisane verveine"}
	for id, w := range want {
		d, _ := g.drinks.Get(id)
		if d.Word() != w {
			t.Errorf("%s mid-sentence should be %q, got %q", id, w, d.Word())
		}
	}
}

// The dialogue box sits low on the left; the cup and the cats' usual spots
// (all but the floor by the radiator, which a tall box may brush) must stay
// in view beside it.
func TestCupAndSpotsClearOfTheBox(t *testing.T) {
	g := newGame(t, Config{})
	boxRight := talkLeft + talkTextW + 4 // first column after the box
	stand := g.scene.CupStand()
	if stand[0] < boxRight {
		t.Errorf("the cup stands at x=%d, under the dialogue box (which ends at %d)", stand[0], boxRight)
	}
	for _, name := range []string{"sill", "shelf_top", "sideboard", "cushion", "table"} {
		sp, ok := g.scene.Spots()[name]
		if !ok {
			t.Errorf("cat spot %q is missing", name)
			continue
		}
		if sp.X < boxRight+6 && sp.Y/2 >= FrameH-12 {
			t.Errorf("cat spot %q at %d,%d would be hidden by the dialogue box", name, sp.X, sp.Y)
		}
	}
}
