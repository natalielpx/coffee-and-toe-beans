package cats

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"time"
)

// Pacing is how often the cats come to you with a moment
// (content/pacing.json; the narrator reads its own part of the same file).
type Pacing struct {
	// MomentMinutes: a moment with choices every this many café minutes.
	MomentMinutes [2]float64 `json:"moment_minutes"`
	// RetryMinutes: when nothing fits right now, look again this much later.
	RetryMinutes float64 `json:"retry_minutes"`
}

// LoadPacing reads the pacing file.
func LoadPacing(fsys fs.FS, name string) (Pacing, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Pacing{}, err
	}
	var p Pacing
	if err := json.Unmarshal(data, &p); err != nil {
		return Pacing{}, fmt.Errorf("%s: %v", name, err)
	}
	if r := p.MomentMinutes; r[0] <= 0 || r[1] < r[0] {
		return Pacing{}, fmt.Errorf("%s: moment_minutes should be [shortest, longest], both above 0", name)
	}
	if p.RetryMinutes <= 0 {
		p.RetryMinutes = 1
	}
	return p, nil
}

// Pacer decides when it's time for a moment. Slow stretches every wait (2
// is half as often), for a quieter café.
type Pacer struct {
	P          Pacing
	Slow       float64
	rng        *rand.Rand
	nextMoment time.Time
}

// NewPacer starts counting from now.
func NewPacer(p Pacing, now time.Time, rng *rand.Rand) *Pacer {
	pc := &Pacer{P: p, Slow: 1, rng: rng}
	pc.nextMoment = now.Add(pc.wait(p.MomentMinutes))
	return pc
}

func (pc *Pacer) wait(r [2]float64) time.Duration {
	m := (r[0] + pc.rng.Float64()*(r[1]-r[0])) * pc.slow()
	return time.Duration(m * float64(time.Minute))
}

func (pc *Pacer) slow() float64 {
	if pc.Slow <= 0 {
		return 1
	}
	return pc.Slow
}

// MomentDue says whether it's time.
func (pc *Pacer) MomentDue(now time.Time) bool { return !now.Before(pc.nextMoment) }

// NextMoment is when the next moment is due.
func (pc *Pacer) NextMoment() time.Time { return pc.nextMoment }

// Moment says a moment just happened: the next one waits its full time.
func (pc *Pacer) Moment(now time.Time) {
	pc.nextMoment = now.Add(pc.wait(pc.P.MomentMinutes))
}

// NotBefore makes sure the next moment doesn't come before t (so a moment
// never follows another line too closely).
func (pc *Pacer) NotBefore(t time.Time) {
	if pc.nextMoment.Before(t) {
		pc.nextMoment = t
	}
}

// RetryMoment: nothing fitted; look again soon.
func (pc *Pacer) RetryMoment(now time.Time) {
	pc.nextMoment = now.Add(time.Duration(pc.P.RetryMinutes * pc.slow() * float64(time.Minute)))
}

// --- Choosing what to share --------------------------------------------

// CanHave says whether a moment can happen now for its cat.
func (w *World) CanHave(m *Moment, c *Cat) bool {
	if c == nil || c.Def.ID != m.Cat || !c.Settled() || c.held {
		return false
	}
	t := c.Tier()
	if t < m.MinTier || t > m.MaxTier {
		return false
	}
	if !w.conditions(c, m.When, m.Weather, m.poses(), m.Spots) {
		return false
	}
	switch m.Drink {
	case "":
	case "any":
		if w.ctx.Drink == "" {
			return false
		}
	default:
		if w.ctx.Drink != m.Drink {
			return false
		}
	}
	if m.ComesTo != "" && m.ComesTo != c.spot {
		// Only as close as the cat's bond allows, and only if there's room.
		info, ok := w.spotInfo(m.ComesTo)
		if !ok || info.Near > maxNear(t, c.Def.Ways.Shy) {
			return false
		}
		room := false
		for _, p := range []string{PoseSit, PoseLoaf, PoseStand, PoseSleep} {
			if info.allows(p) && w.free(c, m.ComesTo, p) {
				room = true
				break
			}
		}
		if !room {
			return false
		}
	}
	return true
}

