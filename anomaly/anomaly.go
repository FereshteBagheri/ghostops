// Package anomaly learns how busy each service normally is and reports time
// periods that look very different from that.
//
// Time is cut into fixed buckets (one minute by default). For every service,
// and for all services together, each finished bucket is compared with the
// buckets seen before it, then added to that history.
package anomaly

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/FereshteBagheri/ghostops/baseline"
	"github.com/FereshteBagheri/ghostops/logentry"
)

const Overall = "(all services)"

// After this many empty buckets in a row, more zeros don't tell us anything new.
const maxIdleBuckets = 60

type Kind int

const (
	ErrorSpike Kind = iota + 1
	VolumeSpike
	Silence
)

func (k Kind) String() string {
	switch k {
	case ErrorSpike:
		return "error-spike"
	case VolumeSpike:
		return "volume-spike"
	case Silence:
		return "silence"
	default:
		return fmt.Sprintf("Kind(%d)", int(k))
	}
}

type Config struct {
	Bucket         time.Duration
	Warmup         int     // buckets of history needed before judging
	MinCount       int     // smaller counts are never a spike
	Threshold      float64 // z-score needed to call a bucket abnormal
	SilenceBuckets int     // empty buckets in a row that count as silence
}

func DefaultConfig() Config {
	return Config{
		Bucket:         time.Minute,
		Warmup:         10,
		MinCount:       5,
		Threshold:      3,
		SilenceBuckets: 3,
	}
}

type Anomaly struct {
	Service string
	Kind    Kind
	Start   time.Time
	End     time.Time
	Peak    int     // highest count in one bucket (0 for silence)
	Mean    float64 // normal count per bucket
	StdDev  float64
	Score   float64 // how many standard deviations away from normal
}

type series struct {
	name    string
	started bool
	bucket  time.Time
	errors  int
	other   int

	errorsBase baseline.Running
	otherBase  baseline.Running
	totalBase  baseline.Running

	open map[Kind]*Anomaly
}

type Detector struct {
	cfg      Config
	series   map[string]*series
	found    []Anomaly
	finished bool

	OutOfOrder int
}

// NewDetector fills any zero field of cfg from DefaultConfig.
func NewDetector(cfg Config) *Detector {
	def := DefaultConfig()
	if cfg.Bucket <= 0 {
		cfg.Bucket = def.Bucket
	}
	if cfg.Warmup <= 0 {
		cfg.Warmup = def.Warmup
	}
	if cfg.MinCount <= 0 {
		cfg.MinCount = def.MinCount
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = def.Threshold
	}
	if cfg.SilenceBuckets <= 0 {
		cfg.SilenceBuckets = def.SilenceBuckets
	}
	return &Detector{cfg: cfg, series: make(map[string]*series)}
}

func (d *Detector) Config() Config {
	return d.cfg
}

func (d *Detector) Observe(e logentry.Entry) {
	if d.finished {
		return
	}
	if late := d.add(d.get(e.Service), e); late {
		d.OutOfOrder++
	}
	d.add(d.get(Overall), e)
}

// Finish closes the last buckets and returns every anomaly sorted by start time.
// Entries observed after Finish are ignored.
func (d *Detector) Finish() []Anomaly {
	if !d.finished {
		d.finished = true
		for _, s := range d.series {
			d.closeBucket(s)
			for kind := range s.open {
				d.finish(s, kind)
			}
		}
		slices.SortFunc(d.found, func(a, b Anomaly) int {
			if c := a.Start.Compare(b.Start); c != 0 {
				return c
			}
			if c := cmp.Compare(a.Service, b.Service); c != 0 {
				return c
			}
			return cmp.Compare(a.Kind, b.Kind)
		})
	}
	return slices.Clone(d.found)
}

// Ready reports how many services have enough history to be judged.
func (d *Detector) Ready() (ready, total int) {
	for name, s := range d.series {
		if name == Overall {
			continue
		}
		total++
		if s.totalBase.N() >= d.cfg.Warmup {
			ready++
		}
	}
	return ready, total
}

func (d *Detector) get(name string) *series {
	s, ok := d.series[name]
	if !ok {
		s = &series{name: name, open: make(map[Kind]*Anomaly)}
		d.series[name] = s
	}
	return s
}

func (d *Detector) add(s *series, e logentry.Entry) (late bool) {
	b := e.Time.Truncate(d.cfg.Bucket)
	switch {
	case !s.started:
		s.started = true
		s.bucket = b
	case b.Before(s.bucket):
		// counted in the current bucket; we can't go back in time
		late = true
	case b.After(s.bucket):
		d.advance(s, b)
	}

	if e.Level >= logentry.Error {
		s.errors++
	} else {
		s.other++
	}
	return late
}

func (d *Detector) advance(s *series, next time.Time) {
	d.closeBucket(s)

	idle := int(next.Sub(s.bucket)/d.cfg.Bucket) - 1
	if idle >= d.cfg.SilenceBuckets {
		d.checkSilence(s, next)
	}
	for range min(idle, maxIdleBuckets) {
		s.bucket = s.bucket.Add(d.cfg.Bucket)
		d.closeBucket(s)
	}
	s.bucket = next
}

func (d *Detector) checkSilence(s *series, next time.Time) {
	base := &s.totalBase
	if base.N() < d.cfg.Warmup || base.Mean() < float64(d.cfg.MinCount) {
		return
	}
	d.found = append(d.found, Anomaly{
		Service: s.name,
		Kind:    Silence,
		Start:   s.bucket.Add(d.cfg.Bucket),
		End:     next,
		Mean:    base.Mean(),
		StdDev:  base.StdDev(),
		Score:   -base.ZScore(0, 1),
	})
}

func (d *Detector) closeBucket(s *series) {
	errSpike := d.check(s, ErrorSpike, s.errors, &s.errorsBase)
	volSpike := d.check(s, VolumeSpike, s.other, &s.otherBase)
	if !errSpike && !volSpike {
		s.totalBase.Add(float64(s.errors + s.other))
	}
	s.errors, s.other = 0, 0
}

// check compares one bucket's count with the history. Abnormal buckets are not
// added to the history, so a long outage doesn't become the new normal.
func (d *Detector) check(s *series, kind Kind, n int, base *baseline.Running) bool {
	z := base.ZScore(float64(n), 1)
	abnormal := base.N() >= d.cfg.Warmup && n >= d.cfg.MinCount && z >= d.cfg.Threshold
	if !abnormal {
		d.finish(s, kind)
		base.Add(float64(n))
		return false
	}

	a, ok := s.open[kind]
	if !ok {
		a = &Anomaly{
			Service: s.name,
			Kind:    kind,
			Start:   s.bucket,
			Mean:    base.Mean(),
			StdDev:  base.StdDev(),
		}
		s.open[kind] = a
	}
	a.End = s.bucket.Add(d.cfg.Bucket)
	a.Peak = max(a.Peak, n)
	a.Score = max(a.Score, z)
	return true
}

func (d *Detector) finish(s *series, kind Kind) {
	if a, ok := s.open[kind]; ok {
		d.found = append(d.found, *a)
		delete(s.open, kind)
	}
}
