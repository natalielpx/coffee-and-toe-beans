package scene

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
	"coffee-and-toe-beans/internal/timeofday"
	"coffee-and-toe-beans/internal/weather"
)

var update = flag.Bool("update", false, "rewrite the golden frames in testdata/golden")

// world bundles everything needed to render a frame of a café.
type world struct {
	scene *Scene
	sched *timeofday.Schedule
	wcfg  *weather.Config
}

func realWorld(t *testing.T, opts Options) world {
	t.Helper()
	lib, err := sprites.Load(content.FS, "sprites")
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(content.FS, "french", lib, opts)
	if err != nil {
		t.Fatal(err)
	}
	sched, err := timeofday.Load(content.FS, "lighting.json")
	if err != nil {
		t.Fatal(err)
	}
	wcfg, err := weather.Load(content.FS, "weather.json")
	if err != nil {
		t.Fatal(err)
	}
	return world{s, sched, wcfg}
}

func fixtureWorld(t *testing.T) world {
	t.Helper()
	fsys := os.DirFS("testdata/content")
	lib, err := sprites.Load(fsys, "sprites")
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(fsys, "tiny", lib, Options{})
	if err != nil {
		t.Fatal(err)
	}
	sched, err := timeofday.Load(content.FS, "lighting.json")
	if err != nil {
		t.Fatal(err)
	}
	wcfg, err := weather.Load(content.FS, "weather.json")
	if err != nil {
		t.Fatal(err)
	}
	return world{s, sched, wcfg}
}

func at(hh, mm int) time.Time { return time.Date(2026, 11, 24, hh, mm, 0, 0, time.UTC) }

// render runs the scene for a few seconds at a moment and returns the frame.
func (w world) render(t *testing.T, when time.Time, sky string, seconds float64) *canvas.Canvas {
	t.Helper()
	ws, err := w.wcfg.NewState(when, sky)
	if err != nil {
		t.Fatal(err)
	}
	in := Inputs{Now: when, Light: w.sched.At(when), Weather: ws.Params(), SnowCover: ws.SnowCover()}
	for s := 0.0; s < seconds; s += 0.1 {
		w.scene.Update(0.1, in)
	}
	pw, ph := w.scene.Size()
	c := canvas.New(pw, (ph+1)/2)
	w.scene.Draw(c, 0, 0)
	return c
}

func avgLuma(c *canvas.Canvas, x0, y0, x1, y1 int) float64 {
	sum, n := 0.0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			sum += c.At(x, y).Luma()
			n++
		}
	}
	return sum / float64(n)
}

func TestFrenchInteriorLoads(t *testing.T) {
	w := realWorld(t, Options{})
	pw, ph := w.scene.Size()
	if pw != 104 || ph != 84 {
		t.Fatalf("the French interior should be 104x84 pixels, got %dx%d", pw, ph)
	}
	for _, spot := range []string{"sill", "shelf_top", "sideboard", "cushion", "table", "floor"} {
		sp, ok := w.scene.Spots()[spot]
		if !ok {
			t.Errorf("cat spot %q is missing", spot)
			continue
		}
		if sp.X < 0 || sp.Y < 0 || sp.X >= pw || sp.Y >= ph {
			t.Errorf("cat spot %q at %d,%d is outside the room", spot, sp.X, sp.Y)
		}
	}
	if len(w.scene.room.panes) != 6 {
		t.Errorf("the window should have 6 panes, found %d", len(w.scene.room.panes))
	}
}

func TestEveryHourAndWeatherRenders(t *testing.T) {
	w := realWorld(t, Options{})
	for _, sky := range w.wcfg.Kinds() {
		for h := 0; h < 24; h += 3 {
			c := w.render(t, at(h, 30), sky, 0.5)
			for y := 0; y < c.PixelH(); y++ {
				for x := 0; x < c.W; x++ {
					p := c.At(x, y)
					if math.IsNaN(p.R+p.G+p.B) || p.R < 0 || p.R > 1 || p.G < 0 || p.G > 1 || p.B < 0 || p.B > 1 {
						t.Fatalf("%s at %02d:30: bad color %v at %d,%d", sky, h, p, x, y)
					}
				}
			}
		}
	}
}

