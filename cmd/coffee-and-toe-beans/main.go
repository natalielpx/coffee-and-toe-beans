// Command coffee-and-toe-beans opens Coffee and Toe Beans, a cozy cat café
// on the rooftops of Paris, in your terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/game"
	"coffee-and-toe-beans/internal/save"
	"coffee-and-toe-beans/internal/text"
	"coffee-and-toe-beans/internal/ui"
)

// version is set when a release is built.
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	// Hidden flags, for testing: --fast, --time, --weather, --fresh, --away.
	fast := flag.Bool("fast", false, "")
	at := flag.String("time", "", "")
	sky := flag.String("weather", "", "")
	colors := flag.String("color", "auto", "")
	fresh := flag.Bool("fresh", false, "")
	away := flag.String("away", "", "")
	showVersion := flag.Bool("version", false, "")
	words, wordsErr := text.LoadUI(content.FS)
	flag.Usage = func() {
		// The help text lives in content/text/ui.json with the other words.
		if wordsErr == nil {
			where, _ := savePath(false)
			fmt.Fprintln(os.Stderr, text.Fill(strings.Join(words.Help, "\n"), map[string]string{"save": where}))
		}
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("coffee-and-toe-beans", version)
		return 0
	}
	if wordsErr != nil {
		return fail(wordsErr)
	}

	now := time.Now()
	var start time.Time
	if *at != "" {
		t, err := game.ParseStart(*at, now)
		if err != nil {
			return fail(fmt.Errorf("--time: %v", err))
		}
		start = t
	}
	var pretend time.Duration
	if *away != "" {
		d, err := game.ParseAway(*away)
		if err != nil {
			return fail(fmt.Errorf("--away: %v", err))
		}
		pretend = d
	}
	mode, auto, err := game.ParseColorMode(*colors)
	if err != nil {
		return fail(err)
	}

	// The café's memory. Test runs (--fast or --time) keep a save of their
	// own, so trying things out never changes your real café.
	var notes []string
	path, perr := savePath(*fast || *at != "")
	var data *save.Data
	if perr != nil {
		notes = append(notes, text.Fill(words.SaveFailed, map[string]string{"error": perr.Error()}))
	} else {
		if *fresh {
			kept, err := save.Fresh(path, now)
			if err != nil {
				return fail(err)
			}
			if kept != "" {
				notes = append(notes, text.Fill(words.FreshDone, map[string]string{"path": kept}))
			}
		}
		d, st, backup, lerr := save.Load(path, now)
		data = d
		switch {
		case st == save.Broken && backup != "":
			notes = append(notes, text.Fill(words.SaveBroken, map[string]string{"path": backup}))
		case st == save.Loaded && backup != "":
			notes = append(notes, text.Fill(words.SaveNewer, map[string]string{"path": backup}))
		}
		if lerr != nil {
			notes = append(notes, text.Fill(words.SaveFailed, map[string]string{"error": lerr.Error()}))
		}
	}
	if data == nil {
		data = save.New()
	}
	detected := game.DetectColorMode(os.Getenv)
	if auto {
		mode = detected
		switch data.Settings.Color {
		case "256":
			mode, _, _ = game.ParseColorMode("256")
		case "truecolor":
			mode, _, _ = game.ParseColorMode("truecolor")
		}
	}
	var store func(*save.Data) error
	if path != "" {
		store = func(d *save.Data) error { return save.Write(path, d) }
	}

	g, err := game.New(game.Config{
		Start:     start,
		Fast:      *fast,
		Weather:   *sky,
		ColorMode: mode,
		AutoColor: detected,
		Save:      data, // with the player's settings
		Store:     store,
		Away:      pretend,
	}, content.FS, now)
	if err != nil {
		return fail(err)
	}
	runErr := ui.Run(g)
	if !g.Done() {
		g.Key(game.KeyQuit) // closed some other way: still remember the visit
	}
	if runErr != nil {
		return fail(runErr)
	}
	fmt.Println(g.Goodbye())
	if err := g.SaveError(); err != nil {
		notes = append(notes, text.Fill(words.SaveFailed, map[string]string{"error": err.Error()}))
	}
	for _, n := range notes {
		fmt.Println(n)
	}
	return 0
}

// savePath is where the café's memory lives; test runs use save-test.json
// next to the real one.
func savePath(test bool) (string, error) {
	p, err := save.Path()
	if err != nil {
		return "", err
	}
	if test {
		p = filepath.Join(filepath.Dir(p), "save-test.json")
	}
	return p, nil
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "coffee-and-toe-beans:", err)
	return 1
}
