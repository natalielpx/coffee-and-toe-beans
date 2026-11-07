// Package drinks is the café's menu and the cup on your table: which drinks
// there are, when they're served, and how a cup cools and slowly empties.
//
// A cup is worked out from when it was ordered, never ticked along, so the
// same order always gives the same cup at the same moment (and saving one
// later only needs the drink and the time).
package drinks

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/noise"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/timeofday"
)

// Drink is one item on the menu.
type Drink struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Tagline string   `json:"tagline"`
	Icon    string   `json:"icon"`
	Cup     string   `json:"cup"`
	Color   string   `json:"color"`
	Top     string   `json:"top"`
	From    string   `json:"from"`
	Until   string   `json:"until"`
	Served  []string `json:"served"`

	color, top  rgb.Color
	from, until float64 // minutes of the day; both -1 when always served
}

// Liquid is the drink's color in the cup, and the color of its top (crema,
// foam, a little sheen).
func (d *Drink) Liquid() (body, top rgb.Color) { return d.color, d.top }

// Always is true for drinks served at any hour.
func (d *Drink) Always() bool { return d.from < 0 }

// Available says whether the drink is served at this café time.
func (d *Drink) Available(t time.Time) bool {
	if d.from < 0 {
		return true
	}
	m := timeofday.MinuteOfDay(t)
	if d.from <= d.until {
		return m >= d.from && m < d.until
	}
	return m >= d.from || m < d.until // through midnight
}

// CupShape is a kind of cup or glass a drink comes in.
type CupShape struct {
	Sprite string `json:"sprite"`
	Steam  [2]int `json:"steam"` // where steam rises, from the sprite's top-left
}

// Pace is how a cup is drunk, in café minutes.
type Pace struct {
	FirstSip float64 `json:"first_sip_minutes"`
	Finish   float64 `json:"finish_minutes"`
	Cool     float64 `json:"cool_minutes"`
	Sips     int     `json:"sips"`
}

// Menu is everything in content/drinks.json.
type Menu struct {
	Drinks []*Drink             `json:"drinks"`
	Cups   map[string]*CupShape `json:"cups"`
	Pace   Pace                 `json:"pace"`

	byID map[string]*Drink
}

// Load reads the menu.
func Load(fsys fs.FS, name string) (*Menu, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return Parse(name, data)
}

// Parse reads a menu from a file's contents and checks it.
func Parse(name string, data []byte) (*Menu, error) {
	var m Menu
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", name, fmt.Sprintf(format, args...))
	}
	p := m.Pace
	if p.Finish <= 0 || p.Cool <= 0 || p.Sips < 1 || p.FirstSip < 0 || p.FirstSip >= p.Finish {
		return nil, fail("\"pace\" needs first_sip_minutes < finish_minutes, a cool_minutes above 0, and at least 1 sip")
	}
	if len(m.Drinks) == 0 {
		return nil, fail("the menu has no drinks")
	}
	m.byID = map[string]*Drink{}
	var err error
	for i, d := range m.Drinks {
		if d == nil || strings.TrimSpace(d.ID) == "" {
			return nil, fail("drink %d has no \"id\"", i+1)
		}
		where := fmt.Sprintf("drink %q", d.ID)
		if _, dup := m.byID[d.ID]; dup {
			return nil, fail("%s is on the menu twice", where)
		}
		if strings.TrimSpace(d.Name) == "" {
			return nil, fail("%s has no \"name\"", where)
		}
		if d.Icon == "" {
			return nil, fail("%s has no \"icon\"", where)
		}
		if _, ok := m.Cups[d.Cup]; !ok {
			return nil, fail("%s comes in cup %q, which isn't in \"cups\"", where, d.Cup)
		}
		if d.color, err = rgb.Hex(d.Color); err != nil {
			return nil, fail("%s \"color\": %v", where, err)
		}
		if d.top, err = rgb.Hex(d.Top); err != nil {
			return nil, fail("%s \"top\": %v", where, err)
		}
		d.from, d.until = -1, -1
		if d.From != "" || d.Until != "" {
			if d.From == "" || d.Until == "" {
				return nil, fail("%s needs both \"from\" and \"until\" (or neither)", where)
			}
			if d.from, err = timeofday.ParseClock(d.From); err != nil {
				return nil, fail("%s \"from\": %v", where, err)
			}
			if d.until, err = timeofday.ParseClock(d.Until); err != nil {
				return nil, fail("%s \"until\": %v", where, err)
			}
		}
		if len(d.Served) == 0 {
			return nil, fail("%s needs at least one \"served\" line", where)
		}
		for _, s := range d.Served {
			if strings.TrimSpace(s) == "" {
				return nil, fail("%s has an empty \"served\" line", where)
			}
		}
		m.byID[d.ID] = d
	}
	for id, c := range m.Cups {
		if c == nil || c.Sprite == "" {
			return nil, fail("cup %q has no \"sprite\"", id)
		}
	}
	return &m, nil
}

// Get finds a drink.
func (m *Menu) Get(id string) (*Drink, bool) {
	d, ok := m.byID[id]
	return d, ok
}

// Order is a drink brought to your table.
type Order struct {
	Drink string
	At    time.Time // café time it arrived
	Seed  uint64    // makes each cup's sips a little different
}

// minutes since the order arrived.
func since(o Order, now time.Time) float64 {
	return math.Max(0, now.Sub(o.At).Minutes())
}

// Level is how full the cup is: 1 full, 0 empty. It goes down in sips.
func (m *Menu) Level(o Order, now time.Time) float64 {
	return 1 - float64(m.SipsTaken(o, now))/float64(m.Pace.Sips)
}

// SipsTaken counts the sips so far.
func (m *Menu) SipsTaken(o Order, now time.Time) int {
	t := since(o, now)
	n := 0
	for i := 0; i < m.Pace.Sips; i++ {
		if t >= m.sipTime(o, i) {
			n++
		}
	}
	return n
}

// sipTime is when sip i (from 0) happens, in minutes after the order. Sips
// are spread out from the first sip to the last, each nudged a little, and
// the last one always lands exactly on the finish time.
func (m *Menu) sipTime(o Order, i int) float64 {
	p := m.Pace
	n := p.Sips
	if i == n-1 {
		return p.Finish
	}
	gap := (p.Finish - p.FirstSip) / float64(max(n-1, 1))
	jitter := (noise.Rand(i, int(o.Seed%1000003), int(o.Seed>>32)) - 0.5) * 0.6 * gap
	if i == 0 {
		jitter = math.Abs(jitter) // never before the first sip time
	}
	return p.FirstSip + float64(i)*gap + jitter
}

// Heat is how hot the cup is: 1 piping hot, 0 cold. It drives the steam.
func (m *Menu) Heat(o Order, now time.Time) float64 {
	if m.Empty(o, now) {
		return 0
	}
	t := since(o, now)
	const hot = 1.0 // the first minute, it's as hot as it gets
	if t <= hot {
		return 1
	}
	x := rgb.Clamp01((t - hot) / math.Max(0.01, m.Pace.Cool-hot))
	return math.Pow(1-x, 1.6)
}

// Empty is true once the last sip is gone.
func (m *Menu) Empty(o Order, now time.Time) bool {
	return since(o, now) >= m.Pace.Finish
}

// EmptySince is when the cup became empty.
func (m *Menu) EmptySince(o Order) time.Time {
	return o.At.Add(time.Duration(m.Pace.Finish * float64(time.Minute)))
}
