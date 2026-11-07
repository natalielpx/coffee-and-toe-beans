package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 7, 22, 15, 0, 0, time.UTC)

func sample() *Data {
	d := New()
	d.FirstVisit = now.Add(-48 * time.Hour)
	d.LastVisit = now
	d.Visits = 3
	d.Seconds = 5400
	d.Bonds["brie"] = 47.5
	d.Bonds["mimi"] = 12
	d.Seen["brie_sigh"] = now.Add(-time.Hour)
	d.Orders["cafe_creme"] = 4
	d.Orders["chai_latte"] = 1
	d.Cup = &Cup{Drink: "cafe_creme", Seed: 99, AgeMinutes: 6.5, PausedMinutes: 1}
	d.RecentTalk = []string{"The radiator ticks quietly as it warms up."}
	d.Settings.Quiet = true
	return d
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coffee-and-toe-beans", "save.json")
	want := sample()
	if err := Write(path, want); err != nil {
		t.Fatal(err)
	}
	got, st, backup, err := Load(path, now)
	if err != nil || st != Loaded || backup != "" {
		t.Fatalf("load: status %v, backup %q, err %v", st, backup, err)
	}
	if got.Version != Version || got.Visits != 3 || got.Seconds != 5400 {
		t.Errorf("basics lost: %+v", got)
	}
	if !got.LastVisit.Equal(now) || !got.FirstVisit.Equal(want.FirstVisit) {
		t.Errorf("visit times lost: %v %v", got.FirstVisit, got.LastVisit)
	}
	if got.Bonds["brie"] != 47.5 || got.Bonds["mimi"] != 12 {
		t.Errorf("bonds lost: %v", got.Bonds)
	}
	if !got.Seen["brie_sigh"].Equal(want.Seen["brie_sigh"]) {
		t.Errorf("seen moments lost: %v", got.Seen)
	}
	if got.Favourite(3) != "cafe_creme" {
		t.Errorf("favourite should be café crème, got %q", got.Favourite(3))
	}
	if got.Cup == nil || *got.Cup != *want.Cup {
		t.Errorf("cup lost: %+v", got.Cup)
	}
	if len(got.RecentTalk) != 1 || !got.Settings.Quiet {
		t.Errorf("small talk or settings lost: %+v", got)
	}
}

func TestMissingSaveIsAFirstVisit(t *testing.T) {
	d, st, backup, err := Load(filepath.Join(t.TempDir(), "save.json"), now)
	if err != nil || st != Missing || backup != "" || d.Returning() {
		t.Fatalf("a missing save should be a quiet first visit: %v %q %v %+v", st, backup, err, d)
	}
	if d.Bonds == nil || d.Seen == nil || d.Orders == nil {
		t.Error("a fresh save should be ready to use")
	}
}

func TestBrokenSavesAreKeptAndReplaced(t *testing.T) {
	good, err := os.ReadFile(writeSample(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"cut off mid-write": good[:len(good)/2],
		"empty":             {},
		"not json":          []byte("meow"),
		"no version":        []byte(`{"bonds": {"brie": 10}}`),
		"wrong types":       []byte(`{"version": 1, "bonds": "lots"}`),
	}
	for name, raw := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, "save.json")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		d, st, backup, err := Load(path, now)
		if err != nil || st != Broken {
			t.Errorf("%s: status %v, err %v", name, st, err)
			continue
		}
		if d.Returning() || len(d.Bonds) != 0 {
			t.Errorf("%s: should start fresh, got %+v", name, d)
		}
		kept, err := os.ReadFile(backup)
		if err != nil || string(kept) != string(raw) {
			t.Errorf("%s: the broken save should be kept as it was at %s", name, backup)
		}
		if !strings.Contains(filepath.Base(backup), "broken") {
			t.Errorf("%s: backup name should say it was broken: %s", name, backup)
		}
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s: the broken save should be moved aside", name)
		}
		// The game can save over it straight away.
		if err := Write(path, d); err != nil {
			t.Errorf("%s: saving afresh: %v", name, err)
		}
	}
}

