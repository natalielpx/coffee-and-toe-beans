package journal

import "strings"

// Card is a cat's card laid out at a given width, for About the cats. Name
// is the first line, in bold, and Age is what follows it on that line (age
// and gender). Body is the sections, each a label, its text, and a blank
// line. Comment is Nala's comment, which the screen puts at the bottom.
type Card struct {
	Name    string
	Age     string
	Body    []Row
	Comment []string
}

// Row is one line of a card's body: a section label, a line of text, or a
// blank line (neither set).
type Row struct {
	Label bool
	Text  string
}

// Lines lays the card out at the given width. Lists have no full stops.
func (l Leaflet) Lines(width int) Card {
	c := Card{Name: l.Name, Age: " - " + l.Age + ", " + l.Gender}
	section := func(label, text string) {
		c.Body = append(c.Body, Row{Label: true, Text: label + ":"})
		for _, line := range Wrap(text, width) {
			c.Body = append(c.Body, Row{Text: line})
		}
		c.Body = append(c.Body, Row{})
	}
	section("Likes", strings.Join(l.Likes, ", "))
	section("Dislikes", strings.Join(l.Dislikes, ", "))
	section("Tips", l.Tips)
	section("Story", l.Story)
	c.Comment = Wrap(l.Comment, width)
	return c
}

// Card returns the leaflet card for a cat, if it has one.
func (c *Content) Card(cat string) (Leaflet, bool) {
	for _, l := range c.Leaflets {
		if l.Cat == cat {
			return l, true
		}
	}
	return Leaflet{}, false
}
