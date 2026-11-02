package analysis

import (
	"slices"
	"time"

	"ghostops/logentry"
)

// Filter limits the analysis to some services and/or a time range.
// The zero Filter lets everything through.
type Filter struct {
	Services []string
	Since    time.Time // inclusive
	Until    time.Time // exclusive
}

func (f Filter) Match(e logentry.Entry) bool {
	if len(f.Services) > 0 && !slices.Contains(f.Services, e.Service) {
		return false
	}
	if !f.Since.IsZero() && e.Time.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && !e.Time.Before(f.Until) {
		return false
	}
	return true
}
