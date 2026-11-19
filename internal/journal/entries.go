package journal

import (
	"math/rand/v2"
	"strings"
	"time"
)

// AsideChance is the share of entries that get an aside. Often there is none.
const AsideChance = 0.35

// Width is how many letters fit on one line of a page.
const Width = 46

// PageLines is how many lines a page can hold, headers included.
const PageLines = 24

// Entry is one written entry. The text is saved as it was written, so later
// content edits never change the player's past.
type Entry struct {
	Cat    string `json:"cat,omitempty"`
	Header string `json:"header"`
	Text   string `json:"text"`
	Aside  string `json:"aside,omitempty"`
}

// State is what the journal remembers, saved in save.json.
type State struct {
	Discovered bool     `json:"discovered,omitempty"` // the first-time line has been shown
	Done       []string `json:"done,omitempty"`       // keys of what has been written (see the Key functions)
	Entries    []Entry  `json:"entries,omitempty"`    // oldest first
	Asides     []string `json:"asides,omitempty"`     // asides used since the pool last ran out
}

// Key functions name each thing that is written once.

// MeetKey is the key for the first moment with a cat.
func MeetKey(cat string) string { return cat + ":meet" }

// StepKey is the key for a bond step up to a tier.
func StepKey(cat, tier string) string { return cat + ":" + tier }

// BadKey is the key for a cat's first bad moment.
func BadKey(cat string) string { return cat + ":bad" }

// MomentKey is the key for a journal moment.
func MomentKey(id string) string { return "moment:" + id }

// Context is what an aside can depend on.
type Context struct {
	Weather string // a weather kind, such as "rain" or "clear"
	When    string // a part of the day, from PartOfDay
	Drink   string // the drink id, or "" for none
}

// Open is what the player sees when they open the journal. The first time
// it is the discovery line; after that the short line. It is remembered.
func (s *State) Open(c *Content) string {
	if !s.Discovered {
		s.Discovered = true
		return c.Text.FirstTime
	}
	return c.Text.TookDown
}

// Written reports whether the thing with this key has already been written.
func (s *State) Written(key string) bool {
	for _, d := range s.Done {
		if d == key {
			return true
		}
	}
	return false
}

// Record writes an entry for key (one of the Key functions) if it has not
// been written before. The text is the cat's part; an aside may follow.
// It reports whether an entry was written.
func (s *State) Record(c *Content, key, cat, text string, at time.Time, weather, drink string, rng *rand.Rand) (Entry, bool) {
	if text == "" || s.Written(key) {
		return Entry{}, false
	}
	e := Entry{
		Cat:    cat,
		Header: Header(at, weather),
		Text:   text,
		Aside:  s.pickAside(c, Context{Weather: weather, When: PartOfDay(at), Drink: drink}, rng),
	}
	s.Entries = append(s.Entries, e)
	s.Done = append(s.Done, key)
	return e, true
}

// pickAside returns one aside that fits the context, or "" for none. An aside
// is not repeated until the ones that fit have all been used.
func (s *State) pickAside(c *Content, ctx Context, rng *rand.Rand) string {
	if rng == nil || rng.Float64() >= AsideChance {
		return ""
	}
	var fits []Aside
	for _, a := range c.Asides {
		if a.fits(ctx) {
			fits = append(fits, a)
		}
	}
	if len(fits) == 0 {
		return ""
	}
	var free []Aside
	for _, a := range fits {
		if !s.usedAside(a.ID) {
			free = append(free, a)
		}
	}
	if len(free) == 0 {
		// Every aside that fits has been used: start that pool again, but
		// not with the one just used.
		last := ""
		if n := len(s.Asides); n > 0 {
			last = s.Asides[n-1]
		}
		for _, a := range fits {
			s.forgetAside(a.ID)
		}
		for _, a := range fits {
			if a.ID != last {
				free = append(free, a)
			}
		}
		if len(free) == 0 {
			free = fits
		}
	}
	pick := free[rng.IntN(len(free))]
	s.Asides = append(s.Asides, pick.ID)
	return pick.Text
}

func (a Aside) fits(ctx Context) bool {
	return matches(a.Weather, ctx.Weather) && matches(a.When, ctx.When) && matches(a.Drink, ctx.Drink)
}

func matches(list []string, v string) bool {
	if len(list) == 0 {
		return true
	}
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *State) usedAside(id string) bool {
	for _, u := range s.Asides {
		if u == id {
			return true
		}
	}
	return false
}

func (s *State) forgetAside(id string) {
	kept := s.Asides[:0]
	for _, u := range s.Asides {
		if u != id {
			kept = append(kept, u)
		}
	}
	s.Asides = kept
}

// Header is the line above an entry: the day, date and time, then the weather
// the café was showing, for example "Wednesday 18 November, 07:40. Clear."
func Header(at time.Time, weather string) string {
	h := at.Format("Monday 2 January, 15:04") + "."
	if w := weatherWords[weather]; w != "" {
		h += " " + w + "."
	}
	return h
}

var weatherWords = map[string]string{
	"clear":      "Clear",
	"cloudy":     "Cloudy",
	"drizzle":    "Drizzle",
	"rain":       "Rain",
	"heavy_rain": "Heavy rain",
	"fog":        "Fog",
	"snow":       "Snow",
}

// PartOfDay names the part of the day an entry was written in, for asides.
func PartOfDay(t time.Time) string {
	switch h := t.Hour(); {
	case h < 5:
		return "late night"
	case h < 7:
		return "dawn"
	case h < 12:
		return "morning"
	case h < 17:
		return "afternoon"
	case h < 21:
		return "evening"
	default:
		return "night"
	}
}

// Page is up to two entries, newest first.
type Page struct {
	Entries []Entry
}

// Spread is two pages, left and right. A page may be empty.
type Spread struct {
	Left, Right Page
}

// Spreads lays the entries out for the book. The first spread's left page is
// the introduction, so the first page of entries is its right-hand page; the
// pages after it go two to a spread. Entries are newest first, one or two to a
// page depending on their length.
func (s *State) Spreads() []Spread {
	var pages []Page
	var cur []Entry
	lines := 0
	for i := len(s.Entries) - 1; i >= 0; i-- {
		e := s.Entries[i]
		n := EntryLines(e)
		if len(cur) > 0 && (len(cur) == 2 || lines+1+n > PageLines) {
			pages = append(pages, Page{Entries: cur})
			cur, lines = nil, 0
		}
		if len(cur) > 0 {
			lines++ // the gap between entries on one page
		}
		cur = append(cur, e)
		lines += n
	}
	if len(cur) > 0 {
		pages = append(pages, Page{Entries: cur})
	}
	var spreads []Spread
	if len(pages) == 0 {
		return nil
	}
	spreads = append(spreads, Spread{Right: pages[0]})
	rest := pages[1:]
	for i := 0; i < len(rest); i += 2 {
		sp := Spread{Left: rest[i]}
		if i+1 < len(rest) {
			sp.Right = rest[i+1]
		}
		spreads = append(spreads, sp)
	}
	return spreads
}

// EntryLines is how many lines an entry takes at Width letters a line.
func EntryLines(e Entry) int {
	n := len(Wrap(e.Header, Width)) + len(Wrap(e.Text, Width))
	if e.Aside != "" {
		n += len(Wrap(e.Aside, Width))
	}
	return n
}

// Wrap breaks text into lines of at most width letters, at spaces. A word
// longer than the width gets a line of its own.
func Wrap(text string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(text) {
		switch {
		case line == "":
			line = w
		case len([]rune(line))+1+len([]rune(w)) <= width:
			line += " " + w
		default:
			lines = append(lines, line)
			line = w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
