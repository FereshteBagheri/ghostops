package analysis

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FereshteBagheri/ghostops/detect"
)

func FuzzAnalyze(f *testing.F) {
	f.Add(input)
	f.Add("2026-10-20T09:00:00Z ERROR api timeout\n2026-10-20T08:00:00Z FATAL api out of memory\n")
	f.Add("0001-01-01T00:00:00Z INFO a b\n9999-12-31T23:59:59Z ERROR a database\n")

	since := time.Date(2026, 10, 20, 8, 1, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, in string) {
		for _, filter := range []Filter{{}, {Services: []string{"api"}, Since: since}} {
			res, err := Analyze(strings.NewReader(in), Options{Rules: detect.DefaultRules(), Filter: filter})
			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}
			s := res.Stats
			if s.Lines != s.Valid+s.Skipped+s.Malformed+s.Filtered {
				t.Fatalf("lines %d != valid %d + skipped %d + malformed %d + filtered %d",
					s.Lines, s.Valid, s.Skipped, s.Malformed, s.Filtered)
			}
			res.Report("fuzz")
		}
	})
}

func BenchmarkAnalyze(b *testing.B) {
	data, err := os.ReadFile("../testdata/anomaly.log")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for range b.N {
		if _, err := Analyze(bytes.NewReader(data), Options{Rules: detect.DefaultRules()}); err != nil {
			b.Fatal(err)
		}
	}
}
