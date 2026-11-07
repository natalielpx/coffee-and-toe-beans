// Package save remembers a guest between visits: how well each cat knows
// you, the moments you've seen, your last visit, the drinks you order, the
// time you've spent in the café and (from milestone 5) your settings.
//
// The file is plain JSON in the user's config folder
// (coffee-and-toe-beans/save.json). It is written safely: first to a
// temporary file next to it, then renamed over the old one, so a save cut
// off half-way never replaces a good one. A save that can't be read is
// moved aside as a backup and the café starts fresh, never crashing.
package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Version is the save format this game writes. Raise it when a field
// changes meaning; new fields can simply be added (old saves leave them
// empty, and the game fills in a default).
const Version = 1

// Data is everything the café remembers.
type Data struct {
	Version int `json:"version"`

	// FirstVisit and LastVisit are real (wall-clock) times. LastVisit is
	// when you last left (or the last autosave, if the game was closed
	// some other way).
	FirstVisit time.Time `json:"first_visit"`
	LastVisit  time.Time `json:"last_visit"`
	Visits     int       `json:"visits"`

	// Seconds is the total real time spent in the café.
	Seconds float64 `json:"seconds_in_cafe"`

	// Bonds is how well each cat knows you (0..100), by cat id.
	Bonds map[string]float64 `json:"bonds"`
	// Seen is when each moment was last shown (café time), by moment id.
	Seen map[string]time.Time `json:"seen_moments"`
	// Orders counts the drinks you've ordered, by drink id.
	Orders map[string]int `json:"orders"`

	// Cup is the drink on your table when you left (nil: none).
	Cup *Cup `json:"cup,omitempty"`

	// RecentTalk are the narrator's last small-talk lines, so a new visit
	// doesn't start by repeating them.
	RecentTalk []string `json:"recent_small_talk,omitempty"`

	// Settings are the comfort settings (the settings screen arrives in
	// milestone 5).
	Settings Settings `json:"settings"`
}

// Cup is the drink left on the table, kept as ages rather than clock
// times, so it means the same whatever the café's clock says.
type Cup struct {
	Drink string `json:"drink"`
	Seed  uint64 `json:"seed"`
	// Minutes since it arrived, and minutes of that it sat untouched (the
	// sipping clock was stopped), at the moment you left.
	AgeMinutes    float64 `json:"age_minutes"`
	PausedMinutes float64 `json:"paused_minutes"`
}

// Settings are the player's comfort settings. Zero values are the
// defaults, so old saves without them still mean "as usual".
type Settings struct {
	ReducedMotion bool   `json:"reduced_motion,omitempty"`
	Quiet         bool   `json:"quiet,omitempty"`
	Color         string `json:"color,omitempty"` // "", "truecolor" or "256"
	TextSpeed     string `json:"text_speed,omitempty"`
	Pacing        string `json:"pacing,omitempty"`
}

// New is an empty save: a guest who has never been here.
func New() *Data {
	d := &Data{Version: Version}
	d.fill()
	return d
}

func (d *Data) fill() {
	if d.Bonds == nil {
		d.Bonds = map[string]float64{}
	}
	if d.Seen == nil {
		d.Seen = map[string]time.Time{}
	}
	if d.Orders == nil {
		d.Orders = map[string]int{}
	}
}

// Returning is true once you've been to the café before.
func (d *Data) Returning() bool { return !d.LastVisit.IsZero() }

// Favourite is the drink you order most: at least min times and more than
// any other ("" if there isn't one yet).
func (d *Data) Favourite(min int) string {
	var ids []string
	for id := range d.Orders {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	best, n, tie := "", 0, false
	for _, id := range ids {
		switch c := d.Orders[id]; {
		case c > n:
			best, n, tie = id, c, false
		case c == n:
			tie = true
		}
	}
	if tie || n < min {
		return ""
	}
	return best
}

// Path is where the save lives: the user's config folder, e.g.
// ~/Library/Application Support/coffee-and-toe-beans/save.json on a Mac
// and ~/.config/coffee-and-toe-beans/save.json on Linux.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "coffee-and-toe-beans", "save.json"), nil
}

// Status says how loading went.
type Status int

// The ways loading can go.
const (
	Loaded  Status = iota // the save was read
	Missing               // there was no save: a first visit
	Broken                // the save couldn't be read: it was backed up, starting fresh
)

// Load reads the save at path. It never fails the game: a missing save is
// a first visit, and a broken one is moved aside (its new name is
// returned as backup) and replaced by a fresh start. err is only set for
// trouble worth telling the player about after they leave, like a folder
// that can't be read; the returned Data is always usable.
func Load(path string, now time.Time) (d *Data, st Status, backup string, err error) {
	raw, rerr := os.ReadFile(path)
	if errors.Is(rerr, fs.ErrNotExist) {
		return New(), Missing, "", nil
	}
	if rerr != nil {
		return New(), Broken, "", rerr
	}
	d, perr := Parse(raw)
	if perr == nil {
		if d.Version > Version {
			// Written by a newer game: keep a copy before this one writes
			// over it without the fields it doesn't know.
			backup = Sibling(path, "newer", now)
			if cerr := os.WriteFile(backup, raw, 0o644); cerr != nil {
				backup = ""
			}
		}
		return d, Loaded, backup, nil
	}
	backup = Sibling(path, "broken", now)
	if mverr := os.Rename(path, backup); mverr != nil {
		return New(), Broken, "", fmt.Errorf("the save at %s couldn't be read (%v) or moved aside (%v)", path, perr, mverr)
	}
	return New(), Broken, backup, nil
}

// Parse reads a save from its contents.
func Parse(raw []byte) (*Data, error) {
	var d Data
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	if d.Version <= 0 {
		return nil, fmt.Errorf("not a save file (no version)")
	}
	d.fill()
	for id, b := range d.Bonds {
		if b != b || b < 0 { // NaN or below zero
			d.Bonds[id] = 0
		} else if b > 100 {
			d.Bonds[id] = 100
		}
	}
	if d.Seconds < 0 || d.Seconds != d.Seconds {
		d.Seconds = 0
	}
	return &d, nil
}

// Sibling is a name next to path for a copy of it: save.broken-20261107-221500.json.
func Sibling(path, why string, now time.Time) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	name := fmt.Sprintf("%s.%s-%s%s", base, why, now.Format("20060102-150405"), ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(name); errors.Is(err, fs.ErrNotExist) {
			return name
		}
		name = fmt.Sprintf("%s.%s-%s-%d%s", base, why, now.Format("20060102-150405"), i, ext)
	}
}

// Write saves d at path safely: to a temporary file in the same folder,
// flushed to disk, then renamed over the old save in one step.
func Write(path string, d *Data) error {
	d.Version = Version
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".save-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // gone after the rename; cleans up if anything failed
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Fresh moves the save at path aside (keeping it as a backup) so the next
// visit is a first visit. It returns the backup's name ("" if there was no
// save).
func Fresh(path string, now time.Time) (string, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	backup := Sibling(path, "before-fresh", now)
	return backup, os.Rename(path, backup)
}
