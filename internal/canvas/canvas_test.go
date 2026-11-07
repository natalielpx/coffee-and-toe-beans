package canvas

import (
	"regexp"
	"strings"
	"testing"

	"coffee-and-toe-beans/internal/rgb"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visible(s string) string { return ansi.ReplaceAllString(s, "") }

func TestLinesHaveExactWidth(t *testing.T) {
	c := New(12, 4)
	for y := 0; y < c.PixelH(); y++ {
		for x := 0; x < c.W; x++ {
			c.Set(x, y, rgb.Color{R: float64(x) / 12, G: float64(y) / 8, B: 0.3})
		}
	}
	c.Text(2, 1, "hello", rgb.White, rgb.Black)
	for _, mode := range []Mode{TrueColor, Color256} {
		lines := c.Lines(mode)
		if len(lines) != 4 {
			t.Fatalf("%v: got %d lines", mode, len(lines))
		}
		for i, l := range lines {
			if w := Width(visible(l)); w != 12 {
				t.Errorf("%v: line %d is %d cells wide: %q", mode, i, w, visible(l))
			}
			if !strings.HasSuffix(l, "\x1b[0m") {
				t.Errorf("%v: line %d does not reset colors", mode, i)
			}
		}
		if !strings.Contains(visible(lines[1]), "hello") {
			t.Errorf("%v: text missing: %q", mode, visible(lines[1]))
		}
	}
}

func TestSameColorPixelsBecomeSpaces(t *testing.T) {
	c := New(3, 1)
	c.Fill(rgb.MustHex("#336699"))
	c.Set(1, 0, rgb.MustHex("#ffffff")) // top pixel of the middle cell
	got := visible(c.String(TrueColor))
	if got != " ▀ " {
		t.Fatalf("got %q", got)
	}
}

func TestColorCodesAreNotRepeated(t *testing.T) {
	c := New(40, 1)
	c.Fill(rgb.MustHex("#336699"))
	line := c.Lines(TrueColor)[0]
	if n := strings.Count(line, "\x1b["); n != 2 { // one color, one reset
		t.Fatalf("expected 2 escape codes for a flat line, got %d: %q", n, line)
	}
}

func TestTrueColorAnd256Codes(t *testing.T) {
	c := New(1, 1)
	c.Set(0, 0, rgb.MustHex("#d75f87"))
	c.Set(0, 1, rgb.MustHex("#5f87af"))
	tc := c.String(TrueColor)
	if !strings.Contains(tc, "38;2;215;95;135") || !strings.Contains(tc, "48;2;95;135;175") {
		t.Fatalf("true color codes wrong: %q", tc)
	}
	c256 := c.String(Color256)
	if strings.Contains(c256, ";2;") || !strings.Contains(c256, "38;5;168") || !strings.Contains(c256, "48;5;67") {
		t.Fatalf("256 color codes wrong: %q", c256)
	}
}

func TestWideTextAndEdges(t *testing.T) {
	c := New(5, 1)
	c.Text(0, 0, "a月b", rgb.White, rgb.Black)
	got := visible(c.String(TrueColor))
	if got != "a月b " {
		t.Fatalf("got %q", got)
	}
	if Width(got) != 5 {
		t.Fatalf("width %d", Width(got))
	}
	c = New(3, 1)
	c.Text(2, 0, "月", rgb.White, rgb.Black) // no room: becomes a space
	if w := Width(visible(c.String(TrueColor))); w != 3 {
		t.Fatalf("width %d", w)
	}
	c.Text(-2, 0, "abcdefg", rgb.White, rgb.Black) // starts off the left edge
	if got := visible(c.String(TrueColor)); got != "cde" {
		t.Fatalf("got %q", got)
	}
}

func TestBlit(t *testing.T) {
	big := New(6, 3)
	big.Fill(rgb.Black)
	small := New(2, 1)
	small.Fill(rgb.White)
	small.Text(0, 0, "x", rgb.Black, rgb.White)
	big.Blit(small, 5, 2) // half off the right edge
	if big.At(5, 4) != rgb.White || big.At(5, 5) != rgb.White {
		t.Fatal("blit pixels missing")
	}
	if !strings.Contains(visible(big.Lines(TrueColor)[2]), "x") {
		t.Fatal("blit text missing")
	}
}

func TestOutsideIsIgnored(t *testing.T) {
	c := New(2, 1)
	c.Set(-1, 0, rgb.White)
	c.Set(5, 5, rgb.White)
	c.Blend(9, 9, rgb.White, 1)
	if c.At(-1, 0) != rgb.Black {
		t.Fatal("outside should read as black")
	}
}

func TestItalicText(t *testing.T) {
	c := New(4, 1)
	c.TextItalic(0, 0, "ab", rgb.White, rgb.Black)
	line := c.Lines(TrueColor)[0]
	if !strings.Contains(line, "\x1b[3m") || !strings.Contains(line, "\x1b[23m") {
		t.Fatalf("italic codes missing: %q", line)
	}
	if visible(line) != "ab  " {
		t.Fatalf("got %q", visible(line))
	}
}
