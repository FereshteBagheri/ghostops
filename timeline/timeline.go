// Package timeline counts events over time for several named rows that share
// one time axis. The number of buckets never goes above a maximum: when time
// runs past the last bucket, neighbouring buckets are merged and each bucket
// becomes twice as wide.
package timeline

import (
	"maps"
	"slices"
	"time"
)

type Timeline struct {
	width   time.Duration
	max     int
	start   time.Time
	started bool
	length  int
	rows    map[string][]int
}

// New uses one minute and 60 buckets if width or maxBuckets are too small.
func New(width time.Duration, maxBuckets int) *Timeline {
	if width <= 0 {
		width = time.Minute
	}
	if maxBuckets < 2 {
		maxBuckets = 60
	}
	return &Timeline{width: width, max: maxBuckets, rows: make(map[string][]int)}
}

// Add counts one event for row name at time at.
func (t *Timeline) Add(name string, at time.Time) {
	if !t.started {
		t.started = true
		t.start = at.Truncate(t.width)
	}
	i := t.index(at)
	for i >= t.max {
		t.merge()
		i = t.index(at)
	}

	row := t.rows[name]
	if len(row) <= i {
		row = append(row, make([]int, i+1-len(row))...)
	}
	row[i]++
	t.rows[name] = row
	t.length = max(t.length, i+1)
}

// Touch makes sure row name exists, even if it never gets an event.
func (t *Timeline) Touch(name string) {
	if _, ok := t.rows[name]; !ok {
		t.rows[name] = nil
	}
}

func (t *Timeline) Start() time.Time {
	return t.start
}

func (t *Timeline) Width() time.Duration {
	return t.width
}

// Len is the number of buckets in use.
func (t *Timeline) Len() int {
	return t.length
}

func (t *Timeline) Names() []string {
	return slices.Sorted(maps.Keys(t.rows))
}

// Row returns a copy of the counts for name, Len buckets long.
func (t *Timeline) Row(name string) []int {
	row := make([]int, t.length)
	copy(row, t.rows[name])
	return row
}

func (t *Timeline) index(at time.Time) int {
	if at.Before(t.start) {
		return 0
	}
	return int(at.Sub(t.start) / t.width)
}

func (t *Timeline) merge() {
	t.width *= 2
	t.length = (t.length + 1) / 2
	for name, row := range t.rows {
		n := (len(row) + 1) / 2
		for j := range n {
			sum := row[2*j]
			if 2*j+1 < len(row) {
				sum += row[2*j+1]
			}
			row[j] = sum
		}
		t.rows[name] = row[:n]
	}
}
