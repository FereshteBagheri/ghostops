package analysis

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/FereshteBagheri/ghostops/detect"
	"github.com/FereshteBagheri/ghostops/linescan"
)

const input = `# comment
2026-10-20T08:00:00Z INFO api started
2026-10-20T08:00:01Z INFO db started

2026-10-20T08:01:00Z WARN api slow
2026-10-20T08:02:00Z ERROR db connection refused
2026-10-20T08:03:00Z FATAL db crashed
garbage
2026-10-20 08:04:00 ERROR api space instead of T
2026-10-20T08:05:00Z DEBUG api unsupported level
2026-10-20T08:06:00Z ERROR
2026-10-20T08:07:00Z INFO cache started
`

// stats runs Analyze with the default rules and returns only the Stats.
func stats(r io.Reader) (*Stats, error) {
	res, err := Analyze(r, Options{Rules: detect.DefaultRules()})
	if err != nil {
		return nil, err
	}
	return res.Stats, nil
}

func TestAnalyze(t *testing.T) {
	s, err := stats(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if s.Lines != 12 || s.Valid != 6 || s.Skipped != 2 || s.Malformed != 4 {
		t.Errorf("lines/valid/skipped/malformed = %d/%d/%d/%d, want 12/6/2/4",
			s.Lines, s.Valid, s.Skipped, s.Malformed)
	}

	wantLevels := []Count{{"INFO", 3}, {"WARN", 1}, {"ERROR", 1}, {"FATAL", 1}}
	if got := s.Levels(); !slices.Equal(got, wantLevels) {
		t.Errorf("Levels() = %v, want %v", got, wantLevels)
	}

	wantServices := []Count{{"db", 3}, {"api", 2}, {"cache", 1}}
	if got := s.Services(); !slices.Equal(got, wantServices) {
		t.Errorf("Services() = %v, want %v", got, wantServices)
	}

	wantReasons := []Count{{"missing fields", 2}, {"bad timestamp", 1}, {"unknown level", 1}}
	if got := s.MalformedReasons(); !slices.Equal(got, wantReasons) {
		t.Errorf("MalformedReasons() = %v, want %v", got, wantReasons)
	}

	if got := s.MalformedExamples(); len(got) != 4 || !strings.HasPrefix(got[0], "line 8:") {
		t.Errorf("MalformedExamples() = %q, want 4 examples starting at line 8", got)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	s, err := stats(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if s.Lines != 0 || len(s.Services()) != 0 || len(s.MalformedReasons()) != 0 {
		t.Errorf("empty input gave %+v", s)
	}
	for _, c := range s.Levels() {
		if c.Count != 0 {
			t.Errorf("level %s = %d, want 0", c.Name, c.Count)
		}
	}
}

func TestMalformedExamplesAreCapped(t *testing.T) {
	s, err := stats(strings.NewReader(strings.Repeat("garbage\n", 100)))
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if s.Malformed != 100 {
		t.Errorf("Malformed = %d, want 100", s.Malformed)
	}
	if n := len(s.MalformedExamples()); n != maxMalformedExamples {
		t.Errorf("kept %d examples, want %d", n, maxMalformedExamples)
	}
}

func TestServiceTiesSortedByName(t *testing.T) {
	var b strings.Builder
	for _, svc := range []string{"zeta", "alpha", "mid"} {
		fmt.Fprintf(&b, "2026-10-20T08:00:00Z INFO %s ok\n", svc)
	}
	s, _ := stats(strings.NewReader(b.String()))

	want := []Count{{"alpha", 1}, {"mid", 1}, {"zeta", 1}}
	if got := s.Services(); !slices.Equal(got, want) {
		t.Errorf("Services() = %v, want %v", got, want)
	}
}

func TestAnalyzeReadErrorIsFatal(t *testing.T) {
	errDisk := errors.New("disk on fire")
	r := io.MultiReader(strings.NewReader(input), iotest.ErrReader(errDisk))

	s, err := stats(r)
	if !errors.Is(err, errDisk) {
		t.Fatalf("Analyze() error = %v, want %v", err, errDisk)
	}
	if s != nil {
		t.Error("Analyze() returned stats together with an error")
	}
}

func TestAnalyzeLineTooLongIsFatal(t *testing.T) {
	r := strings.NewReader(strings.Repeat("x", linescan.DefaultMaxLineSize+1))
	if _, err := stats(r); !errors.Is(err, linescan.ErrLineTooLong) {
		t.Fatalf("Analyze() error = %v, want %v", err, linescan.ErrLineTooLong)
	}
}

func TestAnalyzeRunsDetector(t *testing.T) {
	res, err := Analyze(strings.NewReader(input), Options{Rules: detect.DefaultRules()})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	counts := map[string]int{}
	for _, c := range res.Detector.Counts() {
		counts[c.Rule] = c.Count
	}
	if counts["service-crash"] != 1 || counts["database-failure"] != 1 {
		t.Errorf("rule counts = %v, want 1 service-crash and 1 database-failure", counts)
	}
}
