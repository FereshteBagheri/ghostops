package logentry

import (
	"errors"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Entry
	}{
		{
			name: "simple",
			text: "2026-10-20T08:05:00Z ERROR payment-service Database connection refused",
			want: Entry{Line: 7, Time: time.Date(2026, 10, 20, 8, 5, 0, 0, time.UTC), Level: Error, Service: "payment-service", Message: "Database connection refused"},
		},
		{
			name: "extra spaces between fields, spaces inside message kept",
			text: "  2026-10-20T08:05:00Z   WARN\tapi   Slow  response   ",
			want: Entry{Line: 7, Time: time.Date(2026, 10, 20, 8, 5, 0, 0, time.UTC), Level: Warn, Service: "api", Message: "Slow  response"},
		},
		{
			name: "time zone offset is converted to UTC",
			text: "2026-10-20T11:35:00+03:30 FATAL db crashed",
			want: Entry{Line: 7, Time: time.Date(2026, 10, 20, 8, 5, 0, 0, time.UTC), Level: Fatal, Service: "db", Message: "crashed"},
		},
		{
			name: "fractional seconds",
			text: "2026-10-20T08:05:00.250Z INFO api ok",
			want: Entry{Line: 7, Time: time.Date(2026, 10, 20, 8, 5, 0, 250_000_000, time.UTC), Level: Info, Service: "api", Message: "ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(7, tt.text)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Parse() =\n %+v\nwant\n %+v", got, tt.want)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name string
		text string
		want error
	}{
		{"empty", "", ErrBlankOrComment},
		{"only spaces", "   \t ", ErrBlankOrComment},
		{"comment", "# Burst 1", ErrBlankOrComment},
		{"one word", "garbage", ErrMissingFields},
		{"no service or message", "2026-10-20T08:12:45Z ERROR", ErrMissingFields},
		{"no message", "2026-10-20T08:12:45Z ERROR api", ErrMissingFields},
		{"space instead of T", "2026-10-20 08:12:00 ERROR api message", ErrBadTimestamp},
		{"not a time", "not-a-time INFO api message", ErrBadTimestamp},
		{"plain sentence", "this line is not a log entry at all", ErrBadTimestamp},
		{"unsupported level", "2026-10-20T08:12:50Z DEBUG api message", ErrUnknownLevel},
		{"lowercase level", "2026-10-20T08:12:50Z error api message", ErrUnknownLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(33, tt.text)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Parse() error = %v, want %v", err, tt.want)
			}
			var perr *ParseError
			if !errors.As(err, &perr) {
				t.Fatalf("Parse() error is %T, want *ParseError", err)
			}
			if perr.Line != 33 {
				t.Errorf("ParseError.Line = %d, want 33", perr.Line)
			}
		})
	}
}

func TestParseErrorMessage(t *testing.T) {
	_, err := Parse(34, "2026-10-20T08:12:30Z CRITICAL payment-service Unknown severity level")
	want := `line 34: unknown level: "CRITICAL"`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestLevel(t *testing.T) {
	for _, l := range Levels() {
		got, err := ParseLevel(l.String())
		if err != nil || got != l {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", l.String(), got, err, l)
		}
	}
	if !(Info < Warn && Warn < Error && Error < Fatal) {
		t.Error("levels are not ordered by severity")
	}
	var zero Level
	if zero.String() != "Level(0)" {
		t.Errorf("zero Level = %q, want it to be invalid", zero.String())
	}
}
