package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"ghostops/linescan"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func openLog(path string) (*os.File, error) {
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
	return f, nil
}

func countLines(r io.Reader) (int, error) {
	s := linescan.New(r, 0)
	for s.Scan() {
	}
	return s.Line(), s.Err()
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ghostops", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("o", "", "write the report as JSON to this `file`")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: ghostops [-o report.json] <logfile>")
		fmt.Fprintln(stderr, "\nFlags must come before the log file.")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	if fs.NArg() != 1 {
		fmt.Fprintf(stderr, "ghostops: expected exactly 1 log file, got %d\n", fs.NArg())
		fs.Usage()
		return exitUsage
	}
	logPath := fs.Arg(0)

	f, err := openLog(logPath)
	if err != nil {
		fmt.Fprintf(stderr, "ghostops: %v\n", err)
		return exitError
	}
	defer f.Close()

	n, err := countLines(f)
	if err != nil {
		fmt.Fprintf(stderr, "ghostops: read %s: %v\n", logPath, err)
		return exitError
	}

	fmt.Fprintf(stdout, "read %d lines from %s\n", n, logPath)
	if *outPath != "" {
		fmt.Fprintln(stderr, "I will implement it soon:)")
		return exitError
	}
	return exitOK
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
