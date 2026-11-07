// Package narrator is the café's soft second-person voice: the welcome
// when you step inside, the "while you were away" note when you come back,
// and a little small talk now and then. Every word lives in
// content/narrator; this package only picks what fits.
package narrator

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"path"
	"sort"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/dialogue"
)

// Line is one thing the narrator can say, with when it fits. Empty
// conditions mean "any".
type Line struct {
	Text    dialogue.Text `json:"text"`
	When    []string      `json:"when"`    // parts of the day
	Weather []string      `json:"weather"` // weather kinds
	Visit   string        `json:"visit"`   // welcomes: first, return or regular
	About   string        `json:"about"`   // small talk: what it's about (for reading only)
}

func (l Line) conditions() int {
	n := 0
	if len(l.When) > 0 {
		n++
	}
	if len(l.Weather) > 0 {
		n++
	}
	return n
}

func (l Line) fits(c Context) bool {
	if len(l.When) > 0 && !has(l.When, c.Part) {
		return false
	}
	if len(l.Weather) > 0 && !has(l.Weather, c.Weather) {
		return false
	}
	return true
}

// Context is what the narrator knows about this moment.
type Context struct {
	Part    string // part of the day: "morning", "late night"...
	Weather string // weather kind
	First   bool   // your very first visit
	Regular bool   // you've spent a good while here already
}

// Gap is a length of absence, by name ("minutes", "hours", "day"...).
type Gap struct {
	Name       string  `json:"name"`
	UnderHours float64 `json:"under_hours"` // 0 for the last one (and longer)
}

// Opener starts an away note, for one gap.
type Opener struct {
	Gap  string        `json:"gap"`
	Text dialogue.Text `json:"text"`
}

// Change is a small change in the café, for a second page after a longer
// absence: From is the shortest gap it fits.
type Change struct {
	From string        `json:"from"`
	Text dialogue.Text `json:"text"`
}

// CatLine is what a cat was like while you were away (from the cat's
// moments file): Bond is "new", "knows" or "yours"; Gone is "short" or
// "long".
type CatLine struct {
	Cat  string
	Bond string
	Gone string
	Text dialogue.Text
}

// Bands are how well a cat knows you, as away lines say it.
var Bands = []string{"new", "knows", "yours"}

// Lengths are how long you were gone, as away lines say it.
var Lengths = []string{"short", "long"}

// Content is everything the narrator can say.
type Content struct {
	Welcomes []Line
	Talk     []Line // small talk

	Gaps          []Gap
	LongFromHours float64 // cat lines: "long" from this many hours
	ChangeChance  float64 // chance of a second page, for gaps from ChangeFrom
	ChangeFrom    string
	Openers       []Opener
	Changes       []Change

	RegularHours float64 // "regular" welcomes after this many hours in the café
	cats         map[string][]CatLine
}

type welcomeFile struct {
	RegularAfterHours float64 `json:"regular_after_hours"`
	Lines             []Line  `json:"lines"`
}

type awayFile struct {
	Gaps          []Gap    `json:"gaps"`
	LongFromHours float64  `json:"long_from_hours"`
	ChangeChance  float64  `json:"second_page_chance"`
	ChangeFrom    string   `json:"second_page_from"`
	Openers       []Opener `json:"openers"`
	Changes       []Change `json:"changes"`
}

type talkFile struct {
	Lines []Line `json:"lines"`
}

// Checks is what the content is checked against.
type Checks struct {
	Parts    []string // parts of the day
	Weathers []string
}

// Load reads welcome.json, away.json and small_talk.json from dir.
func Load(fsys fs.FS, dir string, ck Checks) (*Content, error) {
	c := &Content{cats: map[string][]CatLine{}}
	var w welcomeFile
	if err := readJSON(fsys, path.Join(dir, "welcome.json"), &w); err != nil {
		return nil, err
	}
	var a awayFile
	if err := readJSON(fsys, path.Join(dir, "away.json"), &a); err != nil {
		return nil, err
	}
	var t talkFile
	if err := readJSON(fsys, path.Join(dir, "small_talk.json"), &t); err != nil {
		return nil, err
	}
	c.Welcomes, c.RegularHours = w.Lines, w.RegularAfterHours
	c.Talk = t.Lines
	c.Gaps, c.LongFromHours, c.ChangeChance, c.ChangeFrom = a.Gaps, a.LongFromHours, a.ChangeChance, a.ChangeFrom
	c.Openers, c.Changes = a.Openers, a.Changes
	if err := c.check(dir, ck); err != nil {
		return nil, err
	}
	return c, nil
}

