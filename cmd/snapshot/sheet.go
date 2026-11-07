package main

import (
	"fmt"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/canvas"
	"coffee-and-toe-beans/internal/cats"
	"coffee-and-toe-beans/internal/game"
	"coffee-and-toe-beans/internal/rgb"
	"coffee-and-toe-beans/internal/sprites"
)

// sheet prints every cat in every pose and frame (or only the still frame
// of each pose, with stillsOnly), as drawn in daylight on the café's wall
// color, with their names above them.
func sheet(o options, stillsOnly bool) error {
	mode, auto, err := game.ParseColorMode(o.colors)
	if err != nil {
		return err
	}
	if auto {
		mode = canvas.TrueColor
	}
	lib, err := sprites.Load(content.FS, "sprites")
	if err != nil {
		return err
	}
	defs, err := cats.LoadDefs(content.FS, "cats")
	if err != nil {
		return err
	}
	const gap, rowH = 3, 13 // cells between sprites; cells per cat row
	type item struct {
		label string
		sp    *sprites.Sprite
	}
	var rows [][]item
	width := 0
	for _, d := range defs {
		var row []item
		w := 1
		for _, p := range cats.Poses {
			pose := d.Poses[p]
			for _, f := range []string{cats.FrameStill, cats.FrameBlink, cats.FrameEar, cats.FrameTail, cats.FrameBreathe} {
				name, ok := pose.Frames[f]
				if !ok || (stillsOnly && f != cats.FrameStill) {
					continue
				}
				sp, ok := lib.Get(name)
				if !ok {
					return fmt.Errorf("no sprite %q", name)
				}
				label := p
				if f != cats.FrameStill {
					label = p + " " + f
				}
				row = append(row, item{label, sp})
				lw := len(label)
				if sp.W > lw {
					lw = sp.W
				}
				w += lw + gap
			}
		}
		rows = append(rows, row)
		if w > width {
			width = w
		}
	}
	bg := rgb.MustHex("#d7af87")
	ink := rgb.MustHex("#3a2c22")
	c := canvas.New(width, len(rows)*rowH)
	c.Fill(bg)
	for ri, row := range rows {
		x := 1
		top := ri * rowH
		c.Text(1, top, defs[ri].Name, ink, bg)
		for _, it := range row {
			c.Text(x, top+1, it.label, ink, bg)
			y0 := (top+rowH)*2 - 1 - it.sp.H // feet on the row's last pixel
			for y := 0; y < it.sp.H; y++ {
				for xx := 0; xx < it.sp.W; xx++ {
					if p := it.sp.At(xx, y); p.Opaque {
						c.Set(x+xx, y0+y, p.Color)
					}
				}
			}
			lw := len(it.label)
			if it.sp.W > lw {
				lw = it.sp.W
			}
			x += lw + gap
		}
	}
	fmt.Println(c.String(mode))
	return nil
}
