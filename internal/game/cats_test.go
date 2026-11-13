package game

import (
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/text"
)

// catTalk runs the café at its real speed for some minutes, in half-second
// steps. Narrator boxes pass by themselves; a cat's moment waits, so it is
// answered (first choice) once its choices show. It counts the moments,
// the narrator's small talk, and any cat box without choices (there should
// be none: those lines are in Look around now).
func catTalk(t *testing.T, cfg Config, minutes float64) (moments, smallTalk, catLines int, speakers map[string]bool, g *Game) {
	t.Helper()
	g, now := inside(t, cfg)
	speakers = map[string]bool{}
	var last *talkBox
	steps := int(minutes * 60 * 2)
	for i := 0; i < steps; i++ {
		now = now.Add(500 * time.Millisecond)
		g.Tick(now)
		b := g.box
		if b == nil {
			continue
		}
		if b != last {
			last = b
			switch {
			case b.kind == talkMoment:
				moments++
				speakers[b.Speaker()] = true
			case b.kind == talkNarrator && g.greeted && b.Speaker() == "" && len(b.Choices()) == 0:
				smallTalk++
			}
			// A cat's answer right after a moment is part of the moment.
			if b.Speaker() != "" && b.kind != talkMoment && b.kind != talkCatReply {
				catLines++
			}
		}
		if b.kind == talkMoment && !b.Answered() && i%4 == 0 {
			g.Key(KeyConfirm) // read on; at the choices, take the first
		}
	}
	return moments, smallTalk, catLines, speakers, g
}

func TestTheCatsAndTheNarratorTakeTheirTime(t *testing.T) {
	if testing.Short() {
		t.Skip("long session")
	}
	// Three café hours, cats who know you a little so they come and go.
	bonds := map[string]float64{"brie": 30, "hunter": 30, "mimi": 30}
	m, st, lines, who, _ := catTalk(t, Config{Seed: 11, Bonds: bonds, Start: t0.Add(12 * time.Hour)}, 180)
	t.Logf("3 café hours: %d moments, %d small talk", m, st)
	// A moment every 5-10 minutes and small talk every 8-15, but never on
	// top of each other or of the café's own words: fewer than the bare
	// numbers, never more.
	if m < 6 || m > 180/5 {
		t.Errorf("%d cat moments in 3 hours; expected roughly one every 5-10 minutes", m)
	}
	if st < 7 || st > 180/8+1 {
		t.Errorf("%d small talk in 3 hours; expected roughly one every 8-15 minutes", st)
	}
	if lines != 0 {
		t.Errorf("%d cat lines without choices popped up; those belong in Look around", lines)
	}
	for _, name := range []string{"Brie", "Hunter", "Mimi"} {
		if !who[name] {
			t.Errorf("in 3 hours %s never had a moment", name)
		}
	}
	if who[""] {
		t.Error("cat moments should show the cat's name on the box")
	}
}

func TestQuietModeHalvesTheTalk(t *testing.T) {
	if testing.Short() {
		t.Skip("long session")
	}
	bonds := map[string]float64{"brie": 30, "hunter": 30, "mimi": 30}
	start := t0.Add(12 * time.Hour)
	m1, s1, _, _, _ := catTalk(t, Config{Seed: 4, Bonds: bonds, Start: start}, 240)
	m2, s2, _, _, _ := catTalk(t, Config{Seed: 4, Bonds: bonds, Start: start, Quiet: true}, 240)
	t.Logf("4 hours: normal %d moments + %d small talk; quiet %d + %d", m1, s1, m2, s2)
	if float64(s2) > 0.6*float64(s1) {
		t.Errorf("quiet mode: %d small talk, normal %d; quiet should be about half", s2, s1)
	}
	if float64(m2+s2) > 0.6*float64(m1+s1) {
		t.Errorf("quiet mode: %d things said, normal %d; quiet should be about half", m2+s2, m1+s1)
	}
}

