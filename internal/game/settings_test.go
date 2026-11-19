package game

import (
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/save"
	"coffee-and-toe-beans/internal/text"
)

// openSettingsFromCafe goes Esc, down to Settings, Enter.
func openSettingsFromCafe(t *testing.T, g *Game) {
	t.Helper()
	g.Key(KeyBack)
	for g.pause.items[g.pause.sel] != actSettings {
		g.Key(KeyDown)
	}
	g.Key(KeyConfirm)
	if g.settings == nil {
		t.Fatal("Settings didn't open")
	}
}

// selectSetting moves the highlight to a setting by id.
func selectSetting(g *Game, id string) {
	for i, sv := range text.SettingValues {
		if sv.ID == id {
			g.settings.sel = i
		}
	}
}

func TestSettingsScreenShowsEverySettingAndFits(t *testing.T) {
	g, _ := inside(t, Config{})
	g.Resize(FrameW, FrameH)
	openSettingsFromCafe(t, g)
	s := screen(g)
	for _, it := range g.ui.Settings.Items {
		if !strings.Contains(s, it.Label) {
			t.Errorf("the settings screen should show %q", it.Label)
		}
	}
	w, h, _, _ := g.settingsLayout()
	if w > FrameW-4 || h > FrameH-4 {
		t.Errorf("the settings panel is %d x %d; the café is %d x %d", w, h, FrameW, FrameH)
	}
	// Every value's note fits inside the panel.
	for _, sv := range text.SettingValues {
		for _, v := range sv.Values {
			if aw := canvas.Width(g.settingsAbout(sv.ID, v)); aw > w-6 {
				t.Errorf("the note for %s=%s is too wide (%d)", sv.ID, v, aw)
			}
		}
	}
	g.Key(KeyBack)
	if g.settings != nil || g.pause == nil {
		t.Error("Esc in Settings should go back to the menu")
	}
	g.Key(KeyBack)
	if g.pause != nil {
		t.Error("Esc again closes the menu")
	}
}

func TestEverySettingGoesRoundWithTheArrows(t *testing.T) {
	g, _ := inside(t, Config{})
	openSettingsFromCafe(t, g)
	for i, sv := range text.SettingValues {
		g.settings.sel = i
		start := g.settingValue(sv.ID)
		seen := map[string]bool{start: true}
		for range sv.Values {
			g.Key(KeyRight)
			seen[g.settingValue(sv.ID)] = true
		}
		if g.settingValue(sv.ID) != start {
			t.Errorf("%s: → all the way round should come back to %q, got %q", sv.ID, start, g.settingValue(sv.ID))
		}
		if len(seen) != len(sv.Values) {
			t.Errorf("%s: → should visit all %d values, saw %v", sv.ID, len(sv.Values), seen)
		}
		g.Key(KeyLeft)
		g.Key(KeyRight)
		if g.settingValue(sv.ID) != start {
			t.Errorf("%s: ← then → should come back", sv.ID)
		}
	}
	if g.Done() {
		t.Fatal("changing settings must never close the game")
	}
}

func TestSettingsApplyAtOnce(t *testing.T) {
	g, now := inside(t, Config{AutoColor: canvas.Color256})
	g.SetSetting("reduced_motion", "on")
	if !g.cfg.Reduced || !g.scene.Options().ReducedMotion {
		t.Error("reduced motion should apply to the scene at once")
	}
	g.SetSetting("text_speed", "instant")
	if g.letters() != 0 {
		t.Error("instant text should type at 0 letters a second (all at once)")
	}
	now = run(g, now, 3)
	if g.box != nil && g.box.Finished() == false && g.box.Alpha() >= 1 {
		t.Error("with instant text, the welcome should appear whole")
	}
	g.SetSetting("text_speed", "slow")
	if g.letters() >= normalLetters {
		t.Error("slow text should be slower than normal")
	}
	g.SetSetting("color", "truecolor")
	if g.cfg.ColorMode != canvas.TrueColor {
		t.Error("choosing true colour should switch at once")
	}
	g.SetSetting("color", "auto")
	if g.cfg.ColorMode != canvas.Color256 {
		t.Error("Auto should use what was detected")
	}
	_ = now
}

