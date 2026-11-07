package game

import (
	"testing"
	"time"

	"coffee-and-toe-beans/content"
	"coffee-and-toe-beans/internal/canvas"
)

func benchFrame(b *testing.B, w, h int, mode canvas.Mode) {
	g, err := New(Config{ColorMode: mode, Weather: "heavy_rain"}, content.FS, t0)
	if err != nil {
		b.Fatal(err)
	}
	g.Resize(w, h)
	g.Key(KeyConfirm)
	g.Key(KeyConfirm)
	now := t0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		now = now.Add(100 * time.Millisecond)
		g.Tick(now)
		_ = g.View()
	}
}

func BenchmarkFrame80x32(b *testing.B)      { benchFrame(b, 80, 32, canvas.TrueColor) }
func BenchmarkFrame80x32in256(b *testing.B) { benchFrame(b, 80, 32, canvas.Color256) }
func BenchmarkFrame200x60(b *testing.B)     { benchFrame(b, 200, 60, canvas.TrueColor) }