func TestAMomentChangesTheBond(t *testing.T) {
	g, now := inside(t, Config{Bonds: map[string]float64{"mimi": 30}})
	if err := g.PinCat("mimi", "loaf", "shelf_top", ""); err != nil {
		t.Fatal(err)
	}
	g.pinned = nil // let her move again
	mimi := g.Cats().Cat("mimi")
	before := mimi.Bond
	if err := g.ShowMoment("mimi_watch"); err != nil {
		t.Fatal(err)
	}
	now = run(g, now, 1)
	if !strings.Contains(screen(g), "Mimi") {
		t.Error("the moment's box should show Mimi's name")
	}
	g.Key(KeyConfirm) // show the page at once
	now = run(g, now, 0.5)
	pick(t, g, "Call her name") // sudden
	g.Key(KeyConfirm)
	now = run(g, now, 3)
	if mimi.Bond >= before {
		t.Errorf("calling her name should lower Mimi's bond (%v -> %v)", before, mimi.Bond)
	}
	if mimi.Spot() != "" && mimi.Settled() {
		t.Error("after a sudden choice Mimi should leave the room")
	}
	_ = now
}

func TestAMomentWaitsForYou(t *testing.T) {
	g, now := inside(t, Config{Bonds: map[string]float64{"brie": 50}})
	if err := g.PinCat("brie", "sit", "table", ""); err != nil {
		t.Fatal(err)
	}
	g.pinned = nil
	brie := g.Cats().Cat("brie")
	before := brie.Bond
	if err := g.ShowMoment("brie_table"); err != nil {
		t.Fatal(err)
	}
	g.Cats().Hold(brie, true)
	box, moment := g.box, g.box.moment
	now = run(g, now, 600) // ten minutes without touching a key
	if g.box != box || box.Answered() {
		t.Fatal("a cat's moment should wait for you, not pass by itself")
	}
	if box.Page() != 0 {
		t.Error("even its first page should wait for Enter")
	}
	if brie.Spot() != "table" {
		t.Errorf("Brie should stay while her moment waits, not move to %q", brie.Spot())
	}
	// Leaving the café with it still open: it passes quietly and gently.
	g.leave()
	if !box.Answered() {
		t.Fatal("leaving should close the moment")
	}
	r, _ := box.Done()
	if !r.Ignored || r.Choice != moment.Conv.IgnoreChoice() {
		t.Errorf("leaving should take the quiet answer: %+v", r)
	}
	if brie.Bond < before {
		t.Errorf("leaving with a moment open shouldn't hurt (%v -> %v)", before, brie.Bond)
	}
	_ = now
}

func TestCatWordsFit(t *testing.T) {
	g := newGame(t, Config{})
	long := map[string]string{"drink": "tisane verveine", "cat": "Hunter"}
	for _, m := range g.cat.content.Moments {
		for pi, p := range m.Conv.Pages {
			for _, v := range p {
				for k, w := range long {
					v = strings.ReplaceAll(v, "{"+k+"}", w)
				}
				if n := len(dialogue.Wrap(v, talkTextW)); n > talkLines {
					t.Errorf("%s page %d is %d lines (most is %d): %q", m.ID, pi+1, n, talkLines, v)
				}
			}
		}
		for _, ch := range m.Conv.Choices {
			if canvas.Width(ch.Text) > talkTextW-2 {
				t.Errorf("%s: choice %q is too long for one line", m.ID, ch.Text)
			}
		}
		for _, ch := range m.Choices {
			for _, v := range ch.Reply {
				if n := len(dialogue.Wrap(v, talkTextW)); n > talkLines {
					t.Errorf("%s: a cat's reply is %d lines (most is %d): %q", m.ID, n, talkLines, v)
				}
			}
		}
		if m.Conv.Speaker == "" {
			t.Errorf("%s: no name for the box", m.ID)
		}
	}
	// Observations show in Look around after the cat's look line; each
	// should fit beside at least the shortest look line of some tier.
	for _, o := range g.cat.content.Observations {
		lk := g.cat.content.Looks[o.Cat]
		for _, v := range o.Text {
			fits := false
			for tier := cats.Stranger; tier <= cats.YourCat; tier++ {
				for _, l := range lk.Description(tier) {
					if len(dialogue.Wrap(l+" "+v, talkTextW)) <= lookLines {
						fits = true
					}
				}
			}
			if !fits {
				t.Errorf("observation %q never fits in Look around beside %s's look lines", v, o.Cat)
			}
		}
	}
}

