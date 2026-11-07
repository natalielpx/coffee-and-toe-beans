package cats

import (
	"fmt"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/dialogue"
)

// Look is what you see and can do when you look at a cat in interact mode
// (the "look" part of its moments file).
type Look struct {
	Cat string
	// Looks describes the cat, growing with the bond: the game shows the
	// one for the highest tier the cat has reached.
	Looks map[Tier]dialogue.Text
	// Doing says what it's up to, by pose ({where} is the spot, in words).
	Doing map[string]dialogue.Text
	// Actions are the things you can do; none for a stranger.
	Actions []*Action
	// Rest is shown instead of the actions for a while after one.
	Rest dialogue.Text
}

// Action is one thing you can do with a cat.
type Action struct {
	ID      string // for tests and messages: "<cat> action <n>"
	Text    string // the choice, e.g. "Hold out your hand"
	MinTier Tier
	Poses   []string // the cat's pose (empty: awake, sit, stand or loaf)
	Drink   string   // "any", a drink id, or "" (doesn't matter)
	ComesTo string   // the cat moves here first (e.g. "table")
	Reply   []dialogue.Text
	Effect  ChoiceEffect
}

type lookDef struct {
	Looks   map[string]dialogue.Text `json:"looks"`
	Doing   map[string]dialogue.Text `json:"doing"`
	Actions []actionDef              `json:"actions"`
	Rest    dialogue.Text            `json:"rest"`
}

type actionDef struct {
	Text    string          `json:"text"`
	Tier    string          `json:"tier"`
	Poses   []string        `json:"poses"`
	Drink   string          `json:"drink"`
	ComesTo string          `json:"comes_to"`
	Reply   []dialogue.Text `json:"reply"`
	Bond    float64         `json:"bond"`
	Manner  string          `json:"manner"`
	Then    string          `json:"then"`
}

func (d *lookDef) read(name string, cat *Def, ck Checks) (*Look, error) {
	where := fmt.Sprintf("%s: look", name)
	lk := &Look{Cat: cat.ID, Looks: map[Tier]dialogue.Text{}, Doing: map[string]dialogue.Text{}, Rest: d.Rest}
	for k, v := range d.Looks {
		t, err := ParseTier(k)
		if err != nil {
			return nil, fmt.Errorf("%s: looks: %v", where, err)
		}
		if len(v) == 0 {
			return nil, fmt.Errorf("%s: looks.%s has no words", where, k)
		}
		lk.Looks[t] = v
	}
	if _, ok := lk.Looks[Stranger]; !ok {
		return nil, fmt.Errorf("%s: looks needs at least \"stranger\" (how %s looks the first time)", where, cat.Name)
	}
	for _, p := range Poses {
		if len(d.Doing[p]) == 0 {
			return nil, fmt.Errorf("%s: doing needs words for every pose (%s is missing)", where, p)
		}
		lk.Doing[p] = d.Doing[p]
	}
	for p := range d.Doing {
		if !has(Poses, p) {
			return nil, fmt.Errorf("%s: doing.%s isn't a pose (poses: %s)", where, p, strings.Join(Poses, ", "))
		}
	}
	if len(d.Rest) == 0 {
		return nil, fmt.Errorf("%s: needs \"rest\": what's shown for a while after you've done something", where)
	}
	for i, a := range d.Actions {
		aw := fmt.Sprintf("%s: action %d", where, i+1)
		if strings.TrimSpace(a.Text) == "" {
			return nil, fmt.Errorf("%s: needs a \"text\" (the choice)", aw)
		}
		if len(a.Reply) == 0 {
			return nil, fmt.Errorf("%s (%q): needs a \"reply\" (what happens)", aw, a.Text)
		}
		t, err := ParseTier(a.Tier)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", aw, err)
		}
		if t == Stranger {
			return nil, fmt.Errorf("%s (%q): a stranger gets no actions; give it a tier from curious up", aw, a.Text)
		}
		if err := ck.check(aw, nil, nil, a.Poses, nil); err != nil {
			return nil, err
		}
		if a.ComesTo != "" && !has(ck.Spots, a.ComesTo) {
			return nil, fmt.Errorf("%s: comes_to %q isn't a cat spot", aw, a.ComesTo)
		}
		if a.Drink != "" && a.Drink != "any" && !has(ck.Drinks, a.Drink) {
			return nil, fmt.Errorf("%s: drink %q should be \"any\" or a drink id", aw, a.Drink)
		}
		if !has(manners, a.Manner) {
			return nil, fmt.Errorf("%s: manner %q should be calm or sudden (or left out)", aw, a.Manner)
		}
		if !has(thens, a.Then) {
			return nil, fmt.Errorf("%s: then %q should be one of stay, sit, loaf, sleep, leave, away", aw, a.Then)
		}
		lk.Actions = append(lk.Actions, &Action{
			ID: fmt.Sprintf("%s action %d", cat.ID, i+1), Text: a.Text, MinTier: t, Poses: a.Poses,
			Drink: a.Drink, ComesTo: a.ComesTo, Reply: a.Reply,
			Effect: ChoiceEffect{Bond: a.Bond, Manner: a.Manner, Then: a.Then},
		})
	}
	return lk, nil
}

