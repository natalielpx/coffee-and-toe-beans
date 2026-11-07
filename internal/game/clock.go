package game

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/timeofday"
)

// FastSpeed is how much faster the game clock runs with --fast: one real
// minute is half an hour in the café, so a whole day passes in 48 minutes.
const FastSpeed = 30

// Clock turns real time into café time. Normally they're the same; --time
// starts the café at another hour, and --fast makes it run quicker.
type Clock struct {
	realStart time.Time
	gameStart time.Time
	speed     float64
}

// NewClock starts a clock at realNow. If start is not zero, café time begins
// there instead of at realNow.
func NewClock(realNow, start time.Time, speed float64) Clock {
	if speed <= 0 {
		speed = 1
	}
	if start.IsZero() {
		start = realNow
	}
	return Clock{realStart: realNow, gameStart: start, speed: speed}
}

// Now is the café time at a real moment.
func (c Clock) Now(real time.Time) time.Time {
	elapsed := real.Sub(c.realStart)
	return c.gameStart.Add(time.Duration(float64(elapsed) * c.speed))
}

// ParseStart reads a --time value like "19:30" as that time today.
func ParseStart(s string, today time.Time) (time.Time, error) {
	m, err := timeofday.ParseClock(s)
	if err != nil {
		return time.Time{}, err
	}
	y, mo, d := today.Date()
	return time.Date(y, mo, d, int(m)/60, int(m)%60, 0, 0, today.Location()), nil
}

// ParseColorMode reads a --color value.
func ParseColorMode(s string) (mode canvas.Mode, auto bool, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return canvas.TrueColor, true, nil
	case "truecolor", "true", "24bit", "24-bit":
		return canvas.TrueColor, false, nil
	case "256":
		return canvas.Color256, false, nil
	}
	return canvas.TrueColor, false, fmt.Errorf("--color should be auto, truecolor or 256 (got %q)", s)
}

// DetectColorMode guesses whether the terminal shows true color, from the
// environment variables terminals set. When unsure it picks 256 colors,
// which every modern terminal supports and which still looks right.
func DetectColorMode(getenv func(string) string) canvas.Mode {
	ct := strings.ToLower(getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return canvas.TrueColor
	}
	switch getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "vscode", "ghostty", "Hyper", "Tabby", "rio":
		return canvas.TrueColor
	}
	term := strings.ToLower(getenv("TERM"))
	for _, t := range []string{"truecolor", "24bit", "direct", "kitty", "alacritty", "wezterm", "ghostty", "foot", "contour"} {
		if strings.Contains(term, t) {
			return canvas.TrueColor
		}
	}
	if getenv("KITTY_WINDOW_ID") != "" || getenv("WT_SESSION") != "" {
		return canvas.TrueColor
	}
	return canvas.Color256
}

// ParseAway reads a length of time like 45m, 3h, 2d or 1w.
func ParseAway(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("give a length of time, like 3h or 2d")
	}
	unit := map[byte]time.Duration{'m': time.Minute, 'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour}
	u, ok := unit[s[len(s)-1]]
	if !ok {
		return 0, fmt.Errorf("%q should end in m, h, d or w (minutes, hours, days, weeks), like 3h or 2d", s)
	}
	n, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%q should be a number followed by m, h, d or w, like 3h or 2d", s)
	}
	return time.Duration(n * float64(u)), nil
}
