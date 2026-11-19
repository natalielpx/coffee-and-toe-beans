package game

import (
	"coffee-and-toe-beans/internal/dialogue"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/journal"
	"coffee-and-toe-beans/internal/save"
)

// answerAll presses Enter until the conversation on screen has passed.
func answerAll(g *Game, now time.Time) time.Time {
	for i := 0; i < 300; i++ {
		if g.jrnl != nil || (g.box == nil && len(g.queue) == 0) {
			break
		}
		now = run(g, now, 0.3)
		if g.jrnl != nil {
			break // the book has just opened: don't press Enter on it
		}
		g.Key(KeyConfirm)
	}
	return now
}

func boxText(g *Game) string {
	if g.box == nil {
		return ""
	}
	return strings.Join(g.box.Pages(), " ")
}

func TestAMomentWritesTheMeetingEntryOnce(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Weather: "rain"})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	if err := g.ShowMoment("brie_sigh"); err != nil {
		t.Fatal(err)
	}
	now = answerAll(g, now)
	entries := g.mem.data.Journal.Entries
	if len(entries) != 1 {
		t.Fatalf("want one entry after the first moment, got %d", len(entries))
	}
	if entries[0].Cat != "brie" || !strings.Contains(entries[0].Text, "Brie") {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
	// Meeting again writes nothing.
	if err := g.ShowMoment("brie_look"); err != nil {
		t.Fatal(err)
	}
	answerAll(g, now)
	if n := len(g.mem.data.Journal.Entries); n != 1 {
		t.Errorf("the meeting should be written once; entries now %d", n)
	}
}

func TestABondStepWritesOnceAndADropWritesTheFirstBadMoment(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	g.SetBond("brie", 12) // curious
	now = run(g, now, 0.2)
	if n := len(g.mem.data.Journal.Entries); n != 1 {
		t.Fatalf("a step up to curious should write one entry, got %d", n)
	}
	if !g.jState().Written(journal.StepKey("brie", "curious")) {
		t.Error("the curious step should be recorded")
	}
	g.SetBond("brie", 0) // a drop: the first bad moment
	now = run(g, now, 0.2)
	g.SetBond("brie", 12) // climbing back to the same tier writes nothing
	now = run(g, now, 0.2)
	g.SetBond("brie", 0)
	now = run(g, now, 0.2)
	entries := g.mem.data.Journal.Entries
	if len(entries) != 2 {
		t.Fatalf("want the step and one bad moment, got %d entries", len(entries))
	}
	if entries[1].Text != g.jcontent.Cats["brie"].Bad {
		t.Errorf("the second entry should be Brie's first bad moment: %q", entries[1].Text)
	}
}

func TestAMomentLeftOpenWhenLeavingWritesNothing(t *testing.T) {
	g, _ := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	if err := g.ShowMoment("brie_table"); err != nil {
		t.Fatal(err)
	}
	g.leave()
	if n := len(g.mem.data.Journal.Entries); n != 0 {
		t.Errorf("a moment left open must not write an entry; got %d", n)
	}
}

func TestAJournalMomentWritesOnce(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	if err := g.ShowMoment("mimi_lap"); err != nil {
		t.Fatal(err)
	}
	now = answerAll(g, now)
	if err := g.ShowMoment("mimi_lap"); err != nil {
		t.Fatal(err)
	}
	answerAll(g, now)
	var lap int
	for _, e := range g.mem.data.Journal.Entries {
		if e.Text == g.jcontent.Moments["mimi_lap"] {
			lap++
		}
	}
	if lap != 1 {
		t.Errorf("the journal moment should be written once, written %d times", lap)
	}
}

func TestTheCatCardsTurnWithArrowsAndCloseWithEsc(t *testing.T) {
	g, _ := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.jrnl = &journalMode{cards: true}
	if !strings.Contains(visible(g.View()), "The cats") {
		t.Error("the cards should have their title")
	}
	g.Key(KeyRight)
	g.Key(KeyRight)
	g.Key(KeyRight) // no card after the last
	if g.jrnl.card != len(g.jcontent.Leaflets)-1 {
		t.Errorf("← → should move between the cards; on card %d", g.jrnl.card)
	}
	g.Key(KeyBack)
	if g.jrnl != nil {
		t.Error("Esc should close the cards and return to the café")
	}
}

