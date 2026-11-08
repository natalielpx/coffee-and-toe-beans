package game

import (
	"time"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/save"
	"coffee-and-toe-beans/internal/text"
)

// The Settings screen (Esc menu, Settings): comfort settings that apply at
// once and are saved straight away. The words are in content/text/ui.json
// ("settings"); the timings they change are in content/pacing.json.

// Normal letters per second and fade times, when content/pacing.json
// doesn't say.
const (
	normalLetters = 34
	slowLetters   = 20
	normalFade    = 0.35
	reducedFade   = 0.12
)

// settingsMenu is the Settings screen while it's open (over the pause menu).
type settingsMenu struct {
	sel int
}

// normalize fills in the defaults for settings the save leaves empty, and
// turns unknown values into the default.
func (g *Game) normalize(s save.Settings) save.Settings {
	if s.Weather != "" && !g.wcfg.Has(s.Weather) {
		s.Weather = ""
	}
	if _, ok := g.sched.Chosen(s.TimeOfDay); !ok {
		s.TimeOfDay = ""
	}
	return normalizeBasic(s)
}

func normalizeBasic(s save.Settings) save.Settings {
	switch s.TextSpeed {
	case "slow", "normal", "instant":
	default:
		s.TextSpeed = "normal"
	}
	switch s.Pacing {
	case "gentle", "very_slow":
	default:
		s.Pacing = "gentle"
	}
	switch s.Color {
	case "truecolor", "256":
	default:
		s.Color = "" // auto
	}
	return s
}

// Settings are the comfort settings as they are now.
func (g *Game) Settings() save.Settings { return g.set }

