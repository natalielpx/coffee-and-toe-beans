// Package noise gives repeatable "random" patterns: the same inputs always
// give the same result, so clouds, marble veins and lit windows stay put
// between frames and between runs.
package noise

import "math"

// Hash mixes integers into a well-scrambled 32-bit number.
func Hash(x, y, seed int) uint32 {
	h := uint32(seed)*0x9E3779B1 ^ uint32(x)*0x85EBCA77 ^ uint32(y)*0xC2B2AE3D
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 12
	h *= 0x297A2D39
	h ^= h >> 15
	return h
}

// Rand is a repeatable random number in [0, 1) for a grid point.
func Rand(x, y, seed int) float64 {
	return float64(Hash(x, y, seed)>>8) / float64(1<<24)
}

// Value is smooth noise in [0, 1): random values on a grid, blended softly
// in between.
func Value(x, y float64, seed int) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	ix, iy := int(x0), int(y0)
	sx := fx * fx * (3 - 2*fx)
	sy := fy * fy * (3 - 2*fy)
	a := Rand(ix, iy, seed)
	b := Rand(ix+1, iy, seed)
	c := Rand(ix, iy+1, seed)
	d := Rand(ix+1, iy+1, seed)
	top := a + (b-a)*sx
	bot := c + (d-c)*sx
	return top + (bot-top)*sy
}

// Fractal layers several sizes of Value noise for a natural, cloudy look.
func Fractal(x, y float64, seed, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += Value(x, y, seed+i*101) * amp
		norm += amp
		amp *= 0.5
		x *= 2
		y *= 2
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}
