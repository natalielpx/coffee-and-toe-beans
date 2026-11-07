// Command snapshot draws one still frame of the game to the terminal, without
// starting it. It's a tool for checking the art; try
//
//	go run ./cmd/snapshot --time 19:00 --weather rain
//	go run ./cmd/snapshot --screen title
//	go run ./cmd/snapshot --drink chai_latte --sipped 9
//	go run ./cmd/snapshot --screen drinks --time 22:00
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/game"
)

type options struct {
	clock, date, sky, colors, screen, drink string
	width, height                           int
	seconds, sipped                         float64
}

func main() {
	var o options
	flag.StringVar(&o.clock, "time", "", "time of day as HH:MM (default: now)")
	flag.StringVar(&o.date, "date", "", "date as YYYY-MM-DD (default: today)")
	flag.StringVar(&o.sky, "weather", "", "weather to show (default: today's ambient weather)")
	flag.StringVar(&o.colors, "color", "auto", "auto, truecolor or 256")
	flag.StringVar(&o.screen, "screen", "cafe", "title, cafe, menu (the pause menu), drinks or talk")
	flag.StringVar(&o.drink, "drink", "", "put this drink on the table, e.g. cafe_creme")
	flag.Float64Var(&o.sipped, "sipped", 0, "with --drink: minutes since it arrived (cups are empty at 15)")
	flag.IntVar(&o.width, "width", game.FrameW, "terminal width in cells")
	flag.IntVar(&o.height, "height", game.FrameH, "terminal height in cells")
	flag.Float64Var(&o.seconds, "seconds", 4, "seconds of animation to run before the snapshot")
	flag.Parse()

	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	real := time.Now()
	day := real
	if o.date != "" {
		d, err := time.ParseInLocation("2006-01-02", o.date, time.Local)
		if err != nil {
			return fmt.Errorf("--date should look like 2026-11-07")
		}
		day = time.Date(d.Year(), d.Month(), d.Day(), real.Hour(), real.Minute(), real.Second(), 0, time.Local)
	}
	start := day
	if o.clock != "" {
		t, err := game.ParseStart(o.clock, day)
		if err != nil {
			return err
		}
		start = t
	}
	mode, auto, err := game.ParseColorMode(o.colors)
	if err != nil {
		return err
	}
	if auto {
		mode = game.DetectColorMode(os.Getenv)
	}
	// With --sipped, the café opens that many minutes earlier, the drink
	// arrives, and the clock is then moved on to the time asked for.
	sipped := time.Duration(o.sipped * float64(time.Minute))
	g, err := game.New(game.Config{Start: start.Add(-sipped), Weather: o.sky, ColorMode: mode, Seed: 1}, content.FS, real)
	if err != nil {
		return err
	}
	g.Resize(o.width, o.height)
	if o.screen != "title" {
		g.Key(game.KeyConfirm)
		g.Key(game.KeyConfirm)
	}
	if o.drink != "" {
		if err := g.Serve(o.drink); err != nil {
			return err
		}
	}
	real = real.Add(sipped)
	g.Tick(real)
	steps := int(o.seconds * 10)
	for i := 1; i <= steps; i++ {
		g.Tick(real.Add(time.Duration(i) * 100 * time.Millisecond))
	}
	switch o.screen {
	case "title", "cafe":
	case "menu":
		g.Key(game.KeyBack)
	case "drinks":
		g.Key(game.KeyBack)
		for i := 0; i < 2; i++ {
			g.Key(game.KeyDown)
		}
		g.Key(game.KeyConfirm)
	case "talk":
		// The hello arrives a few seconds after stepping inside; let it type.
		end := real.Add(time.Duration(steps) * 100 * time.Millisecond)
		for i := 1; i <= 80; i++ {
			g.Tick(end.Add(time.Duration(i) * 100 * time.Millisecond))
		}
	default:
		return fmt.Errorf("--screen should be title, cafe, menu, drinks or talk")
	}
	fmt.Println(g.View())
	return nil
}