// waitForJournal lets the first-time line pass, so the book opens.
func waitForJournal(g *Game, now time.Time) time.Time {
	for i := 0; i < 600 && g.jrnl == nil; i++ {
		now = run(g, now, 0.1)
	}
	return now
}

func TestTheJournalHasNoCatCards(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyDown) // Read journal
	g.Key(KeyConfirm)
	now = waitForJournal(g, now)
	if g.jrnl == nil {
		t.Fatal("Read journal should open the journal")
	}
	if g.jrnl.cards {
		t.Error("the journal should not open the cat cards")
	}
}

func TestTheRightArrowShowsTheNewestEntry(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	for _, cat := range []string{"brie", "hunter", "mimi"} {
		text, _ := g.jcontent.Cats[cat].Step("meet")
		g.writeEntry(journal.MeetKey(cat), cat, text)
	}
	now = run(g, now, 0.1)
	g.jrnl = &journalMode{}
	g.Key(KeyRight)
	sp := g.spreads()
	if len(sp) == 0 || len(sp[0].Right.Entries) == 0 {
		t.Fatal("the first spread after the note should hold entries")
	}
	if got := sp[0].Right.Entries[0]; got.Cat != "mimi" {
		t.Errorf("the newest entry should come first, got %q", got.Cat)
	}
	if !strings.Contains(visible(g.View()), "Monday") && !strings.Contains(visible(g.View()), "Wednesday") &&
		!strings.Contains(visible(g.View()), "November") {
		t.Error("the entry's date should be on the page")
	}
	_ = now
}

// jsonRoundTrip saves and reads a save, as the game does on disk.
func jsonRoundTrip(d *save.Data) (*save.Data, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return save.Parse(raw)
}

// lookAtBookshelf opens Look around and turns to the bookshelf.
func lookAtBookshelf(t *testing.T, g *Game) {
	t.Helper()
	g.box, g.queue = nil, nil
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyConfirm) // Look around
	for i := 0; i < 8; i++ {
		if tg, _ := g.lookTarget(); tg.id() == "bookshelf" {
			break
		}
		g.Key(KeyLeft)
	}
	if tg, _ := g.lookTarget(); tg.id() != "bookshelf" {
		t.Fatal("the bookshelf should be one of the places to look")
	}
}

func TestReadJournalFromTheMenuOpensAtOnce(t *testing.T) {
	g, _ := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyDown) // Order, Look around, Read journal
	g.Key(KeyConfirm)
	if g.pause != nil {
		t.Fatal("the menu should close")
	}
	if g.jrnl == nil || g.jrnl.cards {
		t.Fatal("Read journal should open the journal at once")
	}
	if g.box != nil {
		t.Errorf("no narrator line should show first, got %q", boxText(g))
	}
}

func TestTheBookshelfAsksWhatToRead(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	lookAtBookshelf(t, g)
	_ = now
	if g.look == nil {
		t.Fatal("the bookshelf should keep Look around open, with the question")
	}
	view := visible(g.View())
	for _, want := range []string{"Read something?", "Journal", "About the cats"} {
		if !strings.Contains(view, want) {
			t.Errorf("the bookshelf panel should show %q", want)
		}
	}
}

func TestTheBookshelfJournalChoiceOpensTheBook(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	lookAtBookshelf(t, g)
	g.Key(KeyUp) // already on Journal; Up keeps it there
	g.Key(KeyConfirm)
	now = waitForJournal(g, now)
	if g.jrnl == nil || g.jrnl.cards {
		t.Fatal("Journal should open the journal, not the cards")
	}
	if !g.jState().Discovered {
		t.Error("the first time from the bookshelf, the discovery line should play and mark it found")
	}
}

func TestTheBookshelfCatChoiceOpensTheCards(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	lookAtBookshelf(t, g)
	g.Key(KeyDown) // About the cats
	g.Key(KeyConfirm)
	now = run(g, now, 0.5)
	if g.jrnl == nil || !g.jrnl.cards {
		t.Fatal("About the cats should open the cat cards")
	}
	_ = now
}

func TestEscOnTheBookshelfOpensNothing(t *testing.T) {
	g, _ := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	lookAtBookshelf(t, g)
	g.Key(KeyBack)
	if g.look != nil || g.jrnl != nil {
		t.Fatal("Esc should close Look around without opening anything")
	}
}

