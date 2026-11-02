package report

import (
	"io"
	"time"
)

type Renderer interface {
	Render(w io.Writer, r Report) error
}

type Report struct {
	Source    string    `json:"source"`
	Settings  Settings  `json:"settings"`
	Summary   Summary   `json:"summary"`
	Health    []Health  `json:"health"`
	Levels    []Count   `json:"levels"`
	Services  []Count   `json:"services"`
	Malformed Malformed `json:"malformed"`
	Incidents Incidents `json:"incidents"`
	Timeline  Timeline  `json:"timeline"`
	Anomalies Anomalies `json:"anomalies"`
}

type Settings struct {
	IncidentWindow string   `json:"incident_window"`
	AnomalyBucket  string   `json:"anomaly_bucket"`
	Services       []string `json:"service_filter,omitempty"`
	Since          string   `json:"since,omitempty"`
	Until          string   `json:"until,omitempty"`
}

type Summary struct {
	Lines       int        `json:"lines"`
	Valid       int        `json:"valid"`
	Malformed   int        `json:"malformed"`
	Skipped     int        `json:"skipped"`
	FilteredOut int        `json:"filtered_out"`
	FirstEntry  *time.Time `json:"first_entry,omitempty"`
	LastEntry   *time.Time `json:"last_entry,omitempty"`
}

type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Malformed struct {
	ByReason []Count  `json:"by_reason"`
	Examples []string `json:"examples"`
}

type Incidents struct {
	OverlapPolicy      string      `json:"overlap_policy"`
	OverlappingEntries int         `json:"overlapping_entries"`
	RuleMatches        []RuleCount `json:"rule_matches"`
	Groups             []Incident  `json:"groups"`
}

type RuleCount struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Entries  int    `json:"entries"`
}

type Incident struct {
	Rule            string    `json:"rule"`
	Severity        string    `json:"severity"`
	Service         string    `json:"service"`
	Count           int       `json:"count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	DurationSeconds float64   `json:"duration_seconds"`
	FirstLine       int       `json:"first_line"`
	Samples         []string  `json:"samples"`
}

type Anomalies struct {
	ReadyServices int       `json:"ready_services"`
	TotalServices int       `json:"total_services"`
	WarmupBuckets int       `json:"warmup_buckets"`
	OutOfOrder    int       `json:"out_of_order_entries"`
	Found         []Anomaly `json:"found"`
}

type Anomaly struct {
	Service string    `json:"service"`
	Kind    string    `json:"kind"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Peak    int       `json:"peak_per_bucket"`
	Mean    float64   `json:"normal_mean"`
	StdDev  float64   `json:"normal_stddev"`
	Score   float64   `json:"z_score"`
}

type Health struct {
	Service   string    `json:"service"`
	Score     int       `json:"score"`
	Status    string    `json:"status"`
	Entries   int       `json:"entries"`
	Errors    int       `json:"errors"`
	ErrorRate float64   `json:"error_rate"`
	Incidents int       `json:"incidents"`
	Anomalies int       `json:"anomalies"`
	Penalties Penalties `json:"penalties"`
}

type Penalties struct {
	Errors    int `json:"errors"`
	Incidents int `json:"incidents"`
	Anomalies int `json:"anomalies"`
}

type Timeline struct {
	Start  *time.Time    `json:"start,omitempty"`
	Bucket string        `json:"bucket"`
	Rows   []TimelineRow `json:"rows"`
}

type TimelineRow struct {
	Name   string `json:"name"`
	Max    int    `json:"max"`
	Counts []int  `json:"counts"`
}
