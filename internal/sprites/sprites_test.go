package sprites

import (
	"strings"
	"testing"
	"testing/fstest"
)

const sample = `
# a comment
palette brass
  b #c9a24a  brass
  B #e8cc7a  highlight

sprite lamp
  use brass
  s #efe0c0 @lamp lampshade
grid
.sss.
sssss
..b..
.BbB.
end

sprite roof
  z #7e8a97 @roof zinc
  w #333333 @window
grid
zz
zw
end
`

func TestParse(t *testing.T) {
	lib, err := Parse("sample.txt", strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	lamp, ok := lib.Get("lamp")
	if !ok {
		t.Fatal("lamp missing")
	}
	if lamp.W != 5 || lamp.H != 4 {
		t.Fatalf("lamp size %dx%d", lamp.W, lamp.H)
	}
	if lamp.At(0, 0).Opaque {
		t.Error("'.' should be transparent")
	}
	if p := lamp.At(1, 0); !p.Opaque || p.Mat != Lamp || p.Color.String() != "#efe0c0" {
		t.Errorf("shade pixel wrong: %+v", p)
	}
	if p := lamp.At(2, 3); p.Color.String() != "#c9a24a" {
		t.Errorf("palette color not used: %+v", p)
	}
	if lamp.At(99, 99).Opaque {
		t.Error("outside should be transparent")
	}
	roof, _ := lib.Get("roof")
	if roof.At(1, 1).Mat != Window || roof.At(0, 0).Mat != Roof {
		t.Error("materials not read")
	}
	if got := strings.Join(lib.Names(), ","); got != "lamp,roof" {
		t.Errorf("names = %s", got)
	}
}

func TestShortRowsArePadded(t *testing.T) {
	lib, err := Parse("t", strings.NewReader("sprite s\n x #ffffff\ngrid\nxxx\nx\nend\n"))
	if err != nil {
		t.Fatal(err)
	}
	s, _ := lib.Get("s")
	if s.W != 3 || s.At(2, 1).Opaque {
		t.Fatal("short row should be padded with transparent pixels")
	}
}

func TestHelpfulErrors(t *testing.T) {
	cases := map[string]string{
		"sprite s\n x #fff\ngrid\nx\nend\n":                "#rrggbb",
		"sprite s\n x #ffffff\ngrid\nxy\nend\n":            "isn't in its palette",
		"sprite s\n use nope\ngrid\n.\nend\n":              "doesn't exist",
		"sprite s\n x #ffffff @sparkly\ngrid\nx\nend\n":    "unknown material",
		"sprite s\n x #ffffff\ngrid\nx\n":                  "no \"end\"",
		"x #ffffff\n":                                      "belongs to",
		"sprite s\ngrid\n.\nend\nsprite s\ngrid\n.\nend\n": "already exists",
		"sprite s\n xy #ffffff\ngrid\n.\nend\n":            "single character",
	}
	for src, want := range cases {
		_, err := Parse("t.txt", strings.NewReader(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("source %q: want error containing %q, got %v", src, want, err)
		}
	}
}

func TestErrorsNameTheLine(t *testing.T) {
	_, err := Parse("cup.txt", strings.NewReader("sprite cup\n c #12\ngrid\nc\nend\n"))
	if err == nil || !strings.Contains(err.Error(), "cup.txt line 2") {
		t.Fatalf("error should name file and line, got %v", err)
	}
}

func TestLoadSharesPalettesAcrossFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"art/a.txt": {Data: []byte("palette p\n q #102030\n")},
		"art/b.txt": {Data: []byte("sprite s\n use p\ngrid\nq\nend\n")},
	}
	lib, err := Load(fsys, "art")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := lib.Get("s")
	if !ok || s.At(0, 0).Color.String() != "#102030" {
		t.Fatal("palette from another file not shared")
	}
}
