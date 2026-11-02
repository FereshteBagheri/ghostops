package analysis

import (
	"strings"
	"testing"
	"time"

	"ghostops/detect"
	"ghostops/logentry"
)

func TestFilterMatch(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 20, h, m, 0, 0, time.UTC) }
	e := logentry.Entry{Time: at(8, 30), Level: logentry.Info, Service: "api"}

	tests := []struct {
		name   string
		filter Filter
		want   bool
	}{
		{"zero filter", Filter{}, true},
		{"service listed", Filter{Services: []string{"db", "api"}}, true},
		{"service not listed", Filter{Services: []string{"db"}}, false},
		{"since is inclusive", Filter{Since: at(8, 30)}, true},
		{"before since", Filter{Since: at(8, 31)}, false},
		{"until is exclusive", Filter{Until: at(8, 30)}, false},
		{"inside range", Filter{Since: at(8, 0), Until: at(9, 0)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(e); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzeWithFilter(t *testing.T) {
	res, err := Analyze(strings.NewReader(input), Options{
		Rules:  detect.DefaultRules(),
		Filter: Filter{Services: []string{"db"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Stats
	if s.Valid != 3 || s.Filtered != 3 || s.Lines != 12 {
		t.Errorf("valid=%d filtered=%d lines=%d, want 3, 3, 12", s.Valid, s.Filtered, s.Lines)
	}
	for _, inc := range res.Detector.Incidents() {
		if inc.Service != "db" {
			t.Errorf("incident for filtered service %q", inc.Service)
		}
	}
}
