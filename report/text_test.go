package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

type brokenWriter struct {
	left int
}

var errBroken = errors.New("pipe closed")

func (w *brokenWriter) Write(p []byte) (int, error) {
	if len(p) > w.left {
		n := w.left
		w.left = 0
		return n, errBroken
	}
	w.left -= len(p)
	return len(p), nil
}

func sampleReport() Report {
	t := time.Date(2026, 10, 20, 8, 5, 0, 0, time.UTC)
	last := t.Add(10 * time.Minute)
	return Report{
		Source:   "app.log",
		Settings: Settings{IncidentWindow: "5m0s", AnomalyBucket: "1m0s"},
		Summary:  Summary{Lines: 3, Valid: 3, FirstEntry: &t, LastEntry: &last},
		Levels:   []Count{{"INFO", 1}, {"WARN", 0}, {"ERROR", 2}, {"FATAL", 0}},
		Services: []Count{{"pay", 3}},
		Incidents: Incidents{Groups: []Incident{{
			Rule: "database-failure", Severity: "HIGH", Service: "pay", Count: 2,
			FirstSeen: t, LastSeen: t.Add(90 * time.Second), DurationSeconds: 90, FirstLine: 2,
			Samples: []string{"database down"},
		}}},
		Anomalies: Anomalies{TotalServices: 1, WarmupBuckets: 10},
	}
}

func TestTextRender(t *testing.T) {
	var buf bytes.Buffer
	if err := (Text{}).Render(&buf, sampleReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"GhostOps report for app.log",
		"Entries from 2026-10-20 08:05:00 to 08:15:00 UTC (10m)",
		"HIGH      database-failure  pay  2 entries  08:05:00 - 08:06:30  1m30s  line 2",
		"            database down",
		"Not enough history yet: a service needs 10 buckets",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q\n%s", want, out)
		}
	}
}

func TestTextRenderReturnsWriteError(t *testing.T) {
	for _, n := range []int{0, 10, 200} {
		err := (Text{}).Render(&brokenWriter{left: n}, sampleReport())
		if !errors.Is(err, errBroken) {
			t.Errorf("after %d bytes: err = %v, want %v", n, err, errBroken)
		}
	}
}

func TestJSONRenderReturnsWriteError(t *testing.T) {
	if err := (JSON{}).Render(&brokenWriter{}, sampleReport()); !errors.Is(err, errBroken) {
		t.Errorf("err = %v, want %v", err, errBroken)
	}
}

func TestShortDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                           "0s",
		90 * time.Second:            "1m30s",
		5 * time.Minute:             "5m",
		time.Hour:                   "1h",
		time.Hour + 59*time.Minute:  "1h59m",
		2*time.Hour + 3*time.Second: "2h0m3s",
	} {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "entry", "entries") != "1 entry" || plural(0, "entry", "entries") != "0 entries" {
		t.Error("plural is wrong")
	}
}

func TestSparkline(t *testing.T) {
	tests := []struct {
		counts []int
		max    int
		want   string
	}{
		{[]int{0, 1, 2, 4, 8}, 8, "·▁▂▄█"},
		{[]int{0, 0}, 0, "··"},
		{[]int{1, 1}, 1, "██"},
		{[]int{1, 100}, 100, "▁█"},
		{nil, 5, ""},
	}
	for _, tt := range tests {
		if got := sparkline(tt.counts, tt.max); got != tt.want {
			t.Errorf("sparkline(%v, %d) = %q, want %q", tt.counts, tt.max, got, tt.want)
		}
	}
}

func TestHealthAndTimeline(t *testing.T) {
	r := sampleReport()
	start := *r.Summary.FirstEntry
	r.Health = []Health{{
		Service: "pay", Score: 55, Status: "unhealthy", Entries: 3, Errors: 2, ErrorRate: 0.667,
		Incidents: 1, Penalties: Penalties{Errors: 40, Incidents: 15},
	}}
	r.Timeline = Timeline{Start: &start, Bucket: "1m0s", Rows: []TimelineRow{
		{Name: "(all entries)", Max: 2, Counts: []int{2, 0, 1}},
		{Name: "pay", Max: 2, Counts: []int{0, 0, 2}},
	}}

	var buf bytes.Buffer
	if err := (Text{}).Render(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"pay   55  unhealthy  3 entries  2 errors  66.7%  1 incident   0 anomalies  (-40 errors, -15 incidents)",
		"Timeline from 08:05:00 to 08:08:00, one column = 1m",
		"  (all entries)  █·▄  max 2",
		"  pay            ··█  max 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q\n%s", want, out)
		}
	}
}
