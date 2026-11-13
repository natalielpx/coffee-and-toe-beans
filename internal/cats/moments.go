package cats

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"coffee-and-toe-beans/internal/dialogue"
)

// Moment is a little scene with one cat, shown in the dialogue box with
// the cat's name on it. It is a conversation (pages, choices, ignore) plus
// when it can happen and what it does to the bond.
type Moment struct {
	ID   string
	Cat  string // the cat's id
	File string
	Conv *dialogue.Conversation

	MinTier, MaxTier Tier
	When             []string // parts of the day ("morning", "evening"...); empty: any
	Weather          []string // weather kinds; empty: any
	Poses            []string // the cat's pose when it starts; empty: awake (sit, stand, loaf)
	Spots            []string // where the cat must be; empty: anywhere
	Drink            string   // "any" (something in your cup), a drink id, or "" (doesn't matter)
	ComesTo          string   // the cat first moves to this spot (e.g. "table")
	Then             string   // afterwards: "" stays, "sit", "loaf", "sleep", "leave" or "away"
	Bond             float64  // bond change when it ends, for moments without choices
	Choices          []ChoiceEffect
}

// Responses are the cat's own replies, by manner, for a choice that has
// no reply of its own: so every moment ends with the cat responding.
// Keys: "calm", "sudden" and "other" (a choice with no manner).
var responseKeys = []string{"calm", "sudden", "other"}

// ChoiceEffect is what a choice in a moment does to the bond.
type ChoiceEffect struct {
	Bond   float64       // added to the bond (can be negative)
	Manner string        // "calm", "sudden" or "" (shy cats care a lot)
	Then   string        // overrides the moment's Then for this choice
	Reply  dialogue.Text // what the cat does in answer (optional)
}

// Observation is a small line about a cat, with no choices.
type Observation struct {
	Cat              string
	Text             dialogue.Text
	MinTier, MaxTier Tier
	When, Weather    []string
	Poses, Spots     []string
}

// Content is every cat's moments, observations and what you can do
// when you look at them.
type Content struct {
	Moments      []*Moment
	Observations []*Observation
	Looks        map[string]*Look               // by cat id
	Away         []AwayLine                     // what each cat was like while you were away
	Responses    map[string]map[string][]string // cat id -> manner -> replies (one page each)
}

// Response is one of the cat's own replies for a manner ("calm", "sudden"
// or "other"), picked by pick (0 to n-1). It falls back to "other", and
// then to nil if the cat has none.
func (c *Content) Response(catID, manner string, pick func(n int) int) dialogue.Text {
	by := c.Responses[catID]
	for _, k := range []string{manner, "other"} {
		if list := by[k]; len(list) > 0 {
			return dialogue.Text{list[pick(len(list))]}
		}
	}
	return nil
}

// AwayLine is a line about a cat for the note when you come back: Bond is
// "new", "knows" or "yours"; Gone is "short" or "long".
type AwayLine struct {
	Cat  string
	Bond string
	Gone string
	Text dialogue.Text
}

type momentFile struct {
	Cat          string                     `json:"cat"`
	Moments      map[string]json.RawMessage `json:"moments"`
	Observations []observationDef           `json:"observations"`
	Look         *lookDef                   `json:"look"`
	Away         []awayDef                  `json:"away"`
	Responses    map[string][]string        `json:"responses"`
}

type awayDef struct {
	Bond string        `json:"bond"`
	Gone string        `json:"gone"`
	Text dialogue.Text `json:"text"`
}

type momentDef struct {
	Tier    string      `json:"tier"`
	MaxTier string      `json:"max_tier"`
	When    []string    `json:"when"`
	Weather []string    `json:"weather"`
	Poses   []string    `json:"poses"`
	Spots   []string    `json:"spots"`
	Drink   string      `json:"drink"`
	ComesTo string      `json:"comes_to"`
	Then    string      `json:"then"`
	Bond    float64     `json:"bond"`
	Choices []choiceDef `json:"choices"`
}

type choiceDef struct {
	Bond   float64       `json:"bond"`
	Manner string        `json:"manner"`
	Then   string        `json:"then"`
	Reply  dialogue.Text `json:"reply"`
}

type observationDef struct {
	Text    dialogue.Text `json:"text"`
	Tier    string        `json:"tier"`
	MaxTier string        `json:"max_tier"`
	When    []string      `json:"when"`
	Weather []string      `json:"weather"`
	Poses   []string      `json:"poses"`
	Spots   []string      `json:"spots"`
}

