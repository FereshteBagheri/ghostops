package baseline

import (
	"math"
	"testing"
)

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestRunning(t *testing.T) {
	var r Running
	for _, x := range []float64{2, 4, 4, 4, 5, 5, 7, 9} {
		r.Add(x)
	}
	if r.N() != 8 || !near(r.Mean(), 5) || !near(r.StdDev(), 2) {
		t.Errorf("n=%d mean=%v stddev=%v, want 8, 5, 2", r.N(), r.Mean(), r.StdDev())
	}
	if z := r.ZScore(11, 1); !near(z, 3) {
		t.Errorf("ZScore(11) = %v, want 3", z)
	}
}

func TestZeroValue(t *testing.T) {
	var r Running
	if r.N() != 0 || r.Mean() != 0 || r.StdDev() != 0 {
		t.Errorf("zero value not empty: %+v", r)
	}
	r.Add(7)
	if r.Mean() != 7 || r.StdDev() != 0 {
		t.Errorf("after one value: mean=%v stddev=%v", r.Mean(), r.StdDev())
	}
}

func TestZScoreFloor(t *testing.T) {
	var r Running
	for range 20 {
		r.Add(3)
	}
	if z := r.ZScore(5, 1); !near(z, 2) {
		t.Errorf("ZScore = %v, want 2", z)
	}
	if z := r.ZScore(5, 0.5); !near(z, 4) {
		t.Errorf("ZScore = %v, want 4", z)
	}
}

func TestMatchesTwoPass(t *testing.T) {
	xs := []float64{12, 0, 3, 3, 150, 7, 7, 8, 1, 0, 0, 44}
	var r Running
	sum := 0.0
	for _, x := range xs {
		r.Add(x)
		sum += x
	}
	mean := sum / float64(len(xs))
	sq := 0.0
	for _, x := range xs {
		sq += (x - mean) * (x - mean)
	}
	want := math.Sqrt(sq / float64(len(xs)))
	if !near(r.Mean(), mean) || !near(r.StdDev(), want) {
		t.Errorf("mean=%v stddev=%v, want %v %v", r.Mean(), r.StdDev(), mean, want)
	}
}
