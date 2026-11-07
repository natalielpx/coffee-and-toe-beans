// Package dialogue is the visual-novel dialogue box: words that type
// themselves out letter by letter, a blinking ▼ when a page is finished,
// and 2–4 choices picked with the arrow keys.
//
// It only keeps the state (what is shown, what is selected, what was
// chosen); the game draws it. Conversations are written in content/dialogue
// as JSON, so they can be read and changed without touching any code.
//
// The narrator's boxes never wait for the player forever: a page that is
// left alone moves on by itself after a while, and choices that are left
// alone resolve gently with the conversation's "ignore" choice. A box made
// with Options.Wait (the cats' moments) waits for the player instead, and
// only takes its "ignore" choice when it is closed with Abandon (leaving
// the café with it still open).
package dialogue

import (
	"math/rand/v2"
	"strings"

	"coffee-and-toe-beans/internal/canvas"
)

// Choice is one answer the player can pick.
type Choice struct {
	Text string `json:"text"`
	// Do names something for the game to do when this is chosen, such as
	// "order" (open the drinks menu). Empty means nothing happens.
	Do string `json:"do"`
}

// Result is how a conversation ended.
type Result struct {
	Choice  int    // index of the choice taken, or -1 if there were none
	Do      string // that choice's action ("" for none)
	Ignored bool   // true if the player let it pass without answering
}

// Options shape how a box behaves.
type Options struct {
	Width    int     // text width in cells
	Lines    int     // most lines on one page
	Speed    float64 // letters per second; 0 shows every page at once
	Linger   float64 // seconds a finished page stays before moving on alone
	PerRune  float64 // extra linger per letter, so long pages stay longer
	Patience float64 // seconds choices wait before the "ignore" choice
	Fade     float64 // seconds to fade in and out
	Wait     bool    // wait for the player on every page (Linger and Patience unused)
}

// DefaultOptions fit the café's dialogue box.
func DefaultOptions() Options {
	return Options{
		Width:    46,
		Lines:    3,
		Speed:    34,
		Linger:   7,
		PerRune:  0.05,
		Patience: 90,
		Fade:     0.35,
	}
}

type state int

const (
	opening state = iota
	typing
	waiting
	closing
	closed
)

// Box is one conversation on screen.
type Box struct {
	opts    Options
	speaker string
	pages   [][]string // each page already wrapped into lines
	page    int
	shown   int     // letters revealed on this page
	budget  float64 // typing time not yet spent
	idle    float64 // seconds since the page finished (or since a key)
	fade    float64 // 0 hidden .. 1 fully shown
	state   state
	choices []Choice
	sel     int
	ignore  int
	result  Result
	paused  bool
	orig    []int // after Shuffle: the written place of each choice shown
}

// Start opens a conversation. pages are the texts to show (placeholders
// already filled in); ignore is the choice taken if the player doesn't
// answer.
func Start(speaker string, pages []string, choices []Choice, ignore int, opts Options) *Box {
	if opts.Width <= 0 {
		opts.Width = DefaultOptions().Width
	}
	if opts.Lines <= 0 {
		opts.Lines = DefaultOptions().Lines
	}
	b := &Box{opts: opts, speaker: speaker, choices: choices, ignore: ignore, result: Result{Choice: -1}}
	for _, p := range pages {
		b.pages = append(b.pages, splitPages(Wrap(p, opts.Width), opts.Lines)...)
	}
	if len(b.pages) == 0 {
		b.pages = [][]string{{""}}
	}
	if b.ignore < 0 || b.ignore >= len(choices) {
		b.ignore = 0
	}
	b.state = opening
	if opts.Fade <= 0 {
		b.fade = 1
		b.state = typing
	}
	if opts.Speed <= 0 {
		b.shown = b.pageLen()
	}
	return b
}

func splitPages(lines []string, max int) [][]string {
	var out [][]string
	for len(lines) > max {
		out = append(out, lines[:max])
		lines = lines[max:]
	}
	return append(out, lines)
}

func (b *Box) pageLen() int {
	n := 0
	for _, l := range b.pages[b.page] {
		n += len([]rune(l))
	}
	return n
}

