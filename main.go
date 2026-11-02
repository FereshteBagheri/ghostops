package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/FereshteBagheri/ghostops/analysis"
	"github.com/FereshteBagheri/ghostops/anomaly"
	"github.com/FereshteBagheri/ghostops/atomicfile"
	"github.com/FereshteBagheri/ghostops/detect"
	"github.com/FereshteBagheri/ghostops/report"
)

const (
	exitOK        = 0
	exitError     = 1
	exitUsage     = 2
	exitIncidents = 3
)

var errUsage = errors.New("usage error")

type config struct {
	logPath string
	outPath string
	failOn  detect.Severity
	opts    analysis.Options
}

func parseArgs(args []string, stderr io.Writer) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("ghostops", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&cfg.outPath, "o", "", "also write the report as JSON to `file`")
	window := fs.Duration("window", detect.DefaultWindow, "start a new incident after this much quiet `time`")
	bucket := fs.Duration("bucket", anomaly.DefaultConfig().Bucket, "bucket `size` for learning normal behaviour")
	services := fs.String("service", "", "only analyze these services (comma-separated `list`)")
	since := fs.String("since", "", "ignore entries before this RFC3339 `time`")
	until := fs.String("until", "", "ignore entries at or after this RFC3339 `time`")
	failOn := fs.String("fail-on", "", "exit with status 3 if an incident of this `severity` or higher is found (low, medium, high, critical)")

	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: ghostops [flags] <logfile>")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Analyzes <logfile> and prints a report. Use - to read standard input.")
		fmt.Fprintln(stderr, "Files ending in .gz are decompressed while reading.")
		fmt.Fprintln(stderr)
		fs.PrintDefaults()
	}

	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return cfg, err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}

	if len(positional) != 1 {
		return cfg, usageErrorf("expected exactly 1 log file, got %d", len(positional))
	}
	cfg.logPath = positional[0]

	if *window <= 0 {
		return cfg, usageErrorf("-window must be positive, got %s", *window)
	}
	if *bucket <= 0 {
		return cfg, usageErrorf("-bucket must be positive, got %s", *bucket)
	}

	cfg.opts = analysis.Options{
		Rules:   detect.DefaultRules(),
		Window:  *window,
		Anomaly: anomaly.Config{Bucket: *bucket},
	}
	for _, s := range strings.Split(*services, ",") {
		if s = strings.TrimSpace(s); s != "" {
			cfg.opts.Filter.Services = append(cfg.opts.Filter.Services, s)
		}
	}

	var err error
	if cfg.opts.Filter.Since, err = parseTime("-since", *since); err != nil {
		return cfg, err
	}
	if cfg.opts.Filter.Until, err = parseTime("-until", *until); err != nil {
		return cfg, err
	}
	f := cfg.opts.Filter
	if !f.Since.IsZero() && !f.Until.IsZero() && !f.Until.After(f.Since) {
		return cfg, usageErrorf("-until must be after -since")
	}

	if *failOn != "" {
		if cfg.failOn, err = detect.ParseSeverity(*failOn); err != nil {
			return cfg, usageErrorf("-fail-on: %v", err)
		}
	}
	return cfg, nil
}

func usageErrorf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errUsage}, args...)...)
}

func parseTime(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, usageErrorf("%s: %q is not an RFC3339 time like 2026-10-20T08:00:00Z", name, value)
	}
	return t.UTC(), nil
}

type gzipFile struct {
	*gzip.Reader
	file *os.File
}

func (g gzipFile) Close() error {
	err := g.Reader.Close()
	if cerr := g.file.Close(); err == nil {
		err = cerr
	}
	return err
}

func openLog(path string, stdin io.Reader) (io.ReadCloser, error) {
	if path == "-" {
		return io.NopCloser(stdin), nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		f.Close()
		return nil, fmt.Errorf("%s is a directory, not a log file", path)
	}

	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("%s is not a valid gzip file: %w", path, err)
		}
		return gzipFile{Reader: gz, file: f}, nil
	}
	return f, nil
}

func writeJSON(path string, r report.Report) error {
	return atomicfile.WriteFile(path, func(w io.Writer) error {
		return report.JSON{}.Render(w, r)
	})
}

func worstIncident(incidents []detect.Incident) detect.Severity {
	var worst detect.Severity
	for _, inc := range incidents {
		worst = max(worst, inc.Severity)
	}
	return worst
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := parseArgs(args, stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
		return exitOK
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "ghostops: %v\n", strings.TrimPrefix(err.Error(), errUsage.Error()+": "))
		fmt.Fprintln(stderr, "run 'ghostops -h' for help")
		return exitUsage
	case err != nil:
		return exitUsage
	}

	in, err := openLog(cfg.logPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "ghostops: %v\n", err)
		return exitError
	}
	defer in.Close()

	res, err := analysis.Analyze(in, cfg.opts)
	if err != nil {
		fmt.Fprintf(stderr, "ghostops: read %s: %v\n", cfg.logPath, err)
		return exitError
	}

	source := cfg.logPath
	if source == "-" {
		source = "standard input"
	}
	rep := res.Report(source)

	if err := (report.Text{}).Render(stdout, rep); err != nil {
		fmt.Fprintf(stderr, "ghostops: write report: %v\n", err)
		return exitError
	}
	if cfg.outPath != "" {
		if err := writeJSON(cfg.outPath, rep); err != nil {
			fmt.Fprintf(stderr, "ghostops: write JSON report: %v\n", err)
			return exitError
		}
		fmt.Fprintf(stderr, "JSON report written to %s\n", cfg.outPath)
	}

	if cfg.failOn > 0 {
		if worst := worstIncident(res.Detector.Incidents()); worst >= cfg.failOn {
			fmt.Fprintf(stderr, "ghostops: found a %s incident (-fail-on %s)\n", worst, cfg.failOn)
			return exitIncidents
		}
	}
	return exitOK
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