func readJSON(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	return nil
}

var visits = []string{"first", "return", "regular"}

func (c *Content) check(dir string, ck Checks) error {
	lines := func(file string, ls []Line, welcome bool) error {
		if len(ls) == 0 {
			return fmt.Errorf("%s/%s: needs some lines", dir, file)
		}
		plain := map[string]bool{}
		for i, l := range ls {
			where := fmt.Sprintf("%s/%s: line %d", dir, file, i+1)
			if err := textOK(l.Text); err != nil {
				return fmt.Errorf("%s: %v", where, err)
			}
			for _, p := range l.When {
				if len(ck.Parts) > 0 && !has(ck.Parts, p) {
					return fmt.Errorf("%s: when %q should be one of: %s", where, p, strings.Join(ck.Parts, ", "))
				}
			}
			for _, w := range l.Weather {
				if len(ck.Weathers) > 0 && !has(ck.Weathers, w) {
					return fmt.Errorf("%s: weather %q should be one of: %s", where, w, strings.Join(ck.Weathers, ", "))
				}
			}
			if welcome {
				if !has(visits, l.Visit) {
					return fmt.Errorf("%s: visit should be first, return or regular (got %q)", where, l.Visit)
				}
				if l.conditions() == 0 {
					plain[l.Visit] = true
				}
			} else if l.Visit != "" {
				return fmt.Errorf("%s: only welcome lines have a \"visit\"", where)
			}
		}
		if welcome {
			for _, v := range []string{"first", "return"} {
				if !plain[v] {
					return fmt.Errorf("%s/%s: needs at least one %q line without a time of day or weather, so there's always something to say", dir, file, v)
				}
			}
		} else {
			ok := false
			for _, l := range ls {
				ok = ok || l.conditions() == 0
			}
			if !ok {
				return fmt.Errorf("%s/%s: needs at least one line without a time of day or weather", dir, file)
			}
		}
		return nil
	}
	if err := lines("welcome.json", c.Welcomes, true); err != nil {
		return err
	}
	if err := lines("small_talk.json", c.Talk, false); err != nil {
		return err
	}
	where := dir + "/away.json"
	if len(c.Gaps) == 0 {
		return fmt.Errorf("%s: needs \"gaps\"", where)
	}
	for i, g := range c.Gaps {
		last := i == len(c.Gaps)-1
		switch {
		case g.Name == "":
			return fmt.Errorf("%s: gap %d needs a name", where, i+1)
		case last && g.UnderHours != 0:
			return fmt.Errorf("%s: the last gap (%s) is for anything longer, so it has no under_hours", where, g.Name)
		case !last && g.UnderHours <= 0:
			return fmt.Errorf("%s: gap %s needs under_hours above 0", where, g.Name)
		case i > 0 && !last && g.UnderHours <= c.Gaps[i-1].UnderHours:
			return fmt.Errorf("%s: gaps should get longer, but %s isn't longer than %s", where, g.Name, c.Gaps[i-1].Name)
		}
	}
	for _, g := range c.Gaps {
		ok := false
		for _, o := range c.Openers {
			ok = ok || o.Gap == g.Name
		}
		if !ok {
			return fmt.Errorf("%s: needs at least one opener for the gap %q", where, g.Name)
		}
	}
	for i, o := range c.Openers {
		if c.gapIndex(o.Gap) < 0 {
			return fmt.Errorf("%s: opener %d: gap %q isn't one of the gaps (%s)", where, i+1, o.Gap, c.gapNames())
		}
		if err := textOK(o.Text); err != nil {
			return fmt.Errorf("%s: opener %d: %v", where, i+1, err)
		}
	}
	if c.ChangeFrom != "" && c.gapIndex(c.ChangeFrom) < 0 {
		return fmt.Errorf("%s: second_page_from %q isn't one of the gaps (%s)", where, c.ChangeFrom, c.gapNames())
	}
	if c.ChangeChance < 0 || c.ChangeChance > 1 {
		return fmt.Errorf("%s: second_page_chance should be between 0 and 1", where)
	}
	for i, ch := range c.Changes {
		if c.gapIndex(ch.From) < 0 {
			return fmt.Errorf("%s: change %d: from %q isn't one of the gaps (%s)", where, i+1, ch.From, c.gapNames())
		}
		if err := textOK(ch.Text); err != nil {
			return fmt.Errorf("%s: change %d: %v", where, i+1, err)
		}
	}
	if c.LongFromHours <= 0 {
		return fmt.Errorf("%s: long_from_hours should be above 0", where)
	}
	return nil
}

