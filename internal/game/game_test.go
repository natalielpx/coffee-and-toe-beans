package game

import (
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/canvas"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visible(s string) string { return ansi.ReplaceAllString(s, "") }

var t0 = time.Date(2026, 11, 7, 2, 18, 0, 0, time.Local)

func newGame(t *testing.T, cfg Config) *Game {
	t.Helper()
	g, err := New(cfg, content.FS, t0)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// run ticks the game forward at 10 frames a second.
func run(g *Game, from time.Time, seconds float64) time.Time {
	n := int(seconds * 10)
	for i := 1; i <= n; i++ {
		g.Tick(from.Add(time.Duration(i) * 100 * time.Millisecond))
	}
	return from.Add(time.Duration(n) * 100 * time.Millisecond)
}

func checkSize(t *testing.T, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Fatalf("view has %d lines, want %d", len(lines), h)
	}
	for i, l := range lines {
		if got := canvas.Width(visible(l)); got != w {
			t.Fatalf("line %d is %d cells wide, want %d", i, got, w)
		}
	}
}

func TestViewFillsTheTerminal(t *testing.T) {
	g := newGame(t, Config{})
	for _, size := range [][2]int{{104, 42}, {140, 50}, {105, 43}, {200, 60}, {80, 32}, {103, 42}, {104, 41}, {10, 3}} {
		g.Resize(size[0], size[1])
		checkSize(t, g.View(), size[0], size[1])
	}
	g.Resize(0, 0)
	if g.View() != "" {
		t.Error("a zero-size terminal should get an empty view")
	}
}

func TestTitleCard(t *testing.T) {
	g := newGame(t, Config{})
	// Words float over the picture, so the gaps between them show pixels.
	v := strings.ReplaceAll(visible(g.View()), "▀", " ")
	for _, want := range []string{"C o f f e e   a n d   T o e   B e a n s", "press Enter to step inside"} {
		if !strings.Contains(v, want) {
			t.Errorf("title card is missing %q", want)
		}
	}
}

func TestTooSmallIsPolite(t *testing.T) {
	g := newGame(t, Config{})
	g.Resize(70, 24)
	v := visible(g.View())
	if !strings.Contains(v, "104×42") || !strings.Contains(v, "70×24") {
		t.Errorf("too-small message should say what's needed and what there is:\n%s", v)
	}
	g.Resize(110, 44)
	if strings.Contains(visible(g.View()), "needs a little more room") {
		t.Error("message should go away once the window is big enough")
	}
}

func TestStepInsideAndLeave(t *testing.T) {
	g := newGame(t, Config{})
	now := t0
	g.Key(KeyConfirm)
	if g.phase != phaseEntering {
		t.Fatal("Enter on the title card should step inside")
	}
	now = run(g, now, 2)
	if g.phase != phaseCafe {
		t.Fatal("the fade into the café should finish within 2 seconds")
	}
	if strings.Contains(visible(g.View()), "press Enter") {
		t.Error("title text should be gone inside the café")
	}

	g.Key(KeyBack)
	if g.pause == nil || !strings.Contains(visible(g.View()), "Leave the café") {
		t.Fatal("Esc should open the pause menu")
	}
	g.Key(KeyBack) // Esc again: stay
	if g.pause != nil || g.phase != phaseCafe {
		t.Fatal("Esc should close the menu and stay")
	}

	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyDown)
	g.Key(KeyConfirm) // the last choice is to leave
	if g.phase != phaseLeaving {
		t.Fatal("choosing to leave should start leaving")
	}
	run(g, now, 2)
	if !g.Done() {
		t.Fatal("the game should close after the fade out")
	}
	if g.Goodbye() == "" {
		t.Error("there should be a goodbye line")
	}
}

func TestEscClosesTheMenu(t *testing.T) {
	g := newGame(t, Config{})
	g.Key(KeyConfirm)
	g.Key(KeyConfirm) // skip the fade
	if g.phase != phaseCafe {
		t.Fatal("Enter should skip the fade")
	}
	g.Key(KeyBack)
	g.Key(KeyDown)
	g.Key(KeyBack)
	if g.pause != nil || g.Done() {
		t.Fatal("Esc in the menu should just close it")
	}
	g.Key(KeyUp)
	g.Key(KeyUp)
	if g.pause != nil || g.menu != nil {
		t.Fatal("arrows shouldn't open anything")
	}
}

func TestNoQuittingByAccident(t *testing.T) {
	// Leaving takes Esc, then choosing "Leave the café" (on the title card
	// it's the only choice, so Esc then Enter). Any other pair of keys, or
	// long runs without Esc, never close the game.
	keys := []Key{KeyUp, KeyDown, KeyLeft, KeyRight, KeyConfirm, KeyBack, KeyNone}
	for _, a := range keys {
		for _, b := range keys {
			g := newGame(t, Config{})
			g.Key(a)
			g.Key(b)
			run(g, t0, 3)
			if a == KeyBack && b == KeyConfirm {
				if !g.Done() {
					t.Fatal("Esc then Enter should leave")
				}
				continue
			}
			if g.Done() {
				t.Fatalf("pressing %v then %v closed the game", a, b)
			}
		}
	}
	rng := rand.New(rand.NewPCG(1, 2))
	g := newGame(t, Config{})
	safe := []Key{KeyUp, KeyDown, KeyLeft, KeyRight, KeyConfirm}
	for i := 0; i < 2000; i++ {
		g.Key(safe[rng.IntN(len(safe))])
	}
	if g.Done() || g.phase == phaseLeaving {
		t.Fatal("keys other than Esc must never lead out of the café")
	}
}

