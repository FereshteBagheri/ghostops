package detect

import (
	"fmt"
	"strings"
)

type Severity int

const (
	Low Severity = iota + 1
	Medium
	High
	Critical
)

func (s Severity) String() string {
	switch s {
	case Low:
		return "LOW"
	case Medium:
		return "MEDIUM"
	case High:
		return "HIGH"
	case Critical:
		return "CRITICAL"
	default:
		return fmt.Sprintf("Severity(%d)", int(s))
	}
}

func ParseSeverity(s string) (Severity, error) {
	for _, sev := range []Severity{Low, Medium, High, Critical} {
		if strings.EqualFold(s, sev.String()) {
			return sev, nil
		}
	}
	return 0, fmt.Errorf("unknown severity %q (want low, medium, high or critical)", s)
}
