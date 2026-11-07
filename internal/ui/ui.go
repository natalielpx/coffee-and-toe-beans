// Package ui connects the game to the terminal using Bubble Tea. It is kept
// deliberately thin: it turns key presses into game keys, asks the game to
// move forward about ten times a second, and prints whatever the game draws.
// Everything else lives in internal/game.
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"coffee-and-toe-beans/internal/game"
)

// frameInterval is the time between frames: 10 frames a second is plenty
// for rain and steam, and keeps the computer quiet.
const frameInterval = 100 * time.Millisecond

type tickMsg time.Time

type model struct {
	g *game.Game
}

func tick() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.g.Resize(msg.Width, msg.Height)
	case tea.KeyMsg:
		m.g.Key(translate(msg))
		if m.g.Done() {
			return m, tea.Quit
		}
	case tickMsg:
		m.g.Tick(time.Time(msg))
		if m.g.Done() {
			return m, tea.Quit
		}
		return m, tick()
	}
	return m, nil
}

func (m model) View() string {
	return m.g.View()
}

// translate turns a terminal key into one of the game's keys.
func translate(k tea.KeyMsg) game.Key {
	switch k.Type {
	case tea.KeyUp:
		return game.KeyUp
	case tea.KeyDown:
		return game.KeyDown
	case tea.KeyLeft:
		return game.KeyLeft
	case tea.KeyRight:
		return game.KeyRight
	case tea.KeyEnter, tea.KeySpace:
		return game.KeyConfirm
	case tea.KeyEsc:
		return game.KeyBack
	case tea.KeyCtrlC:
		return game.KeyQuit
	}
	if k.String() == " " {
		return game.KeyConfirm
	}
	return game.KeyNone
}

// Run shows the game full-screen until the player leaves the café.
func Run(g *game.Game) error {
	p := tea.NewProgram(model{g: g}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