// canSee says whether an observation fits now.
func (w *World) canSee(o *Observation, c *Cat) bool {
	if c == nil || c.Def.ID != o.Cat || !c.Settled() {
		return false
	}
	t := c.Tier()
	if t < o.MinTier || t > o.MaxTier {
		return false
	}
	return w.conditions(c, o.When, o.Weather, o.Poses, o.Spots)
}

func (w *World) conditions(c *Cat, when, weather, poses, spots []string) bool {
	if len(when) > 0 && !has(when, w.ctx.Part) {
		return false
	}
	if len(weather) > 0 && !has(weather, w.ctx.Weather) {
		return false
	}
	if len(poses) > 0 && !has(poses, c.pose) {
		return false
	}
	if len(spots) > 0 && !has(spots, c.spot) {
		return false
	}
	return true
}

// PickMoment chooses a moment that can happen now, or nil. Moments seen
// recently are less likely, and the same cat twice in a row too.
func (w *World) PickMoment(ct *Content, seen map[string]time.Time, lastCat string) (*Cat, *Moment) {
	var cats []*Cat
	var ms []*Moment
	var ws []float64
	for _, m := range ct.Moments {
		c := w.Cat(m.Cat)
		if !w.CanHave(m, c) {
			continue
		}
		wt := 1.0
		if m.Drink != "" || len(m.Weather) > 0 || len(m.When) > 0 || len(m.Spots) > 0 {
			wt *= 1.5 // fits this very moment: more of a treat
		}
		if c.Def.ID == lastCat {
			wt *= 0.5
		}
		if at, ok := seen[m.ID]; ok {
			ago := w.ctx.Now.Sub(at)
			switch {
			case ago < 3*time.Hour:
				wt *= 0.05
			case ago < 24*time.Hour:
				wt *= 0.4
			}
		}
		cats = append(cats, c)
		ms = append(ms, m)
		ws = append(ws, wt)
	}
	i := pick(w.rng, ws)
	if i < 0 {
		return nil, nil
	}
	return cats[i], ms[i]
}

// ObservationsFor are the observations about a cat that fit right now
// (for its Look around panel).
func (w *World) ObservationsFor(ct *Content, c *Cat) []*Observation {
	var out []*Observation
	for _, o := range ct.Observations {
		if w.canSee(o, c) {
			out = append(out, o)
		}
	}
	return out
}

// PickObservation chooses an observation, avoiding the last one shown.
func (w *World) PickObservation(ct *Content, last *Observation) (*Cat, *Observation) {
	var cats []*Cat
	var os []*Observation
	var ws []float64
	for _, o := range ct.Observations {
		c := w.Cat(o.Cat)
		if !w.canSee(o, c) {
			continue
		}
		wt := 1.0
		if o == last {
			wt = 0.02
		}
		if len(o.Poses) > 0 || len(o.Weather) > 0 || len(o.When) > 0 || len(o.Spots) > 0 {
			wt *= 1.5
		}
		cats = append(cats, c)
		os = append(os, o)
		ws = append(ws, wt)
	}
	i := pick(w.rng, ws)
	if i < 0 {
		return nil, nil
	}
	return cats[i], os[i]
}

// Answer applies a moment's result to its cat: choice is the index taken
// (-1 for a moment without choices). Shy cats never warm to a sudden
// choice, and it sends them away; a calm one always helps.
func (w *World) Answer(c *Cat, m *Moment, choice int) ChoiceEffect {
	e := m.Effect(choice)
	if c.Def.Ways.Shy {
		switch e.Manner {
		case "sudden":
			if e.Bond > -1 {
				e.Bond = -1
			}
			e.Then = "leave"
		case "calm":
			if e.Bond < 1 {
				e.Bond = 1
			}
		}
	}
	c.Bond = clampBond(c.Bond + e.Bond)
	w.Hold(c, false)
	w.After(c, e.Then)
	return e
}
