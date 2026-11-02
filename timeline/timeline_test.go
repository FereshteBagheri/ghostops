package timeline

import (
	"slices"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

func at(m float64) time.Time {
	return t0.Add(time.Duration(m * float64(time.Minute)))
}

func sum(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func TestAdd(t *testing.T) {
	tl := New(time.Minute, 10)
	tl.Add("api", at(0.5))
	tl.Add("api", at(0.9))
	tl.Add("api", at(3))
	tl.Add("db", at(1))

	if got := tl.Row("api"); !slices.Equal(got, []int{2, 0, 0, 1}) {
		t.Errorf("api = %v", got)
	}
	if got := tl.Row("db"); !slices.Equal(got, []int{0, 1, 0, 0}) {
		t.Errorf("db = %v, want it padded to the same length", got)
	}
	if !tl.Start().Equal(t0) || tl.Width() != time.Minute || tl.Len() != 4 {
		t.Errorf("start %v width %v len %d", tl.Start(), tl.Width(), tl.Len())
	}
}

func TestStartIsTruncated(t *testing.T) {
	tl := New(time.Minute, 10)
	tl.Add("api", t0.Add(90*time.Second))
	if !tl.Start().Equal(t0.Add(time.Minute)) {
		t.Errorf("start = %v", tl.Start())
	}
}

func TestMerge(t *testing.T) {
	tl := New(time.Minute, 4)
	for m := range 4 {
		tl.Add("api", at(float64(m)))
	}
	tl.Add("db", at(1))
	tl.Add("api", at(4)) // doesn't fit in 4 one-minute buckets

	if tl.Width() != 2*time.Minute {
		t.Fatalf("width = %v, want 2m", tl.Width())
	}
	if got := tl.Row("api"); !slices.Equal(got, []int{2, 2, 1}) {
		t.Errorf("api = %v, want [2 2 1]", got)
	}
	if got := tl.Row("db"); !slices.Equal(got, []int{1, 0, 0}) {
		t.Errorf("db = %v, want [1 0 0]", got)
	}
}

func TestLongRangeStaysSmall(t *testing.T) {
	tl := New(time.Minute, 60)
	const minutes = 365 * 24 * 60
	for m := 0; m < minutes; m += 7 {
		tl.Add("api", at(float64(m)))
	}
	row := tl.Row("api")
	if len(row) > 60 {
		t.Errorf("%d buckets, want at most 60", len(row))
	}
	if want := (minutes + 6) / 7; sum(row) != want {
		t.Errorf("total = %d, want %d", sum(row), want)
	}
	if end := tl.Start().Add(time.Duration(len(row)) * tl.Width()); end.Before(at(minutes - 7)) {
		t.Errorf("last bucket ends at %v, before the last event", end)
	}
}

func TestEarlierTimeGoesToFirstBucket(t *testing.T) {
	tl := New(time.Minute, 10)
	tl.Add("api", at(5))
	tl.Add("api", at(1))
	if got := tl.Row("api"); !slices.Equal(got, []int{2}) {
		t.Errorf("api = %v, want [2]", got)
	}
}

func TestTouch(t *testing.T) {
	tl := New(time.Minute, 10)
	tl.Touch("quiet")
	tl.Add("busy", at(2))
	if !slices.Equal(tl.Names(), []string{"busy", "quiet"}) {
		t.Errorf("names = %v", tl.Names())
	}
	if got := tl.Row("quiet"); !slices.Equal(got, []int{0}) {
		t.Errorf("quiet = %v", got)
	}
}

func TestEmpty(t *testing.T) {
	tl := New(0, 0)
	if tl.Len() != 0 || len(tl.Names()) != 0 || len(tl.Row("x")) != 0 {
		t.Error("empty timeline is not empty")
	}
}

func TestRowIsACopy(t *testing.T) {
	tl := New(time.Minute, 10)
	tl.Add("api", t0)
	tl.Row("api")[0] = 99
	if tl.Row("api")[0] != 1 {
		t.Error("changing a returned row changed the timeline")
	}
}
