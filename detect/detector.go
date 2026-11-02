package detect

import (
	"cmp"
	"slices"
	"time"

	"github.com/FereshteBagheri/ghostops/logentry"
)

const (
	DefaultWindow = 5 * time.Minute
	maxSamples    = 3
)

type RuleCount struct {
	Rule     string
	Severity Severity
	Count    int
}

type Incident struct {
	Rule      string
	Severity  Severity
	Service   string
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
	FirstLine int
	Samples   []string
}

func (i Incident) Duration() time.Duration {
	return i.LastSeen.Sub(i.FirstSeen)
}

// Classify returns the first matching rule (nil if none) and how many rules matched.
func Classify(e logentry.Entry, rules []Rule) (Rule, int) {
	var first Rule
	matches := 0
	for _, r := range rules {
		if !r.Match(e) {
			continue
		}
		if first == nil {
			first = r
		}
		matches++
	}
	return first, matches
}

type groupKey struct {
	rule    string
	service string
}

// Detector groups matching entries into incidents per rule and service.
// A new incident starts after window has passed without a matching entry.
type Detector struct {
	rules       []Rule
	window      time.Duration
	open        map[groupKey]*Incident
	closed      []Incident
	counts      map[string]int
	Overlapping int
}

// NewDetector uses DefaultWindow if window is not positive.
func NewDetector(rules []Rule, window time.Duration) *Detector {
	if window <= 0 {
		window = DefaultWindow
	}
	return &Detector{
		rules:  rules,
		window: window,
		open:   make(map[groupKey]*Incident),
		counts: make(map[string]int),
	}
}

func (d *Detector) Observe(e logentry.Entry) {
	rule, matches := Classify(e, d.rules)
	if rule == nil {
		return
	}
	d.counts[rule.Name()]++
	if matches > 1 {
		d.Overlapping++
	}

	key := groupKey{rule: rule.Name(), service: e.Service}
	inc, ok := d.open[key]
	if ok && e.Time.Sub(inc.LastSeen) > d.window {
		d.closed = append(d.closed, *inc)
		ok = false
	}
	if !ok {
		inc = &Incident{
			Rule:      rule.Name(),
			Severity:  rule.Severity(),
			Service:   e.Service,
			FirstSeen: e.Time,
			LastSeen:  e.Time,
			FirstLine: e.Line,
		}
		d.open[key] = inc
	}

	inc.Count++
	if e.Time.Before(inc.FirstSeen) {
		inc.FirstSeen = e.Time
	}
	if e.Time.After(inc.LastSeen) {
		inc.LastSeen = e.Time
	}
	if len(inc.Samples) < maxSamples && !slices.Contains(inc.Samples, e.Message) {
		inc.Samples = append(inc.Samples, e.Message)
	}
}

func (d *Detector) Window() time.Duration {
	return d.window
}

func (d *Detector) Counts() []RuleCount {
	counts := make([]RuleCount, 0, len(d.rules))
	for _, r := range d.rules {
		counts = append(counts, RuleCount{Rule: r.Name(), Severity: r.Severity(), Count: d.counts[r.Name()]})
	}
	return counts
}

// Incidents are sorted by severity, then time, service and rule.
func (d *Detector) Incidents() []Incident {
	incidents := make([]Incident, 0, len(d.closed)+len(d.open))
	for _, inc := range d.closed {
		incidents = append(incidents, copyIncident(inc))
	}
	for _, inc := range d.open {
		incidents = append(incidents, copyIncident(*inc))
	}

	slices.SortFunc(incidents, func(a, b Incident) int {
		if c := cmp.Compare(b.Severity, a.Severity); c != 0 {
			return c
		}
		if c := a.FirstSeen.Compare(b.FirstSeen); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Service, b.Service); c != 0 {
			return c
		}
		return cmp.Compare(a.Rule, b.Rule)
	})
	return incidents
}

func copyIncident(inc Incident) Incident {
	inc.Samples = slices.Clone(inc.Samples)
	return inc
}
