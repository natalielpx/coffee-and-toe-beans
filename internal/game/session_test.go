package game

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/save"
)

// A headless play session: the same game the terminal runs, with the keys and
// the clock driven from here. Two café hours at --fast, random keys (never
// choosing Leave on purpose), a quit, and a relaunch from what was saved.
func TestTwoHourFastSessionThenRelaunch(t *testing.T) {
	var stored *save.Data
	store := func(d *save.Data) error {
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		back, err := save.Parse(raw)
		if err != nil {
			return err
		}
		stored = back
		return nil
	}
	real := time.Date(2026, 11, 14, 19, 0, 0, 0, time.Local)
	cfg := Config{Fast: true, Store: store, Seed: 7}
	g, err := New(cfg, content.FS, real)
	if err != nil {
		t.Fatal(err)
	}
	g.Resize(FrameW, FrameH)
	rng := rand.New(rand.NewSource(7))
	keys := []Key{KeyUp, KeyDown, KeyLeft, KeyRight, KeyConfirm, KeyConfirm, KeyConfirm}
	step := 100 * time.Millisecond
	// Two café hours at FastSpeed is 2*3600/30 seconds of real time.
	ticks := int(2 * 3600 / float64(FastSpeed) / step.Seconds())
	ticksRun := 0
	for i := 0; i < ticks && !g.Done(); i++ {
		ticksRun++
		real = real.Add(step)
		g.Tick(real)
		if i%5 == 0 {
			g.Key(keys[rng.Intn(len(keys))])
		}
		if i%400 == 0 && g.View() == "" {
			t.Fatal("nothing was drawn")
		}
		if err := g.SaveError(); err != nil {
			t.Fatalf("saving failed during play: %v", err)
		}
	}
	t.Logf("ticks run: %d of %d, left the café: %v, café time: %v", ticksRun, ticks, g.Done(), g.Now().Sub(time.Date(2026, 11, 14, 19, 0, 0, 0, time.Local)).Round(time.Minute))
	if stored == nil {
		t.Fatal("nothing was saved")
	}
	for id, b := range stored.Bonds {
		if b < 0 || b > 100 {
			t.Errorf("bond for %s out of range: %v", id, b)
		}
	}
	bonds := stored.Bonds
	// Relaunch from the saved data.
	g2, err := New(Config{Fast: true, Save: stored, Store: store, Seed: 8}, content.FS, real)
	if err != nil {
		t.Fatal(err)
	}
	g2.Resize(FrameW, FrameH)
	g2.Tick(real.Add(step))
	if g2.View() == "" {
		t.Fatal("nothing was drawn after relaunch")
	}
	for id, b := range bonds {
		if stored.Bonds[id] != b {
			t.Errorf("bond for %s changed across the relaunch: %v -> %v", id, b, stored.Bonds[id])
		}
	}
}