// runeAt is the n-th letter of the current page (lines joined).
func (b *Box) runeAt(n int) rune {
	for _, l := range b.pages[b.page] {
		r := []rune(l)
		if n < len(r) {
			return r[n]
		}
		n -= len(r)
	}
	return ' '
}

// pause is how long typing rests after a letter, like a breath.
func pause(r rune) float64 {
	switch r {
	case '.', '!', '?', '…':
		return 0.32
	case ',', ';', ':', '—':
		return 0.14
	}
	return 0
}

// SetPaused stops the clock (while a menu covers the box, say).
func (b *Box) SetPaused(p bool) { b.paused = p }

// Update moves the box forward by dt seconds of real time.
func (b *Box) Update(dt float64) {
	if b.paused || dt <= 0 {
		return
	}
	fadeStep := 1.0
	if b.opts.Fade > 0 {
		fadeStep = dt / b.opts.Fade
	}
	switch b.state {
	case opening:
		b.fade += fadeStep
		if b.fade >= 1 {
			b.fade = 1
			b.state = typing
		}
	case typing:
		b.typeOn(dt)
	case waiting:
		b.idle += dt
		if b.opts.Wait {
			break
		}
		if len(b.choices) > 0 && b.onLastPage() {
			if b.idle >= b.opts.Patience {
				b.choose(b.ignore, true)
			}
		} else if b.idle >= b.opts.Linger+b.opts.PerRune*float64(b.pageLen()) {
			b.advance(true)
		}
	case closing:
		b.fade -= fadeStep
		if b.fade <= 0 {
			b.fade = 0
			b.state = closed
		}
	}
}

func (b *Box) typeOn(dt float64) {
	total := b.pageLen()
	if b.opts.Speed <= 0 {
		b.shown = total
	}
	b.budget += dt
	for b.shown < total {
		cost := 1 / b.opts.Speed
		if b.shown > 0 {
			prev := b.runeAt(b.shown - 1)
			// Rest after punctuation, unless more punctuation follows ("…", "?!").
			if pause(b.runeAt(b.shown)) == 0 {
				cost += pause(prev)
			}
		}
		if b.budget < cost {
			break
		}
		b.budget -= cost
		b.shown++
	}
	if b.shown >= total {
		b.shown = total
		b.budget = 0
		b.state = waiting
		b.idle = 0
	}
}

func (b *Box) onLastPage() bool { return b.page == len(b.pages)-1 }

// advance goes to the next page, or ends the conversation after the last
// one (only when there are no choices to make).
func (b *Box) advance(ignored bool) {
	if !b.onLastPage() {
		b.page++
		b.shown, b.budget, b.idle = 0, 0, 0
		b.state = typing
		if b.opts.Speed <= 0 {
			b.shown = b.pageLen()
			b.state = waiting
		}
		return
	}
	if len(b.choices) > 0 {
		b.choose(b.ignore, ignored)
		return
	}
	b.result = Result{Choice: -1, Ignored: ignored}
	b.state = closing
}

func (b *Box) choose(i int, ignored bool) {
	written := i
	if b.orig != nil {
		written = b.orig[i]
	}
	b.result = Result{Choice: written, Do: b.choices[i].Do, Ignored: ignored}
	b.state = closing
}

// Shuffle shows the choices in a random order. Results still count
// choices as they are written (Result.Choice is the written place), and
// the "ignore" choice stays the same choice. Call it before the box is
// shown.
func (b *Box) Shuffle(rng *rand.Rand) {
	n := len(b.choices)
	if n < 2 || rng == nil {
		return
	}
	perm := rng.Perm(n)
	shown := make([]Choice, n)
	ignore := 0
	for i, w := range perm {
		shown[i] = b.choices[w]
		if w == b.origIgnore() {
			ignore = i
		}
	}
	b.choices, b.orig, b.ignore = shown, perm, ignore
}

func (b *Box) origIgnore() int {
	if b.orig != nil {
		return b.orig[b.ignore]
	}
	return b.ignore
}

// Confirm is Enter or Space: show the whole page at once, turn the page,
// or pick the selected choice.
func (b *Box) Confirm() {
	b.idle = 0
	switch b.state {
	case opening:
		b.fade = 1
		b.state = typing
		b.shown = b.pageLen()
		b.typeOn(0)
	case typing:
		b.shown = b.pageLen()
		b.typeOn(0)
	case waiting:
		if b.ShowingChoices() {
			b.choose(b.sel, false)
			return
		}
		b.advance(false)
	}
}