func TestNightIsDarkerAndLampsGlow(t *testing.T) {
	w := realWorld(t, Options{})
	day := w.render(t, at(13, 0), "clear", 1)
	night := w.render(t, at(1, 0), "clear", 1)
	pw, ph := w.scene.Size()
	if d, n := avgLuma(day, 0, 0, pw, ph), avgLuma(night, 0, 0, pw, ph); n > d*0.8 {
		t.Errorf("1am (%.2f) should be clearly darker than 1pm (%.2f)", n, d)
	}
	// At night the wall by the left lamp is warmer and brighter than the far
	// end of the wainscot; at midday the lampshade isn't glowing.
	var lamp *light
	for _, l := range w.scene.room.lights {
		if l.name == "sconce_left" {
			lamp = l
		}
	}
	if lamp == nil {
		t.Fatal("the French interior has no sconce_left")
	}
	lx, ly := int(lamp.x), int(lamp.y)
	nearLamp := night.At(lx, ly+10) // the wall just below the left lamp
	farWall := night.At(72, 62)     // the right wainscot panel, far from every lamp
	if nearLamp.Luma() <= farWall.Luma() || nearLamp.R-nearLamp.B < 0.15 {
		t.Errorf("lamp light missing at night: near %v, far %v", nearLamp, farWall)
	}
	shadeDay, shadeNight := day.At(lx, ly), night.At(lx, ly) // the lampshade itself
	if !(shadeNight.R > 0.85 && shadeNight.R-shadeNight.B > shadeDay.R-shadeDay.B) {
		t.Errorf("lampshade should glow warm at night: day %v night %v", shadeDay, shadeNight)
	}
}

func TestWindowShowsTheWeather(t *testing.T) {
	w := realWorld(t, Options{})
	g := w.scene.room.glass
	clear := w.render(t, at(13, 0), "clear", 2)
	rainy := w.render(t, at(13, 0), "rain", 2)
	if avgLuma(rainy, g.x, g.y, g.x+g.w, g.y+g.h/2) >= avgLuma(clear, g.x, g.y, g.x+g.w, g.y+g.h/2) {
		t.Error("a rainy sky should be duller than a clear one")
	}
	snowy := realWorld(t, Options{}).render(t, time.Date(2026, 12, 20, 13, 0, 0, 0, time.UTC), "snow", 2)
	bottom := func(c *canvas.Canvas) float64 { return avgLuma(c, g.x, g.y+g.h-12, g.x+g.w, g.y+g.h-6) }
	if bottom(snowy) <= bottom(clear) {
		t.Error("snow should whiten the rooftops")
	}
}

func TestEiffelLightsFollowTheEvening(t *testing.T) {
	w := realWorld(t, Options{})
	v := w.scene.view
	lit := w.sched.At(at(21, 30))
	if v.eiffelLit(env{minute: 21*60 + 30}, lit) < 0.99 {
		t.Error("the tower should be lit at 21:30")
	}
	if v.eiffelLit(env{minute: 2 * 60}, w.sched.At(at(2, 0))) != 0 {
		t.Error("the tower's lights should be off at 02:00")
	}
	if v.eiffelLit(env{minute: 13 * 60}, w.sched.At(at(13, 0))) != 0 {
		t.Error("the tower's lights should be off at midday")
	}
	if !v.sparkling(env{minute: 22*60 + 2}) || v.sparkling(env{minute: 22*60 + 10}) || v.sparkling(env{minute: 23*60 + 30}) {
		t.Error("the tower should sparkle for the first five minutes of evening hours, until 23:00")
	}
}

