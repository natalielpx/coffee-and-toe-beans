package cats

import (
	"math"
	"math/rand/v2"
	"time"
)

// SpotInfo is what the cats need to know about a place in the room.
type SpotInfo struct {
	Name  string
	Near  int      // 0 far from your seat, 1 nearby, 2 at your table
	Place string   // where you look to see it: "window", "cats" or "table"
	Poses []string // poses that fit there (empty: any)
}

func (s SpotInfo) allows(pose string) bool {
	return len(s.Poses) == 0 || has(s.Poses, pose)
}

// Rect is a rectangle of room pixels (X1, Y1 not included).
type Rect struct{ X0, Y0, X1, Y1 int }

func (r Rect) overlaps(o Rect) bool {
	return r.X0 < o.X1 && o.X0 < r.X1 && r.Y0 < o.Y1 && o.Y0 < r.Y1
}

// Room tells the cats where they can be. The game answers from the scene.
type Room interface {
	Spots() []SpotInfo
	// Box is where a cat in a pose would be drawn at a spot; ok is false
	// if it wouldn't fit inside the room.
	Box(d *Def, pose, spot string) (r Rect, ok bool)
}

// Context is what's going on in the café, as far as the cats care.
type Context struct {
	Now     time.Time // café time
	Part    string    // part of the day: "morning", "evening", "late night"...
	Weather string    // weather kind
	Drink   string    // the drink in your cup ("" if none, or empty)
}

// Mood follows the hour: sleepy at night, playful in the evening.
type Mood int

// The cats' moods through the day.
const (
	Calm Mood = iota
	Sleepy
	Playful
)

// MoodOf is the mood for a part of the day.
func MoodOf(part string) Mood {
	switch part {
	case "night", "late night":
		return Sleepy
	case "evening":
		return Playful
	}
	return Calm
}

type where int

const (
	away where = iota
	here
)

// Fade times, in real seconds.
const (
	fadeIn  = 1.6
	fadeOut = 1.4
)

// Cat is one cat in the café right now.
type Cat struct {
	Def  *Def
	Bond float64

	where where
	spot  string
	pose  string
	alpha float64 // how visible: 0..1
	fade  int     // +1 fading in, -1 fading out, 0 steady
	next  string  // after fading out: the spot to reappear at ("" leaves the room)
	nextP string  // ...and in which pose
	until time.Time
	held  bool // in a moment: holds still

	restUntil time.Time // after you've done something with it: no more for a while
	lookedAt  time.Time // when you last looked at it (in interact mode)

	frame     string  // the frame shown now
	frameLeft float64 // real seconds the frame has left
	twitchIn  float64 // real seconds until the next little movement
	breath    float64 // phase of breathing while asleep
}

// Tier is how well the cat knows you.
func (c *Cat) Tier() Tier { return TierOf(c.Bond) }

// Here is true while the cat is in the room (even fading).
func (c *Cat) Here() bool { return c.where == here }

// Spot is where the cat is ("" when away).
func (c *Cat) Spot() string {
	if c.where != here {
		return ""
	}
	return c.spot
}

// Pose is what the cat is doing.
func (c *Cat) Pose() string { return c.pose }

// Settled is true when the cat is in the room and not coming or going.
func (c *Cat) Settled() bool { return c.where == here && c.fade == 0 && c.alpha >= 1 }

// Drawn is a cat as the scene should draw it.
type Drawn struct {
	Cat    *Cat
	Sprite string
	Spot   string
	Anchor [2]int
	Alpha  float64
}

// World is all the cats.
type World struct {
	room  Room
	spots []SpotInfo
	Cats  []*Cat
	rng   *rand.Rand
	ctx   Context
	calm  bool // reduced motion: fewer little movements
	still bool // interact mode: nobody wanders off (fades in progress finish)

	lastNow time.Time // café time of the last update
}

