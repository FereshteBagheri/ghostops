package report

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type Text struct{}

type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

func (Text) Render(w io.Writer, r Report) error {
	p := &printer{w: w}
	clock := clockLayout(r.Summary)

	p.printf("GhostOps report for %s\n", r.Source)
	if r.Summary.FirstEntry != nil {
		first, last := *r.Summary.FirstEntry, *r.Summary.LastEntry
		p.printf("Entries from %s to %s UTC (%s)\n",
			first.Format(time.DateTime), last.Format(clock), shortDuration(last.Sub(first).Round(time.Second)))
	}
	writeSettings(p, r.Settings)

	s := r.Summary
	p.printf("\nLines: %d total, %d valid, %d malformed, %d blank or comment", s.Lines, s.Valid, s.Malformed, s.Skipped)
	if s.FilteredOut > 0 {
		p.printf(", %d filtered out", s.FilteredOut)
	}
	p.printf("\n")
	if s.Valid == 0 {
		p.printf("No valid entries.\n")
	}

	writeHealth(p, r.Health)

	p.printf("\nLevels\n")
	writeCounts(p, r.Levels)

	p.printf("\nServices\n")
	if len(r.Services) == 0 {
		p.printf("  (none)\n")
	}
	writeCounts(p, r.Services)

	if len(r.Malformed.ByReason) > 0 {
		p.printf("\nMalformed lines\n")
		writeCounts(p, r.Malformed.ByReason)
		p.printf("  first examples:\n")
		for _, ex := range r.Malformed.Examples {
			p.printf("    %s\n", ex)
		}
	}

	writeIncidents(p, r.Incidents, clock)
	writeTimeline(p, r.Timeline, clock)
	writeAnomalies(p, r.Anomalies, r.Settings.AnomalyBucket)
	return p.err
}

func writeSettings(p *printer, s Settings) {
	if len(s.Services) > 0 {
		p.printf("Only services: %v\n", s.Services)
	}
	if s.Since != "" {
		p.printf("Since: %s\n", s.Since)
	}
	if s.Until != "" {
		p.printf("Until: %s\n", s.Until)
	}
}

func writeCounts(p *printer, counts []Count) {
	nameW, numW := 0, 0
	for _, c := range counts {
		nameW = max(nameW, len(c.Name))
		numW = max(numW, len(fmt.Sprint(c.Count)))
	}
	for _, c := range counts {
		p.printf("  %-*s  %*d\n", nameW, c.Name, numW, c.Count)
	}
}

func writeIncidents(p *printer, inc Incidents, clock string) {
	if len(inc.Groups) == 0 {
		p.printf("\nIncidents\n  No incidents detected.\n")
		return
	}

	entries := 0
	for _, g := range inc.Groups {
		entries += g.Count
	}
	p.printf("\nIncidents: %d from %s", len(inc.Groups), plural(entries, "entry", "entries"))
	if inc.OverlappingEntries > 0 {
		p.printf(" (%s matched more than one rule; the first rule wins)",
			plural(inc.OverlappingEntries, "entry", "entries"))
	}
	p.printf("\n")

	ruleW, svcW, countW, durW := 0, 0, 0, 0
	for _, g := range inc.Groups {
		ruleW = max(ruleW, len(g.Rule))
		svcW = max(svcW, len(g.Service))
		countW = max(countW, len(plural(g.Count, "entry", "entries")))
		durW = max(durW, len(incidentDuration(g)))
	}
	for _, g := range inc.Groups {
		p.printf("  %-8s  %-*s  %-*s  %-*s  %s - %s  %-*s  line %d\n",
			g.Severity, ruleW, g.Rule, svcW, g.Service, countW, plural(g.Count, "entry", "entries"),
			g.FirstSeen.Format(clock), g.LastSeen.Format(clock), durW, incidentDuration(g), g.FirstLine)
		for _, msg := range g.Samples {
			p.printf("            %s\n", msg)
		}
	}
}

