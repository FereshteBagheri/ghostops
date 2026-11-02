package baseline

import "math"

type Running struct {
	n    int
	mean float64
	m2   float64
}

func (r *Running) Add(x float64) {
	r.n++
	d := x - r.mean
	r.mean += d / float64(r.n)
	r.m2 += d * (x - r.mean)
}

func (r *Running) N() int {
	return r.n
}

func (r *Running) Mean() float64 {
	return r.mean
}

func (r *Running) StdDev() float64 {
	if r.n < 2 {
		return 0
	}
	return math.Sqrt(r.m2 / float64(r.n))
}

func (r *Running) ZScore(x, minStdDev float64) float64 {
	return (x - r.mean) / max(r.StdDev(), minStdDev)
}
