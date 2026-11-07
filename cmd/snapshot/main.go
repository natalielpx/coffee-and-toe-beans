// Command snapshot draws one still frame of the game to the terminal, without
// starting it. It's a tool for checking the art; try
//
//	go run ./cmd/snapshot --time 19:00 --weather rain
//	go run ./cmd/snapshot --screen title
//	go run ./cmd/snapshot --drink chai_latte --sipped 9
//	go run ./cmd/snapshot --screen drinks --time 22:00
//	go run ./cmd/snapshot --cat brie:sit@sill --cat mimi:sleep@cushion/breathe
//	go run ./cmd/snapshot --cat brie:sit@table --bond brie=friend --moment brie_table
//	go run ./cmd/snapshot --cat mimi:loaf@shelf_top --look mimi   (Look around, on Mimi)
//	go run ./cmd/snapshot --screen poses    (each cat in each pose)
//	go run ./cmd/snapshot --screen sheet    (every cat in every pose and frame)
//	go run ./cmd/snapshot --screen talk --away 3d --bond hunter=your_cat   (coming back after 3 days)
//	go run ./cmd/snapshot --screen talk --away 3d --page 2                 (...the next page of it)
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/game"
)

type options struct {
	clock, date, sky, colors, screen, drink string
	moment, look, away                      string
	page                                    int
	width, height                           int
	seconds, sipped                         float64
	cats, bonds                             catList
}

// catList collects --cat flags: ID:POSE@SPOT, optionally /FRAME.
type catList []string

func (c *catList) String() string     { return strings.Join(*c, " ") }
func (c *catList) Set(s string) error { *c = append(*c, s); return nil }

// parseCat reads "brie:sit@sill" or "brie:sit@sill/blink".
func parseCat(s string) (id, pose, spot, frame string, err error) {
	main, frame, _ := strings.Cut(s, "/")
	who, spot, ok := strings.Cut(main, "@")
	if ok {
		id, pose, ok = strings.Cut(who, ":")
	}
	if !ok || id == "" || pose == "" || spot == "" {
		return "", "", "", "", fmt.Errorf("--cat should look like brie:sit@sill (or brie:sit@sill/blink), got %q", s)
	}
	return id, pose, spot, frame, nil
}

func main() {
	var o options
	flag.StringVar(&o.clock, "time", "", "time of day as HH:MM (default: now)")
	flag.StringVar(&o.date, "date", "", "date as YYYY-MM-DD (default: today)")
	flag.StringVar(&o.sky, "weather", "", "weather to show (default: today's ambient weather)")
	flag.StringVar(&o.colors, "color", "auto", "auto, truecolor or 256")
	flag.StringVar(&o.screen, "screen", "cafe", "title, cafe, menu (the pause menu), drinks, talk, poses (each cat's poses) or sheet (every pose and frame)")
	flag.Var(&o.cats, "cat", "put a cat somewhere: ID:POSE@SPOT or ID:POSE@SPOT/FRAME, e.g. brie:sit@sill (repeatable)")
	flag.Var(&o.bonds, "bond", "how well a cat knows you: ID=TIER or ID=NUMBER (0..100), e.g. mimi=friend (repeatable)")
	flag.StringVar(&o.moment, "moment", "", "show this cat moment in the dialogue box, e.g. brie_sigh")
	flag.StringVar(&o.look, "look", "", "open Look around on this: window, table or a cat's id (e.g. brie)")
	flag.StringVar(&o.drink, "drink", "", "put this drink on the table, e.g. cafe_creme")
	flag.StringVar(&o.away, "away", "", "pretend your last visit was this long ago (45m, 3h, 2d, 1w): shows the welcome back and the away note")
	flag.IntVar(&o.page, "page", 1, "with --screen talk: which page of the conversation to show")
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
	if o.screen == "sheet" || o.screen == "poses" {
		return sheet(o, o.screen == "poses")
	}
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
	var away time.Duration
	if o.away != "" {
		if away, err = game.ParseAway(o.away); err != nil {
			return fmt.Errorf("--away: %v", err)
		}
	}
	g, err := game.New(game.Config{Start: start.Add(-sipped), Weather: o.sky, ColorMode: mode, Seed: 1, Away: away}, content.FS, real)
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
	for _, s := range o.cats {
		id, pose, spot, frame, err := parseCat(s)
		if err != nil {
			return err
		}
		if err := g.PinCat(id, pose, spot, frame); err != nil {
			return err
		}
	}
	for _, s := range o.bonds {
		id, val, ok := strings.Cut(s, "=")
		if !ok {
			return fmt.Errorf("--bond should look like mimi=friend or mimi=30, got %q", s)
		}
		bond, err := strconv.ParseFloat(val, 64)
		if err != nil {
			t, terr := cats.ParseTier(val)
			if terr != nil {
				return fmt.Errorf("--bond %s: %v", s, terr)
			}
			bond = cats.BondFor(t)
		}
		if err := g.SetBond(id, bond); err != nil {
			return err
		}
	}
	if o.moment != "" {
		if err := g.ShowMoment(o.moment); err != nil {
			return err
		}
	}
	real = real.Add(sipped)
	g.Tick(real)
	steps := int(o.seconds * 10)
	for i := 1; i <= steps; i++ {
		g.Tick(real.Add(time.Duration(i) * 100 * time.Millisecond))
	}
	if o.look != "" {
		if err := g.LookAt(o.look); err != nil {
			return err
		}
	}
	switch o.screen {
	case "title", "cafe":
	case "menu":
		g.Key(game.KeyBack)
	case "drinks":
		g.Key(game.KeyBack)    // the pause menu opens on "Order a drink"
		g.Key(game.KeyConfirm) // ...which opens the chalkboard
	case "talk":
		// The hello arrives a few seconds after stepping inside; let it type.
		end := real.Add(time.Duration(steps) * 100 * time.Millisecond)
		tick := 0
		wait := func(n int) {
			for i := 0; i < n; i++ {
				tick++
				g.Tick(end.Add(time.Duration(tick) * 100 * time.Millisecond))
			}
		}
		wait(80)
		for p := 1; p < o.page; p++ {
			g.Key(game.KeyConfirm) // finish typing (if still typing)
			g.Key(game.KeyConfirm) // turn the page
			wait(60)
		}
	default:
		return fmt.Errorf("--screen should be title, cafe, menu, drinks, talk or sheet")
	}
	fmt.Println(g.View())
	return nil
}