// Description is how the cat looks now: the words for the highest tier it
// has reached.
func (lk *Look) Description(t Tier) dialogue.Text {
	for ; t >= Stranger; t-- {
		if v, ok := lk.Looks[t]; ok {
			return v
		}
	}
	return nil
}

// restMinutes is how long (café minutes) a cat wants to be left alone
// after you've done something with it.
const restMinutes = 4

// lookGap is how often (café minutes) looking at a cat counts.
const lookGap = 5

// Resting says whether the cat has had enough attention for now.
func (w *World) Resting(c *Cat) bool { return w.ctx.Now.Before(c.restUntil) }

// Actions are the things you can do with the cat right now: none for a
// stranger, none while it's resting or in a moment with you, and only
// ones that fit.
func (w *World) Actions(lk *Look, c *Cat) []*Action {
	if lk == nil || c == nil || !c.Settled() || c.held || w.Resting(c) {
		return nil
	}
	t := c.Tier()
	var out []*Action
	for _, a := range lk.Actions {
		if t < a.MinTier {
			continue
		}
		poses := a.Poses
		if len(poses) == 0 {
			poses = awake
		}
		if !has(poses, c.pose) {
			continue
		}
		switch a.Drink {
		case "":
		case "any":
			if w.ctx.Drink == "" {
				continue
			}
		default:
			if w.ctx.Drink != a.Drink {
				continue
			}
		}
		if a.ComesTo != "" && a.ComesTo != c.spot {
			info, ok := w.spotInfo(a.ComesTo)
			if !ok || info.Near > maxNear(t, c.Def.Ways.Shy) || !w.roomAt(c, a.ComesTo) {
				continue
			}
		}
		out = append(out, a)
	}
	return out
}

// roomAt says whether the cat could settle at spot in some pose.
func (w *World) roomAt(c *Cat, spot string) bool {
	info, ok := w.spotInfo(spot)
	if !ok {
		return false
	}
	for _, p := range []string{PoseSit, PoseLoaf, PoseStand, PoseSleep} {
		if info.allows(p) && w.free(c, spot, p) {
			return true
		}
	}
	return false
}

// Act does an action: the bond changes (a shy cat never warms to a sudden
// one, and leaves), the cat may move, and then it rests a while.
func (w *World) Act(c *Cat, a *Action) ChoiceEffect {
	e := a.Effect
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
	if a.ComesTo != "" && e.Then != "leave" && e.Then != "away" {
		w.Bring(c, a.ComesTo)
	}
	w.After(c, e.Then)
	c.restUntil = w.ctx.Now.Add(restMinutes * time.Minute)
	return e
}

// Looked says you've just looked at the cat in interact mode. Quietly
// looking helps a little (a shy cat more), once every few minutes.
func (w *World) Looked(c *Cat) {
	if !c.Settled() || w.ctx.Now.Sub(c.lookedAt) < lookGap*time.Minute {
		return
	}
	c.lookedAt = w.ctx.Now
	gain := 0.5
	if c.Def.Ways.Shy {
		gain = 1
	}
	c.Bond = clampBond(c.Bond + gain)
}