func TestReducedMotionHoldsStill(t *testing.T) {
	calm := realWorld(t, Options{ReducedMotion: true})
	a := calm.render(t, at(22, 0), "clear", 1).At(17, 12)
	b := calm.render(t, at(22, 0), "clear", 1).At(17, 12)
	if a != b {
		t.Error("with reduced motion the lamps must not flicker")
	}
	busy := realWorld(t, Options{})
	busy.render(t, at(13, 0), "heavy_rain", 1)
	calm.render(t, at(13, 0), "heavy_rain", 1)
	if len(calm.scene.view.rain) >= len(busy.scene.view.rain) {
		t.Errorf("reduced motion should have sparser rain (%d vs %d)", len(calm.scene.view.rain), len(busy.scene.view.rain))
	}
}

func TestDropsStayOnTheGlass(t *testing.T) {
	w := realWorld(t, Options{})
	w.render(t, at(13, 0), "heavy_rain", 60)
	if len(w.scene.drops) == 0 {
		t.Fatal("heavy rain should leave drops on the glass")
	}
	for _, d := range w.scene.drops {
		p := w.scene.room.panes[d.pane]
		if d.x < float64(p.x) || d.x >= float64(p.x+p.w) || d.y < float64(p.y) || d.y > float64(p.y+p.h) {
			t.Fatalf("a drop wandered off its pane: %+v in %+v", d, p)
		}
	}
	w.render(t, at(13, 0), "clear", 120)
	if len(w.scene.drops) != 0 {
		t.Errorf("drops should dry up once the rain stops, %d left", len(w.scene.drops))
	}
}

func Test256ColorsAvoidHarshEntries(t *testing.T) {
	w := realWorld(t, Options{})
	for _, h := range []int{7, 13, 19, 1} {
		c := w.render(t, at(h, 0), "rain", 0.5)
		for _, line := range c.Lines(canvas.Color256) {
			if strings.Contains(line, ";2;") {
				t.Fatal("true color code in 256-color output")
			}
		}
	}
}

func TestCupAndSteam(t *testing.T) {
	w := realWorld(t, Options{})
	if err := w.scene.PutCup(&Cup{Sprite: "no_such_cup"}); err == nil {
		t.Error("unknown cup sprite should be refused")
	}
	if err := w.scene.PutCup(nil); err != nil {
		t.Error(err)
	}
	w.render(t, at(9, 0), "clear", 3)
	if len(w.scene.steam) != 0 {
		t.Error("no cup, no steam")
	}
	cup := &Cup{Sprite: "cup_tall_glass", Steam: [2]int{4, 0}, Body: rgb.MustHex("#ff0000"), Top: rgb.MustHex("#00ff00"), Level: 1}
	if err := w.scene.PutCup(cup); err != nil {
		t.Fatal(err)
	}
	stand := *w.scene.room.cup.Stand
	if got := w.scene.cupAt[1] + w.scene.cup.H - 1; got != stand[1] || w.scene.cupAt[0] != stand[0] {
		t.Errorf("cups should stand on the interior's spot %v, got top-left %v", stand, w.scene.cupAt)
	}
	w.scene.SetSteam(1)
	w.render(t, at(9, 0), "clear", 3)
	if len(w.scene.steam) == 0 {
		t.Error("a hot cup should steam")
	}
}

func TestCupEmptiesFromTheTop(t *testing.T) {
	w := realWorld(t, Options{})
	body, top := rgb.MustHex("#ff0000"), rgb.MustHex("#00ff00")
	if err := w.scene.PutCup(&Cup{Sprite: "cup_tall_glass", Body: body, Top: top, Level: 1}); err != nil {
		t.Fatal(err)
	}
	rows := w.scene.liquid.rows
	if len(rows) < 4 {
		t.Fatalf("the tall glass should show several rows of drink, got %d", len(rows))
	}
	empty := rgb.MustHex("#123456")
	count := func(level float64) (filled int, topRow int) {
		w.scene.SetLevel(level)
		topRow = -1
		for _, y := range rows {
			switch w.scene.drinkColor(y, empty) {
			case top:
				filled++
				topRow = y
			case body:
				filled++
			}
		}
		return
	}
	full, top1 := count(1)
	half, top2 := count(0.5)
	none, _ := count(0)
	if full != len(rows) || top1 != rows[0] {
		t.Errorf("a full cup is full to the top: %d of %d rows, top at %d", full, len(rows), top1)
	}
	if half <= 0 || half >= full || top2 <= top1 {
		t.Errorf("half a cup should be lower: %d rows, top at %d", half, top2)
	}
	if none != 0 {
		t.Errorf("an empty cup has no drink left, got %d rows", none)
	}
	if got := w.scene.drinkColor(rows[len(rows)-1], empty); got == empty {
		t.Error("an empty cup keeps a faint ring at the bottom")
	}
}

