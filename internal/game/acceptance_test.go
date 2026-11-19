package game

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/save"
)

// Acceptance checks from brief.md that a headless run can cover. The rest
// (how it looks in a real terminal, Mac and Linux builds) needs a real
// terminal and is listed in PROGRESS.md.

// playCafe plays minutes of café time with random arrow keys and Enter, never
// Esc or Ctrl+C, so the guest stays. It fails on a panic or an empty frame.
func playCafe(t *testing.T, g *Game, now time.Time, minutes float64, seed uint64) time.Time {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, 99))
	keys := []Key{KeyUp, KeyDown, KeyLeft, KeyRight, KeyConfirm, KeyConfirm}
	step := 100 * time.Millisecond
	ticks := int(minutes * 60 / float64(FastSpeed) / step.Seconds())
	for i := 0; i < ticks; i++ {
		now = now.Add(step)
		g.Tick(now)
		if i%4 == 0 && g.jrnl == nil && g.look == nil && g.pause == nil {
			g.Key(keys[r.IntN(len(keys))])
		}
		if i%300 == 0 && g.View() == "" {
			t.Fatal("nothing was drawn during play")
		}
	}
	return now
}

// Every comfort setting, through the real Settings screen, with each value
// the arrows reach, and a stretch of café time under each.
func TestEverySettingIsChangedAndPlayedUnder(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Seed: 11})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	g.Key(KeyBack) // Esc menu
	g.Key(KeyDown)
	g.Key(KeyDown)
	g.Key(KeyDown)
	g.Key(KeyConfirm) // Settings
	if g.settings == nil {
		t.Fatal("Settings should open from the Esc menu")
	}
	for item := 0; item < 7; item++ {
		for press := 0; press < 9; press++ {
			g.Key(KeyRight)
			if g.View() == "" {
				t.Fatalf("setting %d: nothing drawn", item)
			}
			if g.Remember().Settings != g.set {
				t.Errorf("setting %d value %d: the save doesn't hold the chosen value", item, press)
			}
			now = playCafe(t, g, now, 0.5, uint64(item*10+press))
		}
		g.Key(KeyDown)
	}
	g.Key(KeyBack) // back to the menu
	g.Key(KeyBack) // back to the café
	now = playCafe(t, g, now, 5, 3)
	if g.Done() {
		t.Error("playing under the settings should not leave the café")
	}
}

// Quit with Ctrl+C, then relaunch three days later. The settings, the
// favourite drink and the bonds come back, and the away note has a gap to
// talk about.
func TestQuitAndRelaunchKeepTheCafe(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Seed: 12})
	g.Resize(FrameW, FrameH)
	g.set.Quiet = true
	g.applySettings(false)
	g.saveSettings() // what the Settings screen does on each change
	g.mem.data.Orders["chai_latte"] = 5
	g.SetBond("brie", 40)
	now = playCafe(t, g, now, 2, 4)
	g.Key(KeyQuit)
	if !g.Done() {
		t.Fatal("Ctrl+C should quit")
	}
	raw, err := json.Marshal(g.Remember())
	if err != nil {
		t.Fatal(err)
	}
	saved, err := save.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	later := now.Add(72 * time.Hour)
	g2, err := New(Config{Fast: true, Save: saved, Seed: 13}, content.FS, later)
	if err != nil {
		t.Fatal(err)
	}
	g2.Resize(FrameW, FrameH)
	if !g2.set.Quiet {
		t.Error("the quiet setting should come back")
	}
	if fav := g2.favourite(); fav == nil || fav.ID != "chai_latte" {
		t.Errorf("the favourite drink should come back, got %v", fav)
	}
	if got := g2.cat.world.Cat("brie").Bond; got < 39 || got > 41 {
		t.Errorf("Brie's bond should come back about where it was, got %v", got)
	}
	if g2.Away() < 70*time.Hour {
		t.Errorf("the away note should know about three days, got %v", g2.Away())
	}
	g2.Tick(later.Add(time.Second))
	if g2.View() == "" {
		t.Error("nothing drawn after relaunch")
	}
}

// Leaving with the Esc menu: Esc, then Leave, then Enter.
func TestLeavingFromTheEscMenu(t *testing.T) {
	g, now := inside(t, Config{Fast: true, Seed: 14})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	g.Key(KeyBack)
	for i := 0; i < 4; i++ {
		g.Key(KeyDown) // Order, Look around, Read the journal, Settings, Leave
	}
	g.Key(KeyConfirm)
	if g.phase != phaseLeaving {
		t.Fatal("Leave should start leaving")
	}
	run(g, now, 3)
	if !g.Done() {
		t.Fatal("the café should close after leaving")
	}
}
