package analysis

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/FereshteBagheri/ghostops/detect"
	"github.com/FereshteBagheri/ghostops/report"
)

func TestReport(t *testing.T) {
	res, err := Analyze(strings.NewReader(input), Options{Rules: detect.DefaultRules()})
	if err != nil {
		t.Fatal(err)
	}
	r := res.Report("input.log")

	if r.Source != "input.log" || r.Summary.Valid != 6 || r.Summary.Malformed != 4 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if r.Summary.FirstEntry == nil || r.Summary.FirstEntry.Format("15:04") != "08:00" ||
		r.Summary.LastEntry.Format("15:04") != "08:07" {
		t.Errorf("time range = %v - %v", r.Summary.FirstEntry, r.Summary.LastEntry)
	}
	if len(r.Incidents.Groups) != 2 || r.Incidents.Groups[0].Severity != "CRITICAL" {
		t.Errorf("incidents = %+v", r.Incidents.Groups)
	}
	if r.Settings.IncidentWindow != "5m0s" || r.Settings.AnomalyBucket != "1m0s" {
		t.Errorf("settings = %+v", r.Settings)
	}
}

func TestEmptyReportHasNoNulls(t *testing.T) {
	res, err := Analyze(strings.NewReader(""), Options{Rules: detect.DefaultRules()})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := (report.JSON{}).Render(&buf, res.Report("empty.log")); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("null")) {
		t.Errorf("JSON contains null:\n%s", buf.String())
	}
	var back map[string]any
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := back["summary"].(map[string]any)["first_entry"]; ok {
		t.Error("first_entry should be left out when there are no entries")
	}
}

func TestReportHealthAndTimeline(t *testing.T) {
	res, err := Analyze(strings.NewReader(input), Options{Rules: detect.DefaultRules()})
	if err != nil {
		t.Fatal(err)
	}
	r := res.Report("input.log")

	// db has a FATAL and an ERROR (two incidents), api and cache have none
	var order []string
	for _, h := range r.Health {
		order = append(order, h.Service)
	}
	if want := []string{"db", "api", "cache"}; !slices.Equal(order, want) {
		t.Errorf("health order = %v, want %v", order, want)
	}
	db := r.Health[0]
	if db.Entries != 3 || db.Errors != 2 || db.Incidents != 2 || db.Score >= 50 {
		t.Errorf("db health = %+v", db)
	}
	if r.Health[1].Score != 100 || r.Health[1].Status != "healthy" {
		t.Errorf("api health = %+v", r.Health[1])
	}

	if len(r.Timeline.Rows) != 4 || r.Timeline.Rows[0].Name != AllEntries {
		t.Fatalf("timeline rows = %+v", r.Timeline.Rows)
	}
	all := r.Timeline.Rows[0]
	if total := sum(all.Counts); total != 6 {
		t.Errorf("all entries row counts %d entries, want 6", total)
	}
	for _, row := range r.Timeline.Rows[1:] {
		if len(row.Counts) != len(all.Counts) {
			t.Errorf("row %s has %d columns, want %d", row.Name, len(row.Counts), len(all.Counts))
		}
	}
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}
