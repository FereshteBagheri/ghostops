package analysis

import (
	"cmp"
	"math"
	"slices"
	"time"

	"ghostops/anomaly"
	"ghostops/detect"
	"ghostops/health"
	"ghostops/report"
)

const overlapPolicy = "first-match-by-priority"

// Report turns the result into the report model. Every slice is non-nil so
// empty sections are written as [] in JSON, not null.
func (res *Result) Report(source string) report.Report {
	s := res.Stats
	cfg := res.Anomalies.Config()

	r := report.Report{
		Source: source,
		Settings: report.Settings{
			IncidentWindow: res.Detector.Window().String(),
			AnomalyBucket:  cfg.Bucket.String(),
			Services:       res.Filter.Services,
			Since:          formatTime(res.Filter.Since),
			Until:          formatTime(res.Filter.Until),
		},
		Summary: report.Summary{
			Lines:       s.Lines,
			Valid:       s.Valid,
			Malformed:   s.Malformed,
			Skipped:     s.Skipped,
			FilteredOut: s.Filtered,
		},
		Levels:   toCounts(s.Levels()),
		Services: toCounts(s.Services()),
		Malformed: report.Malformed{
			ByReason: toCounts(s.MalformedReasons()),
			Examples: s.MalformedExamples(),
		},
		Incidents: report.Incidents{
			OverlapPolicy:      overlapPolicy,
			OverlappingEntries: res.Detector.Overlapping,
			RuleMatches:        []report.RuleCount{},
			Groups:             []report.Incident{},
		},
	}
	if s.Valid > 0 {
		first, last := s.First, s.Last
		r.Summary.FirstEntry = &first
		r.Summary.LastEntry = &last
	}

	for _, c := range res.Detector.Counts() {
		r.Incidents.RuleMatches = append(r.Incidents.RuleMatches, report.RuleCount{
			Rule: c.Rule, Severity: c.Severity.String(), Entries: c.Count,
		})
	}
	for _, inc := range res.Detector.Incidents() {
		r.Incidents.Groups = append(r.Incidents.Groups, toIncident(inc))
	}

	ready, total := res.Anomalies.Ready()
	r.Anomalies = report.Anomalies{
		ReadyServices: ready,
		TotalServices: total,
		WarmupBuckets: cfg.Warmup,
		OutOfOrder:    res.Anomalies.OutOfOrder,
		Found:         []report.Anomaly{},
	}
	r.Health = res.health()
	r.Timeline = res.timeline()

	for _, a := range res.Anomalies.Finish() {
		r.Anomalies.Found = append(r.Anomalies.Found, report.Anomaly{
			Service: a.Service,
			Kind:    a.Kind.String(),
			Start:   a.Start,
			End:     a.End,
			Peak:    a.Peak,
			Mean:    round2(a.Mean),
			StdDev:  round2(a.StdDev),
			Score:   round2(a.Score),
		})
	}
	return r
}

// health scores every service, worst first.
func (res *Result) health() []report.Health {
	inputs := make(map[string]*health.Input)
	input := func(service string) *health.Input {
		in, ok := inputs[service]
		if !ok {
			in = &health.Input{}
			inputs[service] = in
		}
		return in
	}
	for _, inc := range res.Detector.Incidents() {
		in := input(inc.Service)
		switch inc.Severity {
		case detect.Critical:
			in.Critical++
		case detect.High:
			in.High++
		case detect.Medium:
			in.Medium++
		default:
			in.Low++
		}
	}
	for _, a := range res.Anomalies.Finish() {
		if a.Service != anomaly.Overall {
			input(a.Service).Anomalies++
		}
	}

	out := make([]report.Health, 0, len(res.Stats.byService))
	for _, c := range res.Stats.Services() {
		in := input(c.Name)
		in.Entries = c.Count
		in.Errors = res.Stats.Errors(c.Name)
		score := health.Score(*in)
		out = append(out, report.Health{
			Service:   c.Name,
			Score:     score.Score,
			Status:    score.Status,
			Entries:   in.Entries,
			Errors:    in.Errors,
			ErrorRate: math.Round(float64(in.Errors)/float64(in.Entries)*1000) / 1000,
			Incidents: in.Critical + in.High + in.Medium + in.Low,
			Anomalies: in.Anomalies,
			Penalties: report.Penalties{
				Errors:    score.ErrorPenalty,
				Incidents: score.IncidentPenalty,
				Anomalies: score.AnomalyPenalty,
			},
		})
	}
	slices.SortFunc(out, func(a, b report.Health) int {
		if c := cmp.Compare(a.Score, b.Score); c != 0 {
			return c
		}
		return cmp.Compare(a.Service, b.Service)
	})
	return out
}

func (res *Result) timeline() report.Timeline {
	tl := res.Timeline
	out := report.Timeline{Bucket: tl.Width().String(), Rows: []report.TimelineRow{}}
	if tl.Len() == 0 {
		return out
	}
	start := tl.Start()
	out.Start = &start

	names := tl.Names()
	names = slices.DeleteFunc(names, func(n string) bool { return n == AllEntries })
	for _, name := range append([]string{AllEntries}, names...) {
		counts := tl.Row(name)
		out.Rows = append(out.Rows, report.TimelineRow{Name: name, Max: slices.Max(counts), Counts: counts})
	}
	return out
}

func toIncident(inc detect.Incident) report.Incident {
	return report.Incident{
		Rule:            inc.Rule,
		Severity:        inc.Severity.String(),
		Service:         inc.Service,
		Count:           inc.Count,
		FirstSeen:       inc.FirstSeen,
		LastSeen:        inc.LastSeen,
		DurationSeconds: math.Round(inc.Duration().Seconds()*1000) / 1000,
		FirstLine:       inc.FirstLine,
		Samples:         inc.Samples,
	}
}

func toCounts(counts []Count) []report.Count {
	out := make([]report.Count, 0, len(counts))
	for _, c := range counts {
		out = append(out, report.Count{Name: c.Name, Count: c.Count})
	}
	return out
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func round2(x float64) float64 {
	return math.Round(x*100) / 100
}