// only puts the given cats in the room (the others leave), each at a spot
// in a pose, with a bond.
func only(t *testing.T, g *Game, put ...struct {
	id, spot, pose string
	bond           float64
}) {
	t.Helper()
	w := g.Cats()
	for _, c := range w.Cats {
		w.Put(c, "", "")
	}
	for _, p := range put {
		c := w.Cat(p.id)
		w.Put(c, p.spot, p.pose)
		c.Bond = p.bond
	}
}

type placed = struct {
	id, spot, pose string
	bond           float64
}

func TestLookAroundFromTheMenu(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Weather: "rain"})
	g.Resize(FrameW, FrameH)
	only(t, g, placed{"brie", "sill", cats.PoseSit, 0}, placed{"mimi", "shelf_top", cats.PoseLoaf, 0})
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyConfirm)
	if g.look == nil {
		t.Fatal("Esc, ↓, Enter should open Look around")
	}
	var ids []string
	for _, tg := range g.look.targets {
		ids = append(ids, tg.id())
	}
	if got := strings.Join(ids, " "); got != "mimi window brie table" {
		t.Errorf("targets should go left to right: got %q", got)
	}
	if tg, _ := g.lookTarget(); tg.id() != "mimi" {
		t.Errorf("Look around should start on the first cat, not %q", tg.id())
	}
	// The cats hold still, however long you look.
	now = run(g, now, 120) // an hour of café time at --fast
	if g.Cats().Cat("brie").Spot() != "sill" || g.Cats().Cat("mimi").Spot() != "shelf_top" {
		t.Error("while you look around, the cats stay where they are")
	}
	g.Key(KeyLeft)
	g.Key(KeyLeft)
	if tg, _ := g.lookTarget(); tg.id() != "mimi" {
		t.Error("← stops at the leftmost thing")
	}
	for i := 0; i < 5; i++ {
		g.Key(KeyRight)
	}
	if tg, _ := g.lookTarget(); tg.id() != "table" {
		t.Error("→ stops at your table")
	}
	s := screen(g)
	if !strings.Contains(s, "Your table") {
		t.Error("the panel shows the table's name")
	}
	g.Key(KeyBack)
	if g.look != nil || g.pause != nil || g.Done() {
		t.Error("Esc closes Look around, back to the café")
	}
	_ = now
}

func TestStrangersOnlyGetADescription(t *testing.T) {
	g, _ := inside(t, Config{Weather: "rain"})
	g.Resize(FrameW, FrameH)
	only(t, g, placed{"brie", "sill", cats.PoseSit, 0})
	if err := g.LookAt("brie"); err != nil {
		t.Fatal(err)
	}
	if n := len(g.lookActions()); n != 0 {
		t.Errorf("a stranger offers no actions, got %d", n)
	}
	s := screen(g)
	if !strings.Contains(s, "Brie") || !strings.Contains(s, "fluffy cream cat") {
		t.Errorf("looking at Brie as a stranger should describe her; screen:\n%s", s)
	}
	if strings.Contains(s, "▸") {
		t.Error("no choices for a stranger")
	}
	g.Key(KeyConfirm) // nothing to do
	if g.look.reply != nil {
		t.Error("Enter does nothing when there's nothing to do")
	}
}