func textOK(t dialogue.Text) error {
	if len(t) == 0 {
		return fmt.Errorf("needs a \"text\"")
	}
	for _, v := range t {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("has an empty text")
		}
	}
	return nil
}

func (c *Content) gapIndex(name string) int {
	for i, g := range c.Gaps {
		if g.Name == name {
			return i
		}
	}
	return -1
}

func (c *Content) gapNames() string {
	var n []string
	for _, g := range c.Gaps {
		n = append(n, g.Name)
	}
	return strings.Join(n, ", ")
}

// AddCat adds a cat's away lines (from its moments file).
func (c *Content) AddCat(id string, lines []CatLine) error {
	for i, l := range lines {
		if !has(Bands, l.Bond) {
			return fmt.Errorf("away line %d: bond should be new, knows or yours (got %q)", i+1, l.Bond)
		}
		if !has(Lengths, l.Gone) {
			return fmt.Errorf("away line %d: gone should be short or long (got %q)", i+1, l.Gone)
		}
		if err := textOK(l.Text); err != nil {
			return fmt.Errorf("away line %d: %v", i+1, err)
		}
		l.Cat = id
		c.cats[id] = append(c.cats[id], l)
	}
	return nil
}

// CatLines are a cat's away lines.
func (c *Content) CatLines(id string) []CatLine { return c.cats[id] }

// pick chooses one of lines that fit, preferring those with more
// conditions (they fit this very moment); avoid lists texts not to use
// unless nothing else fits.
func pick(lines []Line, ok func(Line) bool, avoid []string, rng *rand.Rand) (string, bool) {
	try := func(skip bool) (string, bool) {
		var texts []string
		var ws []float64
		for _, l := range lines {
			if !ok(l) {
				continue
			}
			w := []float64{1, 2.5, 4}[min(l.conditions(), 2)]
			for _, t := range l.Text {
				if skip && has(avoid, t) {
					continue
				}
				texts = append(texts, t)
				ws = append(ws, w/float64(len(l.Text)))
			}
		}
		if i := weighted(rng, ws); i >= 0 {
			return texts[i], true
		}
		return "", false
	}
	if s, ok := try(true); ok {
		return s, true
	}
	return try(false)
}

func weighted(rng *rand.Rand, ws []float64) int {
	total := 0.0
	for _, w := range ws {
		total += w
	}
	if total <= 0 {
		return -1
	}
	x := rng.Float64() * total
	for i, w := range ws {
		if x < w {
			return i
		}
		x -= w
	}
	return len(ws) - 1
}

// Welcome is the line for stepping inside.
func (c *Content) Welcome(ctx Context, rng *rand.Rand) string {
	ok := func(l Line) bool {
		switch l.Visit {
		case "first":
			if !ctx.First {
				return false
			}
		case "regular":
			if ctx.First || !ctx.Regular {
				return false
			}
		default:
			if ctx.First {
				return false
			}
		}
		return l.fits(ctx)
	}
	s, _ := pick(c.Welcomes, ok, nil, rng)
	return s
}

// SmallTalk is a line of small talk that fits now, avoiding the texts in
// recent if it can.
func (c *Content) SmallTalk(ctx Context, recent []string, rng *rand.Rand) string {
	s, _ := pick(c.Talk, func(l Line) bool { return l.fits(ctx) }, recent, rng)
	return s
}

// GapOf names an absence.
func (c *Content) GapOf(d time.Duration) string {
	h := d.Hours()
	for _, g := range c.Gaps {
		if g.UnderHours == 0 || h < g.UnderHours {
			return g.Name
		}
	}
	return c.Gaps[len(c.Gaps)-1].Name
}

// Cat is a cat as the away note needs it.
type Cat struct {
	ID   string
	Band string // new, knows or yours
	Bond float64
}

// BandOf is how a tier (0 stranger .. 4 your cat) is said in away lines.
func BandOf(tier int) string {
	switch {
	case tier >= 4:
		return "yours"
	case tier >= 2:
		return "knows"
	}
	return "new"
}