// --- Golden frames ------------------------------------------------------------
//
// A tiny test café (testdata/content) is drawn at fixed moments and compared
// with saved frames, so changes to the drawing code are noticed. Editing the
// real content never affects these. If a change is intended, regenerate them:
//
//	go test ./internal/scene -update
//
// Small differences (a few steps per channel) are allowed, because Mac and
// Linux processors round floating-point math slightly differently.

func frameText(c *canvas.Canvas) string {
	var b strings.Builder
	for y := 0; y < c.PixelH(); y++ {
		for x := 0; x < c.W; x++ {
			if x > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strings.TrimPrefix(c.At(x, y).String(), "#"))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func compareFrames(want, got string) error {
	wl, gl := strings.Split(strings.TrimSpace(want), "\n"), strings.Split(strings.TrimSpace(got), "\n")
	if len(wl) != len(gl) {
		return fmt.Errorf("frame height %d, want %d", len(gl), len(wl))
	}
	bad := 0
	first := ""
	for y := range wl {
		wp, gp := strings.Fields(wl[y]), strings.Fields(gl[y])
		if len(wp) != len(gp) {
			return fmt.Errorf("row %d width %d, want %d", y, len(gp), len(wp))
		}
		for x := range wp {
			a, _ := strconv.ParseUint(wp[x], 16, 32)
			b, _ := strconv.ParseUint(gp[x], 16, 32)
			for shift := 0; shift <= 16; shift += 8 {
				d := int(a>>shift&0xff) - int(b>>shift&0xff)
				if d > 3 || d < -3 {
					bad++
					if first == "" {
						first = fmt.Sprintf("pixel %d,%d is #%s, want #%s", x, y, gp[x], wp[x])
					}
					break
				}
			}
		}
	}
	// A couple of stray pixels may flip on another processor (a value landing
	// exactly on a threshold); a real change to the drawing moves many more.
	if bad > 3 {
		return fmt.Errorf("%d pixels differ; first: %s", bad, first)
	}
	return nil
}

func TestGoldenFrames(t *testing.T) {
	cases := []struct {
		name    string
		when    time.Time
		sky     string
		seconds float64
	}{
		{"afternoon_clear", at(13, 0), "clear", 1},
		{"dawn_fog", at(7, 0), "fog", 1},
		{"evening_rain", at(19, 0), "rain", 3},
		{"night_clear", at(1, 0), "clear", 1},
		{"winter_snow", time.Date(2027, 1, 10, 22, 0, 0, 0, time.UTC), "snow", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := fixtureWorld(t)
			got := frameText(w.render(t, tc.when, tc.sky, tc.seconds))
			path := filepath.Join("testdata", "golden", tc.name+".txt")
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run `go test ./internal/scene -update` to create it)", err)
			}
			if err := compareFrames(string(want), got); err != nil {
				t.Errorf("%s: %v\nIf this change is intended, run: go test ./internal/scene -update", tc.name, err)
			}
		})
	}
}

func TestGoldenFramesDiffer(t *testing.T) {
	// Guard against a golden test that passes because everything is blank.
	w := fixtureWorld(t)
	a := frameText(w.render(t, at(13, 0), "clear", 1))
	b := frameText(fixtureWorld(t).render(t, at(1, 0), "clear", 1))
	if a == b {
		t.Fatal("day and night frames are identical")
	}
}