func TestTheJournalSurvivesAQuitAndRelaunch(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	g.SetBond("hunter", 12)
	now = run(g, now, 0.2)
	lookAtBookshelf(t, g)
	g.Key(KeyConfirm) // the bookshelf's panel: Journal is first
	now = waitForJournal(g, now)
	now = answerAll(g, now)
	g.Key(KeyBack)
	g.leave()
	raw, err := jsonRoundTrip(g.mem.data)
	if err != nil {
		t.Fatal(err)
	}
	g2, err := New(Config{Fast: true, Save: raw, Seed: 9}, content.FS, now)
	if err != nil {
		t.Fatal(err)
	}
	if !g2.jState().Discovered || len(g2.jState().Entries) != len(g.jState().Entries) {
		t.Errorf("the journal should come back after a relaunch: discovered %v, entries %d",
			g2.jState().Discovered, len(g2.jState().Entries))
	}
	before := len(g2.jState().Entries)
	now = run(g2, now, 0.5)
	if n := len(g2.jState().Entries); n != before {
		t.Errorf("relaunching must not write entries for bonds already there; %d -> %d", before, n)
	}
}

// The cursor on the bookshelf's choices must stay where the arrows put it,
// while the café keeps running (the Look panel is refreshed every tick).
func TestTheBookshelfArrowsStayPutWhileTimePasses(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	lookAtBookshelf(t, g)
	now = run(g, now, 2)
	g.Key(KeyDown)
	now = run(g, now, 2)
	if g.look == nil || g.look.choice != 1 {
		t.Fatalf("↓ should stay on About the cats, on choice %d", g.look.choice)
	}
	if !strings.Contains(visible(g.View()), "▸ About the cats") {
		t.Error("the marker should show on About the cats")
	}
	g.Key(KeyUp)
	now = run(g, now, 2)
	if g.look.choice != 0 {
		t.Fatalf("↑ should go back to Journal, on choice %d", g.look.choice)
	}
	g.Key(KeyUp) // no choice above the first
	_ = now
	if g.look.choice != 0 {
		t.Errorf("↑ above the first choice should stay put, on %d", g.look.choice)
	}
}

func TestTheCatCardFitsTheScreen(t *testing.T) {
	g, _ := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	w, h := g.cardSize()
	if h > FrameH-2 || w > FrameW-2 {
		t.Fatalf("the card (%d × %d) should fit inside the frame (%d × %d)", w, h, FrameW, FrameH)
	}
}

// A conversation that is already playing does not hold the journal back: the
// notebook's own line comes to the front at once, and the conversation goes on
// when the book closes.
func TestTheJournalTakesOverFromAConversationAlreadyPlaying(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	now = run(g, now, 0.2)
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyConfirm) // Look around
	g.queue = append(g.queue, &talkBox{
		Box:  dialogue.Start("", []string{"A background line that is still playing."}, nil, 0, g.talkOptions()),
		kind: talkNarrator,
	})
	g.nextTalk()
	now = run(g, now, 0.5)
	for i := 0; i < 8; i++ { // turn to the bookshelf without clearing the conversation
		if tg, _ := g.lookTarget(); tg.id() == "bookshelf" {
			break
		}
		g.Key(KeyLeft)
	}
	g.Key(KeyConfirm)
	g.Key(KeyConfirm) // Journal
	now = run(g, now, 0.5)
	if !strings.Contains(boxText(g), "notebook") {
		t.Fatalf("the notebook's line should come to the front at once, got %q", boxText(g))
	}
	now = waitForJournal(g, now)
	if g.jrnl == nil || g.jrnl.cards {
		t.Fatal("the journal should open")
	}
	g.Key(KeyBack)
	if g.box == nil && len(g.queue) == 0 {
		t.Error("the background conversation should go on after the journal closes")
	}
	_ = now
}

// From the Esc menu, the book opens at once over a conversation that is playing.
func TestReadJournalFromTheMenuOverAConversation(t *testing.T) {
	g, now := inside(t, Config{Fast: true})
	g.Resize(FrameW, FrameH)
	now = run(g, now, 0.2)
	g.queue = append(g.queue, &talkBox{
		Box:  dialogue.Start("", []string{"A background line that is still playing."}, nil, 0, g.talkOptions()),
		kind: talkNarrator,
	})
	g.nextTalk()
	now = run(g, now, 0.5)
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyDown)
	g.Key(KeyConfirm)
	if g.jrnl == nil {
		t.Fatal("Read journal should open the book at once, over the conversation")
	}
	_ = now
}