// AwayNote is what you missed, as pages: an opener for how long you were
// gone with a line about one cat (on one page if fits says they fit
// together, else on two), and sometimes a small change in the café on a
// page of its own.
func (c *Content) AwayNote(away time.Duration, cats []Cat, rng *rand.Rand, fits func(string) bool) []string {
	gap := c.GapOf(away)
	var openers []string
	for _, o := range c.Openers {
		if o.Gap == gap {
			openers = append(openers, o.Text...)
		}
	}
	page := ""
	if len(openers) > 0 {
		page = openers[rng.IntN(len(openers))]
	}
	gone := "short"
	if away.Hours() >= c.LongFromHours {
		gone = "long"
	}
	var pages []string
	line := c.catLine(cats, gone, rng)
	switch {
	case page != "" && line != "" && fits != nil && !fits(page+" "+line):
		pages = []string{page, line}
	case page+line != "":
		pages = []string{strings.TrimSpace(page + " " + line)}
	default:
		return nil
	}
	if c.ChangeFrom != "" && c.gapIndex(gap) >= c.gapIndex(c.ChangeFrom) && rng.Float64() < c.ChangeChance {
		var changes []string
		for _, ch := range c.Changes {
			if c.gapIndex(gap) >= c.gapIndex(ch.From) {
				changes = append(changes, ch.Text...)
			}
		}
		if len(changes) > 0 {
			pages = append(pages, changes[rng.IntN(len(changes))])
		}
	}
	return pages
}

// catLine picks one cat, usually the one who knows you best, and a line
// for it.
func (c *Content) catLine(cats []Cat, gone string, rng *rand.Rand) string {
	lines := func(k Cat) []string {
		var out []string
		for _, l := range c.cats[k.ID] {
			if l.Bond == k.Band && l.Gone == gone {
				out = append(out, l.Text...)
			}
		}
		return out
	}
	var ok []Cat
	for _, k := range cats {
		if len(lines(k)) > 0 {
			ok = append(ok, k)
		}
	}
	if len(ok) == 0 {
		return ""
	}
	sort.SliceStable(ok, func(i, j int) bool { return ok[i].Bond > ok[j].Bond })
	k := ok[0]
	if len(ok) > 1 && (ok[0].Bond == ok[1].Bond || rng.Float64() >= 0.65) {
		k = ok[rng.IntN(len(ok))]
	}
	ls := lines(k)
	return ls[rng.IntN(len(ls))]
}

// Pacing is the narrator's timing (content/pacing.json).
type Pacing struct {
	SmallTalkMinutes [2]float64 `json:"small_talk_minutes"`
	// QuietAfterMinutes: small talk waits at least this long after any
	// other line or moment has ended.
	QuietAfterMinutes float64 `json:"quiet_after_minutes"`
	QuestionSeconds   float64 `json:"question_seconds"`
	CupKeptMinutes    float64 `json:"cup_kept_minutes"`
	FavouriteOrders   int     `json:"favourite_after_orders"`
	AutosaveSeconds   float64 `json:"autosave_seconds"`
}

// LoadPacing reads the narrator's part of the pacing file.
func LoadPacing(fsys fs.FS, name string) (Pacing, error) {
	var p Pacing
	if err := readJSON(fsys, name, &p); err != nil {
		return p, err
	}
	if r := p.SmallTalkMinutes; r[0] <= 0 || r[1] < r[0] {
		return p, fmt.Errorf("%s: small_talk_minutes should be [shortest, longest], both above 0", name)
	}
	if p.QuietAfterMinutes < 0 {
		p.QuietAfterMinutes = 0
	}
	if p.QuestionSeconds <= 0 {
		p.QuestionSeconds = 90
	}
	if p.CupKeptMinutes < 0 {
		p.CupKeptMinutes = 0
	}
	if p.FavouriteOrders <= 0 {
		p.FavouriteOrders = 3
	}
	if p.AutosaveSeconds <= 0 {
		p.AutosaveSeconds = 60
	}
	return p, nil
}

// Wait is a random wait between r's shortest and longest café minutes,
// stretched by slow (2 is half as often).
func Wait(r [2]float64, slow float64, rng *rand.Rand) time.Duration {
	if slow <= 0 {
		slow = 1
	}
	m := (r[0] + rng.Float64()*(r[1]-r[0])) * slow
	return time.Duration(m * float64(time.Minute))
}

func has(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