// NewWorld puts the cats in the café at the start of a visit. Some are
// already in the room; the others come in later, sooner if they know you.
func NewWorld(defs []*Def, room Room, bonds map[string]float64, ctx Context, rng *rand.Rand) *World {
	w := &World{room: room, spots: room.Spots(), rng: rng, ctx: ctx, lastNow: ctx.Now}
	for _, d := range defs {
		c := &Cat{Def: d, Bond: clampBond(bonds[d.ID]), frame: FrameStill}
		c.twitchIn = 2 + rng.Float64()*4
		w.Cats = append(w.Cats, c)
	}
	for _, c := range w.Cats {
		if rng.Float64() < c.Def.Ways.InRoom {
			if spot, pose, ok := w.pickPlace(c, false); ok {
				c.where, c.spot, c.pose, c.alpha = here, spot, pose, 1
				c.until = ctx.Now.Add(w.stay(c) / 2)
				continue
			}
		}
		c.where = away
		// Cats that know you come to say hello sooner.
		wait := w.minutes(c.Def.Ways.AwayMinutes) * (1 - 0.18*float64(c.Tier()))
		c.until = ctx.Now.Add(time.Duration(wait * 0.5 * float64(time.Minute)))
	}
	return w
}

// SetStill keeps every cat where it is while you look around (interact
// mode). They still blink and breathe, and a cat already coming or going
// finishes doing so.
func (w *World) SetStill(still bool) {
	if w.still && !still {
		// Don't all move the moment you look away.
		for _, c := range w.Cats {
			if c.until.Before(w.ctx.Now.Add(time.Minute)) {
				c.until = w.ctx.Now.Add(time.Minute + time.Duration(w.rng.Float64()*float64(2*time.Minute)))
			}
		}
	}
	w.still = still
}

// SetCalm turns down the little movements (reduced motion).
func (w *World) SetCalm(calm bool) { w.calm = calm }

// Cat finds a cat by id.
func (w *World) Cat(id string) *Cat {
	for _, c := range w.Cats {
		if c.Def.ID == id {
			return c
		}
	}
	return nil
}

// Place is where you'd look to see a spot ("" if unknown).
func (w *World) Place(spot string) string {
	for _, s := range w.spots {
		if s.Name == spot {
			return s.Place
		}
	}
	return ""
}

func (w *World) spotInfo(name string) (SpotInfo, bool) {
	for _, s := range w.spots {
		if s.Name == name {
			return s, true
		}
	}
	return SpotInfo{}, false
}

func (w *World) minutes(r [2]float64) float64 {
	return r[0] + w.rng.Float64()*(r[1]-r[0])
}

// stay is how long a cat stays put: lively cats and playful evenings make
// it shorter, sleepy nights longer.
func (w *World) stay(c *Cat) time.Duration {
	m := w.minutes(c.Def.Ways.StayMinutes) / math.Sqrt(c.Def.Ways.Energy)
	switch MoodOf(w.ctx.Part) {
	case Sleepy:
		m *= 1.6
	case Playful:
		m *= 0.7
	}
	return time.Duration(m * float64(time.Minute))
}

// Update moves the cats along: dt is real seconds (for fades and little
// movements); decisions follow café time.
func (w *World) Update(ctx Context, dt float64) {
	w.ctx = ctx
	mins := w.cafeMinutes()
	for _, c := range w.Cats {
		w.updateFade(c, dt)
		w.animate(c, dt)
		w.bondDrift(c, ctx, mins)
		if w.still || c.held || c.fade != 0 {
			continue
		}
		if ctx.Now.Before(c.until) {
			continue
		}
		if c.where == away {
			w.tryEnter(c)
		} else {
			w.decide(c)
		}
	}
}

func (w *World) updateFade(c *Cat, dt float64) {
	switch c.fade {
	case 1:
		c.alpha += dt / fadeIn
		if c.alpha >= 1 {
			c.alpha, c.fade = 1, 0
		}
	case -1:
		c.alpha -= dt / fadeOut
		if c.alpha > 0 {
			return
		}
		c.alpha, c.fade = 0, 0
		if c.next != "" {
			c.spot, c.pose, c.next = c.next, c.nextP, ""
			c.fade = 1
			c.frame = FrameStill
			return
		}
		c.where = away
		c.spot = ""
		c.until = w.ctx.Now.Add(time.Duration(w.minutes(c.Def.Ways.AwayMinutes) * float64(time.Minute)))
	}
}

