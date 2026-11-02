# GhostOps

[![CI](https://github.com/FereshteBagheri/ghostops/actions/workflows/ci.yml/badge.svg)](https://github.com/FereshteBagheri/ghostops/actions/workflows/ci.yml)

GhostOps is a command-line tool that reads an application log, counts what is in it,
finds incidents with a set of predefined rules and reports services that behave
differently from their own history. It reads the log one line at a time, so the
size of the file does not matter. Only the Go standard library is used.

## Background

GhostOps started as an assignment in a Go bootcamp. The brief asked for a CLI
that streams a log, counts entries by level and service, groups rule matches
into incidents, handles overlapping rules and malformed lines, and writes a
text and a JSON report.

Beyond the brief I added:

- anomaly detection that learns each service's normal rate online (Welford's
  algorithm) and reports error spikes, traffic spikes and silences
- a 0-100 health score per service that can be explained from its parts
- a timeline whose memory stays fixed however long the log is
- `-service`, `-since`, `-until` filters, `.gz` and stdin input, and
  `-fail-on` so the tool can fail a CI job
- JSON output that is never half-written, and an exit code for every kind of failure
- fuzz tests and benchmarks

## Highlights

- **Bounded memory.** O(n) time in the number of lines; memory depends only on
  the number of services, incident groups and anomalies. 20 million lines
  (1.3 GB) with no incidents peak at under 10 MB (see [Decisions](#decisions)).
- **Online statistics.** Mean and variance are updated per bucket without
  storing history, and abnormal buckets are kept out of the baseline.
- **Deterministic output.** Same input, same report, byte for byte.
- **Errors as values.** Bad lines are data and get counted; failures stop the
  program with no report, never a partial one.
- **Small interfaces.** Only where there is more than one implementation.
- **Tests.** Table-driven tests, golden files, fuzzing of the parser, the line
  scanner and the whole pipeline, and the race detector in CI.

## Quick start

Requires Go 1.23 or newer (see `go.mod`).

```bash
go install github.com/FereshteBagheri/ghostops@latest
```

or from a clone:

```bash
go build -o ghostops .
./ghostops testdata/sample.log
./ghostops -o report.json testdata/sample.log    # text on screen + JSON file
go test ./...
```

The start of the report for `testdata/sample.log` (full output in `docs/example-output.txt`):

```
GhostOps report for testdata/sample.log
Entries from 2026-10-20 08:00:00 to 08:35:00 UTC (35m)

Lines: 63 total, 41 valid, 6 malformed, 16 blank or comment

Service health (0-100, worst first)
  inventory-service   15  critical   9 entries   5 errors  55.6%  3 incidents  0 anomalies  (-40 errors, -45 incidents)
  payment-service     15  critical   12 entries  7 errors  58.3%  3 incidents  0 anomalies  (-40 errors, -45 incidents)
  auth-service        52  unhealthy  5 entries   1 error   20.0%  1 incident   0 anomalies  (-40 errors, -8 incidents)
```

## Usage

```
ghostops [flags] <logfile>
```

`<logfile>` can be a path, a `.gz` file (decompressed while reading) or `-` for
standard input. Flags may come before or after the file name.

| Flag | Default | Meaning |
|------|---------|---------|
| `-o file` | | also write the report as JSON to `file` |
| `-window d` | `5m` | an incident ends after this much time without a matching entry |
| `-bucket d` | `1m` | bucket size used to learn normal behaviour |
| `-service a,b` | all | only analyze these services |
| `-since t`, `-until t` | | only analyze entries in `[since, until)`, RFC3339 times |
| `-fail-on sev` | | exit with status 3 if an incident of `sev` (low, medium, high, critical) or worse is found |

Exit codes:

| Code | Meaning |
|------|---------|
| 0 | report printed (and written, with `-o`) |
| 1 | the analysis or the output failed: missing file, read error, line too long, JSON could not be written |
| 2 | wrong flags or arguments |
| 3 | `-fail-on` was given and a matching incident was found |

Errors always go to standard error, the report to standard output, so
`ghostops app.log > report.txt` never mixes the two.

Examples:

```bash
./ghostops -service payment-service -since 2026-10-20T08:20:00Z testdata/sample.log
zcat app.log.gz | ./ghostops -
./ghostops -fail-on high app.log || echo "something is on fire"
```

## Log format

```
<RFC3339 timestamp> <LEVEL> <service> <message>
2026-10-20T08:05:00Z ERROR payment-service Database connection refused: postgres:5432
```

- Levels are `INFO`, `WARN`, `ERROR` and `FATAL`, in upper case.
- Fields are separated by spaces or tabs; the message is everything after the
  service and keeps its own spacing.
- Times with an offset (`+03:30`) are converted to UTC; fractional seconds are fine.
- Empty lines and lines starting with `#` are skipped and counted separately.
- Any other line that does not fit is **malformed**: it is counted by reason
  (missing fields, bad timestamp, unknown level), the first five are shown with
  their line numbers, and the analysis goes on.

## What the report contains

- number of lines: valid, malformed, skipped, filtered out; time range of the entries
- a health score for every service (see below)
- entries per level (all four, even when zero) and per service
- malformed lines by reason, with examples
- incidents (see below)
- a timeline of entries and errors (see below)
- anomalies (see below)

`docs/example-output.txt` and `docs/example-report.json` are the text and JSON
reports for `testdata/sample.log`. `docs/example-anomalies.txt` shows the health,
timeline and anomaly sections for `testdata/anomaly.log`.

The report is the same on every run for the same input: everything is sorted
and there is no "generated at" time. Empty sections are `[]` in JSON, never
`null`, and an empty file still gives a complete report.

## Incidents

Every valid entry is checked against these rules, in this order:

| Rule | Severity | Matches |
|------|----------|---------|
| `service-crash` | CRITICAL | any FATAL entry |
| `out-of-memory` | CRITICAL | ERROR or worse, message has "out of memory" or "oom" |
| `database-failure` | HIGH | ERROR or worse, message has "database", "deadlock" or "connection refused" |
| `auth-failure` | MEDIUM | WARN or worse, message has "unauthorized", "authentication failed" or "permission denied" |
| `timeout` | MEDIUM | WARN or worse, message has "timeout" or "timed out" |
| `error-burst` | LOW | any other ERROR |

Keywords are matched without regard to case.

**Overlapping rules.** An entry like `ERROR ... Database query timed out` matches
three rules. GhostOps gives each entry to the **first** rule in the list that
matches, so every entry belongs to at most one incident and nothing is counted
twice. The report says how many entries matched more than one rule. Because
`error-burst` is a catch-all, every ERROR that a more specific rule takes is
counted there too.

**Grouping.** Matching entries are grouped by rule and service. A group ends when
`-window` (5 minutes) passes without another matching entry, so two separate
database outages in the same service become two incidents. Each incident has its
rule, severity, service, number of entries, first and last time, duration, the
line of its first entry and up to three different sample messages.

## Anomalies

Rules only find problems somebody thought of in advance. The anomaly detector
learns what is normal for each service, and for all services together, and
reports when that changes:

- `error-spike`: far more ERROR/FATAL entries than usual
- `volume-spike`: far more other entries than usual (for example a retry storm)
- `silence`: a service that normally logs at least 5 lines per bucket logs nothing for 3 buckets or more

How it works: time is split into buckets (`-bucket`, 1 minute by default). When a
bucket is finished its counts are compared with that service's earlier buckets
using the mean and standard deviation, kept with Welford's running algorithm, so no
old buckets are stored. A bucket is abnormal when the service has at least 10
buckets of history, the count is at least 5, and it is at least 3 standard
deviations above the mean. The standard deviation is never taken as less than
1, so a very steady service doesn't get a huge score for a small change.
Abnormal buckets are not added to the history, so a long outage does not become
"normal". Consecutive abnormal buckets are reported as one anomaly. A gap in the
log adds empty buckets to the history, at most 60 in a row; after that more
zeros don't tell anything new.

Limits: a service needs about 10 buckets of history before it is judged, so short
logs report "not enough history" instead of guessing. The log is expected to be
roughly in time order; entries that go back in time are counted in the current
bucket and the number is shown in the report. With a z-score of 3 over hundreds of
thousands of buckets, an occasional small spike (5 errors where 0-1 is normal)
will be reported.

`testdata/anomaly.log` is two hours of generated traffic with an error spike in
payment-service, a traffic spike in api-gateway and six silent minutes in
order-service. GhostOps finds exactly those (see `docs/example-anomalies.txt`).

## Service health

Every service gets a score from 0 to 100 so the worst one is easy to spot. It
starts at 100 and loses points in three ways, each with its own limit so that
one problem can't hide the others:

| Part | Points lost | Limit |
|------|-------------|-------|
| errors | error rate (ERROR and FATAL / all entries) × 200, so 20 % errors loses the full 40 | 40 |
| incidents | 25 per CRITICAL, 15 per HIGH, 8 per MEDIUM, 3 per LOW | 45 |
| anomalies | 10 per anomaly of that service | 20 |

Status: 90 and above is `healthy`, 70 `degraded`, 40 `unhealthy`, below 40
`critical`. Services are listed worst first and the report shows how many points
each part took, so a score can always be explained:

```
  payment-service     32  critical   1277 entries  85 errors   6.7%  9 incidents  1 anomaly    (-13 errors, -45 incidents, -10 anomalies)
  inventory-service   79  degraded   1216 entries  6 errors    0.5%  5 incidents  0 anomalies  (-1 errors, -20 incidents)
```

The weights are a judgement call, not a standard. They live in one small
function (`health.Score`) so they are easy to change.

## Timeline

The timeline shows when things happened, one character per time bucket. The
first row counts all entries, the other rows count errors per service:

```
Timeline from 08:00:00 to 10:00:00, one column = 2m (first row: all entries, then errors per service)
  (all entries)      ▆▆▆▅▆▅▅▅▆▆▅▆▅▆▅▆▆▅▅▅█▆▅▆▆▆▆▅▅▅▅▆▆▆▅█▇▆▅▅▆▆▅▆▅▆▅▅▄▄▅▆▆▅▅▆▅▆▆▅  max 129
  payment-service    ···········▁···········▁·▁····▁····█▄·····▁······▁····▁··▁·▁  max 50
```

- Every row is scaled to its own maximum, which is printed at the end. With a
  shared scale a busy service would flatten all the others.
- `·` means zero, so quiet stretches are visible.
- The timeline never has more than 60 columns. It starts with one-minute
  buckets; when the log runs past the last column, neighbouring buckets are
  added together and each column becomes twice as long. Memory stays the same
  for an hour or a year of logs, and all rows keep the same columns.
- A silent service shows up as a dip in the first row only, because the service
  rows count errors. The anomaly section reports silence explicitly.

## Design

```
main.go          command line: flags, opening the input, exit codes
linescan/        reads lines with line numbers and a maximum line length
logentry/        Entry, Level and Parse for one log line
detect/          Rule interface, the built-in rules, grouping into incidents
baseline/        running mean and standard deviation (Welford)
anomaly/         per-service buckets and anomaly detection
health/          the 0-100 health score
timeline/        counts over time with a fixed number of buckets
analysis/        runs a log through all of the above, builds the report
report/          the report model and the text and JSON renderers
atomicfile/      writes a file through a temporary file and a rename
tools/genlog/    generates test logs
```

Data flows in one direction:

```
input ─► linescan ─► logentry.Parse ─┬─ error ─► counted as malformed/skipped
                                     └─ entry ─► filter ─► Stats, detect.Detector, anomaly.Detector, timeline
end of input ─► analysis.Result.Report() ─► report.Text / report.JSON
```

`linescan`, `logentry`, `detect`, `baseline`, `health`, `timeline` and
`atomicfile` don't know anything about GhostOps and could be used by other programs. `analysis` is the glue.

### Decisions

- **Streaming.** Nothing keeps lines or entries. Memory depends on the number of
  services, incident groups and anomalies. Measured by piping generated logs
  through `ghostops -` (no file on disk):

  | Input | Lines | Incidents | Peak memory | Time |
  |-------|-------|-----------|-------------|------|
  | 13 MB | 200 000 | 0 | 8.4 MB | 0.3 s |
  | 134 MB | 2 000 000 | 0 | 9.3 MB | 2.5 s |
  | 1.3 GB | 20 000 000 | 0 | 9.7 MB | 23 s |
  | 1.3 GB | 20 000 000 | 102 000 | 120 MB | 29 s |

  The last row shows the cost of keeping incident groups (about 1 KB each), which
  have to stay in memory until the report is printed.

- **Bad lines versus failures.** A line that can't be parsed is data, not an
  error: it is counted and skipped. Problems that make the report wrong or
  incomplete stop the program with exit code 1 and no report: the file can't be
  opened, reading fails halfway, a line is longer than 1 MiB, or the output can't
  be written. `Analyze` returns either a full result or an error, never both.

- **Errors as values.** `logentry` returns a `*ParseError` with the line number
  that wraps one of four sentinel errors; `analysis` counts malformed lines by
  reason with `errors.Is`. `linescan` turns `bufio.ErrTooLong` into its own
  `ErrLineTooLong`, so callers don't depend on `bufio`.

- **Interfaces only where there is more than one implementation.**
  - `detect.Rule`: a new kind of rule doesn't need any change to the detector.
  - `report.Renderer`: text and JSON.
  - `analysis.Observer` (`Observe(logentry.Entry)`) is defined where it is used.
    `Stats`, `detect.Detector` and `anomaly.Detector` satisfy it without knowing it
    exists, and `Analyze` passes every entry to each of them. The timeline was
    added later with a four-line adapter (`errorTimeline`, which embeds
    `*timeline.Timeline`) and one more item in that list; the loop didn't change.
  - `io.Reader` and `io.Writer` everywhere else, which is what makes stdin, gzip
    and the tests easy.

  Stats and the parser have no interface because there is only one of each.

- **A separate report model.** `report.Report` has plain fields and JSON tags and
  is filled from the internal types in one place (`analysis/report.go`). Internal
  types can change without changing the JSON format.

- **No half-written files.** The JSON report is written to a temporary file in the
  same directory and renamed when complete. If writing fails, an older report with
  the same name is left alone.

- **Write errors are checked.** The text renderer remembers the first write error
  and returns it, so a full disk or a closed output ends with exit code 1 instead
  of a silent success.

- **No goroutines.** The work is reading a file in order; grouping and anomaly
  detection depend on that order. One goroutine already processes about 700 000
  lines per second.

## Tests

```bash
go test ./...                 # all packages
go test -race ./...           # with the race detector
go test . -update             # rewrite the golden files after an intended change
go test -run '^$' -fuzz FuzzParse ./logentry
go test -run '^$' -bench . -benchmem ./logentry ./analysis
```

- table-driven tests for parsing, rules, overlaps, grouping windows, filters
- golden files in `testdata/golden/` for the full text and JSON reports
- every exit code of the CLI, stdin, gzip, flags after the file name
- read errors in the middle of the input, lines that are too long, outputs that can't be written
- fuzz tests:
  - `FuzzParse`: any input is either a valid entry or a `*ParseError` that wraps exactly one sentinel
  - `FuzzScan`: the line scanner agrees with `strings.Split`
  - `FuzzAnalyze`: nothing panics, and every line is counted exactly once (valid, skipped, malformed or filtered)

Coverage is about 91 % of statements across packages (`go test -coverpkg=./... ./...`).

Benchmarks on an Intel i5-10300H:

| Benchmark | Result |
|-----------|--------|
| `BenchmarkParse` | 180 ns per line, 0 allocations |
| `BenchmarkAnalyze` (`testdata/anomaly.log`, 4 897 lines) | 2.8 ms, about 1.7 million lines/s, 120 MB/s |

`Analyze` is faster than the CLI figure above because it doesn't read from
disk or render a report.

Test logs: `testdata/sample.log` (hand-written, every rule, overlaps and malformed
lines), `testdata/anomaly.log` (generated with
`go run ./tools/genlog -minutes 120 -inject`), `testdata/empty.log`.

## Limitations and future work

- **Order.** Grouping and anomaly buckets assume the log is roughly in time
  order. Merged logs from several hosts would need a small reordering buffer
  first.
- **The anomaly model.** A z-score assumes counts are roughly normal. For
  low-rate series that is not true, so the detector needs a minimum count and
  a minimum standard deviation. A Poisson model, or a robust baseline (median
  and MAD) or an exponentially weighted mean that follows slow trends would fit
  better.
- **Rules are code.** The built-in rules are compiled in; loading them from a
  file would need no change to the detector, only a new `Rule` implementation.
- **One goroutine.** Parsing could be spread over several goroutines with the
  results put back in order, but at about 700 000 lines per second the program is
  usually limited by the disk.

## References

- B. P. Welford, "Note on a method for calculating corrected sums of squares
  and products", *Technometrics* 4(3), 1962, pp. 419-420.
- D. E. Knuth, *The Art of Computer Programming*, Vol. 2, 3rd ed., section 4.2.2.

## License

MIT, see [LICENSE](LICENSE).
