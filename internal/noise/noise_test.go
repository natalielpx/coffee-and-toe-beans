package noise

import "testing"

func TestRepeatableAndInRange(t *testing.T) {
	for x := -20; x < 20; x++ {
		for y := -20; y < 20; y++ {
			r := Rand(x, y, 7)
			if r < 0 || r >= 1 || r != Rand(x, y, 7) {
				t.Fatalf("Rand(%d,%d) = %v", x, y, r)
			}
			v := Fractal(float64(x)*0.37, float64(y)*0.21, 3, 3)
			if v < 0 || v >= 1 {
				t.Fatalf("Fractal out of range: %v", v)
			}
		}
	}
	if Rand(1, 2, 3) == Rand(1, 2, 4) {
		t.Error("different seeds should differ")
	}
}
