package linescan

import (
	"slices"
	"strings"
	"testing"
)

func FuzzScan(f *testing.F) {
	for _, s := range []string{"", "a", "a\n", "a\n\nb", "a\r\nb\r\n", "\r", "\r\r\n", "x\n\n\n"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		s := New(strings.NewReader(input), 0)
		got, numbers := readAll(s)
		if err := s.Err(); err != nil {
			t.Fatalf("Err() = %v", err)
		}

		var want []string
		if input != "" {
			want = strings.Split(strings.TrimSuffix(input, "\n"), "\n")
			for i, l := range want {
				want[i] = strings.TrimSuffix(l, "\r")
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("lines = %q, want %q", got, want)
		}
		for i, n := range numbers {
			if n != i+1 {
				t.Fatalf("line number %d = %d, want %d", i, n, i+1)
			}
		}
	})
}
