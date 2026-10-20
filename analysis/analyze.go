package analysis

import (
	"errors"
	"io"

	"ghostops/linescan"
	"ghostops/logentry"
)

func Analyze(r io.Reader) (*Stats, error) {
	stats := NewStats()
	sc := linescan.New(r, 0)

	for sc.Scan() {
		entry, err := logentry.Parse(sc.Line(), sc.Text())
		switch {
		case err == nil:
			stats.AddEntry(entry)
		case errors.Is(err, logentry.ErrBlankOrComment):
			stats.AddSkipped()
		default:
			stats.AddMalformed(err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}
