// Package baseline keeps running statistics of a stream of numbers
// without storing the numbers themselves.
package baseline

import "math"

// Running tracks count, mean and variance using Welford's online algorithm.
// The zero value is ready to use.
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

// StdDev is the population standard deviation; 0 until two values were added.
func (r *Running) StdDev() float64 {
	if r.n < 2 {
		return 0
	}
	return math.Sqrt(r.m2 / float64(r.n))
}

// ZScore says how many standard deviations x is above the mean.
// minStdDev keeps very stable series from producing huge scores for tiny changes.
func (r *Running) ZScore(x, minStdDev float64) float64 {
	return (x - r.mean) / max(r.StdDev(), minStdDev)
}
