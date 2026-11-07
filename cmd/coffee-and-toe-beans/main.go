// Command coffee-and-toe-beans opens Coffee and Toe Beans, a cozy cat café
// on the rooftops of Paris, in your terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/game"
	"coffee-and-toe-beans/internal/text"
	"coffee-and-toe-beans/internal/ui"
)

// version is set when a release is built.
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	fast := flag.Bool("fast", false, "")
	at := flag.String("time", "", "")
	sky := flag.String("weather", "", "")
	colors := flag.String("color", "auto", "")
	showVersion := flag.Bool("version", false, "")
	flag.Usage = func() {
		// The help text lives in content/text/ui.json with the other words.
		if words, err := text.LoadUI(content.FS); err == nil {
			fmt.Fprintln(os.Stderr, strings.Join(words.Help, "\n"))
		}
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("coffee-and-toe-beans", version)
		return 0
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
	mode, auto, err := game.ParseColorMode(*colors)
	if err != nil {
		return fail(err)
	}
	if auto {
		mode = game.DetectColorMode(os.Getenv)
	}

	g, err := game.New(game.Config{
		Start:     start,
		Fast:      *fast,
		Weather:   *sky,
		ColorMode: mode,
	}, content.FS, now)
	if err != nil {
		return fail(err)
	}
	if err := ui.Run(g); err != nil {
		return fail(err)
	}
	if g.Done() {
		fmt.Println(g.Goodbye())
	}
	return 0
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "coffee-and-toe-beans:", err)
	return 1
}