func writeHealth(p *printer, health []Health) {
	if len(health) == 0 {
		return
	}
	p.printf("\nService health (0-100, worst first)\n")

	nameW, entW, errW := 0, 0, 0
	for _, h := range health {
		nameW = max(nameW, len(h.Service))
		entW = max(entW, len(plural(h.Entries, "entry", "entries")))
		errW = max(errW, len(plural(h.Errors, "error", "errors")))
	}
	for _, h := range health {
		p.printf("  %-*s  %3d  %-9s  %-*s  %-*s  %5s  %-11s  %-11s",
			nameW, h.Service, h.Score, h.Status,
			entW, plural(h.Entries, "entry", "entries"),
			errW, plural(h.Errors, "error", "errors"), fmt.Sprintf("%.1f%%", h.ErrorRate*100),
			plural(h.Incidents, "incident", "incidents"), plural(h.Anomalies, "anomaly", "anomalies"))

		var lost []string
		if h.Penalties.Errors > 0 {
			lost = append(lost, fmt.Sprintf("-%d errors", h.Penalties.Errors))
		}
		if h.Penalties.Incidents > 0 {
			lost = append(lost, fmt.Sprintf("-%d incidents", h.Penalties.Incidents))
		}
		if h.Penalties.Anomalies > 0 {
			lost = append(lost, fmt.Sprintf("-%d anomalies", h.Penalties.Anomalies))
		}
		if len(lost) > 0 {
			p.printf("  (%s)", strings.Join(lost, ", "))
		}
		p.printf("\n")
	}
}

func writeTimeline(p *printer, tl Timeline, clock string) {
	if len(tl.Rows) == 0 {
		return
	}
	bucket := tl.Bucket
	width, err := time.ParseDuration(bucket)
	if err == nil {
		bucket = shortDuration(width)
	}
	end := tl.Start.Add(time.Duration(len(tl.Rows[0].Counts)) * width)
	p.printf("\nTimeline from %s to %s, one column = %s (first row: all entries, then errors per service)\n",
		tl.Start.Format(clock), end.Format(clock), bucket)

	nameW := 0
	for _, row := range tl.Rows {
		nameW = max(nameW, len(row.Name))
	}
	for _, row := range tl.Rows {
		p.printf("  %-*s  %s  max %d\n", nameW, row.Name, sparkline(row.Counts, row.Max), row.Max)
	}
}

var sparks = []rune("▁▂▃▄▅▆▇█")

func sparkline(counts []int, max int) string {
	var b strings.Builder
	for _, n := range counts {
		if n <= 0 || max <= 0 {
			b.WriteRune('·')
			continue
		}
		i := (n*len(sparks) - 1) / max
		b.WriteRune(sparks[min(i, len(sparks)-1)])
	}
	return b.String()
}

func writeAnomalies(p *printer, a Anomalies, bucket string) {
	if d, err := time.ParseDuration(bucket); err == nil {
		bucket = shortDuration(d)
	}
	p.printf("\nAnomalies (compared with each service's own history, %s buckets)\n", bucket)
	switch {
	case a.TotalServices == 0:
		p.printf("  Nothing to learn from.\n")
		return
	case a.ReadyServices == 0:
		p.printf("  Not enough history yet: a service needs %d buckets of logs before it can be judged.\n", a.WarmupBuckets)
		return
	}
	p.printf("  %d of %d services had enough history.\n", a.ReadyServices, a.TotalServices)
	if a.OutOfOrder > 0 {
		p.printf("  %s were out of time order and counted in a later bucket.\n",
			plural(a.OutOfOrder, "entry", "entries"))
	}
	if len(a.Found) == 0 {
		p.printf("  No anomalies found.\n")
		return
	}

	kindW, svcW := 0, 0
	for _, an := range a.Found {
		kindW = max(kindW, len(an.Kind))
		svcW = max(svcW, len(an.Service))
	}
	layout := "15:04"
	if len(a.Found) > 0 && a.Found[0].Start.YearDay() != a.Found[len(a.Found)-1].End.YearDay() {
		layout = "01-02 15:04"
	}
	for _, an := range a.Found {
		p.printf("  %-*s  %-*s  %s - %s  ", kindW, an.Kind, svcW, an.Service,
			an.Start.Format(layout), an.End.Format(layout))
		if an.Kind == "silence" {
			p.printf("no logs, normally %.1f per bucket\n", an.Mean)
		} else {
			p.printf("peak %d per bucket, normally %.1f ± %.1f (z=%.1f)\n", an.Peak, an.Mean, an.StdDev, an.Score)
		}
	}
}

func incidentDuration(g Incident) string {
	d := time.Duration(g.DurationSeconds * float64(time.Second)).Round(time.Second)
	return shortDuration(d)
}

func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func clockLayout(s Summary) string {
	if s.FirstEntry == nil {
		return time.TimeOnly
	}
	y1, m1, d1 := s.FirstEntry.Date()
	y2, m2, d2 := s.LastEntry.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return time.TimeOnly
	}
	return time.DateTime
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
