package linescan

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

func readAll(s *Scanner) (lines []string, numbers []int) {
	for s.Scan() {
		lines = append(lines, s.Text())
		numbers = append(numbers, s.Line())
	}
	return lines, numbers
}

func TestScan(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty input", "", nil},
		{"one line without newline", "a", []string{"a"}},
		{"one line with newline", "a\n", []string{"a"}},
		{"blank lines are kept", "a\n\nb\n", []string{"a", "", "b"}},
		{"windows line endings", "a\r\nb\r\n", []string{"a", "b"}},
		{"spaces are not trimmed", "  a  \n", []string{"  a  "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(strings.NewReader(tt.input), 0)
			got, numbers := readAll(s)

			if err := s.Err(); err != nil {
				t.Fatalf("Err() = %v, want nil", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("lines = %q, want %q", got, tt.want)
			}
			for i, n := range numbers {
				if n != i+1 {
					t.Errorf("line number %d = %d, want %d", i, n, i+1)
				}
			}
		})
	}
}

func TestMaxLineSize(t *testing.T) {
	const max = 10

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantRead int
	}{
		{"exactly max", strings.Repeat("x", max) + "\n", false, 1},
		{"exactly max with CRLF", strings.Repeat("x", max) + "\r\n", false, 1},
		{"exactly max at EOF", strings.Repeat("x", max), false, 1},
		{"one byte over", strings.Repeat("x", max+1) + "\n", true, 0},
		{"one byte over at EOF", strings.Repeat("x", max+1), true, 0},
		{"far over", strings.Repeat("x", max*100) + "\n", true, 0},
		{"long line after good lines", "ok\nok\n" + strings.Repeat("x", max*100) + "\nok\n", true, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(strings.NewReader(tt.input), max)
			lines, _ := readAll(s)

			if len(lines) != tt.wantRead {
				t.Errorf("read %d lines, want %d", len(lines), tt.wantRead)
			}
			err := s.Err()
			if tt.wantErr != (err != nil) {
				t.Fatalf("Err() = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrLineTooLong) {
				t.Errorf("Err() = %v, want it to wrap ErrLineTooLong", err)
			}
		})
	}
}

func TestErrorNamesLine(t *testing.T) {
	s := New(strings.NewReader("ok\nok\n"+strings.Repeat("x", 50)), 10)
	readAll(s)

	want := "line 3: line too long (limit is 10 bytes)"
	if err := s.Err(); err == nil || err.Error() != want {
		t.Errorf("Err() = %v, want %q", err, want)
	}
}

func TestReadError(t *testing.T) {
	errDisk := errors.New("disk on fire")
	r := io.MultiReader(strings.NewReader("a\nb\n"), iotest.ErrReader(errDisk))

	s := New(r, 0)
	lines, _ := readAll(s)

	if !slices.Equal(lines, []string{"a", "b"}) {
		t.Errorf("lines = %q, want [a b]", lines)
	}
	err := s.Err()
	if !errors.Is(err, errDisk) {
		t.Fatalf("Err() = %v, want it to wrap %v", err, errDisk)
	}
	if !strings.HasPrefix(err.Error(), "line 3:") {
		t.Errorf("Err() = %q, want it to start with %q", err, "line 3:")
	}
	if s.Scan() {
		t.Error("Scan() after an error = true, want false")
	}
}
