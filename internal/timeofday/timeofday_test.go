package timeofday

import (
	"math"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/rgb"
)

func load(t *testing.T) *Schedule {
	t.Helper()
	s, err := Load(content.FS, "lighting.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Paris winter and summer time, fixed so the tests give the same answer on
// any computer (GitHub's test machines run on UTC) and on any day of the year.
var (
	parisWinter = time.FixedZone("CET", 3600)
	parisSummer = time.FixedZone("CEST", 7200)
)

func at(hh, mm int) time.Time {
	return time.Date(2026, 11, 7, hh, mm, 0, 0, parisWinter)
}

// look sums up how different two moments look: sky, rooftops and room.
func look(a, b Light) float64 {
	return rgb.Distance(a.SkyTop, b.SkyTop) +
		rgb.Distance(a.SkyHorizon, b.SkyHorizon) +
		rgb.Distance(a.Ambient, b.Ambient) +
		math.Abs(a.Lamps-b.Lamps)*0.2
}

func TestKeyTimesLookDifferent(t *testing.T) {
	// Acceptance: dawn, day, dusk and night look visibly different (on the
	// model day: 07:00, 13:00, 19:00 and 01:00).
	s := load(t)
	times := map[string]Light{
		"07:00": s.AtMinute(7 * 60),
		"13:00": s.AtMinute(13 * 60),
		"19:00": s.AtMinute(19 * 60),
		"01:00": s.AtMinute(1 * 60),
	}
	for a, la := range times {
		for b, lb := range times {
			if a >= b {
				continue
			}
			if d := look(la, lb); d < 0.25 {
				t.Errorf("%s and %s look too alike (difference %.3f)", a, b, d)
			}
		}
	}
	if times["01:00"].Lamps < 0.9 || times["13:00"].Lamps > 0.1 {
		t.Error("lamps should be on at night and off at midday")
	}
	if times["13:00"].Ambient.Luma() < 2*times["01:00"].Ambient.Luma() {
		t.Error("midday should be much brighter than 1am")
	}
}

func TestTransitionsAreGradual(t *testing.T) {
	// Every 30 seconds through the whole day, the change must be tiny.
	s := load(t)
	prev := s.AtMinute(0)
	worst := 0.0
	for m := 0.5; m <= 1440; m += 0.5 {
		cur := s.AtMinute(m)
		if d := look(prev, cur); d > worst {
			worst = d
		}
		prev = cur
	}
	// 0.02 is about the smallest color change a person can notice side by
	// side; staying under it every 30 seconds means no visible steps.
	if worst > 0.015 {
		t.Errorf("biggest change in 30 seconds is %.4f; transitions should be smoother", worst)
	}
}

func TestMidnightWraps(t *testing.T) {
	s := load(t)
	before := s.AtMinute(1439.99)
	after := s.AtMinute(0)
	if look(before, after) > 0.001 {
		t.Fatal("there's a jump at midnight")
	}
	if look(s.AtMinute(-60), s.AtMinute(1380)) > 1e-9 {
		t.Fatal("negative minutes should wrap")
	}
}

func TestKeyframeIsExact(t *testing.T) {
	s := load(t)
	l := s.AtMinute(13 * 60)
	if l.SkyTop.String() != "#5c9bd8" {
		t.Fatalf("13:00 sky top = %v", l.SkyTop)
	}
}

func TestPhases(t *testing.T) {
	s := load(t)
	// Around the spring equinox, Paris's real sunrise and sunset (about 06:59
	// and 19:13) are close to the model day's, so the clock reads naturally.
	spring := func(hh, mm int) time.Time { return time.Date(2026, 3, 21, hh, mm, 0, 0, parisWinter) }
	cases := []struct {
		at   time.Time
		want string
	}{
		{spring(2, 18), "late night"},
		{spring(6, 0), "dawn"},
		{spring(9, 0), "morning"},
		{spring(15, 0), "afternoon"},
		{spring(19, 0), "evening"},
		{spring(23, 0), "night"},
		// The seasons move the hours: 18:30 is already evening in November
		// (sunset about 17:20) but still afternoon in June (sunset about 21:58).
		{time.Date(2026, 11, 7, 18, 30, 0, 0, parisWinter), "evening"},
		{time.Date(2026, 6, 21, 18, 30, 0, 0, parisSummer), "afternoon"},
		{time.Date(2026, 6, 21, 22, 0, 0, 0, parisSummer), "evening"},
	}
	for _, c := range cases {
		if got := s.Phase(c.at); got != c.want {
			t.Errorf("%s: got %q want %q", c.at.Format("2 Jan 15:04"), got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	good := `{"time":"00:00","sky_top":"#000000","sky_horizon":"#000000","outdoor_light":"#000000","room_light":"#000000"}`
	cases := map[string]string{
		`{"keyframes":[` + good + `]}`:                                                    "at least two",
		`{"keyframes":[` + good + `,` + good + `]}`:                                       "same time",
		`{"keyframes":[` + strings.Replace(good, "00:00", "25:00", 1) + `,` + good + `]}`: "HH:MM",
		`{"keyframes":[` + strings.Replace(good, `"#000000"`, `"blue"`, 1) + `,` + strings.Replace(good, "00:00", "01:00", 1) + `]}`: "sky_top",
		`{"keyframes":[` + strings.Replace(good, `}`, `,"lamps":2}`, 1) + `,` + strings.Replace(good, "00:00", "01:00", 1) + `]}`:    "between 0 and 1",
		`{oops`: "lighting.json",
	}
	for src, want := range cases {
		_, err := Parse("lighting.json", []byte(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want error containing %q, got %v\nsource: %s", want, err, src)
		}
	}
}

func TestMoonPhase(t *testing.T) {
	newMoon := time.Date(2024, 4, 8, 18, 21, 0, 0, time.UTC) // the 2024 total eclipse
	p := MoonPhase(newMoon)
	if p > 0.02 && p < 0.98 {
		t.Errorf("2024-04-08 should be a new moon, got phase %.3f", p)
	}
	full := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	if p := MoonPhase(full); math.Abs(p-0.5) > 0.04 {
		t.Errorf("2026-11-24 should be a full moon, got phase %.3f", p)
	}
}

func TestParisSunTimes(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no time zone data")
	}
	cases := []struct {
		date      time.Time
		rise, set string
	}{
		{time.Date(2026, 6, 21, 0, 0, 0, 0, paris), "05:47", "21:58"},
		{time.Date(2026, 12, 21, 0, 0, 0, 0, paris), "08:42", "16:56"},
		{time.Date(2026, 11, 7, 0, 0, 0, 0, paris), "07:46", "17:22"},
	}
	for _, c := range cases {
		rise, set, ok := SunTimes(c.date, 48.8566, 2.3522)
		wr, _ := ParseClock(c.rise)
		ws, _ := ParseClock(c.set)
		if !ok || math.Abs(rise-wr) > 4 || math.Abs(set-ws) > 4 {
			t.Errorf("%s: sunrise %.0f sunset %.0f, want about %s and %s",
				c.date.Format("2 Jan"), rise, set, c.rise, c.set)
		}
	}
}

func TestSeasonsChangeTheEvening(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no time zone data")
	}
	s := load(t)
	winter := s.At(time.Date(2026, 12, 21, 18, 0, 0, 0, paris))
	summer := s.At(time.Date(2026, 6, 21, 18, 0, 0, 0, paris))
	if winter.Ambient.Luma() >= summer.Ambient.Luma() || winter.Lamps <= summer.Lamps {
		t.Errorf("18:00 should be dark with lamps on in December, bright in June: %v / %v", winter, summer)
	}
	if !(s.At(time.Date(2026, 6, 21, 21, 0, 0, 0, paris)).Daylight > 0.2) {
		t.Error("a June evening at 21:00 should still have daylight")
	}
	// Still smooth through a whole real day, including both mapped seams.
	day := time.Date(2026, 3, 29, 0, 0, 0, 0, paris) // the night clocks change
	prev := s.At(day)
	for i := 1; i <= 2*24*60; i++ {
		cur := s.At(day.Add(time.Duration(i) * 30 * time.Second))
		if d := look(prev, cur); d > 0.02 {
			t.Fatalf("jump of %.3f at %v", d, day.Add(time.Duration(i)*30*time.Second))
		}
		prev = cur
	}
}
