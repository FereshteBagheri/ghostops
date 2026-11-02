package anomaly

import (
	"testing"
	"time"

	"github.com/FereshteBagheri/ghostops/logentry"
)

var t0 = time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)

type stream struct {
	entries []logentry.Entry
}

func (s *stream) minute(m int, service string, level logentry.Level, n int) *stream {
	for i := range n {
		at := t0.Add(time.Duration(m)*time.Minute + time.Duration(i)*time.Second)
		s.entries = append(s.entries, logentry.Entry{Time: at, Level: level, Service: service, Message: "x"})
	}
	return s
}

func (s *stream) steady(from, to int, service string) *stream {
	for m := from; m < to; m++ {
		s.minute(m, service, logentry.Info, 8+m%5)
		s.minute(m, service, logentry.Error, m%2)
	}
	return s
}

func (s *stream) run() ([]Anomaly, *Detector) {
	d := NewDetector(DefaultConfig())
	for _, e := range s.entries {
		d.Observe(e)
	}
	return d.Finish(), d
}

func find(found []Anomaly, service string, kind Kind) []Anomaly {
	var out []Anomaly
	for _, a := range found {
		if a.Service == service && a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

func TestSteadyTrafficIsNormal(t *testing.T) {
	found, d := new(stream).steady(0, 60, "api").run()
	if len(found) != 0 {
		t.Errorf("got %+v, want no anomalies", found)
	}
	if ready, total := d.Ready(); ready != 1 || total != 1 {
		t.Errorf("Ready() = %d, %d; want 1, 1", ready, total)
	}
}

func TestErrorSpike(t *testing.T) {
	s := new(stream).steady(0, 30, "pay")
	s.minute(30, "pay", logentry.Info, 10).minute(30, "pay", logentry.Error, 20)
	s.steady(31, 40, "pay")

	found, _ := s.run()
	spikes := find(found, "pay", ErrorSpike)
	if len(spikes) != 1 {
		t.Fatalf("got %+v, want one error spike for pay", found)
	}
	a := spikes[0]
	if !a.Start.Equal(t0.Add(30*time.Minute)) || !a.End.Equal(t0.Add(31*time.Minute)) || a.Peak != 20 {
		t.Errorf("spike = %+v, want 08:30-08:31 with peak 20", a)
	}
	if a.Mean > 1 || a.Score < 3 {
		t.Errorf("mean=%v score=%v, want mean below 1 and score of at least 3", a.Mean, a.Score)
	}
	if len(find(found, "pay", VolumeSpike)) != 0 {
		t.Error("errors should not count as a volume spike")
	}
	if len(find(found, Overall, ErrorSpike)) != 1 {
		t.Errorf("got %+v, want the spike to show in the overall series too", found)
	}
}

func TestConsecutiveBucketsMerge(t *testing.T) {
	s := new(stream).steady(0, 20, "pay")
	for m := 20; m < 23; m++ {
		s.minute(m, "pay", logentry.Error, 15+m)
	}
	s.steady(23, 30, "pay")

	spikes := find(must(s.run()), "pay", ErrorSpike)
	if len(spikes) != 1 {
		t.Fatalf("got %d spikes, want 1", len(spikes))
	}
	if d := spikes[0].End.Sub(spikes[0].Start); d != 3*time.Minute || spikes[0].Peak != 37 {
		t.Errorf("spike lasts %v with peak %d, want 3m and 37", d, spikes[0].Peak)
	}
}

func TestVolumeSpike(t *testing.T) {
	s := new(stream).steady(0, 20, "api")
	s.minute(20, "api", logentry.Info, 80)
	s.steady(21, 25, "api")

	found := must(s.run())
	if len(find(found, "api", VolumeSpike)) != 1 || len(find(found, "api", ErrorSpike)) != 0 {
		t.Errorf("got %+v, want only a volume spike", found)
	}
}

func TestSilence(t *testing.T) {
	s := new(stream).steady(0, 20, "order").steady(25, 30, "order")

	silences := find(must(s.run()), "order", Silence)
	if len(silences) != 1 {
		t.Fatalf("got %+v, want one silence", silences)
	}
	a := silences[0]
	if !a.Start.Equal(t0.Add(20*time.Minute)) || !a.End.Equal(t0.Add(25*time.Minute)) {
		t.Errorf("silence %v - %v, want 08:20 - 08:25", a.Start, a.End)
	}
}

func TestShortGapIsNotSilence(t *testing.T) {
	s := new(stream).steady(0, 20, "order").steady(22, 30, "order")
	if found := find(must(s.run()), "order", Silence); len(found) != 0 {
		t.Errorf("got %+v, want no silence for a 2 minute gap", found)
	}
}

func TestQuietServiceIsNeverSilent(t *testing.T) {
	s := new(stream)
	for m := 0; m < 60; m += 10 {
		s.minute(m, "cron", logentry.Info, 1)
	}
	if found := must(s.run()); len(found) != 0 {
		t.Errorf("got %+v, want nothing for a service that is always quiet", found)
	}
}

func TestNeedsWarmup(t *testing.T) {
	s := new(stream).steady(0, 5, "pay")
	s.minute(5, "pay", logentry.Error, 50)

	found, d := s.run()
	if len(found) != 0 {
		t.Errorf("got %+v, want nothing before warm-up", found)
	}
	if ready, total := d.Ready(); ready != 0 || total != 1 {
		t.Errorf("Ready() = %d, %d; want 0, 1", ready, total)
	}
}

func TestOnlyTheSpikingServiceIsReported(t *testing.T) {
	s := new(stream).steady(0, 30, "api").steady(0, 30, "pay")
	s.minute(30, "pay", logentry.Error, 25)
	s.steady(30, 35, "api").steady(31, 35, "pay")

	found := must(s.run())
	if len(find(found, "api", ErrorSpike)) != 0 {
		t.Errorf("api should be normal, got %+v", found)
	}
	if len(find(found, "pay", ErrorSpike)) != 1 {
		t.Errorf("pay should have a spike, got %+v", found)
	}
}

func TestOutOfOrder(t *testing.T) {
	d := NewDetector(Config{})
	d.Observe(logentry.Entry{Time: t0.Add(5 * time.Minute), Level: logentry.Info, Service: "api"})
	d.Observe(logentry.Entry{Time: t0.Add(1 * time.Minute), Level: logentry.Info, Service: "api"})
	d.Observe(logentry.Entry{Time: t0.Add(5 * time.Minute), Level: logentry.Info, Service: "api"})
	if d.OutOfOrder != 1 {
		t.Errorf("OutOfOrder = %d, want 1", d.OutOfOrder)
	}
}

func TestFinishTwice(t *testing.T) {
	s := new(stream).steady(0, 30, "pay")
	s.minute(30, "pay", logentry.Error, 40)
	_, d := s.run()
	if a, b := d.Finish(), d.Finish(); len(a) != len(b) || len(a) == 0 {
		t.Errorf("Finish() gave %d then %d anomalies", len(a), len(b))
	}
}

func TestNoEntries(t *testing.T) {
	d := NewDetector(DefaultConfig())
	if found := d.Finish(); len(found) != 0 {
		t.Errorf("got %+v", found)
	}
	if ready, total := d.Ready(); ready != 0 || total != 0 {
		t.Errorf("Ready() = %d, %d", ready, total)
	}
}

func must(found []Anomaly, _ *Detector) []Anomaly {
	return found
}