func writeSample(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "save.json")
	if err := Write(path, sample()); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStrangeValuesAreTamed(t *testing.T) {
	d, err := Parse([]byte(`{"version": 1, "bonds": {"brie": -5, "hunter": 400}, "seconds_in_cafe": -1}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Bonds["brie"] != 0 || d.Bonds["hunter"] != 100 || d.Seconds != 0 {
		t.Errorf("bonds should stay 0..100 and time 0 or more: %+v", d)
	}
}

func TestWritingIsSafe(t *testing.T) {
	path := writeSample(t)
	// A second save replaces the first in one step and leaves no
	// temporary files behind.
	d := sample()
	d.Bonds["brie"] = 80
	if err := Write(path, d); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("only save.json should be left, found %v", names)
	}
	got, _, _, _ := Load(path, now)
	if got.Bonds["brie"] != 80 {
		t.Errorf("the second save should win: %v", got.Bonds)
	}
	// A temporary file left by a crash mid-write doesn't matter.
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".save-123.tmp"), []byte(`{"vers`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, st, _, _ := Load(path, now); st != Loaded || got.Bonds["brie"] != 80 {
		t.Error("a leftover temporary file shouldn't affect loading")
	}
}

func TestNewerSavesAreBackedUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	raw := []byte(`{"version": 7, "bonds": {"brie": 30}, "something_new": true}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	d, st, backup, err := Load(path, now)
	if err != nil || st != Loaded || d.Bonds["brie"] != 30 {
		t.Fatalf("a newer save should still be read: %v %v %+v", st, err, d)
	}
	if kept, err := os.ReadFile(backup); err != nil || string(kept) != string(raw) {
		t.Error("a newer save should be copied before this version writes over it")
	}
}

func TestOldSavesGetDefaults(t *testing.T) {
	// A version 1 save from before a field existed simply leaves it empty.
	d, err := Parse([]byte(`{"version": 1, "last_visit": "2026-11-07T20:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Returning() || d.Bonds == nil || d.Orders == nil || d.Seen == nil || d.Cup != nil {
		t.Errorf("missing fields should get defaults: %+v", d)
	}
}

func TestFavourite(t *testing.T) {
	d := New()
	if d.Favourite(3) != "" {
		t.Error("no orders, no favourite")
	}
	d.Orders["chai_latte"] = 2
	if d.Favourite(3) != "" {
		t.Error("two orders aren't enough for a favourite")
	}
	d.Orders["chai_latte"] = 3
	if d.Favourite(3) != "chai_latte" {
		t.Error("three chai lattes make it the favourite")
	}
	d.Orders["earl_grey"] = 3
	if d.Favourite(3) != "" {
		t.Error("a tie is no favourite")
	}
	d.Orders["earl_grey"] = 5
	if d.Favourite(3) != "earl_grey" {
		t.Error("the most ordered wins")
	}
}

func TestFresh(t *testing.T) {
	path := writeSample(t)
	backup, err := Fresh(path, now)
	if err != nil || backup == "" {
		t.Fatalf("fresh: %q %v", backup, err)
	}
	if _, st, _, _ := Load(path, now); st != Missing {
		t.Error("after --fresh the next visit is a first visit")
	}
	if d, err := os.ReadFile(backup); err != nil || !strings.Contains(string(d), "brie") {
		t.Error("the old save should be kept")
	}
	if b, err := Fresh(path, now); err != nil || b != "" {
		t.Error("fresh with no save does nothing")
	}
}

func TestPathIsInTheConfigFolder(t *testing.T) {
	p, err := Path()
	if err != nil {
		t.Skip("no config folder here:", err)
	}
	if filepath.Base(p) != "save.json" || filepath.Base(filepath.Dir(p)) != "coffee-and-toe-beans" {
		t.Errorf("save should be coffee-and-toe-beans/save.json, got %s", p)
	}
}
