package rgb

import (
	"math"
	"testing"
)

func TestHexRoundTrip(t *testing.T) {
	for _, s := range []string{"#000000", "#ffffff", "#d6c8ad", "#0b1026", "#c9a24a"} {
		c, err := Hex(s)
		if err != nil {
			t.Fatalf("Hex(%q): %v", s, err)
		}
		if got := c.String(); got != s {
			t.Errorf("round trip %q -> %q", s, got)
		}
	}
}

func TestHexErrors(t *testing.T) {
	for _, s := range []string{"", "#fff", "#gggggg", "12345", "#1234567"} {
		if _, err := Hex(s); err == nil {
			t.Errorf("Hex(%q) should fail", s)
		}
	}
	if c, err := Hex("abcdef"); err != nil || c.String() != "#abcdef" {
		t.Errorf("Hex without # should work, got %v %v", c, err)
	}
}

func TestLerpAndClamp(t *testing.T) {
	a, b := MustHex("#000000"), MustHex("#ffffff")
	mid := a.Lerp(b, 0.5)
	if math.Abs(mid.R-0.5) > 1e-9 {
		t.Fatalf("lerp midpoint = %v", mid)
	}
	over := Color{1.5, -0.2, 0.5}.Clamp()
	if over != (Color{1, 0, 0.5}) {
		t.Fatalf("clamp = %v", over)
	}
	r, g, bb := Color{2, -1, 0.5}.Bytes()
	if r != 255 || g != 0 || bb != 128 {
		t.Fatalf("bytes = %d %d %d", r, g, bb)
	}
}

func TestTo256NeverUsesThemeColors(t *testing.T) {
	for r := 0; r < 256; r += 15 {
		for g := 0; g < 256; g += 15 {
			for b := 0; b < 256; b += 15 {
				idx := To256(FromBytes(uint8(r), uint8(g), uint8(b)))
				if idx < 16 {
					t.Fatalf("color %d,%d,%d mapped to theme color %d", r, g, b, idx)
				}
			}
		}
	}
}

func TestTo256ExactMatches(t *testing.T) {
	// Colors that exist in the palette must map to themselves (except the
	// harsh and loud ones, which the room never uses).
	for _, i := range []uint8{16, 59, 95, 137, 168, 180, 231, 232, 240, 255} {
		c := Xterm(i)
		got := To256(c)
		if Xterm(got) != c {
			t.Errorf("palette color %d (%v) mapped to %d (%v)", i, c, got, Xterm(got))
		}
	}
}

func TestTo256KeepsWarmthAndCoolness(t *testing.T) {
	// A warm lamp tone should stay warm and a dusk-sky tone should stay
	// cool, rather than both collapsing to grey.
	warm := Xterm(To256(MustHex("#c88a52")))
	if !(warm.R > warm.B+0.15) {
		t.Errorf("warm lamp tone lost its warmth: %v", warm)
	}
	for _, h := range []string{"#3d5a8f", "#5c9bd8", "#6670ae"} {
		cool := Xterm(To256(MustHex(h)))
		if !(cool.B > cool.R+0.03) {
			t.Errorf("sky tone %s lost its blue: %v", h, cool)
		}
	}
}

func TestTo256AvoidsHarshPrimaries(t *testing.T) {
	// Warm wood under lamplight must not turn into pure red, nor a night
	// sky into pure blue.
	for _, h := range []string{"#6b3a1e", "#8a4a2a", "#0e1431", "#1b2758", "#2a3a30"} {
		idx := To256(MustHex(h))
		if harsh(idx) {
			t.Errorf("%s matched harsh palette color %d (%v)", h, idx, Xterm(idx))
		}
	}
	if !harsh(88) || !harsh(17) || harsh(94) || harsh(16) || harsh(240) {
		t.Error("harsh() misclassifies palette entries")
	}
}

func TestSmoothstep(t *testing.T) {
	if Smoothstep(0, 1, -1) != 0 || Smoothstep(0, 1, 2) != 1 {
		t.Fatal("smoothstep should clamp")
	}
	if math.Abs(Smoothstep(0, 1, 0.5)-0.5) > 1e-9 {
		t.Fatal("smoothstep midpoint")
	}
}

func TestLoudOrangesAreLeftOutOfTheRoom(t *testing.T) {
	// Lamp light on the walls used to fall onto pure orange stripes.
	for _, hex := range []string{"#ff8700", "#af5f00", "#d75f00", "#ffaf00", "#875f00"} {
		got := Xterm(To256(MustHex(hex)))
		if got == MustHex(hex) {
			t.Errorf("%s should not be used for the room", hex)
		}
	}
	// ...but a cat may use them: Hunter stays ginger.
	ginger := MustHex("#ff8700")
	if got := Xterm(To256At(ginger, Cat)); got != ginger {
		t.Errorf("a cat's #ff8700 should stay #ff8700, got %v", got)
	}
}

func TestDimCatsStayLighterThanTheRoom(t *testing.T) {
	// Brie's fur at night (a dusky cream) matched the room's mauve; in a
	// dim place it lands on something lighter. Her darker lines don't move.
	fur, line := MustHex("#8f7063"), MustHex("#5b3c31")
	room := Xterm(To256(fur))
	dim := Xterm(To256At(fur, DimCat))
	if dim.Luma() <= room.Luma()+0.03 {
		t.Errorf("dim fur %v should be lighter than the room's match %v", dim, room)
	}
	if To256At(line, DimCat) != To256At(line, Cat) {
		t.Error("a cat's dark lines shouldn't be lifted")
	}
	// In good light, a cat is matched as it is.
	cream := MustHex("#f0e0c8")
	if To256At(cream, DimCat) != To256At(cream, Cat) {
		t.Error("light fur shouldn't be lifted")
	}
}
