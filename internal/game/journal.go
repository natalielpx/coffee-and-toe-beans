package game

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/dialogue"
	"coffee-and-toe-beans/internal/journal"
)

// The journal is the notebook on the café's bookshelf. Its rules are in
// internal/journal; this file connects them to the game and draws the book.

// The journal spread is drawn in the middle of the café.
const (
	journalW = 100 // two pages of journal.Width letters, and a gutter
	journalH = 38
)

// journalMode is what's open. The journal is a book: spread 0 is Nala's note,
// then the entries. The cat cards (About the cats) are a separate view, one
// card at a time, with cards set.
type journalMode struct {
	spread int
	cards  bool
	card   int
}

// loadJournal reads the journal's words and the bookshelf's place in the room.
func (g *Game) loadJournal(fsys fs.FS) error {
	jc, err := journal.Load(fsys)
	if err != nil {
		return err
	}
	var ids []string
	for _, m := range g.cat.content.Moments {
		if m.Journal {
			ids = append(ids, m.ID)
		}
	}
	if err := jc.CheckMoments(ids); err != nil {
		return err
	}
	g.jcontent = jc
	return g.loadBookshelf(fsys)
}

// loadBookshelf finds the bookshelf in this café's room, so Look around can
// point at it. The place comes from the interior file, not from code.
func (g *Game) loadBookshelf(fsys fs.FS) error {
	raw, err := fs.ReadFile(fsys, path.Join("interiors", g.cfg.Interior+".json"))
	if err != nil {
		return err
	}
	var f struct {
		Elements []json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("interiors/%s.json: %v", g.cfg.Interior, err)
	}
	for _, e := range f.Elements {
		var el struct {
			Type string `json:"type"`
			Rect [4]int `json:"rect"`
		}
		if err := json.Unmarshal(e, &el); err != nil {
			continue
		}
		if el.Type == "bookshelf" {
			g.bookX = (el.Rect[0] + el.Rect[2]) / 2
			g.bookTop = el.Rect[1]
			g.hasBookshelf = true
			return nil
		}
	}
	return fmt.Errorf("interiors/%s.json has no bookshelf, so the journal has nowhere to be", g.cfg.Interior)
}

// syncJournalBonds notes each cat's bond as it is now, so only changes made
// from here on are written in the journal (a returning guest gets no entries
// for what was already there).
func (g *Game) syncJournalBonds() {
	g.jBonds = map[string]float64{}
	for _, c := range g.cat.world.Cats {
		g.jBonds[c.Def.ID] = c.Bond
	}
}

// jState is the journal as it is saved.
func (g *Game) jState() *journal.State { return &g.mem.data.Journal }

// writeEntry writes an entry for key, if it hasn't been written, and says
// softly that you made a note.
func (g *Game) writeEntry(key, cat, text string) {
	if g.jcontent == nil || g.mem.data == nil || text == "" {
		return
	}
	drink := ""
	if g.order != nil && g.order.drink != nil {
		drink = g.order.drink.ID
	}
	_, ok := g.jState().Record(g.jcontent, key, cat, text, g.Now(), g.wstate.Kind(), drink, g.rng)
	if !ok {
		return
	}
	tb := &talkBox{Box: dialogue.Start("", []string{g.jcontent.Text.NoteWritten}, nil, 0, g.talkOptions()), kind: talkNarrator}
	g.queue = append(g.queue, tb)
	g.nextTalk()
}

// checkBonds writes the journal entries for bond changes since the last look:
// a step up to a new tier, and the first time a cat's bond goes down.
func (g *Game) checkBonds() {
	if g.jcontent == nil || g.jBonds == nil {
		return
	}
	for _, c := range g.cat.world.Cats {
		id := c.Def.ID
		prev, cur := g.jBonds[id], c.Bond
		g.jBonds[id] = cur
		lines := g.jcontent.Cats[id]
		if cur < prev-1e-9 {
			text, _ := lines.Step("bad")
			g.writeEntry(journal.BadKey(id), id, text)
		}
		for t := cats.TierOf(prev) + 1; t <= cats.TierOf(cur); t++ {
			text, _ := lines.Step(t.String())
			g.writeEntry(journal.StepKey(id, t.String()), id, text)
		}
	}
}

// recordMoment writes the entries that a finished moment earns: the first
// moment with a cat, and a journal moment the first time it happens. A moment
// you left open (ignored) writes nothing.
func (g *Game) recordMoment(b *talkBox, r dialogue.Result) {
	if r.Ignored || g.jcontent == nil {
		return
	}
	id := b.catOf.Def.ID
	if !g.jState().Written(journal.MeetKey(id)) {
		text, _ := g.jcontent.Cats[id].Step("meet")
		g.writeEntry(journal.MeetKey(id), id, text)
	}
	if b.moment.Journal {
		g.writeEntry(journal.MomentKey(b.moment.ID), id, g.jcontent.Moments[b.moment.ID])
	}
}

// openJournalNow opens the book at once, from the Esc menu.
func (g *Game) openJournalNow() {
	g.pause = nil
	g.jrnl = &journalMode{}
	g.holdTalk(true)
}

// openJournal takes the notebook down from the bookshelf. The first time, the
// discovery line plays first; the book opens when that line has passed.
func (g *Game) openJournal() {
	g.pause = nil
	line := g.jState().Open(g.jcontent)
	tb := &talkBox{
		Box:  dialogue.Start("", []string{line}, nil, 0, g.talkOptions()),
		kind: talkNarrator,
		then: func() {
			g.jrnl = &journalMode{}
			g.holdTalk(true)
		},
	}
	if g.box != nil {
		// A conversation is playing: the notebook takes over now. The
		// conversation waits first in the queue and goes on when the book closes.
		g.queue = append([]*talkBox{g.box}, g.queue...)
		g.box = nil
	}
	g.box = tb
}

