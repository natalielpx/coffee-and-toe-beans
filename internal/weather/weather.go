// Package weather decides what's happening outside the window.
//
// In ambient mode (the only mode in version 1) each day gets its own
// weather story, made up in advance from that day's date: "drizzle until
// 01:40, then rain until 03:10, then cloudy...". Because it comes from the
// date, closing and reopening the game shows the same weather, and nothing
// needs saving. Changes fade in and out over a few minutes.
//
// The kinds of weather and how likely each one is to follow another live in
// content/weather.json.
package weather

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"math/rand/v2"
	"sort"
	"time"
)

// Params are the continuous dials the scene draws from.
type Params struct {
	Cloud float64 `json:"cloud"` // 0 clear sky .. 1 fully overcast
	Rain  float64 `json:"rain"`  // 0 dry .. 1 downpour
	Snow  float64 `json:"snow"`  // 0 none .. 1 heavy snowfall
	Fog   float64 `json:"fog"`   // 0 crisp .. 1 thick fog
	Wind  float64 `json:"wind"`  // how much rain and snow slant, -1..1
}

func (p Params) lerp(q Params, t float64) Params {
	return Params{
		Cloud: p.Cloud + (q.Cloud-p.Cloud)*t,
		Rain:  p.Rain + (q.Rain-p.Rain)*t,
		Snow:  p.Snow + (q.Snow-p.Snow)*t,
		Fog:   p.Fog + (q.Fog-p.Fog)*t,
		Wind:  p.Wind + (q.Wind-p.Wind)*t,
	}
}

type kindDef struct {
	Params
	Lasts [2]float64 `json:"lasts_minutes"`
}

type ambientDef struct {
	First        map[string]float64            `json:"first"`
	Next         map[string]map[string]float64 `json:"next"`
	WinterOnly   []string                      `json:"winter_only"`
	WinterMonths []int                         `json:"winter_months"`
}

type file struct {
	FadeMinutes float64            `json:"fade_minutes"`
	Kinds       map[string]kindDef `json:"kinds"`
	Ambient     ambientDef         `json:"ambient"`
}

// Config is the weather rulebook.
type Config struct {
	fade         float64
	kinds        map[string]kindDef
	names        []string
	first        map[string]float64
	next         map[string]map[string]float64
	winterOnly   map[string]bool
	winterMonths map[time.Month]bool
}

// Load reads the weather rules from a JSON file.
func Load(fsys fs.FS, name string) (*Config, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return Parse(name, data)
}

// Parse reads the weather rules from JSON bytes and checks them.
func Parse(name string, data []byte) (*Config, error) {
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	c := &Config{
		fade:         f.FadeMinutes,
		kinds:        f.Kinds,
		first:        f.Ambient.First,
		next:         f.Ambient.Next,
		winterOnly:   map[string]bool{},
		winterMonths: map[time.Month]bool{},
	}
	if c.fade <= 0 {
		c.fade = 6
	}
	if len(c.kinds) == 0 {
		return nil, fmt.Errorf("%s: no kinds of weather", name)
	}
	for k, d := range c.kinds {
		c.names = append(c.names, k)
		if d.Lasts[0] <= 0 || d.Lasts[1] < d.Lasts[0] {
			return nil, fmt.Errorf("%s: %s: lasts_minutes should be [shortest, longest], both above 0", name, k)
		}
		for _, v := range []float64{d.Cloud, d.Rain, d.Snow, d.Fog} {
			if v < 0 || v > 1 {
				return nil, fmt.Errorf("%s: %s: cloud, rain, snow and fog should be between 0 and 1", name, k)
			}
		}
	}
	sort.Strings(c.names)
	check := func(where string, weights map[string]float64) error {
		total := 0.0
		for k, w := range weights {
			if _, ok := c.kinds[k]; !ok {
				return fmt.Errorf("%s: %s mentions %q, which isn't one of the kinds", name, where, k)
			}
			if w < 0 {
				return fmt.Errorf("%s: %s: weights can't be negative", name, where)
			}
			total += w
		}
		if total <= 0 {
			return fmt.Errorf("%s: %s needs at least one weight above 0", name, where)
		}
		return nil
	}
	if err := check("ambient.first", c.first); err != nil {
		return nil, err
	}
	for _, k := range c.names {
		w, ok := c.next[k]
		if !ok {
			return nil, fmt.Errorf("%s: ambient.next has no entry for %q", name, k)
		}
		if err := check("ambient.next."+k, w); err != nil {
			return nil, err
		}
	}
	for _, k := range f.Ambient.WinterOnly {
		if _, ok := c.kinds[k]; !ok {
			return nil, fmt.Errorf("%s: winter_only mentions %q, which isn't one of the kinds", name, k)
		}
		c.winterOnly[k] = true
	}
	for _, m := range f.Ambient.WinterMonths {
		if m < 1 || m > 12 {
			return nil, fmt.Errorf("%s: winter_months should be numbers from 1 to 12", name)
		}
		c.winterMonths[time.Month(m)] = true
	}
	return c, nil
}

// Kinds lists every kind of weather, sorted.
func (c *Config) Kinds() []string {
	return append([]string(nil), c.names...)
}

