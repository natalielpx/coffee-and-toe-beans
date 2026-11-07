package dialogue

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"path"
	"sort"
	"strings"
)

// Text is something to say, written either as one string or as a list of
// strings; with a list, one is picked at random each time, so the café
// doesn't repeat itself word for word.
type Text []string

// UnmarshalJSON accepts "words" or ["words", "other words"].
func (t *Text) UnmarshalJSON(data []byte) error {
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		*t = Text{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return fmt.Errorf("should be a text in quotes, or a list of them in [ ]")
	}
	*t = many
	return nil
}

// Pick chooses one of the versions.
func (t Text) Pick(rng *rand.Rand) string {
	if len(t) == 0 {
		return ""
	}
	if len(t) == 1 || rng == nil {
		return t[0]
	}
	return t[rng.IntN(len(t))]
}

// Conversation is one exchange, as written in a content file.
type Conversation struct {
	ID      string   `json:"-"`
	File    string   `json:"-"`
	Speaker string   `json:"speaker"`
	Pages   []Text   `json:"pages"`
	Choices []Choice `json:"choices"`
	Ignore  *int     `json:"ignore"`
}

// Lines picks a version of every page.
func (c *Conversation) Lines(rng *rand.Rand) []string {
	out := make([]string, len(c.Pages))
	for i, p := range c.Pages {
		out[i] = p.Pick(rng)
	}
	return out
}

// IgnoreChoice is the choice taken when the player doesn't answer.
func (c *Conversation) IgnoreChoice() int {
	if c.Ignore == nil {
		return 0
	}
	return *c.Ignore
}

// Library holds every conversation by name.
type Library struct {
	convs map[string]*Conversation
}

type file struct {
	Conversations map[string]*Conversation `json:"conversations"`
}

// Load reads every .json file in dir.
func Load(fsys fs.FS, dir string) (*Library, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	lib := &Library{convs: map[string]*Conversation{}}
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		if err := lib.add(name, data); err != nil {
			return nil, err
		}
	}
	return lib, nil
}

// Parse reads conversations from one file's contents (handy for tests).
func Parse(name string, data []byte) (*Library, error) {
	lib := &Library{convs: map[string]*Conversation{}}
	if err := lib.add(name, data); err != nil {
		return nil, err
	}
	return lib, nil
}

func (l *Library) add(name string, data []byte) error {
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("%s: %v", name, err)
	}
	for id, c := range f.Conversations {
		if c == nil {
			return fmt.Errorf("%s: conversation %q is empty", name, id)
		}
		if old, dup := l.convs[id]; dup {
			return fmt.Errorf("%s: conversation %q is already in %s", name, id, old.File)
		}
		c.ID, c.File = id, name
		if err := c.check(); err != nil {
			return fmt.Errorf("%s: conversation %q: %v", name, id, err)
		}
		l.convs[id] = c
	}
	return nil
}

func (c *Conversation) check() error {
	if len(c.Pages) == 0 {
		return fmt.Errorf("needs at least one page")
	}
	for i, p := range c.Pages {
		if len(p) == 0 {
			return fmt.Errorf("page %d has no text", i+1)
		}
		for _, v := range p {
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("page %d has an empty text", i+1)
			}
		}
	}
	if n := len(c.Choices); n == 1 || n > 4 {
		return fmt.Errorf("has %d choices; use 2 to 4 (or none)", n)
	}
	for i, ch := range c.Choices {
		if strings.TrimSpace(ch.Text) == "" {
			return fmt.Errorf("choice %d has no text", i+1)
		}
	}
	if len(c.Choices) > 0 {
		if c.Ignore == nil {
			return fmt.Errorf("has choices, so it needs \"ignore\": which choice happens if the player lets it pass (1 for the first)")
		}
		// Written counting from 1, like a person would.
		if *c.Ignore < 1 || *c.Ignore > len(c.Choices) {
			return fmt.Errorf("\"ignore\" is %d, but there are only %d choices", *c.Ignore, len(c.Choices))
		}
		i := *c.Ignore - 1
		c.Ignore = &i
	}
	return nil
}

// Get finds a conversation.
func (l *Library) Get(id string) (*Conversation, bool) {
	c, ok := l.convs[id]
	return c, ok
}

// IDs lists every conversation, sorted.
func (l *Library) IDs() []string {
	var out []string
	for id := range l.convs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Open starts a conversation with placeholders like {drink} filled in.
func (c *Conversation) Open(vals map[string]string, rng *rand.Rand, opts Options) *Box {
	lines := c.Lines(rng)
	for i := range lines {
		lines[i] = fill(lines[i], vals)
	}
	choices := make([]Choice, len(c.Choices))
	for i, ch := range c.Choices {
		choices[i] = Choice{Text: fill(ch.Text, vals), Do: ch.Do}
	}
	return Start(c.Speaker, lines, choices, c.IgnoreChoice(), opts)
}

func fill(s string, vals map[string]string) string {
	for k, v := range vals {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}
