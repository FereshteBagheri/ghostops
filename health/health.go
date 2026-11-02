package health

import "math"

const (
	maxErrorPenalty    = 40
	maxIncidentPenalty = 45
	maxAnomalyPenalty  = 20
)

type Input struct {
	Entries int
	Errors  int

	Critical int
	High     int
	Medium   int
	Low      int

	Anomalies int
}

type Result struct {
	Score  int
	Status string

	ErrorPenalty    int
	IncidentPenalty int
	AnomalyPenalty  int
}

func Score(in Input) Result {
	var r Result
	if in.Entries > 0 {
		rate := float64(in.Errors) / float64(in.Entries)
		r.ErrorPenalty = min(maxErrorPenalty, int(math.Round(rate*200)))
	}
	r.IncidentPenalty = min(maxIncidentPenalty, 25*in.Critical+15*in.High+8*in.Medium+3*in.Low)
	r.AnomalyPenalty = min(maxAnomalyPenalty, 10*in.Anomalies)

	r.Score = max(0, 100-r.ErrorPenalty-r.IncidentPenalty-r.AnomalyPenalty)
	r.Status = Status(r.Score)
	return r
}

func Status(score int) string {
	switch {
	case score >= 90:
		return "healthy"
	case score >= 70:
		return "degraded"
	case score >= 40:
		return "unhealthy"
	default:
		return "critical"
	}
}
