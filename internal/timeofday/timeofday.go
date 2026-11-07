// Package timeofday turns the clock into light: sky colors, how bright the
// room is, when the lamps come on. The look of each hour is written as
// "keyframes" in content/lighting.json, and every moment in between is a
// smooth blend of the two nearest keyframes, so the café drifts from dawn to
// night without any visible jumps.
//
// The keyframes describe a "model day" with a fixed sunrise and sunset. The
// real clock is mapped onto it using Paris's actual sunrise and sunset for
// the date, so the seasons come through: early dark in winter, long light
// evenings in summer.
package timeofday

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/rgb"
)

// Light describes the lighting at one moment.
type Light struct {
	SkyTop     rgb.Color // sky color at the top of the window
	SkyHorizon rgb.Color // sky color just above the rooftops
	Exterior   rgb.Color // how the rooftops are lit (multiplies their colors)
	Ambient    rgb.Color // how the room is lit before the lamps (multiplies)
	Daylight   float64   // extra light falling in through the window, 0..1
	Lamps      float64   // how bright the café's lamps are, 0 (off) .. 1
	Windows    float64   // share of distant windows lit, 0..1
	Stars      float64   // how visible the stars and moon are, 0..1
}

// Lerp blends two lights.
func (a Light) Lerp(b Light, t float64) Light {
	return Light{
		SkyTop:     a.SkyTop.Lerp(b.SkyTop, t),
		SkyHorizon: a.SkyHorizon.Lerp(b.SkyHorizon, t),
		Exterior:   a.Exterior.Lerp(b.Exterior, t),
		Ambient:    a.Ambient.Lerp(b.Ambient, t),
		Daylight:   a.Daylight + (b.Daylight-a.Daylight)*t,
		Lamps:      a.Lamps + (b.Lamps-a.Lamps)*t,
		Windows:    a.Windows + (b.Windows-a.Windows)*t,
		Stars:      a.Stars + (b.Stars-a.Stars)*t,
	}
}

type keyframe struct {
	minute float64
	name   string
	light  Light
}

type phase struct {
	minute float64
	name   string
}

// Schedule is the whole day's lighting.
type Schedule struct {
	keys   []keyframe
	phases []phase
	sun    *sunPlace
}

type sunPlace struct {
	lat, lon              float64
	styledRise, styledSet float64
}

type fileKeyframe struct {
	Time       string  `json:"time"`
	Name       string  `json:"name"`
	SkyTop     string  `json:"sky_top"`
	SkyHorizon string  `json:"sky_horizon"`
	Exterior   string  `json:"outdoor_light"`
	Ambient    string  `json:"room_light"`
	Daylight   float64 `json:"daylight"`
	Lamps      float64 `json:"lamps"`
	Windows    float64 `json:"lit_windows"`
	Stars      float64 `json:"stars"`
}

type filePhase struct {
	From string `json:"from"`
	Name string `json:"name"`
}

type fileSun struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Sunrise   string  `json:"sunrise_looks_like"`
	Sunset    string  `json:"sunset_looks_like"`
}

type file struct {
	Keyframes []fileKeyframe `json:"keyframes"`
	Phases    []filePhase    `json:"phases"`
	Sun       *fileSun       `json:"sun"`
}

// Load reads the lighting schedule from a JSON file.
func Load(fsys fs.FS, name string) (*Schedule, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return Parse(name, data)
}

