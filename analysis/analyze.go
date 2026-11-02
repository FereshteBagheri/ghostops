package analysis

import (
	"errors"
	"io"
	"time"

	"github.com/FereshteBagheri/ghostops/anomaly"
	"github.com/FereshteBagheri/ghostops/detect"
	"github.com/FereshteBagheri/ghostops/linescan"
	"github.com/FereshteBagheri/ghostops/logentry"
	"github.com/FereshteBagheri/ghostops/timeline"
)

const AllEntries = "(all entries)"

const timelineBuckets = 60

type Observer interface {
	Observe(e logentry.Entry)
}

type Options struct {
	Rules   []detect.Rule
	Window  time.Duration
	Anomaly anomaly.Config
	Filter  Filter
}

type Result struct {
	Stats     *Stats
	Detector  *detect.Detector
	Anomalies *anomaly.Detector
	Timeline  *timeline.Timeline
	Filter    Filter
}

type errorTimeline struct {
	*timeline.Timeline
}

func (t errorTimeline) Observe(e logentry.Entry) {
	t.Add(AllEntries, e.Time)
	if e.Level >= logentry.Error {
		t.Add(e.Service, e.Time)
	} else {
		t.Touch(e.Service)
	}
}

func Analyze(r io.Reader, opts Options) (*Result, error) {
	res := &Result{
		Stats:     NewStats(),
		Detector:  detect.NewDetector(opts.Rules, opts.Window),
		Anomalies: anomaly.NewDetector(opts.Anomaly),
		Timeline:  timeline.New(time.Minute, timelineBuckets),
		Filter:    opts.Filter,
	}
	observers := []Observer{res.Stats, res.Detector, res.Anomalies, errorTimeline{res.Timeline}}

	sc := linescan.New(r, 0)
	for sc.Scan() {
		entry, err := logentry.Parse(sc.Line(), sc.Text())
		switch {
		case err == nil && !opts.Filter.Match(entry):
			res.Stats.AddFiltered()
		case err == nil:
			for _, o := range observers {
				o.Observe(entry)
			}
		case errors.Is(err, logentry.ErrBlankOrComment):
			res.Stats.AddSkipped()
		default:
			res.Stats.AddMalformed(err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	res.Anomalies.Finish()
	return res, nil
}