// bondDrift: simply sharing the room helps a little. Café time, so it's
// slow; looking at a cat quietly (Looked) helps more.
func (w *World) bondDrift(c *Cat, ctx Context, mins float64) {
	if !c.Settled() {
		return
	}
	const per = 0.01 // bond per café minute, just for sharing the room
	c.Bond = clampBond(c.Bond + per*mins)
}

// cafeMinutes is how many café minutes passed since the last update.
func (w *World) cafeMinutes() float64 {
	m := w.ctx.Now.Sub(w.lastNow).Minutes()
	w.lastNow = w.ctx.Now
	if m < 0 || m > 10 {
		return 0
	}
	return m
}

// tryEnter brings an away cat into the room, standing a moment first if
// the spot allows it.
func (w *World) tryEnter(c *Cat) {
	if w.rng.Float64() > 0.25+c.Def.Ways.InRoom {
		c.until = w.ctx.Now.Add(time.Duration(w.minutes(c.Def.Ways.AwayMinutes) * 0.5 * float64(time.Minute)))
		return
	}
	spot, pose, ok := w.pickPlace(c, true)
	if !ok {
		c.until = w.ctx.Now.Add(2 * time.Minute)
		return
	}
	c.where, c.spot, c.pose = here, spot, pose
	c.alpha, c.fade, c.frame = 0, 1, FrameStill
	c.until = w.ctx.Now.Add(w.arrivalPause(c))
}

// arrivalPause is how long a cat stands after arriving before settling.
func (w *World) arrivalPause(c *Cat) time.Duration {
	if c.pose != PoseStand {
		return w.stay(c)
	}
	return time.Duration((20 + w.rng.Float64()*40) * float64(time.Second))
}

// decide: a settled cat changes pose, moves, or leaves.
func (w *World) decide(c *Cat) {
	if c.pose == PoseStand {
		// Done looking around: settle here.
		if p, ok := w.settlePose(c, c.spot, false); ok {
			c.pose = p
		}
		c.until = w.ctx.Now.Add(w.stay(c))
		return
	}
	r := w.rng.Float64()
	leave := 0.12 + 0.3*(1-c.Def.Ways.InRoom)
	move := 0.3 * c.Def.Ways.Energy
	switch {
	case r < leave:
		w.leave(c)
	case r < leave+move:
		if spot, pose, ok := w.pickPlace(c, true); ok && spot != c.spot {
			w.moveTo(c, spot, pose)
		} else {
			w.repose(c)
		}
	default:
		w.repose(c)
	}
	c.until = w.ctx.Now.Add(w.stay(c))
}

func (w *World) repose(c *Cat) {
	if p, ok := w.settlePose(c, c.spot, true); ok {
		c.pose = p
	}
}

func (w *World) leave(c *Cat) {
	c.fade, c.next = -1, ""
}

func (w *World) moveTo(c *Cat, spot, pose string) {
	c.fade, c.next, c.nextP = -1, spot, pose
}

// claimed is the spot a cat holds: where it is, or where it's going.
func (c *Cat) claimed() string {
	if c.next != "" {
		return c.next
	}
	if c.where == here {
		return c.spot
	}
	return ""
}

// free says whether a cat could be at spot in pose without sharing it or
// overlapping another cat on screen.
func (w *World) free(c *Cat, spot, pose string) bool {
	box, ok := w.room.Box(c.Def, pose, spot)
	if !ok {
		return false
	}
	for _, o := range w.Cats {
		if o == c {
			continue
		}
		s := o.claimed()
		if s == "" {
			continue
		}
		if s == spot {
			return false
		}
		p := o.pose
		if o.next != "" {
			p = o.nextP
		}
		if ob, ok := w.room.Box(o.Def, p, s); ok && ob.overlaps(box) {
			return false
		}
	}
	return true
}

