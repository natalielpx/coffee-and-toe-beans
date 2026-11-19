// Package journal is the notebook on the café's bookshelf: the player's
// story with the cats, written down as entries, with a note from Nala and a
// leaflet about the cats.
//
// It uses only the standard library, so its rules can be tested without the
// terminal libraries. Every word it shows is in content/journal, and each
// cat's leaflet card is in content/cats/<id>.json.
package journal

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Dir is the folder of journal content inside the content file system.
const Dir = "journal"

// Text is every fixed line in the journal (content/journal/text.json).
type Text struct {
	Menu            string   `json:"menu"`
	BookshelfLook   string   `json:"bookshelf_look"`
	FirstTime       string   `json:"first_time"`
	TookDown        string   `json:"took_down"`
	NoteWritten     string   `json:"note_written"`
	ReadAsk         string   `json:"read_ask"`
	ReadJournal     string   `json:"read_journal"`
	ReadCats        string   `json:"read_cats"`
	BookControls    string   `json:"book_controls"`
	LeafletControls string   `json:"leaflet_controls"`
	LeafletOne      string   `json:"leaflet_controls_one"`
	LeafletTitle    string   `json:"leaflet_title"`
	Note            []string `json:"note"`
}

// CatLines is one cat's milestone entries (content/journal/entries.json).
// The keys match the tiers, plus meet (the first moment) and bad (the first
// choice that lowers the bond).
type CatLines struct {
	Meet     string `json:"meet"`
	Curious  string `json:"curious"`
	Familiar string `json:"familiar"`
	Friend   string `json:"friend"`
	YourCat  string `json:"your_cat"`
	Bad      string `json:"bad"`
}

// Step returns the line for one step: meet, curious, familiar, friend,
// your_cat or bad.
func (c CatLines) Step(step string) (string, bool) {
	var s string
	switch step {
	case "meet":
		s = c.Meet
	case "curious":
		s = c.Curious
	case "familiar":
		s = c.Familiar
	case "friend":
		s = c.Friend
	case "your_cat":
		s = c.YourCat
	case "bad":
		s = c.Bad
	}
	return s, s != ""
}

// Aside is a short extra line that can follow an entry
// (content/journal/asides.json). An empty condition list means any.
type Aside struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Weather []string `json:"weather"`
	When    []string `json:"when"`
	Drink   []string `json:"drink"`
}

// Leaflet is one cat's card in the leaflet (the "leaflet" part of
// content/cats/<id>.json).
type Leaflet struct {
	Cat      string   `json:"-"`
	Name     string   `json:"name"`
	Age      string   `json:"age"`
	Gender   string   `json:"gender"`
	Likes    []string `json:"likes"`
	Dislikes []string `json:"dislikes"`
	Tips     string   `json:"tips"`
	Story    string   `json:"story"`
	Comment  string   `json:"comment"`
}

// Content is everything the journal shows.
type Content struct {
	Text     Text
	Cats     map[string]CatLines // by cat id
	Moments  map[string]string   // journal moment id -> entry text
	Asides   []Aside
	Leaflets []Leaflet // one per cat, in cat id order
}

// Load reads the journal's content and every cat's leaflet card from fsys.
// catIDs are the cats in the game (the names of content/cats/*.json).
func Load(fsys fs.FS) (*Content, error) {
	c := &Content{Cats: map[string]CatLines{}, Moments: map[string]string{}}
	if err := readJSON(fsys, path.Join(Dir, "text.json"), &c.Text); err != nil {
		return nil, err
	}
	var entries map[string]json.RawMessage
	if err := readJSON(fsys, path.Join(Dir, "entries.json"), &entries); err != nil {
		return nil, err
	}
	for id, raw := range entries {
		if id == "_about" {
			continue
		}
		var lines CatLines
		if err := json.Unmarshal(raw, &lines); err != nil {
			return nil, fmt.Errorf("journal/entries.json: %q: %v", id, err)
		}
		c.Cats[id] = lines
	}
	var moments map[string]json.RawMessage
	if err := readJSON(fsys, path.Join(Dir, "moments.json"), &moments); err != nil {
		return nil, err
	}
	for id, raw := range moments {
		if id == "_about" {
			continue
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, fmt.Errorf("journal/moments.json: %q: %v", id, err)
		}
		c.Moments[id] = text
	}
	var asides struct {
		Asides []Aside `json:"asides"`
	}
	if err := readJSON(fsys, path.Join(Dir, "asides.json"), &asides); err != nil {
		return nil, err
	}
	c.Asides = asides.Asides

	files, err := fs.Glob(fsys, "cats/*.json")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, f := range files {
		id := strings.TrimSuffix(path.Base(f), ".json")
		var card struct {
			Leaflet *Leaflet `json:"leaflet"`
		}
		if err := readJSON(fsys, f, &card); err != nil {
			return nil, err
		}
		if card.Leaflet == nil {
			continue
		}
		card.Leaflet.Cat = id
		c.Leaflets = append(c.Leaflets, *card.Leaflet)
	}
	if err := c.check(files); err != nil {
		return nil, err
	}
	return c, nil
}

// check makes sure every cat has all of its entries and every card and
// line has words in it, so a missing line is found here and not in play.
func (c *Content) check(catFiles []string) error {
	for _, f := range catFiles {
		id := strings.TrimSuffix(path.Base(f), ".json")
		lines, ok := c.Cats[id]
		if !ok {
			return fmt.Errorf("journal/entries.json has no entries for the cat %q", id)
		}
		for _, step := range []string{"meet", "curious", "familiar", "friend", "your_cat", "bad"} {
			if _, ok := lines.Step(step); !ok {
				return fmt.Errorf("journal/entries.json: %q has no %q entry", id, step)
			}
		}
	}
	for _, a := range c.Asides {
		if a.ID == "" || a.Text == "" {
			return fmt.Errorf("journal/asides.json: an aside has no id or text")
		}
	}
	if len(c.Text.Note) == 0 || c.Text.FirstTime == "" || c.Text.TookDown == "" {
		return fmt.Errorf("journal/text.json: the note or the narrator lines are missing")
	}
	if c.Text.Menu == "" || c.Text.ReadAsk == "" || c.Text.ReadJournal == "" || c.Text.ReadCats == "" {
		return fmt.Errorf("journal/text.json: the menu or the bookshelf question is missing")
	}
	return nil
}

// CheckMoments returns the journal moments that have no entry text. The game
// calls it with the ids of the moments marked "journal": true.
func (c *Content) CheckMoments(ids []string) error {
	for _, id := range ids {
		if c.Moments[id] == "" {
			return fmt.Errorf("journal/moments.json has no entry for the journal moment %q", id)
		}
	}
	return nil
}

func readJSON(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %v (look for a missing comma or quote)", name, err)
	}
	return nil
}
