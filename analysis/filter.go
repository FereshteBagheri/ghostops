package analysis

import (
	"slices"
	"time"

	"github.com/FereshteBagheri/ghostops/logentry"
)

type Filter struct {
	Services []string
	Since    time.Time
	Until    time.Time
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
