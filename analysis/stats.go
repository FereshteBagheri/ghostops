package analysis

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"github.com/FereshteBagheri/ghostops/logentry"
)

const maxMalformedExamples = 5

var malformedReasons = []error{
	logentry.ErrMissingFields,
	logentry.ErrBadTimestamp,
	logentry.ErrUnknownLevel,
}

type Count struct {
	Name  string
	Count int
}

type Stats struct {
	Lines     int
	Valid     int
	Skipped   int
	Malformed int
	Filtered  int

	First time.Time
	Last  time.Time

	byLevel           map[logentry.Level]int
	byService         map[string]int
	errorsByService   map[string]int
	malformedByReason map[string]int
	malformedExamples []string
}

func NewStats() *Stats {
	return &Stats{
		byLevel:           make(map[logentry.Level]int),
		byService:         make(map[string]int),
		errorsByService:   make(map[string]int),
		malformedByReason: make(map[string]int),
		malformedExamples: make([]string, 0, maxMalformedExamples),
	}
}

func (s *Stats) Observe(e logentry.Entry) {
	s.Lines++
	s.Valid++
	s.byLevel[e.Level]++
	s.byService[e.Service]++
	if e.Level >= logentry.Error {
		s.errorsByService[e.Service]++
	}
	if s.First.IsZero() || e.Time.Before(s.First) {
		s.First = e.Time
	}
	if e.Time.After(s.Last) {
		s.Last = e.Time
	}
}

func (s *Stats) AddFiltered() {
	s.Lines++
	s.Filtered++
}

func (s *Stats) AddSkipped() {
	s.Lines++
	s.Skipped++
}

func (s *Stats) AddMalformed(err error) {
	s.Lines++
	s.Malformed++
	s.malformedByReason[reason(err)]++
	if len(s.malformedExamples) < maxMalformedExamples {
		s.malformedExamples = append(s.malformedExamples, err.Error())
	}
}

func (s *Stats) Levels() []Count {
	counts := make([]Count, 0, len(logentry.Levels()))
	for _, l := range logentry.Levels() {
		counts = append(counts, Count{Name: l.String(), Count: s.byLevel[l]})
	}
	return counts
}

func (s *Stats) Services() []Count {
	return sortedCounts(s.byService)
}

func (s *Stats) Errors(service string) int {
	return s.errorsByService[service]
}

func (s *Stats) MalformedReasons() []Count {
	return sortedCounts(s.malformedByReason)
}

func (s *Stats) MalformedExamples() []string {
	return slices.Clone(s.malformedExamples)
}

func reason(err error) string {
	for _, r := range malformedReasons {
		if errors.Is(err, r) {
			return r.Error()
		}
	}
	return "other"
}

func sortedCounts(m map[string]int) []Count {
	counts := make([]Count, 0, len(m))
	for name, n := range m {
		counts = append(counts, Count{Name: name, Count: n})
	}
	slices.SortFunc(counts, func(a, b Count) int {
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return counts
}