// Known words, for checking content files.
var (
	PartsOfDay = []string{"late night", "dawn", "morning", "afternoon", "evening", "night"}
	thens      = []string{"", "stay", "sit", "loaf", "sleep", "leave", "away"}
	manners    = []string{"", "calm", "sudden"}
)

// Checks is what a content file is checked against.
type Checks struct {
	Cats     []*Def
	Spots    []string
	Weathers []string
	Drinks   []string
}

// LoadContent reads every file in dir (one per cat).
func LoadContent(fsys fs.FS, dir string, ck Checks) (*Content, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := &Content{Looks: map[string]*Look{}, Responses: map[string]map[string][]string{}}
	seen := map[string]string{}
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		if err := out.add(name, data, ck, seen); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ParseContent reads one moments file (handy for tests).
func ParseContent(name string, data []byte, ck Checks) (*Content, error) {
	out := &Content{Looks: map[string]*Look{}, Responses: map[string]map[string][]string{}}
	return out, out.add(name, data, ck, map[string]string{})
}

func (c *Content) add(name string, data []byte, ck Checks, seen map[string]string) error {
	var f momentFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	var cat *Def
	for _, d := range ck.Cats {
		if d.ID == f.Cat {
			cat = d
		}
	}
	if cat == nil {
		return fmt.Errorf("%s: \"cat\" is %q, but there's no cat with that id in content/cats", name, f.Cat)
	}
	var ids []string
	for id := range f.Moments {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		raw := f.Moments[id]
		if old, dup := seen[id]; dup {
			return fmt.Errorf("%s: moment %q is already in %s", name, id, old)
		}
		seen[id] = name
		conv, err := dialogue.ParseConversation(id, name, raw)
		if err != nil {
			return err
		}
		if conv.Speaker == "" {
			conv.Speaker = cat.Name
		}
		var d momentDef
		if err := json.Unmarshal(raw, &d); err != nil {
			return fmt.Errorf("%s: %q: %v", name, id, err)
		}
		where := fmt.Sprintf("%s: moment %q", name, id)
		m := &Moment{ID: id, Cat: cat.ID, File: name, Conv: conv, When: d.When, Weather: d.Weather,
			Poses: d.Poses, Spots: d.Spots, Drink: d.Drink,
			ComesTo: d.ComesTo, Then: d.Then, Bond: d.Bond}
		if m.MinTier, m.MaxTier, err = tiers(d.Tier, d.MaxTier); err != nil {
			return fmt.Errorf("%s: %v", where, err)
		}
		if err := ck.check(where, d.When, d.Weather, d.Poses, d.Spots); err != nil {
			return err
		}
		if d.ComesTo != "" && !has(ck.Spots, d.ComesTo) {
			return fmt.Errorf("%s: comes_to %q isn't a cat spot (spots: %s)", where, d.ComesTo, strings.Join(ck.Spots, ", "))
		}
		if d.Drink != "" && d.Drink != "any" && !has(ck.Drinks, d.Drink) {
			return fmt.Errorf("%s: drink %q should be \"any\" or a drink id (%s)", where, d.Drink, strings.Join(ck.Drinks, ", "))
		}
		if !has(thens, d.Then) {
			return fmt.Errorf("%s: then %q should be one of stay, sit, loaf, sleep, leave, away", where, d.Then)
		}
		for i, ch := range d.Choices {
			if !has(manners, ch.Manner) {
				return fmt.Errorf("%s: choice %d: manner %q should be calm or sudden (or left out)", where, i+1, ch.Manner)
			}
			if !has(thens, ch.Then) {
				return fmt.Errorf("%s: choice %d: then %q should be one of stay, sit, loaf, sleep, leave, away", where, i+1, ch.Then)
			}
			m.Choices = append(m.Choices, ChoiceEffect{Bond: ch.Bond, Manner: ch.Manner, Then: ch.Then, Reply: ch.Reply})
		}
		if len(conv.Choices) < 2 || len(conv.Choices) > 4 {
			// A moment waits for your answer, so it needs something to answer:
			// two to four choices, each with its own reply.
			return fmt.Errorf("%s: needs 2 to 4 choices (a line with no choices belongs in \"observations\", shown in Look around)", where)
		}
		for i, ch := range d.Choices {
			if len(ch.Reply) == 0 {
				return fmt.Errorf("%s: choice %d (%q) needs a \"reply\" (what the cat does)", where, i+1, conv.Choices[i].Text)
			}
		}
		c.Moments = append(c.Moments, m)
	}
	for i, od := range f.Observations {
		where := fmt.Sprintf("%s: observation %d", name, i+1)
		if len(od.Text) == 0 {
			return fmt.Errorf("%s: needs a \"text\"", where)
		}
		o := &Observation{Cat: cat.ID, Text: od.Text, When: od.When, Weather: od.Weather,
			Poses: od.Poses, Spots: od.Spots}
		var err error
		if o.MinTier, o.MaxTier, err = tiers(od.Tier, od.MaxTier); err != nil {
			return fmt.Errorf("%s: %v", where, err)
		}
		if err := ck.check(where, od.When, od.Weather, od.Poses, od.Spots); err != nil {
			return err
		}
		c.Observations = append(c.Observations, o)
	}
	if f.Look == nil {
		return fmt.Errorf("%s: needs a \"look\" part (what you see and can do when you look at %s)", name, cat.Name)
	}
	lk, err := f.Look.read(name, cat, ck)
	if err != nil {
		return err
	}
	c.Looks[cat.ID] = lk
	if c.Responses == nil {
		c.Responses = map[string]map[string][]string{}
	}
	c.Responses[cat.ID] = map[string][]string{}
	for k, lines := range f.Responses {
		if !has(responseKeys, k) {
			return fmt.Errorf("%s: responses: %q should be calm, sudden or other", name, k)
		}
		for i, l := range lines {
			if strings.TrimSpace(l) == "" {
				return fmt.Errorf("%s: responses.%s line %d is empty", name, k, i+1)
			}
		}
		c.Responses[cat.ID][k] = lines
	}
	if len(c.Responses[cat.ID]) == 0 {
		return fmt.Errorf("%s: needs \"responses\" (what %s does after you answer a moment)", name, cat.Name)
	}
	for i, a := range f.Away {
		where := fmt.Sprintf("%s: away line %d", name, i+1)
		if !has([]string{"new", "knows", "yours"}, a.Bond) {
			return fmt.Errorf("%s: bond should be new, knows or yours (got %q)", where, a.Bond)
		}
		if !has([]string{"short", "long"}, a.Gone) {
			return fmt.Errorf("%s: gone should be short or long (got %q)", where, a.Gone)
		}
		if len(a.Text) == 0 {
			return fmt.Errorf("%s: needs a \"text\"", where)
		}
		c.Away = append(c.Away, AwayLine{Cat: cat.ID, Bond: a.Bond, Gone: a.Gone, Text: a.Text})
	}
	return nil
}

func tiers(lo, hi string) (Tier, Tier, error) {
	a, err := ParseTier(lo)
	if err != nil {
		return 0, 0, err
	}
	b := YourCat
	if hi != "" {
		if b, err = ParseTier(hi); err != nil {
			return 0, 0, err
		}
	}
	if b < a {
		return 0, 0, fmt.Errorf("max_tier %s is below tier %s", b, a)
	}
	return a, b, nil
}

func (ck Checks) check(where string, when, weather, poses, spots []string) error {
	for _, w := range when {
		if !has(PartsOfDay, w) {
			return fmt.Errorf("%s: when %q should be one of: %s", where, w, strings.Join(PartsOfDay, ", "))
		}
	}
	for _, w := range weather {
		if len(ck.Weathers) > 0 && !has(ck.Weathers, w) {
			return fmt.Errorf("%s: weather %q should be one of: %s", where, w, strings.Join(ck.Weathers, ", "))
		}
	}
	for _, p := range poses {
		if !has(Poses, p) {
			return fmt.Errorf("%s: pose %q should be one of: %s", where, p, strings.Join(Poses, ", "))
		}
	}
	for _, s := range spots {
		if len(ck.Spots) > 0 && !has(ck.Spots, s) {
			return fmt.Errorf("%s: spot %q isn't a cat spot (spots: %s)", where, s, strings.Join(ck.Spots, ", "))
		}
	}
	return nil
}

func has(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// awake are the poses a moment needs when it doesn't say.
var awake = []string{PoseSit, PoseStand, PoseLoaf}

func (m *Moment) poses() []string {
	if len(m.Poses) == 0 {
		return awake
	}
	return m.Poses
}

// Effect is what choosing (or ignoring) choice i does.
func (m *Moment) Effect(i int) ChoiceEffect {
	if i >= 0 && i < len(m.Choices) {
		e := m.Choices[i]
		if e.Then == "" {
			e.Then = m.Then
		}
		return e
	}
	return ChoiceEffect{Bond: m.Bond, Then: m.Then}
}