func TestQuietAndVerySlowStretchTheWaits(t *testing.T) {
	g, now := inside(t, Config{})
	base := g.cat.pacer.NextMoment().Sub(g.Now())
	if g.slow() != 1 {
		t.Fatalf("by default nothing is stretched, got %v", g.slow())
	}
	g.SetSetting("quiet", "on")
	if g.slow() != 2 {
		t.Errorf("quiet mode should halve how often things happen, got %v", g.slow())
	}
	g.SetSetting("pacing", "very_slow")
	if g.slow() != 3 {
		t.Errorf("quiet and very slow together: 2 x 1.5 = 3, got %v", g.slow())
	}
	got := g.cat.pacer.NextMoment().Sub(g.Now())
	if d := got.Seconds() - base.Seconds()*3; d < -2 || d > 2 {
		t.Errorf("the wait already running should stretch too: %v -> %v", base, got)
	}
	g.SetSetting("quiet", "off")
	g.SetSetting("pacing", "gentle")
	_ = now
}

// talkWith counts cat moments and small talk over some café minutes, with
// settings changed the way a player does (as catTalk, in cats_test.go).
func talkWith(t *testing.T, settings map[string]string, minutes float64) (moments, smallTalk int) {
	t.Helper()
	bonds := map[string]float64{"brie": 30, "hunter": 30, "mimi": 30}
	g, now := inside(t, Config{Seed: 4, Bonds: bonds, Start: t0.Add(12 * time.Hour)})
	for id, v := range settings {
		g.SetSetting(id, v)
	}
	var last *talkBox
	steps := int(minutes * 60 * 2)
	for i := 0; i < steps; i++ {
		now = now.Add(500 * time.Millisecond)
		g.Tick(now)
		b := g.box
		if b == nil {
			continue
		}
		if b != last {
			last = b
			switch {
			case b.kind == talkMoment:
				moments++
			case b.kind == talkNarrator && g.greeted && b.Speaker() == "" && len(b.Choices()) == 0:
				smallTalk++
			}
		}
		if b.kind == talkMoment && !b.Answered() && i%4 == 0 {
			g.Key(KeyConfirm)
		}
	}
	return moments, smallTalk
}

func TestQuietAndPacingSettingsCutTheTalk(t *testing.T) {
	if testing.Short() {
		t.Skip("long session")
	}
	m1, s1 := talkWith(t, nil, 240)
	m2, s2 := talkWith(t, map[string]string{"quiet": "on"}, 240)
	m3, s3 := talkWith(t, map[string]string{"pacing": "very_slow"}, 240)
	t.Logf("4 hours: normal %d moments + %d small talk; quiet %d + %d; very slow %d + %d", m1, s1, m2, s2, m3, s3)
	if float64(s2) > 0.6*float64(s1) || float64(m2+s2) > 0.6*float64(m1+s1) {
		t.Errorf("quiet mode should say about half as much: %d+%d vs %d+%d", m2, s2, m1, s1)
	}
	if float64(m3+s3) > 0.8*float64(m1+s1) || m3+s3 < m2+s2 {
		t.Errorf("very slow should sit between gentle and quiet: %d+%d (gentle %d+%d, quiet %d+%d)", m3, s3, m1, s1, m2, s2)
	}
}

func TestTimeOfDayKeepsTheLookButNotTheTimers(t *testing.T) {
	// 10:00 on a November morning, but the café is set to night.
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local)
	g, err := New(Config{Start: start, Seed: 3}, content.FS, t0)
	if err != nil {
		t.Fatal(err)
	}
	day := g.sched.At(g.Now())
	g.SetSetting("time_of_day", "night")
	night := g.sched.At(g.dayNow(g.Now()))
	if night.Lamps <= day.Lamps || night.Daylight >= day.Daylight {
		t.Errorf("night should have lamps on and no daylight: %+v vs %+v", night, day)
	}
	if p := g.sched.Phase(g.dayNow(g.Now())); p != "night" {
		t.Errorf("the cats and narrator should think it's night, got %q", p)
	}
	before := g.Now()
	run(g, t0, 60)
	if g.Now().Sub(before) < 59*time.Second {
		t.Error("the café clock (cups, cats coming by) should keep running")
	}
	// The chosen look holds whatever the season.
	for _, date := range []time.Time{
		time.Date(2026, 6, 21, 12, 0, 0, 0, time.Local),
		time.Date(2026, 12, 21, 12, 0, 0, 0, time.Local),
	} {
		for _, id := range []string{"dawn", "morning", "afternoon", "golden", "night"} {
			g.SetSetting("time_of_day", id)
			want, _ := g.sched.Chosen(id)
			got := g.sched.ModelMinute(g.dayNow(date))
			if d := got - want; d < -2 || d > 2 {
				t.Errorf("%s on %s should look like %.0f on the model day, got %.0f", id, date.Format("2 Jan"), want, got)
			}
		}
	}
	// Evening drinks follow the time shown.
	g.SetSetting("time_of_day", "night")
	for _, d := range g.drinks.Drinks {
		if d.ID == "vin_chaud" && !d.Available(g.dayNow(g.Now())) {
			t.Error("vin chaud should be served when the café shows night")
		}
	}
	g.SetSetting("time_of_day", "real")
	if !g.dayNow(g.Now()).Equal(g.Now()) {
		t.Error("Real clock should show the café's own time")
	}
}

