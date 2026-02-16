package main

import (
	"math"
	"math/rand"
)

// SimplexNoise generates 2D simplex noise with a seed-shuffled permutation table.
type SimplexNoise struct {
	perm [512]int
}

// NewSimplexNoise creates a new noise generator with the given seed.
func NewSimplexNoise(seed int64) *SimplexNoise {
	sn := &SimplexNoise{}
	r := rand.New(rand.NewSource(seed))

	// Initialize and shuffle permutation table
	p := make([]int, 256)
	for i := range p {
		p[i] = i
	}
	r.Shuffle(256, func(i, j int) { p[i], p[j] = p[j], p[i] })

	for i := 0; i < 512; i++ {
		sn.perm[i] = p[i&255]
	}
	return sn
}

// grad2 computes the dot product of a gradient vector and (x, y).
func grad2(hash int, x, y float64) float64 {
	h := hash & 7
	u, v := x, y
	if h >= 4 {
		u, v = y, x
	}
	if h&1 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	return u + v
}

const (
	f2 = 0.3660254037844386  // (sqrt(3) - 1) / 2
	g2 = 0.21132486540518713 // (3 - sqrt(3)) / 6
)

// Noise2D returns 2D simplex noise in the range [-1, 1].
func (sn *SimplexNoise) Noise2D(x, y float64) float64 {
	// Skew input space to determine which simplex cell we're in
	s := (x + y) * f2
	i := math.Floor(x + s)
	j := math.Floor(y + s)

	t := (i + j) * g2
	x0 := x - (i - t)
	y0 := y - (j - t)

	// Determine which simplex we're in
	var i1, j1 int
	if x0 > y0 {
		i1, j1 = 1, 0
	} else {
		i1, j1 = 0, 1
	}

	x1 := x0 - float64(i1) + g2
	y1 := y0 - float64(j1) + g2
	x2 := x0 - 1.0 + 2.0*g2
	y2 := y0 - 1.0 + 2.0*g2

	ii := int(i) & 255
	jj := int(j) & 255

	// Calculate contributions from the three corners
	var n0, n1, n2 float64

	t0 := 0.5 - x0*x0 - y0*y0
	if t0 > 0 {
		t0 *= t0
		n0 = t0 * t0 * grad2(sn.perm[ii+sn.perm[jj]], x0, y0)
	}

	t1 := 0.5 - x1*x1 - y1*y1
	if t1 > 0 {
		t1 *= t1
		n1 = t1 * t1 * grad2(sn.perm[ii+i1+sn.perm[jj+j1]], x1, y1)
	}

	t2 := 0.5 - x2*x2 - y2*y2
	if t2 > 0 {
		t2 *= t2
		n2 = t2 * t2 * grad2(sn.perm[ii+1+sn.perm[jj+1]], x2, y2)
	}

	// Scale to [-1, 1]
	return 70.0 * (n0 + n1 + n2)
}

// Fractal generates multi-octave fractal noise normalized to [0, 1].
func (sn *SimplexNoise) Fractal(x, y, freq float64, octaves int, lacunarity, persistence float64) float64 {
	var total float64
	var maxAmp float64
	amp := 1.0

	for i := 0; i < octaves; i++ {
		total += sn.Noise2D(x*freq, y*freq) * amp
		maxAmp += amp
		freq *= lacunarity
		amp *= persistence
	}

	// Normalize from [-1,1] to [0,1]
	return (total/maxAmp + 1.0) / 2.0
}
