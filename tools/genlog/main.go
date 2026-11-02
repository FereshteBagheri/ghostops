package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"
)

type line struct {
	at      time.Time
	level   string
	service string
	msg     string
}

type message struct {
	format string
	max    int
}

var messages = map[string][]message{
	"INFO": {
		{"GET /orders 200 %dms", 300},
		{"GET /health 200 %dms", 20},
		{"Order %d created", 99999},
		{"Payment %d authorized", 99999},
		{"Stock reserved for order %d", 99999},
		{"Cache hit ratio %d%%", 100},
	},
	"WARN": {
		{"Slow response from upstream: %dms", 5000},
		{"Retrying request, attempt %d", 5},
		{"Low stock for SKU %d", 9999},
	},
	"ERROR": {
		{"Failed to parse SKU %d", 9999},
		{"Upstream returned 502 Bad Gateway after %dms", 3000},
		{"Request timeout after %dms", 30000},
	},
}

func main() {
	minutes := flag.Int("minutes", 120, "how many minutes of logs to write")
	rate := flag.Int("rate", 10, "average lines per service per minute")
	start := flag.String("start", "2026-10-20T08:00:00Z", "time of the first line")
	seed := flag.Uint64("seed", 1, "random seed")
	services := flag.String("services", "api-gateway,order-service,payment-service,inventory-service", "comma-separated service names")
	inject := flag.Bool("inject", false, "add an error spike, a traffic spike and a silence")
	errorRate := flag.Int("errors", 6, "random ERROR lines per 1000 lines")
	flag.Parse()

	t0, err := time.Parse(time.RFC3339, *start)
	if err != nil {
		fmt.Fprintln(os.Stderr, "genlog: bad -start:", err)
		os.Exit(2)
	}
	if *minutes <= 0 || *rate <= 0 {
		fmt.Fprintln(os.Stderr, "genlog: -minutes and -rate must be positive")
		os.Exit(2)
	}

	rng := rand.New(rand.NewPCG(*seed, *seed))
	names := strings.Split(*services, ",")
	out := bufio.NewWriter(os.Stdout)

	var batch []line
	for m := range *minutes {
		minute := t0.Add(time.Duration(m) * time.Minute)
		batch = batch[:0]
		for _, svc := range names {
			n := *rate - 2 + rng.IntN(5)
			errors := 0
			if *inject {
				n, errors = injected(m, svc, n)
			}
			for range max(n, 0) {
				batch = append(batch, randomLine(rng, minute, svc, *errorRate))
			}
			for range errors {
				batch = append(batch, line{
					at:      minute.Add(time.Duration(rng.IntN(60_000)) * time.Millisecond),
					level:   "ERROR",
					service: svc,
					msg:     "Database connection refused: postgres:5432",
				})
			}
		}
		slices.SortStableFunc(batch, func(a, b line) int { return a.at.Compare(b.at) })
		for _, l := range batch {
			fmt.Fprintf(out, "%s %s %s %s\n", l.at.Format("2006-01-02T15:04:05.000Z07:00"), l.level, l.service, l.msg)
		}
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "genlog:", err)
		os.Exit(1)
	}
}

func injected(minute int, service string, n int) (lines, errors int) {
	switch {
	case service == "api-gateway" && minute == 40:
		return n * 6, 0
	case service == "payment-service" && minute >= 70 && minute < 73:
		return n, 25
	case service == "order-service" && minute >= 95 && minute < 101:
		return 0, 0
	}
	return n, 0
}

func randomLine(rng *rand.Rand, minute time.Time, service string, errorRate int) line {
	level := "INFO"
	switch p := rng.IntN(1000); {
	case p < errorRate:
		level = "ERROR"
	case p < errorRate+44:
		level = "WARN"
	}
	m := messages[level][rng.IntN(len(messages[level]))]
	return line{
		at:      minute.Add(time.Duration(rng.IntN(60_000)) * time.Millisecond),
		level:   level,
		service: service,
		msg:     fmt.Sprintf(m.format, 1+rng.IntN(m.max)),
	}
}
