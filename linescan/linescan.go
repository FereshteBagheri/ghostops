package linescan

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const DefaultMaxLineSize = 1024 * 1024

var ErrLineTooLong = errors.New("line too long")

type Scanner struct {
	sc          *bufio.Scanner
	maxLineSize int
	line        int
	text        string
	err         error
}

func New(r io.Reader, maxLineSize int) *Scanner {
	if maxLineSize <= 0 {
		maxLineSize = DefaultMaxLineSize
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(nil, maxLineSize+2)
	return &Scanner{sc: sc, maxLineSize: maxLineSize}
}

func (s *Scanner) Scan() bool {
	if s.err != nil {
		return false
	}
	if !s.sc.Scan() {
		s.text = ""
		if err := s.sc.Err(); err != nil {
			s.err = s.lineError(s.line+1, err)
		}
		return false
	}

	s.line++
	s.text = s.sc.Text()

	if len(s.text) > s.maxLineSize {
		s.text = ""
		s.err = s.lineError(s.line, bufio.ErrTooLong)
		return false
	}
	return true
}

func (s *Scanner) Text() string {
	return s.text
}

func (s *Scanner) Line() int {
	return s.line
}

func (s *Scanner) Err() error {
	return s.err
}

func (s *Scanner) lineError(line int, err error) error {
	if errors.Is(err, bufio.ErrTooLong) {
		return fmt.Errorf("line %d: %w (limit is %d bytes)", line, ErrLineTooLong, s.maxLineSize)
	}
	return fmt.Errorf("line %d: %w", line, err)
}
