package logentry

import (
	"bufio"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

var sentinels = []error{ErrBlankOrComment, ErrMissingFields, ErrBadTimestamp, ErrUnknownLevel}

func FuzzParse(f *testing.F) {
	file, err := os.Open("../testdata/sample.log")
	if err != nil {
		f.Fatal(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		f.Add(sc.Text())
	}
	f.Add("2026-10-20T11:35:00.123456789+03:30 FATAL\tdb   crashed  hard ")

	f.Fuzz(func(t *testing.T, text string) {
		e, err := Parse(3, text)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) || pe.Line != 3 {
				t.Fatalf("error %v is not a *ParseError for line 3", err)
			}
			n := 0
			for _, s := range sentinels {
				if errors.Is(err, s) {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("error %v wraps %d sentinels, want 1", err, n)
			}
			return
		}

		if e.Line != 3 || e.Time.Location() != time.UTC || !slices.Contains(Levels(), e.Level) {
			t.Fatalf("bad entry %+v", e)
		}
		if e.Service == "" || strings.ContainsAny(e.Service, " \t") {
			t.Fatalf("service %q is empty or has spaces", e.Service)
		}
		if e.Message == "" || e.Message != strings.TrimSpace(e.Message) {
			t.Fatalf("message %q is empty or not trimmed", e.Message)
		}
	})
}

func BenchmarkParse(b *testing.B) {
	const line = "2026-10-20T08:05:00.250Z ERROR payment-service Database connection refused: postgres:5432"
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	for range b.N {
		if _, err := Parse(1, line); err != nil {
			b.Fatal(err)
		}
	}
}
