package health

import "testing"

func TestScore(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want Result
	}{
		{"no problems", Input{Entries: 100}, Result{Score: 100, Status: "healthy"}},
		{"no entries", Input{}, Result{Score: 100, Status: "healthy"}},
		{"5% errors", Input{Entries: 100, Errors: 5}, Result{Score: 90, Status: "healthy", ErrorPenalty: 10}},
		{"error penalty is capped", Input{Entries: 10, Errors: 10}, Result{Score: 60, Status: "unhealthy", ErrorPenalty: 40}},
		{"one high incident", Input{Entries: 100, High: 1}, Result{Score: 85, Status: "degraded", IncidentPenalty: 15}},
		{"mixed incidents", Input{Entries: 100, Medium: 2, Low: 3}, Result{Score: 75, Status: "degraded", IncidentPenalty: 25}},
		{"incident penalty is capped", Input{Entries: 100, Critical: 3}, Result{Score: 55, Status: "unhealthy", IncidentPenalty: 45}},
		{"anomaly penalty is capped", Input{Entries: 100, Anomalies: 5}, Result{Score: 80, Status: "degraded", AnomalyPenalty: 20}},
		{
			"everything at once",
			Input{Entries: 10, Errors: 9, Critical: 2, Anomalies: 3},
			Result{Score: 0, Status: "critical", ErrorPenalty: 40, IncidentPenalty: 45, AnomalyPenalty: 20},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.in); got != tt.want {
				t.Errorf("Score() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStatusBoundaries(t *testing.T) {
	for score, want := range map[int]string{
		100: "healthy", 90: "healthy", 89: "degraded", 70: "degraded",
		69: "unhealthy", 40: "unhealthy", 39: "critical", 0: "critical",
	} {
		if got := Status(score); got != want {
			t.Errorf("Status(%d) = %q, want %q", score, got, want)
		}
	}
}