func TestActionsGrowWithTheBond(t *testing.T) {
	g, _ := inside(t, Config{})
	for _, id := range []string{"brie", "hunter", "mimi"} {
		last := -1
		for tier := cats.Stranger; tier <= cats.YourCat; tier++ {
			only(t, g, placed{id, "sill", cats.PoseSit, cats.BondFor(tier)})
			n := len(g.Cats().Actions(g.cat.content.Looks[id], g.Cats().Cat(id)))
			if tier == cats.Stranger && n != 0 {
				t.Errorf("%s: a stranger gets no actions (%d)", id, n)
			}
			if n < last {
				t.Errorf("%s: fewer actions at %s (%d) than before (%d)", id, tier, n, last)
			}
			last = n
		}
		if last < 4 {
			t.Errorf("%s: as \"your cat\" there should be several things to do (%d)", id, last)
		}
	}
}

func TestDoingSomethingWithACat(t *testing.T) {
	g, now := inside(t, Config{Weather: "rain"})
	g.Resize(FrameW, FrameH)
	only(t, g, placed{"brie", "sill", cats.PoseSit, cats.BondFor(cats.Familiar)})
	brie := g.Cats().Cat("brie")
	before := brie.Bond
	if err := g.LookAt("brie"); err != nil {
		t.Fatal(err)
	}
	if len(g.lookActions()) == 0 {
		t.Fatal("a familiar Brie should offer something to do")
	}
	g.Key(KeyConfirm)
	if g.look.reply == nil {
		t.Fatal("doing something should show what happens")
	}
	if brie.Bond <= before {
		t.Errorf("a kind action should raise Brie's bond (%v -> %v)", before, brie.Bond)
	}
	now = run(g, now, 30) // the reply turns its pages and goes
	if g.look.reply != nil {
		t.Fatal("the reply should pass by itself")
	}
	if g.look == nil {
		t.Fatal("you're still looking around after a reply")
	}
	closing := g.look.done["brie"]
	if len(g.lookActions()) != 0 || len(closing) == 0 {
		t.Fatal("afterwards: no more actions, and a quiet line about Brie instead")
	}
	if !strings.Contains(screen(g), closing[0]) || strings.Contains(screen(g), "enough attention") {
		t.Errorf("the panel should end with the quiet line %q:\n%s", closing[0], screen(g))
	}
	if !neutral(strings.Join(closing, " ")) {
		t.Errorf("the closing line should be about Brie, not you: %q", closing)
	}
	desc := strings.Join(g.look.desc, " ")
	// Long after her rest is over, still nothing more this time round, and
	// looking away and back shows the same words.
	now = run(g, now, 600)
	g.Key(KeyLeft)
	g.Key(KeyRight)
	g.Key(KeyRight)
	g.Key(KeyLeft)
	if err := g.LookAt("brie"); err != nil {
		t.Fatal(err)
	}
	if len(g.lookActions()) != 0 {
		t.Error("one interaction per cat each time you look around")
	}
	if got := strings.Join(g.lookDescription(lookTarget{kind: lookCat, cat: brie}), " "); got != desc {
		t.Errorf("coming back to Brie should show the same words:\n%q\n%q", desc, got)
	}
	// Next time you look around, she's ready again.
	g.Key(KeyBack)
	if err := g.LookAt("brie"); err != nil {
		t.Fatal(err)
	}
	if len(g.lookActions()) == 0 {
		t.Error("a new Look around (after her rest) offers things to do again")
	}
}

func TestReopeningDuringTheRestShowsTheRestLine(t *testing.T) {
	g, now := inside(t, Config{Weather: "rain"})
	g.Resize(FrameW, FrameH)
	only(t, g, placed{"brie", "sill", cats.PoseSit, cats.BondFor(cats.Familiar)})
	if err := g.LookAt("brie"); err != nil {
		t.Fatal(err)
	}
	g.Key(KeyConfirm)
	now = run(g, now, 30)
	g.Key(KeyBack)
	if err := g.LookAt("brie"); err != nil {
		t.Fatal(err)
	}
	if len(g.lookActions()) != 0 || !strings.Contains(screen(g), "enough attention") {
		t.Error("looking again straight away: Brie is still resting")
	}
	_ = now
}