// settingValue is a setting's current value, by its id in
// text.SettingValues.
func (g *Game) settingValue(id string) string {
	s := g.set
	switch id {
	case "reduced_motion":
		return onOff(s.ReducedMotion)
	case "quiet":
		return onOff(s.Quiet)
	case "text_speed":
		return s.TextSpeed
	case "pacing":
		return s.Pacing
	case "color":
		if s.Color == "" {
			return "auto"
		}
		return s.Color
	case "weather":
		if s.Weather == "" {
			return "ambient"
		}
		return s.Weather
	case "time_of_day":
		if s.TimeOfDay == "" {
			return "real"
		}
		return s.TimeOfDay
	}
	return ""
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// SetSetting changes one setting (by its id and value id, as in
// text.SettingValues), applies it at once and saves it.
func (g *Game) SetSetting(id, value string) {
	s := g.set
	switch id {
	case "reduced_motion":
		s.ReducedMotion = value == "on"
	case "quiet":
		s.Quiet = value == "on"
	case "text_speed":
		s.TextSpeed = value
	case "pacing":
		s.Pacing = value
	case "color":
		if value == "auto" {
			value = ""
		}
		s.Color = value
	case "weather":
		if value == "ambient" {
			value = ""
		}
		s.Weather = value
	case "time_of_day":
		if value == "real" {
			value = ""
		}
		s.TimeOfDay = value
	}
	g.set = g.normalize(s)
	g.applySettings(id == "color")
	g.saveSettings()
}

// applySettings makes the settings felt everywhere. The colour mode is only
// changed when asked (at the start, main has already chosen it, taking
// --color into account).
func (g *Game) applySettings(color bool) {
	s := g.set
	now := g.Now()
	g.cfg.Reduced = s.ReducedMotion
	g.cfg.Quiet = s.Quiet
	g.scene.SetReducedMotion(s.ReducedMotion)
	g.cat.world.SetCalm(s.ReducedMotion)

	slow := g.slow()
	if g.cat.pacer != nil {
		g.cat.pacer.SetSlow(slow, now)
	}
	if old := g.mem.talkSlow; old > 0 && old != slow {
		if left := g.mem.nextTalk.Sub(now); left > 0 {
			g.mem.nextTalk = now.Add(time.Duration(float64(left) * slow / old))
		}
	}
	g.mem.talkSlow = slow

	if g.cfg.Weather == "" { // --weather (for previews) wins
		_ = g.wstate.Pin(s.Weather, now)
	}

	letters, fade := g.letters(), g.fade()
	if g.box != nil {
		g.box.SetSpeed(letters, fade)
	}
	for _, q := range g.queue {
		q.SetSpeed(letters, fade)
	}

	if color {
		switch s.Color {
		case "truecolor":
			g.cfg.ColorMode = canvas.TrueColor
		case "256":
			g.cfg.ColorMode = canvas.Color256
		default:
			g.cfg.ColorMode = g.cfg.AutoColor
		}
	}
}

// dayNow is the time of day the café shows at café time now: the real
// clock, or the time picked in Settings (Time of day), on the same date.
// Light, sky, the cats' moods, the narrator and the drinks served follow
// it; timers (cups cooling, cats coming by) follow the clock as always.
func (g *Game) dayNow(now time.Time) time.Time {
	m, ok := g.sched.Chosen(g.set.TimeOfDay)
	if !ok {
		return now
	}
	clock := g.sched.ClockFor(now, m)
	y, mo, d := now.Date()
	return time.Date(y, mo, d, 0, 0, 0, 0, now.Location()).Add(time.Duration(clock * float64(time.Minute)))
}

// slow is how much the settings stretch the waits for moments and small
// talk: quiet mode (half as often) and "Very slow" pacing multiply.
func (g *Game) slow() float64 {
	f := 1.0
	p := g.mem.pace
	if g.set.Quiet {
		q := p.QuietFactor
		if q < 1 {
			q = 2
		}
		f *= q
	}
	if g.set.Pacing == "very_slow" {
		v := p.VerySlowFactor
		if v < 1 {
			v = 1.5
		}
		f *= v
	}
	return f
}

// letters is how fast words type out (0: each page at once).
func (g *Game) letters() float64 {
	switch g.set.TextSpeed {
	case "instant":
		return 0
	case "slow":
		if v := g.mem.pace.LettersPerSecond["slow"]; v > 0 {
			return v
		}
		return slowLetters
	}
	if v := g.mem.pace.LettersPerSecond["normal"]; v > 0 {
		return v
	}
	return normalLetters
}

func (g *Game) fade() float64 {
	if g.set.ReducedMotion {
		return reducedFade
	}
	return normalFade
}

// saveSettings saves the settings at once. Before you've stepped inside,
// only the settings change: the visit itself isn't counted yet.
func (g *Game) saveSettings() {
	m := &g.mem
	m.data.Settings = g.set
	if g.cfg.Store == nil {
		return
	}
	if g.inCafe > 0 {
		g.store()
		return
	}
	before := m.before
	before.Settings = g.set
	m.saveErr = g.cfg.Store(&before)
}

// --- Keys ----------------------------------------------------------------------

func (g *Game) openSettings() {
	g.settings = &settingsMenu{}
}

func (g *Game) settingsKey(k Key) {
	m := g.settings
	n := len(text.SettingValues)
	switch k {
	case KeyUp:
		if m.sel > 0 {
			m.sel--
		}
	case KeyDown:
		if m.sel < n-1 {
			m.sel++
		}
	case KeyLeft:
		g.stepSetting(m.sel, -1)
	case KeyRight, KeyConfirm:
		g.stepSetting(m.sel, 1)
	case KeyBack:
		g.settings = nil // back to the pause menu
	}
}

// stepSetting moves a setting to its next (or previous) value, going round.
func (g *Game) stepSetting(i, dir int) {
	sv := text.SettingValues[i]
	cur := g.settingValue(sv.ID)
	at := 0
	for j, v := range sv.Values {
		if v == cur {
			at = j
		}
	}
	n := len(sv.Values)
	g.SetSetting(sv.ID, sv.Values[((at+dir)%n+n)%n])
}

// --- Drawing ---------------------------------------------------------------------

// settingsAbout is the note under the list for the highlighted setting.
func (g *Game) settingsAbout(id, value string) string {
	sw := g.ui.Settings
	it, _ := sw.Setting(id)
	vw, _ := it.Value(value)
	auto := sw.AutoTrueColor
	if g.cfg.AutoColor == canvas.Color256 {
		auto = sw.Auto256
	}
	return text.Fill(vw.About, map[string]string{"auto": auto})
}

// settingsLayout works out the Settings panel's size and columns.
func (g *Game) settingsLayout() (w, h, valueCol, valueW int) {
	sw := g.ui.Settings
	labelW, inner := 0, canvas.Width(sw.Title)
	if hw := canvas.Width(sw.Hint); hw > inner {
		inner = hw
	}
	for _, sv := range text.SettingValues {
		it, _ := sw.Setting(sv.ID)
		if lw := canvas.Width(it.Label); lw > labelW {
			labelW = lw
		}
		for _, v := range sv.Values {
			vw, _ := it.Value(v)
			if n := canvas.Width(vw.Name) + 4; n > valueW {
				valueW = n // room for "‹ Name ›"
			}
			if aw := canvas.Width(g.settingsAbout(sv.ID, v)) + 2; aw > inner {
				inner = aw // the note starts two cells in, under the labels
			}
		}
	}
	valueCol = 4 + labelW + 3
	if n := valueCol + valueW - 2; n > inner {
		inner = n
	}
	w = inner + 4
	h = 4 + len(text.SettingValues) + 4 // title, items, the note, the hint
	if sw.Hint == "" {
		h--
	}
	return w, h, valueCol, valueW
}

func (g *Game) drawSettings(c *canvas.Canvas, ox, oy int) {
	sw := g.ui.Settings
	m := g.settings
	w, h, valueCol, _ := g.settingsLayout()
	p := panel{c: c, x: ox + (FrameW-w)/2, y: oy + (FrameH-h)/2, w: w, h: h, alpha: 1}
	p.draw()
	p.writeStyled(4, 1, sw.Title, softInk, true) // in line with the labels
	for i, sv := range text.SettingValues {
		row := 3 + i
		it, _ := sw.Setting(sv.ID)
		val := g.settingValue(sv.ID)
		vw, _ := it.Value(val)
		// Two columns: the label, then the value right after it ("‹ Snow ›"
		// when highlighted), all values starting at the same place.
		if i == m.sel {
			p.write(2, row, "▸", highlight)
			p.write(4, row, it.Label, boxInk)
			p.write(valueCol, row, "‹ "+vw.Name+" ›", highlight)
		} else {
			p.write(4, row, it.Label, boxDim)
			p.write(valueCol+2, row, vw.Name, boxDim)
		}
	}
	sel := text.SettingValues[m.sel]
	about := g.settingsAbout(sel.ID, g.settingValue(sel.ID))
	p.writeStyled(4, 4+len(text.SettingValues), about, boxDim, true)
	if sw.Hint != "" {
		p.write((w-canvas.Width(sw.Hint))/2, h-2, sw.Hint, boxFaint)
	}
}
