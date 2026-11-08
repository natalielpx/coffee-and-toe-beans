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
	Title          string   `json:"title"`
	Subtitle       string   `json:"subtitle"` // optional line under the title
	Prompt         string   `json:"prompt"`
	Controls       string   `json:"controls"`
	TooSmall       []string `json:"too_small"`
	MenuTitle      string   `json:"menu_title"` // optional line above the pause menu
	MenuLeave      string   `json:"menu_leave"`
	MenuOrder      string   `json:"menu_order"`
	MenuLook       string   `json:"menu_look"`
	MenuSettings   string   `json:"menu_settings"`
	LookHint       string   `json:"look_hint"`           // Look around, nothing to do
	LookHintChoose string   `json:"look_hint_choose"`    // Look around, with things to do
	LookMarker     string   `json:"look_marker"`         // arrow color, e.g. "#ffffff"
	LookOutline    string   `json:"look_marker_outline"` // its outline color ("" for none)
	DrinksTitle    string   `json:"drinks_title"`
	DrinksHint     string   `json:"drinks_hint"`
	DrinksLater    string   `json:"drinks_later"`
	Goodbye        string   `json:"goodbye"`
	Help           []string `json:"help"`
	// Notes printed after you leave, about the save file.
	SaveBroken string `json:"save_broken"` // {path}: where the unreadable save was kept
	SaveNewer  string `json:"save_newer"`  // {path}: a copy of a save from a newer version
	SaveFailed string `json:"save_failed"` // {error}: what went wrong
	FreshDone  string `json:"fresh_done"`  // {path}: where the old save was kept (--fresh)

	Settings SettingsWords `json:"settings"`
}

// SettingsWords are the words of the Settings screen.
type SettingsWords struct {
	Title         string        `json:"title"`
	Hint          string        `json:"hint"`
	AutoTrueColor string        `json:"auto_truecolor"` // fills {auto} when Auto picked true colour
	Auto256       string        `json:"auto_256"`       // ...or 256 colours
	Items         []SettingWord `json:"items"`
}

// SettingWord is one setting: its label and the names of its values.
type SettingWord struct {
	ID     string      `json:"id"`
	Label  string      `json:"label"`
	Values []ValueWord `json:"values"`
}

// ValueWord is one value of a setting, with a one-line note about it.
type ValueWord struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	About string `json:"about"`
}

// Setting finds a setting's words by id.
func (s SettingsWords) Setting(id string) (SettingWord, bool) {
	for _, it := range s.Items {
		if it.ID == id {
			return it, true
		}
	}
	return SettingWord{}, false
}

// Value finds one value's words by id.
func (s SettingWord) Value(id string) (ValueWord, bool) {
	for _, v := range s.Values {
		if v.ID == id {
			return v, true
		}
	}
	return ValueWord{}, false
}

// SettingValues are the settings the game has, with the values each can
// take, in the order they're shown. content/text/ui.json must name them all.
var SettingValues = []struct {
	ID     string
	Values []string
}{
	{"reduced_motion", []string{"off", "on"}},
	{"quiet", []string{"off", "on"}},
	{"text_speed", []string{"slow", "normal", "instant"}},
	{"pacing", []string{"gentle", "very_slow"}},
	{"weather", []string{"ambient", "clear", "cloudy", "drizzle", "rain", "heavy_rain", "fog", "snow"}},
	{"time_of_day", []string{"real", "dawn", "morning", "afternoon", "golden", "night"}},
	{"color", []string{"auto", "truecolor", "256"}},
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
		{"menu_leave", u.MenuLeave}, {"menu_order", u.MenuOrder}, {"menu_look", u.MenuLook},
		{"menu_settings", u.MenuSettings}, {"settings.title", u.Settings.Title},
		{"look_hint", u.LookHint}, {"look_hint_choose", u.LookHintChoose},
		{"drinks_title", u.DrinksTitle}, {"drinks_later", u.DrinksLater},
	} {
		if strings.TrimSpace(f.val) == "" {
			return nil, fmt.Errorf("%s: %q is missing or empty", name, f.key)
		}
	}
	for _, sv := range SettingValues {
		it, ok := u.Settings.Setting(sv.ID)
		if !ok {
			return nil, fmt.Errorf("%s: settings: the setting %q is missing", name, sv.ID)
		}
		if strings.TrimSpace(it.Label) == "" {
			return nil, fmt.Errorf("%s: settings: %q needs a label", name, sv.ID)
		}
		for _, v := range sv.Values {
			vw, ok := it.Value(v)
			if !ok || strings.TrimSpace(vw.Name) == "" {
				return nil, fmt.Errorf("%s: settings: %q needs a value %q with a name", name, sv.ID, v)
			}
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
