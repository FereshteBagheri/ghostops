package logentry

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrBlankOrComment = errors.New("blank or comment line")
	ErrMissingFields  = errors.New("missing fields")
	ErrBadTimestamp   = errors.New("bad timestamp")
	ErrUnknownLevel   = errors.New("unknown level")
)

type Entry struct {
	Line    int
	Time    time.Time
	Level   Level
	Service string
	Message string
}

type ParseError struct {
	Line int
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %v", e.Line, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

func Parse(line int, text string) (Entry, error) {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "#") {
		return Entry{}, &ParseError{Line: line, Err: ErrBlankOrComment}
	}

	ts, rest := nextField(text)
	lvl, rest := nextField(rest)
	service, rest := nextField(rest)
	message := strings.TrimSpace(rest)
	if message == "" {
		return Entry{}, &ParseError{Line: line, Err: fmt.Errorf("%w: want <timestamp> <level> <service> <message>", ErrMissingFields)}
	}

	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return Entry{}, &ParseError{Line: line, Err: fmt.Errorf("%w: %q", ErrBadTimestamp, ts)}
	}

	level, err := ParseLevel(lvl)
	if err != nil {
		return Entry{}, &ParseError{Line: line, Err: err}
	}

	return Entry{
		Line:    line,
		Time:    t.UTC(),
		Level:   level,
		Service: service,
		Message: message,
	}, nil
}

func nextField(s string) (field, rest string) {
	s = strings.TrimLeft(s, " \t")
	i := strings.IndexAny(s, " \t")
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}
