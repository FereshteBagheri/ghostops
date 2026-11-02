package detect

import (
	"slices"
	"testing"
	"time"

	"github.com/FereshteBagheri/ghostops/logentry"
)

var t0 = time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

func at(minutes float64, line int, level logentry.Level, service, msg string) logentry.Entry {
	return logentry.Entry{
		Line:    line,
		Time:    t0.Add(time.Duration(minutes * float64(time.Minute))),
		Level:   level,
		Service: service,
		Message: msg,
	}
}

func observeAll(d *Detector, entries ...logentry.Entry) []Incident {
	for _, e := range entries {
		d.Observe(e)
	}
	return d.Incidents()
}

func TestGroupingWindow(t *testing.T) {
	tests := []struct {
		name       string
		gapMinutes float64
		wantGroups int
	}{
		{"gap shorter than window", 4, 1},
		{"gap equal to window", 5, 1},
		{"gap longer than window", 5.5, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := observeAll(NewDetector(DefaultRules(), 5*time.Minute),
				at(0, 1, logentry.Error, "pay", "database down"),
				at(tt.gapMinutes, 2, logentry.Error, "pay", "database down"),
			)
			if len(got) != tt.wantGroups {
				t.Errorf("got %d incidents, want %d", len(got), tt.wantGroups)
			}
		})
	}
}

func TestGapIsMeasuredFromLastEntry(t *testing.T) {
	got := observeAll(NewDetector(DefaultRules(), 5*time.Minute),
		at(0, 1, logentry.Error, "pay", "database down"),
		at(4, 2, logentry.Error, "pay", "database down"),
		at(8, 3, logentry.Error, "pay", "database down"),
		at(12, 4, logentry.Error, "pay", "database down"),
	)
	if len(got) != 1 || got[0].Count != 4 || got[0].Duration() != 12*time.Minute {
		t.Errorf("got %+v, want 1 incident of 4 entries lasting 12m", got)
	}
}

func TestIncidentFields(t *testing.T) {
	got := observeAll(NewDetector(DefaultRules(), 0),
		at(0, 10, logentry.Error, "pay", "database down"),
		at(1, 11, logentry.Error, "pay", "database down"),
		at(2, 12, logentry.Error, "pay", "deadlock detected"),
		at(3, 13, logentry.Fatal, "pay", "cannot reach database"),
	)

	want := []Incident{
		{
			Rule: "service-crash", Severity: Critical, Service: "pay", Count: 1,
			FirstSeen: t0.Add(3 * time.Minute), LastSeen: t0.Add(3 * time.Minute), FirstLine: 13,
			Samples: []string{"cannot reach database"},
		},
		{
			Rule: "database-failure", Severity: High, Service: "pay", Count: 3,
			FirstSeen: t0, LastSeen: t0.Add(2 * time.Minute), FirstLine: 10,
			Samples: []string{"database down", "deadlock detected"},
		},
	}
	if !slices.EqualFunc(got, want, equalIncident) {
		t.Errorf("Incidents() =\n %+v\nwant\n %+v", got, want)
	}
}

func TestServicesAreGroupedSeparately(t *testing.T) {
	got := observeAll(NewDetector(DefaultRules(), 0),
		at(0, 1, logentry.Error, "pay", "database down"),
		at(0, 2, logentry.Error, "order", "database down"),
		at(1, 3, logentry.Error, "pay", "database down"),
	)
	if len(got) != 2 {
		t.Fatalf("got %d incidents, want 2", len(got))
	}
	if got[0].Service != "order" || got[0].Count != 1 || got[1].Service != "pay" || got[1].Count != 2 {
		t.Errorf("got %+v", got)
	}
}

func TestSamplesAreDistinctAndCapped(t *testing.T) {
	d := NewDetector(DefaultRules(), 0)
	for i, msg := range []string{"db a", "db a", "db b", "db c", "db d", "db e"} {
		d.Observe(at(0, i+1, logentry.Error, "pay", msg))
	}
	got := d.Incidents()[0].Samples
	if want := []string{"db a", "db b", "db c"}; !slices.Equal(got, want) {
		t.Errorf("Samples = %q, want %q", got, want)
	}
}

func TestOutOfOrderEntryExtendsFirstSeen(t *testing.T) {
	got := observeAll(NewDetector(DefaultRules(), 0),
		at(2, 1, logentry.Error, "pay", "database down"),
		at(1, 2, logentry.Error, "pay", "database down"),
	)
	if len(got) != 1 || !got[0].FirstSeen.Equal(t0.Add(time.Minute)) || !got[0].LastSeen.Equal(t0.Add(2*time.Minute)) {
		t.Errorf("got %+v, want one incident from 08:01 to 08:02", got)
	}
}

func TestIncidentsSortOrder(t *testing.T) {
	got := observeAll(NewDetector(DefaultRules(), 0),
		at(0, 1, logentry.Error, "b", "failed"),
		at(0, 2, logentry.Error, "a", "failed"),
		at(5, 3, logentry.Fatal, "z", "crash"),
		at(1, 4, logentry.Error, "a", "database down"),
	)
	var order []string
	for _, inc := range got {
		order = append(order, inc.Rule+"/"+inc.Service)
	}
	want := []string{"service-crash/z", "database-failure/a", "error-burst/a", "error-burst/b"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestIncidentsReturnsCopies(t *testing.T) {
	d := NewDetector(DefaultRules(), 0)
	d.Observe(at(0, 1, logentry.Error, "pay", "database down"))
	d.Incidents()[0].Samples[0] = "changed"
	if d.Incidents()[0].Samples[0] != "database down" {
		t.Error("changing a returned incident changed the detector's state")
	}
}

func TestNoIncidents(t *testing.T) {
	got := observeAll(NewDetector(DefaultRules(), 0), at(0, 1, logentry.Info, "pay", "all good"))
	if got == nil || len(got) != 0 {
		t.Errorf("Incidents() = %#v, want an empty, non-nil slice", got)
	}
}

func equalIncident(a, b Incident) bool {
	return a.Rule == b.Rule && a.Severity == b.Severity && a.Service == b.Service &&
		a.Count == b.Count && a.FirstSeen.Equal(b.FirstSeen) && a.LastSeen.Equal(b.LastSeen) &&
		a.FirstLine == b.FirstLine && slices.Equal(a.Samples, b.Samples)
}