func TestMimiLeavesAfterSomethingSudden(t *testing.T) {
	g, now := inside(t, Config{})
	only(t, g, placed{"mimi", "shelf_top", cats.PoseLoaf, cats.BondFor(cats.Curious)})
	mimi := g.Cats().Cat("mimi")
	if err := g.LookAt("mimi"); err != nil {
		t.Fatal(err)
	}
	acts := g.lookActions()
	sudden := -1
	for i, a := range acts {
		if a.Effect.Manner == "sudden" {
			sudden = i
		}
	}
	if sudden < 0 {
		t.Fatal("curious Mimi should offer one sudden choice to avoid")
	}
	for i := 0; i < sudden; i++ {
		g.Key(KeyDown)
	}
	before := mimi.Bond
	g.Key(KeyConfirm)
	now = run(g, now, 30)
	if mimi.Bond >= before {
		t.Errorf("a sudden action lowers Mimi's bond (%v -> %v)", before, mimi.Bond)
	}
	for _, tg := range g.look.targets {
		if tg.id() == "mimi" {
			t.Error("after something sudden, Mimi slips away, even while you look around")
		}
	}
}

func TestLookingQuietlyHelpsMimi(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	only(t, g, placed{"mimi", "shelf_top", cats.PoseLoaf, 0})
	mimi := g.Cats().Cat("mimi")
	if err := g.LookAt("mimi"); err != nil {
		t.Fatal(err)
	}
	b1 := mimi.Bond
	if b1 <= 0 {
		t.Error("looking at Mimi quietly should help her trust you a little")
	}
	g.Key(KeyRight)
	g.Key(KeyLeft)
	if mimi.Bond != b1 {
		t.Error("looking again straight away shouldn't count twice")
	}
	now = run(g, now, 12) // six café minutes
	g.Key(KeyRight)
	g.Key(KeyLeft)
	if mimi.Bond <= b1 {
		t.Error("a few minutes later, looking again helps again")
	}
}

func TestLookWordsFit(t *testing.T) {
	g := newGame(t, Config{})
	longSpot := ""
	for _, s := range g.lookWords.Spots {
		if len(s) > len(longSpot) {
			longSpot = s
		}
	}
	vals := map[string]string{"where": longSpot, "drink": "tisane verveine", "cat": "Hunter"}
	for id, lk := range g.cat.content.Looks {
		for tier, looks := range lk.Looks {
			for _, l := range looks {
				for pose, doing := range lk.Doing {
					for _, d := range doing {
						s := text.Fill(l, vals) + " " + text.Fill(d, vals)
						if n := len(dialogue.Wrap(s, talkTextW)); n > lookLines {
							t.Errorf("%s %s/%s: %d lines (most is %d): %q", id, tier, pose, n, lookLines, s)
						}
					}
				}
			}
		}
		for _, a := range lk.Actions {
			if canvas.Width(a.Text) > talkTextW-2 {
				t.Errorf("%s: action %q is too long for one line", id, a.Text)
			}
			for _, p := range a.Reply {
				for _, v := range p {
					if n := len(dialogue.Wrap(text.Fill(v, vals), talkTextW)); n > talkLines {
						t.Errorf("%s: reply is %d lines (most is %d): %q", id, n, talkLines, v)
					}
				}
			}
		}
		for _, r := range lk.Rest {
			if canvas.Width(r) > talkTextW {
				t.Errorf("%s: rest line %q is too long", id, r)
			}
		}
	}
	for _, notes := range [][]noteDef{g.lookWords.Window, g.lookWords.Table} {
		for _, n := range notes {
			for _, v := range n.Text {
				if k := len(dialogue.Wrap(text.Fill(v, vals), talkTextW)); k > lookLines {
					t.Errorf("%q is %d lines (most is %d)", v, k, lookLines)
				}
			}
		}
	}
	for _, h := range []string{g.ui.LookHint, g.ui.LookHintChoose} {
		if canvas.Width(h) > talkTextW {
			t.Errorf("hint %q is too long for the panel", h)
		}
	}
	// The tallest panel still fits on screen.
	g.Resize(FrameW, FrameH)
	g.Key(KeyConfirm)
	g.Key(KeyConfirm)
	only(t, g, placed{"hunter", "floor", cats.PoseSit, 100})
	if err := g.LookAt("hunter"); err != nil {
		t.Fatal(err)
	}
	checkSize(t, g.View(), FrameW, FrameH)
}