// Parse reads the lighting schedule from JSON bytes.
func Parse(name string, data []byte) (*Schedule, error) {
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	if len(f.Keyframes) < 2 {
		return nil, fmt.Errorf("%s: needs at least two keyframes", name)
	}
	s := &Schedule{}
	seen := map[float64]bool{}
	for i, k := range f.Keyframes {
		where := fmt.Sprintf("%s keyframe %d (%s)", name, i+1, k.Time)
		m, err := ParseClock(k.Time)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", where, err)
		}
		if seen[m] {
			return nil, fmt.Errorf("%s: two keyframes share the same time", where)
		}
		seen[m] = true
		var l Light
		for _, c := range []struct {
			dst *rgb.Color
			src string
			key string
		}{
			{&l.SkyTop, k.SkyTop, "sky_top"},
			{&l.SkyHorizon, k.SkyHorizon, "sky_horizon"},
			{&l.Exterior, k.Exterior, "outdoor_light"},
			{&l.Ambient, k.Ambient, "room_light"},
		} {
			col, err := rgb.Hex(c.src)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %v", where, c.key, err)
			}
			*c.dst = col
		}
		for _, v := range []struct {
			val float64
			key string
		}{{k.Daylight, "daylight"}, {k.Lamps, "lamps"}, {k.Windows, "lit_windows"}, {k.Stars, "stars"}} {
			if v.val < 0 || v.val > 1 {
				return nil, fmt.Errorf("%s: %s should be between 0 and 1", where, v.key)
			}
		}
		l.Daylight, l.Lamps, l.Windows, l.Stars = k.Daylight, k.Lamps, k.Windows, k.Stars
		s.keys = append(s.keys, keyframe{minute: m, name: k.Name, light: l})
	}
	sort.Slice(s.keys, func(i, j int) bool { return s.keys[i].minute < s.keys[j].minute })
	for _, p := range f.Phases {
		m, err := ParseClock(p.From)
		if err != nil {
			return nil, fmt.Errorf("%s: phase %q: %v", name, p.Name, err)
		}
		s.phases = append(s.phases, phase{minute: m, name: p.Name})
	}
	sort.Slice(s.phases, func(i, j int) bool { return s.phases[i].minute < s.phases[j].minute })
	if f.Sun != nil {
		rise, err := ParseClock(f.Sun.Sunrise)
		if err != nil {
			return nil, fmt.Errorf("%s: sun.sunrise_looks_like: %v", name, err)
		}
		set, err := ParseClock(f.Sun.Sunset)
		if err != nil {
			return nil, fmt.Errorf("%s: sun.sunset_looks_like: %v", name, err)
		}
		if rise >= set {
			return nil, fmt.Errorf("%s: sun: sunrise must come before sunset", name)
		}
		if math.Abs(f.Sun.Latitude) > 90 || math.Abs(f.Sun.Longitude) > 180 {
			return nil, fmt.Errorf("%s: sun: latitude or longitude out of range", name)
		}
		s.sun = &sunPlace{lat: f.Sun.Latitude, lon: f.Sun.Longitude, styledRise: rise, styledSet: set}
	}
	return s, nil
}

// ParseClock reads "HH:MM" and returns minutes after midnight.
func ParseClock(s string) (float64, error) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("time %q should look like HH:MM", s)
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q should look like HH:MM (00:00 to 23:59)", s)
	}
	return float64(h*60 + m), nil
}

// MinuteOfDay returns minutes after midnight, including the fraction.
func MinuteOfDay(t time.Time) float64 {
	return float64(t.Hour()*60+t.Minute()) + float64(t.Second())/60 + float64(t.Nanosecond())/6e10
}

// At returns the light for a moment, following the season's real sunrise
// and sunset.
func (s *Schedule) At(t time.Time) Light {
	return s.AtMinute(s.ModelMinute(t))
}

// ModelMinute is where a real moment falls on the model day (minutes after
// midnight). Without a "sun" section in the file, it's just the clock time.
func (s *Schedule) ModelMinute(t time.Time) float64 {
	m := MinuteOfDay(t)
	if s.sun == nil {
		return m
	}
	rise, set, ok := SunTimes(t, s.sun.lat, s.sun.lon)
	if !ok || rise <= 0 || set >= 1440 || rise >= set {
		return m
	}
	return styledMinute(m, rise, set, s.sun.styledRise, s.sun.styledSet)
}

// AtMinute returns the light at a number of minutes after midnight.
func (s *Schedule) AtMinute(m float64) Light {
	m = math.Mod(m, 1440)
	if m < 0 {
		m += 1440
	}
	n := len(s.keys)
	// Find the last keyframe at or before m (wrapping around midnight).
	i := sort.Search(n, func(i int) bool { return s.keys[i].minute > m }) - 1
	var a, b keyframe
	var start, end float64
	if i < 0 {
		a, b = s.keys[n-1], s.keys[0]
		start, end = a.minute-1440, b.minute
	} else if i == n-1 {
		a, b = s.keys[n-1], s.keys[0]
		start, end = a.minute, b.minute+1440
	} else {
		a, b = s.keys[i], s.keys[i+1]
		start, end = a.minute, b.minute
	}
	t := (m - start) / (end - start)
	return a.light.Lerp(b.light, t)
}

// Phase names the part of the day ("dawn", "night"...) for narration.
func (s *Schedule) Phase(t time.Time) string {
	if len(s.phases) == 0 {
		return ""
	}
	m := s.ModelMinute(t)
	name := s.phases[len(s.phases)-1].name // before the first phase: still yesterday's last
	for _, p := range s.phases {
		if m >= p.minute {
			name = p.name
		}
	}
	return name
}

// synodicMonth is the average time from one new moon to the next.
const synodicMonth = 29.530588853

// knownNewMoon is a well-documented new moon (6 January 2000, 18:14 UTC).
var knownNewMoon = time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)

// MoonPhase returns where the moon is in its cycle: 0 new, 0.25 first
// quarter, 0.5 full, 0.75 last quarter.
func MoonPhase(t time.Time) float64 {
	days := t.Sub(knownNewMoon).Hours() / 24
	p := math.Mod(days/synodicMonth, 1)
	if p < 0 {
		p++
	}
	return p
}
