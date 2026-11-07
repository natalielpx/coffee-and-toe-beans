// Package cats is the café's cats: who they are (read from content/cats),
// where they are, what pose they're in, how they feel about you, and when
// they have a moment to share. It uses only the Go standard library; the
// game draws the cats through the scene and shows their moments in the
// dialogue box.
package cats

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// The poses every cat has. Each pose is a set of sprites (frames).
const (
	PoseSit   = "sit"   // sitting up, awake
	PoseStand = "stand" // standing: just arrived, or about to go
	PoseLoaf  = "loaf"  // lying down with paws tucked, awake
	PoseSleep = "sleep" // curled up asleep
)

// Poses lists them in a fixed order.
var Poses = []string{PoseSit, PoseStand, PoseLoaf, PoseSleep}

// The frames a pose can have. Only "still" is required; the others are
// little movements shown now and then.
const (
	FrameStill   = "still"
	FrameBlink   = "blink"   // eyes closed for a moment
	FrameEar     = "ear"     // an ear twitch
	FrameTail    = "tail"    // a tail flick
	FrameBreathe = "breathe" // asleep: breathing out
)

var frameNames = map[string]bool{FrameStill: true, FrameBlink: true, FrameEar: true, FrameTail: true, FrameBreathe: true}

// Pose is one way a cat can be: its frames and where its middle is.
type Pose struct {
	// Anchor is the pixel of the sprites that sits on a spot: under the
	// middle of the cat's body, on the bottom row.
	Anchor [2]int `json:"anchor"`
	// Frames names a sprite for each frame ("still", "blink"...).
	Frames map[string]string `json:"frames"`
}

// Sprite is the sprite for a frame, or the still frame if the pose
// doesn't have that one.
func (p Pose) Sprite(frame string) string {
	if s, ok := p.Frames[frame]; ok {
		return s
	}
	return p.Frames[FrameStill]
}

// Has says whether the pose has a frame.
func (p Pose) Has(frame string) bool {
	_, ok := p.Frames[frame]
	return ok
}

// Def is a cat as described in its content file.
type Def struct {
	ID    string          `json:"-"` // from the file name: brie.json is "brie"
	File  string          `json:"-"`
	Name  string          `json:"name"`
	Poses map[string]Pose `json:"poses"`
	Ways  Ways            `json:"ways"`
}

// LoadDefs reads every cat in dir (one .json file per cat), sorted by id.
func LoadDefs(fsys fs.FS, dir string) ([]*Def, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var out []*Def
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		d, err := ParseDef(name, data)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: there are no cats (one .json file per cat)", dir)
	}
	return out, nil
}

// ParseDef reads one cat's file.
func ParseDef(name string, data []byte) (*Def, error) {
	d := &Def{Ways: DefaultWays()}
	if err := json.Unmarshal(data, d); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	d.File = name
	d.ID = strings.TrimSuffix(path.Base(name), ".json")
	if strings.TrimSpace(d.Name) == "" {
		return nil, fmt.Errorf("%s: the cat needs a \"name\"", name)
	}
	for _, p := range Poses {
		pose, ok := d.Poses[p]
		if !ok {
			return nil, fmt.Errorf("%s: the pose %q is missing (every cat needs %s)", name, p, strings.Join(Poses, ", "))
		}
		if pose.Frames[FrameStill] == "" {
			return nil, fmt.Errorf("%s: pose %q needs at least a \"still\" frame", name, p)
		}
		for f := range pose.Frames {
			if !frameNames[f] {
				return nil, fmt.Errorf("%s: pose %q has a frame called %q; frames can be still, blink, ear, tail or breathe", name, p, f)
			}
		}
	}
	for p := range d.Poses {
		found := false
		for _, known := range Poses {
			found = found || p == known
		}
		if !found {
			return nil, fmt.Errorf("%s: unknown pose %q (poses are %s)", name, p, strings.Join(Poses, ", "))
		}
	}
	if err := d.Ways.check(); err != nil {
		return nil, fmt.Errorf("%s: ways: %v", name, err)
	}
	return d, nil
}

// SpriteNames lists every sprite the cat uses, for checking they exist.
func (d *Def) SpriteNames() []string {
	var out []string
	for _, p := range Poses {
		var frames []string
		for f := range d.Poses[p].Frames {
			frames = append(frames, f)
		}
		sort.Strings(frames)
		for _, f := range frames {
			out = append(out, d.Poses[p].Frames[f])
		}
	}
	return out
}