func TestTheTableLineFollowsYourCup(t *testing.T) {
	g, _ := inside(t, Config{})
	only(t, g)
	if err := g.Serve("cafe_creme"); err != nil {
		t.Fatal(err)
	}
	if err := g.LookAt("table"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		g.look.descKey = ""
		d := strings.Join(g.lookDescription(lookTarget{kind: lookTable}), " ")
		if strings.Contains(d, "No cup yet") || strings.Contains(d, "empty") {
			t.Fatalf("a fresh café crème is on the table, but the line said %q", d)
		}
	}
}

func TestCatsAreDrawnAndStayInTheRoom(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Seed: 8, Bonds: map[string]float64{"brie": 80, "hunter": 80, "mimi": 80}})
	g.Resize(FrameW, FrameH)
	drawn := map[string]bool{}
	for i := 0; i < 600; i++ { // five café hours
		now = run(g, now, 1)
		for _, d := range g.catsToDraw() {
			drawn[d.Sprite[:strings.Index(d.Sprite, "_")]] = true
		}
		if i%50 == 0 {
			checkSize(t, g.View(), FrameW, FrameH)
		}
	}
	for _, id := range []string{"brie", "hunter", "mimi"} {
		if !drawn[id] {
			t.Errorf("in five café hours %s never appeared", id)
		}
	}
}

func TestLookAroundShowsObservations(t *testing.T) {
	g, now := inside(t, Config{Bonds: map[string]float64{"brie": 30}})
	if err := g.PinCat("brie", "sleep", "cushion", ""); err != nil {
		t.Fatal(err)
	}
	g.pinned = nil
	run(g, now, 1)
	brie := g.Cats().Cat("brie")
	obs := map[string]bool{}
	for _, o := range g.cat.content.Observations {
		for _, s := range o.Text {
			obs[s] = true
		}
	}
	saw := 0
	for i := 0; i < 100; i++ {
		d := g.catDescription(brie)
		for s := range obs {
			if strings.HasSuffix(d, s) {
				saw++
			}
		}
		if n := len(dialogue.Wrap(d, talkTextW)); n > lookLines {
			t.Fatalf("Look around text is %d lines: %q", n, d)
		}
	}
	if saw < 20 || saw > 80 {
		t.Errorf("an observation should show about half the time in Look around, got %d of 100", saw)
	}
}

func TestEveryCatHasAQuietClosingLine(t *testing.T) {
	spots := map[string]string{cats.PoseSit: "sideboard", cats.PoseLoaf: "cushion", cats.PoseSleep: "floor", cats.PoseStand: "floor"}
	for _, id := range []string{"brie", "hunter", "mimi"} {
		for pose, spot := range spots {
			g, now := inside(t, Config{Weather: "rain"})
			g.Resize(FrameW, FrameH)
			only(t, g, placed{id, spot, pose, cats.BondFor(cats.Friend)})
			run(g, now, 1)
			if err := g.LookAt(id); err != nil {
				t.Fatal(err)
			}
			c := g.Cats().Cat(id)
			for i := 0; i < 20; i++ {
				l := g.closingLine(c)
				if len(l) == 0 {
					t.Fatalf("%s %s: no closing line", id, pose)
				}
				s := strings.Join(l, " ")
				if !neutral(s) || strings.Contains(s, "{") {
					t.Errorf("%s %s: closing line %q should be about the cat alone", id, pose, s)
				}
			}
		}
	}
}

// pick moves the selection to the choice with these words (choices are
// shown in a random order).
func pick(t *testing.T, g *Game, words string) {
	t.Helper()
	for g.box.Selected() > 0 {
		g.Key(KeyUp)
	}
	for i, ch := range g.box.Choices() {
		if ch.Text == words {
			for k := 0; k < i; k++ {
				g.Key(KeyDown)
			}
			return
		}
	}
	t.Fatalf("no choice %q", words)
}

