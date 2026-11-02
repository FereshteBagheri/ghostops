package detect

import (
	"strings"

	"ghostops/logentry"
)

type Rule interface {
	Name() string
	Severity() Severity
	Match(e logentry.Entry) bool
}

// KeywordRule matches entries at minLevel or above whose message contains one
// of the keywords (case-insensitive). Without keywords every message matches.
type KeywordRule struct {
	name     string
	severity Severity
	minLevel logentry.Level
	keywords []string
}

func NewKeywordRule(name string, severity Severity, minLevel logentry.Level, keywords ...string) KeywordRule {
	lower := make([]string, len(keywords))
	for i, k := range keywords {
		lower[i] = strings.ToLower(k)
	}
	return KeywordRule{name: name, severity: severity, minLevel: minLevel, keywords: lower}
}

func (r KeywordRule) Name() string {
	return r.name
}

func (r KeywordRule) Severity() Severity {
	return r.severity
}

func (r KeywordRule) Match(e logentry.Entry) bool {
	if e.Level < r.minLevel {
		return false
	}
	if len(r.keywords) == 0 {
		return true
	}
	msg := strings.ToLower(e.Message)
	for _, k := range r.keywords {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}

// DefaultRules are in priority order: when several rules match, the first one wins.
func DefaultRules() []Rule {
	return []Rule{
		NewKeywordRule("service-crash", Critical, logentry.Fatal),
		NewKeywordRule("out-of-memory", Critical, logentry.Error, "out of memory", "oom"),
		NewKeywordRule("database-failure", High, logentry.Error, "database", "deadlock", "connection refused"),
		NewKeywordRule("auth-failure", Medium, logentry.Warn, "unauthorized", "authentication failed", "permission denied"),
		NewKeywordRule("timeout", Medium, logentry.Warn, "timeout", "timed out"),
		NewKeywordRule("error-burst", Low, logentry.Error),
	}
}
