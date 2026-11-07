// Package text loads the words shown on screen from content/text, so they
// can be read and changed without touching any code.
package text

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// UI holds the interface words: title card, menus and notices.
type UI struct {
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle"` // optional line under the title
	Prompt      string   `json:"prompt"`
	Controls    string   `json:"controls"`
	TooSmall    []string `json:"too_small"`
	MenuTitle   string   `json:"menu_title"` // optional line above the pause menu
	MenuLeave   string   `json:"menu_leave"`
	MenuStay    string   `json:"menu_stay"`
	MenuOrder   string   `json:"menu_order"`
	DrinksTitle string   `json:"drinks_title"`
	DrinksHint  string   `json:"drinks_hint"`
	DrinksLater string   `json:"drinks_later"`
	Goodbye     string   `json:"goodbye"`
	Help        []string `json:"help"`
}

// LoadUI reads content/text/ui.json and checks nothing is missing.
func LoadUI(fsys fs.FS) (*UI, error) {
	const name = "text/ui.json"
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var u UI
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, fmt.Errorf("%s: %v", name, err)
	}
	for _, f := range []struct{ key, val string }{
		{"title", u.Title}, {"prompt", u.Prompt},
		{"menu_leave", u.MenuLeave}, {"menu_stay", u.MenuStay}, {"menu_order", u.MenuOrder},
		{"drinks_title", u.DrinksTitle}, {"drinks_later", u.DrinksLater},
	} {
		if strings.TrimSpace(f.val) == "" {
			return nil, fmt.Errorf("%s: %q is missing or empty", name, f.key)
		}
	}
	if len(u.TooSmall) == 0 {
		return nil, fmt.Errorf("%s: \"too_small\" needs at least one line", name)
	}
	return &u, nil
}

// Fill replaces {name} placeholders with values.
func Fill(s string, vals map[string]string) string {
	for k, v := range vals {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}