func TestCtrlCLeavesAtOnce(t *testing.T) {
	g := newGame(t, Config{})
	g.Key(KeyQuit)
	if !g.Done() {
		t.Fatal("Ctrl+C should close the game right away")
	}
}

func TestClockFlags(t *testing.T) {
	start, err := ParseStart("19:30", t0)
	if err != nil {
		t.Fatal(err)
	}
	g := newGame(t, Config{Start: start})
	if got := g.Now().Format("15:04"); got != "19:30" {
		t.Fatalf("--time 19:30 should start the café at 19:30, got %s", got)
	}
	run(g, t0, 10)
	if got := g.Now().Sub(start); got != 10*time.Second {
		t.Errorf("normal clock should move 10s in 10s, moved %v", got)
	}

	f := newGame(t, Config{Fast: true})
	run(f, t0, 10)
	if got := f.Now().Sub(t0); got != 10*FastSpeed*time.Second {
		t.Errorf("--fast should move %v in 10s, moved %v", 10*FastSpeed*time.Second, got)
	}

	for _, bad := range []string{"7pm", "25:00", "12", ""} {
		if _, err := ParseStart(bad, t0); err == nil {
			t.Errorf("--time %q should be refused", bad)
		}
	}
}

func TestUnknownWeatherIsRefused(t *testing.T) {
	if _, err := New(Config{Weather: "hail"}, content.FS, t0); err == nil {
		t.Fatal("--weather hail should be refused")
	}
	kinds, err := WeatherKinds(content.FS)
	if err != nil || len(kinds) < 5 {
		t.Fatalf("weather kinds: %v %v", kinds, err)
	}
}

func TestColorModes(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	cases := []struct {
		vars map[string]string
		want canvas.Mode
	}{
		{map[string]string{"COLORTERM": "truecolor"}, canvas.TrueColor},
		{map[string]string{"TERM_PROGRAM": "iTerm.app", "TERM": "xterm-256color"}, canvas.TrueColor},
		{map[string]string{"TERM_PROGRAM": "Apple_Terminal", "TERM": "xterm-256color"}, canvas.Color256},
		{map[string]string{"TERM": "xterm-kitty"}, canvas.TrueColor},
		{map[string]string{"TERM": "xterm-256color"}, canvas.Color256},
		{map[string]string{}, canvas.Color256},
	}
	for _, c := range cases {
		if got := DetectColorMode(env(c.vars)); got != c.want {
			t.Errorf("%v: got %v want %v", c.vars, got, c.want)
		}
	}
	for in, want := range map[string]canvas.Mode{"256": canvas.Color256, "truecolor": canvas.TrueColor} {
		m, auto, err := ParseColorMode(in)
		if err != nil || auto || m != want {
			t.Errorf("--color %s: %v %v %v", in, m, auto, err)
		}
	}
	if _, auto, err := ParseColorMode("auto"); err != nil || !auto {
		t.Error("--color auto")
	}
	if _, _, err := ParseColorMode("16"); err == nil {
		t.Error("--color 16 should be refused")
	}
}

func TestLongFastSession(t *testing.T) {
	// A whole café day at --fast speed (48 real minutes), in one-second
	// steps, drawing every few seconds, in both color modes. Nothing may
	// panic and every frame must keep its size.
	if testing.Short() {
		t.Skip("long session")
	}
	for _, mode := range []canvas.Mode{canvas.TrueColor, canvas.Color256} {
		g := newGame(t, Config{Fast: true, ColorMode: mode})
		g.Resize(90, 34)
		g.Key(KeyConfirm)
		now := t0
		for i := 0; i < 48*60; i++ {
			now = now.Add(time.Second)
			g.Tick(now)
			if i%97 == 0 {
				checkSize(t, g.View(), 90, 34)
			}
			if i%600 == 0 {
				g.Key(KeyBack)
				g.Key(KeyBack)
			}
			if i%37 == 0 { // answer whatever is on screen: orders drinks now and then
				g.Key(KeyConfirm)
			}
			if i%53 == 0 {
				g.Key(KeyDown)
				g.Key(KeyRight)
			}
		}
		if g.Done() {
			t.Fatal("the game closed on its own")
		}
		if g.order == nil {
			t.Error("in a whole day, some drink should have been ordered")
		}
	}
}

func TestReducedMotionStillWorks(t *testing.T) {
	g := newGame(t, Config{Reduced: true})
	g.Key(KeyConfirm)
	run(g, t0, 1)
	if g.phase != phaseCafe {
		t.Fatal("with reduced motion the fade should be quicker")
	}
	checkSize(t, g.View(), FrameW, FrameH)
}