// Abandon ends the conversation at once, as if it had been left to pass:
// with choices, the "ignore" choice is taken. Used when the player leaves
// the café with a box still open. It does nothing to a finished box.
func (b *Box) Abandon() {
	if b.Answered() {
		return
	}
	if len(b.choices) > 0 {
		b.choose(b.ignore, true)
	} else {
		b.result = Result{Choice: -1, Ignored: true}
	}
	b.state = closed
	b.fade = 0
}

// Pages are all the conversation's pages, each as one line of text (for
// tests and previews).
func (b *Box) Pages() []string {
	var out []string
	for _, p := range b.pages {
		out = append(out, strings.Join(p, " "))
	}
	return out
}

// Page is the page shown now (0 for the first).
func (b *Box) Page() int { return b.page }

// Waits is true for a box that waits for the player (Options.Wait).
func (b *Box) Waits() bool { return b.opts.Wait }

// Up moves the selection up (it stops at the top; no wrapping around).
func (b *Box) Up() {
	b.idle = 0
	if b.ShowingChoices() && b.sel > 0 {
		b.sel--
	}
}

// Down moves the selection down.
func (b *Box) Down() {
	b.idle = 0
	if b.ShowingChoices() && b.sel < len(b.choices)-1 {
		b.sel++
	}
}

// Done reports the result once the box has fully faded away.
func (b *Box) Done() (Result, bool) {
	return b.result, b.state == closed
}

// Answered is true as soon as the conversation is over, even while the box
// is still fading out.
func (b *Box) Answered() bool { return b.state == closing || b.state == closed }

// Speaker is the name shown on the box ("" for the narrator).
func (b *Box) Speaker() string { return b.speaker }

// Alpha is how visible the box is, 0..1.
func (b *Box) Alpha() float64 { return b.fade }

// Lines is the text revealed so far on this page, one string per line.
func (b *Box) Lines() []string {
	left := b.shown
	out := make([]string, 0, len(b.pages[b.page]))
	for _, l := range b.pages[b.page] {
		r := []rune(l)
		if left >= len(r) {
			out = append(out, l)
			left -= len(r)
			continue
		}
		out = append(out, string(r[:left]))
		left = 0
	}
	return out
}

// PageLines is the full text of this page (for laying out the box before
// it has finished typing, so it never changes size).
func (b *Box) PageLines() []string { return b.pages[b.page] }

// MaxPageLines is the most lines any page of this conversation has.
func (b *Box) MaxPageLines() int {
	n := 1
	for _, p := range b.pages {
		if len(p) > n {
			n = len(p)
		}
	}
	return n
}

// Finished is true when this page has finished typing.
func (b *Box) Finished() bool { return b.state == waiting }

// MoreToCome is true when ▼ should show: the page is finished and Enter
// will turn to another page or close the box.
func (b *Box) MoreToCome() bool { return b.state == waiting && !b.ShowingChoices() }

// ShowingChoices is true once the last page has typed out and has choices.
func (b *Box) ShowingChoices() bool {
	return len(b.choices) > 0 && b.onLastPage() && (b.state == waiting || b.state == closing)
}

// Choices are the answers on offer.
func (b *Box) Choices() []Choice { return b.choices }

// Selected is the index of the highlighted choice.
func (b *Box) Selected() int { return b.sel }

// Wrap breaks text into lines no wider than width cells, at spaces. A
// newline in the text always starts a new line. A word longer than a whole
// line is cut.
func Wrap(text string, width int) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		line := ""
		for _, w := range words {
			for canvas.Width(w) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				head, tail := cut(w, width)
				out = append(out, head)
				w = tail
			}
			switch {
			case line == "":
				line = w
			case canvas.Width(line)+1+canvas.Width(w) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
	}
	return out
}

func cut(w string, width int) (string, string) {
	n := 0
	for i, r := range w {
		if n+canvas.RuneWidth(r) > width {
			return w[:i], w[i:]
		}
		n += canvas.RuneWidth(r)
	}
	return w, ""
}