// Has reports whether name is a known kind of weather.
func (c *Config) Has(name string) bool {
	_, ok := c.kinds[name]
	return ok
}

// Segment is one stretch of a day's weather.
type Segment struct {
	Kind       string
	Start, End float64 // minutes after midnight
}

// Day makes up the weather story for one date. The same date always gives
// the same story.
func (c *Config) Day(date time.Time) []Segment {
	y, m, d := date.Date()
	seed := uint64(y)*10000 + uint64(m)*100 + uint64(d)
	rng := rand.New(rand.NewPCG(seed, 0x6361666521)) // "cafe!"
	winter := c.winterMonths[m]

	pick := func(weights map[string]float64) string {
		total := 0.0
		for _, k := range c.names {
			if c.winterOnly[k] && !winter {
				continue
			}
			total += weights[k]
		}
		if total <= 0 {
			return ""
		}
		r := rng.Float64() * total
		for _, k := range c.names {
			if c.winterOnly[k] && !winter {
				continue
			}
			r -= weights[k]
			if r < 0 {
				return k
			}
		}
		return c.names[len(c.names)-1]
	}

	var out []Segment
	kind := pick(c.first)
	if kind == "" {
		kind = c.names[0]
	}
	for t := 0.0; t < 1440; {
		def := c.kinds[kind]
		lasts := def.Lasts[0] + rng.Float64()*(def.Lasts[1]-def.Lasts[0])
		end := math.Min(t+lasts, 1440)
		out = append(out, Segment{Kind: kind, Start: t, End: end})
		t = end
		if n := pick(c.next[kind]); n != "" {
			kind = n
		}
	}
	return out
}

// KindAt is the ambient weather at a moment.
func (c *Config) KindAt(t time.Time) string {
	m := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60
	for _, s := range c.Day(t) {
		if m >= s.Start && m < s.End {
			return s.Kind
		}
	}
	return c.names[0]
}

// State is the weather as it is right now, fading smoothly between kinds.
type State struct {
	cfg       *Config
	pinned    string
	cur       Params
	snowCover float64
	last      time.Time
	dayKey    int
	day       []Segment
}

// warmUp is how far back we start simulating, so that the weather is
// already settled (and snow already lying) when the game opens.
const warmUp = 3 * time.Hour

// NewState starts the weather at time t. If pinned names a kind of weather,
// it stays that way (for previews); otherwise it follows the ambient story.
func (c *Config) NewState(t time.Time, pinned string) (*State, error) {
	if pinned != "" && !c.Has(pinned) {
		return nil, fmt.Errorf("unknown weather %q (try one of: %v)", pinned, c.names)
	}
	s := &State{cfg: c, pinned: pinned}
	s.reset(t)
	return s, nil
}

func (s *State) reset(t time.Time) {
	start := t.Add(-warmUp)
	s.cur = s.target(start)
	s.snowCover = 0
	s.last = start
	s.Advance(t)
}

func (s *State) kindAt(t time.Time) string {
	if s.pinned != "" {
		return s.pinned
	}
	y, m, d := t.Date()
	key := y*10000 + int(m)*100 + d
	if key != s.dayKey || s.day == nil {
		s.dayKey = key
		s.day = s.cfg.Day(t)
	}
	mins := float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60
	for _, seg := range s.day {
		if mins >= seg.Start && mins < seg.End {
			return seg.Kind
		}
	}
	return s.day[len(s.day)-1].Kind
}

func (s *State) target(t time.Time) Params {
	return s.cfg.kinds[s.kindAt(t)].Params
}

// Advance moves the weather forward to time t.
func (s *State) Advance(t time.Time) {
	dt := t.Sub(s.last).Minutes()
	if dt <= 0 {
		return
	}
	if dt > warmUp.Minutes() {
		// A long gap (the computer slept): start fresh rather than crawl.
		s.reset(t)
		return
	}
	tau := s.cfg.fade / 3 // ~95% of the way there after fade_minutes
	for dt > 0 {
		step := math.Min(dt, 1)
		s.last = s.last.Add(time.Duration(step * float64(time.Minute)))
		dt -= step
		target := s.target(s.last)
		s.cur = s.cur.lerp(target, 1-math.Exp(-step/tau))
		if s.cur.Snow > 0.25 {
			s.snowCover += step / 50 * s.cur.Snow // snow settles over an hour or so
		} else {
			s.snowCover -= step / 240 // and melts over a few hours
		}
		s.snowCover = math.Max(0, math.Min(1, s.snowCover))
	}
}

// Pin keeps the weather at one kind from now on ("" follows the ambient
// story again). The sky changes at once, as when the café opens.
func (s *State) Pin(kind string, t time.Time) error {
	if kind != "" && !s.cfg.Has(kind) {
		return fmt.Errorf("unknown weather %q (try one of: %v)", kind, s.cfg.names)
	}
	if kind == s.pinned {
		return nil
	}
	s.pinned = kind
	s.reset(t)
	return nil
}

// Params is the current weather.
func (s *State) Params() Params { return s.cur }

// SnowCover is how much snow lies on the rooftops, 0..1.
func (s *State) SnowCover() float64 { return s.snowCover }

// Kind is the kind of weather the sky is heading toward right now.
func (s *State) Kind() string { return s.kindAt(s.last) }