// poseWeights: what a cat feels like doing, by mood and energy.
func poseWeights(m Mood, energy float64) map[string]float64 {
	var w map[string]float64
	switch m {
	case Sleepy:
		w = map[string]float64{PoseSleep: 6, PoseLoaf: 2.5, PoseSit: 1}
	case Playful:
		w = map[string]float64{PoseSit: 5, PoseStand: 2, PoseLoaf: 2.5, PoseSleep: 0.5}
	default:
		w = map[string]float64{PoseSit: 3.5, PoseLoaf: 4, PoseSleep: 2.5}
	}
	w[PoseSleep] /= energy
	w[PoseSit] *= energy
	return w
}

// settlePose picks a pose that fits at spot; stand says whether standing
// is one of the choices.
func (w *World) settlePose(c *Cat, spot string, stand bool) (string, bool) {
	info, ok := w.spotInfo(spot)
	if !ok {
		return "", false
	}
	weights := poseWeights(MoodOf(w.ctx.Part), c.Def.Ways.Energy)
	var poses []string
	var ws []float64
	for _, p := range []string{PoseSit, PoseLoaf, PoseSleep, PoseStand} {
		if p == PoseStand && !stand {
			continue
		}
		if wt := weights[p]; wt > 0 && info.allows(p) && w.free(c, spot, p) {
			poses = append(poses, p)
			ws = append(ws, wt)
		}
	}
	i := pick(w.rng, ws)
	if i < 0 {
		return "", false
	}
	return poses[i], true
}

// pickPlace chooses where a cat goes: a free spot it likes, no closer to
// you than its bond allows, and a pose that fits there. arriving: stand
// first if the spot allows it.
func (w *World) pickPlace(c *Cat, arriving bool) (string, string, bool) {
	tier := c.Tier()
	limit := maxNear(tier, c.Def.Ways.Shy)
	var names []string
	var ws []float64
	for _, s := range w.spots {
		if s.Near > limit || s.Name == c.spot {
			continue
		}
		wt := c.Def.Ways.Likes(s.Name) * (1 + 0.45*float64(tier)*float64(s.Near))
		if wt <= 0 {
			continue
		}
		names = append(names, s.Name)
		ws = append(ws, wt)
	}
	for len(names) > 0 {
		i := pick(w.rng, ws)
		spot := names[i]
		names = append(names[:i], names[i+1:]...)
		ws = append(ws[:i], ws[i+1:]...)
		info, _ := w.spotInfo(spot)
		if arriving && MoodOf(w.ctx.Part) != Sleepy && info.allows(PoseStand) && w.free(c, spot, PoseStand) {
			return spot, PoseStand, true
		}
		if p, ok := w.settlePose(c, spot, false); ok {
			return spot, p, true
		}
	}
	return "", "", false
}

func pick(rng *rand.Rand, ws []float64) int {
	total := 0.0
	for _, v := range ws {
		total += v
	}
	if total <= 0 {
		return -1
	}
	r := rng.Float64() * total
	for i, v := range ws {
		if r < v {
			return i
		}
		r -= v
	}
	return len(ws) - 1
}

// --- Little movements ----------------------------------------------------

