package weather

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
)

func load(t *testing.T) *Config {
	t.Helper()
	c, err := Load(content.FS, "weather.json")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestSameDateSameWeather(t *testing.T) {
	c := load(t)
	a := c.Day(day(2026, 11, 7))
	b := c.Day(day(2026, 11, 7).Add(15 * time.Hour))
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same date should always give the same weather")
	}
	if reflect.DeepEqual(a, c.Day(day(2026, 11, 8))) {
		t.Fatal("different days should usually differ")
	}
}

func TestSegmentsCoverTheDay(t *testing.T) {
	c := load(t)
	for d := 1; d <= 60; d++ {
		segs := c.Day(day(2026, 1, d))
		if segs[0].Start != 0 || segs[len(segs)-1].End != 1440 {
			t.Fatalf("day %d doesn't cover 00:00-24:00", d)
		}
		for i, s := range segs {
			if i > 0 && s.Start != segs[i-1].End {
				t.Fatalf("day %d has a gap at %v", d, s.Start)
			}
			def := c.kinds[s.Kind]
			if dur := s.End - s.Start; dur > def.Lasts[1]+1e-9 || (s.End < 1440 && dur < def.Lasts[0]-1e-9) {
				t.Fatalf("day %d: %s lasts %.0f minutes, outside %v", d, s.Kind, dur, def.Lasts)
			}
		}
	}
}

// share returns, for a range of days, the fraction of time spent in each kind.
func share(c *Config, from time.Time, days int) map[string]float64 {
	out := map[string]float64{}
	for i := 0; i < days; i++ {
		for _, s := range c.Day(from.AddDate(0, 0, i)) {
			out[s.Kind] += (s.End - s.Start) / (1440 * float64(days))
		}
	}
	return out
}

func TestMostlyRain(t *testing.T) {
	c := load(t)
	s := share(c, day(2026, 3, 1), 365)
	wet := s["rain"] + s["drizzle"] + s["heavy_rain"]
	if wet < 0.5 {
		t.Errorf("ambient weather should be mostly rain; it's wet %.0f%% of the time (%v)", wet*100, s)
	}
	for _, k := range []string{"clear", "fog", "drizzle", "rain"} {
		if s[k] < 0.03 {
			t.Errorf("%s should show up sometimes for variety, got %.1f%%", k, s[k]*100)
		}
	}
}

func TestSnowOnlyInWinterAndRare(t *testing.T) {
	c := load(t)
	for d := 0; d < 365; d++ {
		date := day(2026, 3, 1).AddDate(0, 0, d)
		if m := date.Month(); m == 12 || m == 1 || m == 2 {
			continue
		}
		for _, s := range c.Day(date) {
			if s.Kind == "snow" {
				t.Fatalf("snow on %s, which isn't winter", date.Format("2 Jan"))
			}
		}
	}
	w := share(c, day(2026, 12, 1), 90)
	if w["snow"] == 0 || w["snow"] > 0.2 {
		t.Errorf("snow should be rare but possible in winter, got %.1f%%", w["snow"]*100)
	}
}

func TestPinnedWeatherStays(t *testing.T) {
	c := load(t)
	start := time.Date(2026, 11, 7, 2, 0, 0, 0, time.Local)
	s, err := c.NewState(start, "fog")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		s.Advance(start.Add(time.Duration(i) * time.Minute))
		if s.Kind() != "fog" {
			t.Fatal("pinned weather changed")
		}
	}
	if math.Abs(s.Params().Fog-c.kinds["fog"].Fog) > 0.01 {
		t.Fatalf("fog should have settled, got %v", s.Params())
	}
	if _, err := c.NewState(start, "hail"); err == nil {
		t.Fatal("unknown weather should be refused")
	}
}

func TestChangesFadeGently(t *testing.T) {
	c := load(t)
	start := time.Date(2026, 11, 7, 0, 0, 0, 0, time.Local)
	s, _ := c.NewState(start, "")
	prev := s.Params()
	for i := 1; i <= 24*60*4; i++ { // every 15 seconds for a day
		s.Advance(start.Add(time.Duration(i) * 15 * time.Second))
		p := s.Params()
		for _, d := range []float64{p.Cloud - prev.Cloud, p.Rain - prev.Rain, p.Fog - prev.Fog, p.Snow - prev.Snow} {
			if math.Abs(d) > 0.15 {
				t.Fatalf("weather jumped by %.2f in 15 seconds at %v", d, start.Add(time.Duration(i)*15*time.Second))
			}
		}
		prev = p
	}
}

func TestSnowSettlesAndMelts(t *testing.T) {
	c := load(t)
	start := time.Date(2026, 12, 20, 18, 0, 0, 0, time.Local)
	s, _ := c.NewState(start, "snow")
	if s.SnowCover() < 0.8 {
		t.Fatalf("after 3 hours of snow the roofs should be white, cover = %.2f", s.SnowCover())
	}
	r, _ := c.NewState(start, "rain")
	if r.SnowCover() != 0 {
		t.Fatal("no snow should lie after rain")
	}
}

func TestLongGapDoesNotHang(t *testing.T) {
	c := load(t)
	start := time.Date(2026, 11, 7, 2, 0, 0, 0, time.Local)
	s, _ := c.NewState(start, "")
	s.Advance(start.Add(72 * time.Hour))
	s.Advance(start) // going backwards is ignored
	_ = s.Params()
}

func TestParseErrors(t *testing.T) {
	base := `"kinds":{"a":{"cloud":0,"lasts_minutes":[10,20]}}`
	cases := map[string]string{
		`{` + base + `,"ambient":{"first":{"b":1},"next":{"a":{"a":1}}}}`:                            "isn't one of the kinds",
		`{` + base + `,"ambient":{"first":{"a":1},"next":{}}}`:                                       "no entry",
		`{` + base + `,"ambient":{"first":{"a":0},"next":{"a":{"a":1}}}}`:                            "at least one weight",
		`{"kinds":{"a":{"lasts_minutes":[20,10]}},"ambient":{"first":{"a":1},"next":{"a":{"a":1}}}}`: "lasts_minutes",
		`{"kinds":{}}`: "no kinds",
	}
	for src, want := range cases {
		_, err := Parse("weather.json", []byte(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want error containing %q, got %v", want, err)
		}
	}
}