// openCatCards opens the cat cards: one card at a time, turned with ← →.
func (g *Game) openCatCards() {
	g.jrnl = &journalMode{cards: true}
	g.holdTalk(true)
}

// closeJournal puts the notebook back and returns to the café.
func (g *Game) closeJournal() {
	g.jrnl = nil
	g.holdTalk(g.busy())
	g.nextTalk()
}

// spreads is how many spreads the book has after the first one.
func (g *Game) spreads() []journal.Spread { return g.jState().Spreads() }

func (g *Game) journalKey(k Key) {
	j := g.jrnl
	if j.cards {
		cards := len(g.jcontent.Leaflets)
		switch k {
		case KeyBack, KeyConfirm:
			g.closeJournal()
		case KeyLeft:
			if j.card > 0 {
				j.card--
			}
		case KeyRight:
			if j.card < cards-1 {
				j.card++
			}
		}
		return
	}
	switch k {
	case KeyBack:
		g.closeJournal()
	case KeyLeft:
		if j.spread > 0 {
			j.spread--
		}
	case KeyRight:
		if j.spread < len(g.spreads())-1 {
			j.spread++
		}
	}
}

// drawJournal draws the open book over the dimmed café. The cat cards are
// their own card-sized panel, not the book.
func (g *Game) drawJournal(c *canvas.Canvas, ox, oy int) {
	j := g.jrnl
	g.dim(c, ox, oy, 0.5)
	if j.cards {
		g.drawCards(c, ox, oy, j.card)
		return
	}
	x := ox + (FrameW-journalW)/2
	y := oy + (FrameH-journalH)/2
	p := panel{c: c, x: x, y: y, w: journalW, h: journalH, alpha: 1}
	p.draw()
	const left, right = 4, 52 // where each page's words start, inside the panel
	for row := 2; row < journalH-3; row++ {
		p.write(journal.Width+4, row, "│", boxFaint)
	}
	if j.spread == 0 {
		g.drawNote(p, left)
		if sp := g.spreads(); len(sp) > 0 {
			drawEntries(p, right, 3, sp[0].Right)
		}
	} else {
		sp := g.spreads()[j.spread]
		drawEntries(p, left, 3, sp.Left)
		drawEntries(p, right, 3, sp.Right)
	}
	hint := g.jcontent.Text.BookControls
	p.write((journalW-canvas.Width(hint))/2, journalH-2, hint, boxFaint)
}

// cardSize is the size of the cat card: tall enough for the longest card, so
// the panel stays the same size as you turn between cats.
func (g *Game) cardSize() (w, h int) {
	w = journal.Width + 6
	h = 9
	for _, l := range g.jcontent.Leaflets {
		c := l.Lines(journal.Width)
		if n := len(c.Body) + len(c.Comment) + 9; n > h {
			h = n
		}
	}
	return w, h
}

// drawCards draws one cat's card as a card-sized panel: the title, the name
// in bold with age and gender, the sections, and Nala's comment under the
// story. The hint is at the bottom.
func (g *Game) drawCards(c *canvas.Canvas, ox, oy, card int) {
	w, h := g.cardSize()
	x := ox + (FrameW-w)/2
	y := oy + (FrameH-h)/2
	p := panel{c: c, x: x, y: y, w: w, h: h, alpha: 1}
	p.draw()
	const left = 3
	title := g.jcontent.Text.LeafletTitle
	p.writeStyled((w-canvas.Width(title))/2, 2, title, softInk, true)
	hint := g.jcontent.Text.LeafletControls
	if len(g.jcontent.Leaflets) < 2 {
		hint = g.jcontent.Text.LeafletOne
	}
	p.write((w-canvas.Width(hint))/2, h-2, hint, boxFaint)
	cards := g.jcontent.Leaflets
	if card >= len(cards) {
		return
	}
	lines := cards[card].Lines(journal.Width)
	p.writeBold(left, 4, lines.Name, boxInk)
	p.write(left+canvas.Width(lines.Name), 4, lines.Age, boxDim)
	row := 6
	for _, r := range lines.Body {
		switch {
		case r.Label:
			p.write(left, row, r.Text, boxDim)
		case r.Text != "":
			p.write(left, row, r.Text, boxInk)
		}
		row++
	}
	// Nala's comment, in italics, straight under the story.
	for _, line := range lines.Comment {
		p.writeStyled(left, row, line, boxDim, true)
		row++
	}
}

// drawNote writes Nala's note on the left-hand page of the first spread.
func (g *Game) drawNote(p panel, left int) {
	row := 3
	for i, para := range g.jcontent.Text.Note {
		if i > 0 {
			row++
		}
		if i == len(g.jcontent.Text.Note)-1 { // the signature, on the right
			p.write(left+journal.Width-canvas.Width(para), row, para, boxDim)
			continue
		}
		for _, line := range journal.Wrap(para, journal.Width) {
			p.write(left, row, line, boxInk)
			row++
		}
	}
}

// drawEntries writes one page of entries from row down.
func drawEntries(p panel, col, row int, pg journal.Page) {
	for i, e := range pg.Entries {
		if i > 0 {
			row++
		}
		for _, l := range journal.Wrap(e.Header, journal.Width) {
			p.writeStyled(col, row, l, boxDim, true)
			row++
		}
		for _, l := range journal.Wrap(e.Text, journal.Width) {
			p.write(col, row, l, boxInk)
			row++
		}
		if e.Aside != "" {
			for _, l := range journal.Wrap(e.Aside, journal.Width) {
				p.writeStyled(col, row, l, boxDim, true)
				row++
			}
		}
	}
}