func TestMomentChoicesComeInARandomOrder(t *testing.T) {
	orders := map[string]bool{}
	for seed := uint64(1); seed <= 20; seed++ {
		g, now := inside(t, Config{Seed: seed, Bonds: map[string]float64{"brie": 10}})
		if err := g.ShowMoment("brie_sigh"); err != nil {
			t.Fatal(err)
		}
		run(g, now, 1)
		var texts []string
		for _, ch := range g.box.Choices() {
			texts = append(texts, ch.Text)
		}
		orders[strings.Join(texts, "|")] = true
	}
	if len(orders) < 3 {
		t.Errorf("over 20 visits the choices should come in different orders, saw %d", len(orders))
	}
}

func TestLookAroundPanelIsAlwaysBottomLeft(t *testing.T) {
	g, now := inside(t, Config{})
	g.Resize(FrameW, FrameH)
	// Brie on the floor, right where the panel sits: it still stays put.
	only(t, g, placed{"brie", "floor", cats.PoseSit, cats.BondFor(cats.Familiar)})
	run(g, now, 1)
	for _, what := range []string{"brie", "window", "table"} {
		if err := g.LookAt(what); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(screen(g), "\n")
		top, left := -1, -1
		for i, l := range lines {
			if j := strings.Index(l, "╭"); j >= 0 && top < 0 {
				top, left = i, len([]rune(l[:j]))
			}
		}
		bottom := -1
		for i, l := range lines {
			if strings.Contains(l, "╰") {
				bottom = i
			}
		}
		if left != talkLeft || bottom != FrameH-2 {
			t.Errorf("looking at %s: panel at column %d, bottom row %d; it should be bottom-left (column %d, row %d)", what, left, bottom, talkLeft, FrameH-2)
		}
	}
}

func TestLookAroundOffersAFewThingsAtMostOneSudden(t *testing.T) {
	for _, tier := range []cats.Tier{cats.Familiar, cats.Friend, cats.YourCat} {
		for seed := uint64(1); seed <= 15; seed++ {
			g, now := inside(t, Config{Seed: seed})
			g.Resize(FrameW, FrameH)
			only(t, g, placed{"mimi", "sideboard", cats.PoseSit, cats.BondFor(tier)})
			run(g, now, 1)
			if err := g.LookAt("mimi"); err != nil {
				t.Fatal(err)
			}
			acts, sudden := g.lookActions(), 0
			for _, a := range acts {
				if a.Effect.Manner == "sudden" {
					sudden++
				}
			}
			if len(acts) < 2 || len(acts) > maxActions || sudden > 1 {
				t.Fatalf("%s Mimi offers %d things, %d sudden", tier, len(acts), sudden)
			}
			again := g.lookActions()
			for i := range acts {
				if acts[i] != again[i] {
					t.Fatal("the list shouldn't change while you look")
				}
			}
		}
	}
}

// After you answer a moment, the cat answers too: her reply is shown in
// her name before the café goes back to its quiet.
func TestAMomentEndsWithTheCatAnswering(t *testing.T) {
	g, now := inside(t, Config{Bonds: map[string]float64{"mimi": 30}})
	if err := g.ShowMoment("mimi_watch"); err != nil {
		t.Fatal(err)
	}
	now = run(g, now, 1)
	g.Key(KeyConfirm) // show the page at once
	now = run(g, now, 0.5)
	pick(t, g, "Call her name")
	g.Key(KeyConfirm)
	saw := false
	for i := 0; i < 300 && !saw; i++ {
		now = run(g, now, 0.1)
		if g.box != nil && g.box.kind == talkCatReply {
			saw = true
			if g.box.Speaker() != "Mimi" {
				t.Errorf("the answer should be Mimi's, got %q", g.box.Speaker())
			}
		}
	}
	if !saw {
		t.Fatal("after a moment the cat should answer, but nothing followed")
	}
	_ = now
}