// animate shows a blink, an ear twitch or a tail flick now and then, and
// slow breathing while asleep.
func (w *World) animate(c *Cat, dt float64) {
	if c.where != here {
		return
	}
	pose := c.Def.Poses[c.pose]
	if c.pose == PoseSleep && pose.Has(FrameBreathe) {
		speed := 2 * math.Pi / 3.6 // one breath every 3.6 seconds
		if w.calm {
			speed /= 1.4
		}
		c.breath = math.Mod(c.breath+dt*speed, 2*math.Pi)
	}
	if c.frameLeft > 0 {
		c.frameLeft -= dt
		if c.frameLeft <= 0 {
			c.frame = FrameStill
		}
		return
	}
	c.twitchIn -= dt
	if c.twitchIn > 0 {
		return
	}
	// Pick a little movement the pose has.
	type move struct {
		frame  string
		weight float64
		length float64
	}
	tail := 1.0
	if MoodOf(w.ctx.Part) == Playful {
		tail = 2 * c.Def.Ways.Energy
	}
	moves := []move{{FrameBlink, 4, 0.18}, {FrameEar, 1.2, 0.35}, {FrameTail, tail, 0.6}}
	if c.pose == PoseSleep {
		moves = []move{{FrameEar, 1, 0.3}}
	}
	if w.calm {
		moves = moves[:1] // only blinks (or, asleep, an ear now and then)
	}
	var ws []float64
	for _, m := range moves {
		if pose.Has(m.frame) {
			ws = append(ws, m.weight)
		} else {
			ws = append(ws, 0)
		}
	}
	if i := pick(w.rng, ws); i >= 0 {
		c.frame, c.frameLeft = moves[i].frame, moves[i].length
	}
	gap := 2.5 + w.rng.Float64()*5
	if c.pose == PoseSleep {
		gap = 8 + w.rng.Float64()*20
	}
	if w.calm {
		gap *= 2
	}
	c.twitchIn = gap
}

// Draw lists the cats to draw this frame.
func (w *World) Draw() []Drawn {
	var out []Drawn
	for _, c := range w.Cats {
		if c.where != here || c.alpha <= 0 {
			continue
		}
		pose := c.Def.Poses[c.pose]
		frame := c.frame
		if c.pose == PoseSleep && frame == FrameStill && pose.Has(FrameBreathe) && math.Sin(c.breath) < -0.2 {
			frame = FrameBreathe
		}
		out = append(out, Drawn{Cat: c, Sprite: pose.Sprite(frame), Spot: c.spot, Anchor: pose.Anchor, Alpha: c.alpha})
	}
	return out
}

// --- For moments and previews -------------------------------------------

// Hold keeps a cat where it is (during its moment) or lets it go again.
func (w *World) Hold(c *Cat, hold bool) {
	c.held = hold
	if !hold && w.ctx.Now.After(c.until.Add(-time.Minute)) {
		c.until = w.ctx.Now.Add(time.Minute) // don't wander off the instant it ends
	}
}

// Bring moves a cat to a spot (for a moment like "Brie jumps onto your
// table"), settling in a pose that fits. It reports whether it could.
func (w *World) Bring(c *Cat, spot string) bool {
	if c.claimed() == spot {
		return true
	}
	var pose string
	for _, p := range []string{PoseSit, PoseLoaf, PoseStand, PoseSleep} {
		if info, ok := w.spotInfo(spot); ok && info.allows(p) && w.free(c, spot, p) {
			pose = p
			break
		}
	}
	if pose == "" {
		return false
	}
	if c.where == here && c.alpha > 0 {
		w.moveTo(c, spot, pose)
	} else {
		c.where, c.spot, c.pose, c.alpha, c.fade = here, spot, pose, 0, 1
	}
	c.until = w.ctx.Now.Add(w.stay(c))
	return true
}

// After does what a moment's "then" says.
func (w *World) After(c *Cat, then string) {
	switch then {
	case "", "stay":
	case "leave", "away":
		if c.where == here {
			w.leave(c)
		}
	case PoseSit, PoseLoaf, PoseSleep:
		if c.where == here && c.fade == 0 {
			if info, ok := w.spotInfo(c.spot); ok && info.allows(then) && w.free(c, c.spot, then) {
				c.pose = then
			}
		}
	}
	c.until = w.ctx.Now.Add(w.stay(c))
}

// Put puts a cat somewhere at once (previews and tests). An empty spot
// sends it away.
func (w *World) Put(c *Cat, spot, pose string) {
	if spot == "" {
		c.where, c.spot, c.alpha, c.fade, c.next = away, "", 0, 0, ""
		c.until = w.ctx.Now.Add(time.Hour)
		return
	}
	c.where, c.spot, c.pose, c.alpha, c.fade, c.next = here, spot, pose, 1, 0, ""
	c.until = w.ctx.Now.Add(w.stay(c))
}