func TestWeatherSetting(t *testing.T) {
	g, now := inside(t, Config{})
	for _, kind := range g.wcfg.Kinds() {
		g.SetSetting("weather", kind)
		now = run(g, now, 1)
		if g.wstate.Kind() != kind {
			t.Errorf("chose %s, the sky is %s", kind, g.wstate.Kind())
		}
	}
	g.SetSetting("weather", "snow")
	run(g, now, 1)
	if g.wstate.Params().Snow < 0.3 {
		t.Error("snow chosen in Settings should be falling straight away")
	}
	g.SetSetting("weather", "ambient")
	if g.settingValue("weather") != "ambient" {
		t.Error("ambient should follow the café's own weather again")
	}
	// --weather (previews) wins over the setting.
	p, _ := inside(t, Config{Weather: "fog"})
	p.SetSetting("weather", "clear")
	if p.wstate.Kind() != "fog" {
		t.Error("--weather should keep its weather")
	}
}

func TestSettingsAreSavedAndComeBack(t *testing.T) {
	g, stored := visit(t, nil, t0, 5)
	g.SetSetting("reduced_motion", "on")
	g.SetSetting("quiet", "on")
	g.SetSetting("text_speed", "slow")
	g.SetSetting("pacing", "very_slow")
	g.SetSetting("weather", "snow")
	g.SetSetting("time_of_day", "golden")
	g.SetSetting("color", "256")
	run(g, t0, 1)
	d := last(stored)
	if d == nil {
		t.Fatal("changing a setting should save at once")
	}
	want := save.Settings{ReducedMotion: true, Quiet: true, TextSpeed: "slow", Pacing: "very_slow", Weather: "snow", TimeOfDay: "golden", Color: "256"}
	if d.Settings != want {
		t.Errorf("saved %+v, want %+v", d.Settings, want)
	}
	again, _ := visit(t, d, t0.Add(time.Hour), 6)
	if again.Settings() != want {
		t.Errorf("came back with %+v, want %+v", again.Settings(), want)
	}
	if !again.cfg.Reduced || !again.cfg.Quiet || again.wstate.Kind() != "snow" {
		t.Error("the saved settings should be in force on the next visit")
	}
}

func TestSettingsFromTheTitleCardDontCountAVisit(t *testing.T) {
	var stored []*save.Data
	store := func(d *save.Data) error { c := *d; stored = append(stored, &c); return nil }
	old := save.New()
	old.Visits = 4
	old.LastVisit = t0.Add(-48 * time.Hour)
	g, err := New(Config{Seed: 2, Save: old, Store: store}, content.FS, t0)
	if err != nil {
		t.Fatal(err)
	}
	g.Key(KeyBack)
	if g.pause.items[0] != actSettings {
		t.Fatal("on the title card, Settings comes first")
	}
	g.Key(KeyConfirm)
	selectSetting(g, "color")
	g.Key(KeyRight)
	if len(stored) == 0 {
		t.Fatal("a setting changed on the title card should be saved")
	}
	d := stored[len(stored)-1]
	if d.Visits != 4 || !d.LastVisit.Equal(old.LastVisit) {
		t.Errorf("only the settings should change: visits %d, last visit %v", d.Visits, d.LastVisit)
	}
	if d.Settings.Color == "" {
		t.Error("the colour setting should be saved")
	}
	g.Key(KeyBack) // back to the menu
	g.Key(KeyDown) // Leave the café
	g.Key(KeyConfirm)
	run(g, t0, 3)
	if !g.Done() {
		t.Error("leaving from the title card should still work")
	}
}

func TestReducedMotionStopsTheTitlePulseAndBlink(t *testing.T) {
	g := newGame(t, Config{})
	g.SetSetting("reduced_motion", "on")
	if !g.blink() {
		t.Error("with reduced motion the ▼ shouldn't blink")
	}
	if g.fadeTime(1) >= 1 {
		t.Error("fades should be gentler (shorter) with reduced motion")
	}
}
