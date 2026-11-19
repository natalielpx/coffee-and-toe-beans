package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Golden frames: the journal's three main views as plain text. If a change
// is meant, run the tests with UPDATE_GOLDEN=1 to rewrite the files, then
// look at them.

func goldenCheck(t *testing.T, name string, g *Game) {
	t.Helper()
	got := visible(g.View())
	path := filepath.Join("testdata", "golden", name+".txt")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden frame %s (run with UPDATE_GOLDEN=1 to make it): %v", path, err)
	}
	if strings.TrimRight(string(want), "\n") != strings.TrimRight(got, "\n") {
		t.Errorf("the frame %s has changed; if that is intended, run with UPDATE_GOLDEN=1", name)
	}
}

func journalScene(t *testing.T) *Game {
	t.Helper()
	g, now := inside(t, Config{Fast: true, Weather: "rain", Seed: 7})
	g.Resize(FrameW, FrameH)
	g.box, g.queue = nil, nil
	g.jrnl = &journalMode{}
	_ = now
	return g
}

func TestGoldenFirstSpread(t *testing.T) {
	g := journalScene(t)
	goldenCheck(t, "journal_first_spread", g)
}

func TestGoldenCatCards(t *testing.T) {
	g := journalScene(t)
	g.jrnl = &journalMode{cards: true}
	goldenCheck(t, "journal_cat_cards", g)
}

func TestGoldenEntrySpread(t *testing.T) {
	g := journalScene(t)
	at := time.Date(2026, 11, 18, 7, 40, 0, 0, time.UTC)
	for _, cat := range []string{"brie", "hunter", "mimi"} {
		text, _ := g.jcontent.Cats[cat].Step("curious")
		g.jState().Record(g.jcontent, cat+":curious", cat, text, at, "clear", "", g.rng)
	}
	bad, _ := g.jcontent.Cats["mimi"].Step("bad")
	g.jState().Record(g.jcontent, "mimi:bad", "mimi", bad, at.Add(3*time.Hour), "rain", "", g.rng)
	g.jState().Record(g.jcontent, "moment:mimi_lap", "mimi", g.jcontent.Moments["mimi_lap"], at.AddDate(0, 0, 3), "heavy_rain", "", g.rng)
	g.Key(KeyRight)
	goldenCheck(t, "journal_entry_spread", g)
}
