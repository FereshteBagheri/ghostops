package logentry

import (
	"fmt"
)

type Level int

const (
	Info Level = iota + 1
	Warn
	Error
	Fatal
)

func Levels() []Level {
	return []Level{Info, Warn, Error, Fatal}
}

func (l Level) String() string {
	switch l {
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	case Fatal:
		return "FATAL"
	default:
		return fmt.Sprintf("Level(%d)", int(l))
	}
}

func ParseLevel(s string) (Level, error) {
	for _, l := range Levels() {
		if s == l.String() {
			return l, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrUnknownLevel, s)
}
