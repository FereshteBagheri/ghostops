package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return 0, fmt.Errorf("%s is a directory, not a log file", path)
	}

	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
	}

	if err := scanner.Err(); err != nil {
		return n, fmt.Errorf("read %s at line %d: %w", path, n+1, err)
	}
	return n, nil
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

	n, err := countLines(logPath)
	if err != nil {
		fmt.Fprintf(stderr, "ghostops: %v\n", err)
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
