package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/game"
)

func TestTranslate(t *testing.T) {
	cases := map[game.Key]tea.KeyMsg{
		game.KeyUp:      {Type: tea.KeyUp},
		game.KeyDown:    {Type: tea.KeyDown},
		game.KeyLeft:    {Type: tea.KeyLeft},
		game.KeyRight:   {Type: tea.KeyRight},
		game.KeyConfirm: {Type: tea.KeyEnter},
		game.KeyBack:    {Type: tea.KeyEsc},
		game.KeyQuit:    {Type: tea.KeyCtrlC},
	}
	for want, msg := range cases {
		if got := translate(msg); got != want {
			t.Errorf("%v: got %v want %v", msg, got, want)
		}
	}
	if translate(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}) != game.KeyConfirm {
		t.Error("space should confirm")
	}
	if translate(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}) != game.KeyNone {
		t.Error("q must not do anything (no quitting by accident)")
	}
}

func TestModelFlow(t *testing.T) {
	g, err := game.New(game.Config{}, content.FS, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var m tea.Model = model{g: g}
	if m.Init() == nil {
		t.Fatal("Init should start the frame timer")
	}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.View() == "" {
		t.Fatal("nothing drawn")
	}
	_, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("each frame should schedule the next")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil || !g.Done() {
		t.Fatal("Ctrl+C should quit")
	}
}
