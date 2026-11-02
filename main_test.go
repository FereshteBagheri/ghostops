package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

func runCLI(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run 'go test -update' to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s; run 'go test -update' if the change is intended\n--- got:\n%s", path, got)
	}
}

func TestTextReports(t *testing.T) {
	for _, name := range []string{"sample", "anomaly", "empty"} {
		t.Run(name, func(t *testing.T) {
			code, out, errOut := runCLI(t, "", "testdata/"+name+".log")
			if code != exitOK {
				t.Fatalf("exit %d, stderr: %s", code, errOut)
			}
			checkGolden(t, name+".txt", []byte(out))
		})
	}
}

func TestJSONReport(t *testing.T) {
	for _, name := range []string{"sample", "empty"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.json")
			code, _, errOut := runCLI(t, "", "-o", path, "testdata/"+name+".log")
			if code != exitOK {
				t.Fatalf("exit %d, stderr: %s", code, errOut)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(data) {
				t.Fatal("report is not valid JSON")
			}
			checkGolden(t, name+".json", data)
		})
	}
}

func TestSameOutputEveryRun(t *testing.T) {
	_, first, _ := runCLI(t, "", "testdata/anomaly.log")
	for range 5 {
		if _, again, _ := runCLI(t, "", "testdata/anomaly.log"); again != first {
			t.Fatal("two runs gave different reports")
		}
	}
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name     string
		args     []string
		want     int
		wantErr  string
		wantNone bool // nothing on stdout
	}{
		{"no arguments", nil, exitUsage, "expected exactly 1 log file", true},
		{"two files", []string{"a.log", "b.log"}, exitUsage, "got 2", true},
		{"help", []string{"-h"}, exitOK, "usage: ghostops", true},
		{"unknown flag", []string{"-x", "testdata/sample.log"}, exitUsage, "flag provided but not defined", true},
		{"bad window", []string{"-window", "0", "testdata/sample.log"}, exitUsage, "-window must be positive", true},
		{"bad bucket", []string{"-bucket", "-1m", "testdata/sample.log"}, exitUsage, "-bucket must be positive", true},
		{"bad since", []string{"-since", "yesterday", "testdata/sample.log"}, exitUsage, "not an RFC3339 time", true},
		{"until before since", []string{"-since", "2026-10-20T09:00:00Z", "-until", "2026-10-20T08:00:00Z", "testdata/sample.log"}, exitUsage, "-until must be after -since", true},
		{"bad severity", []string{"-fail-on", "urgent", "testdata/sample.log"}, exitUsage, "unknown severity", true},
		{"missing file", []string{"testdata/nope.log"}, exitError, "no such file", true},
		{"directory", []string{"testdata"}, exitError, "is a directory", true},
		{"output dir missing", []string{"-o", filepath.Join(dir, "no/such/dir/r.json"), "testdata/sample.log"}, exitError, "write JSON report", false},
		{"output is a directory", []string{"-o", dir, "testdata/sample.log"}, exitError, "write JSON report", false},
		{"flags after file", []string{"testdata/sample.log", "-window", "30m"}, exitOK, "", false},
		{"fail-on hit", []string{"-fail-on", "critical", "testdata/sample.log"}, exitIncidents, "found a CRITICAL incident", false},
		{"fail-on not hit", []string{"-fail-on", "critical", "testdata/anomaly.log"}, exitOK, "", false},
		{"fail-on empty file", []string{"-fail-on", "low", "testdata/empty.log"}, exitOK, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errOut := runCLI(t, "", tt.args...)
			if code != tt.want {
				t.Errorf("exit %d, want %d; stderr: %s", code, tt.want, errOut)
			}
			if !strings.Contains(errOut, tt.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", errOut, tt.wantErr)
			}
			if tt.wantNone && out != "" {
				t.Errorf("stdout = %q, want nothing", out)
			}
		})
	}
}

func TestLongLineIsFatal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.log")
	os.WriteFile(path, []byte("2026-10-20T08:00:00Z INFO api ok\n"+strings.Repeat("x", 2<<20)+"\n"), 0o644)

	code, out, errOut := runCLI(t, "", path)
	if code != exitError || out != "" || !strings.Contains(errOut, "line 2: line too long") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

func TestStdin(t *testing.T) {
	data, _ := os.ReadFile("testdata/sample.log")
	code, out, _ := runCLI(t, string(data), "-")
	if code != exitOK || !strings.HasPrefix(out, "GhostOps report for standard input") {
		t.Errorf("exit %d, output starts %q", code, out[:min(60, len(out))])
	}
	if !strings.Contains(out, "41 valid") {
		t.Error("stdin gave a different result than the file")
	}
}

func TestGzip(t *testing.T) {
	data, _ := os.ReadFile("testdata/sample.log")
	path := filepath.Join(t.TempDir(), "sample.log.gz")
	f, _ := os.Create(path)
	zw := gzip.NewWriter(f)
	zw.Write(data)
	zw.Close()
	f.Close()

	code, out, errOut := runCLI(t, "", path)
	if code != exitOK || !strings.Contains(out, "41 valid") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}

	bad := filepath.Join(t.TempDir(), "bad.log.gz")
	os.WriteFile(bad, []byte("not gzip"), 0o644)
	if code, _, errOut := runCLI(t, "", bad); code != exitError || !strings.Contains(errOut, "gzip") {
		t.Errorf("bad gzip: exit %d, stderr %q", code, errOut)
	}
}

func TestFilters(t *testing.T) {
	code, out, _ := runCLI(t, "", "-service", "payment-service, auth-service", "-since", "2026-10-20T08:10:00Z", "testdata/sample.log")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"Only services: [payment-service auth-service]", "filtered out", "auth-failure"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q", want)
		}
	}
	if strings.Contains(out, "inventory-service") {
		t.Error("filtered service shows up in the report")
	}
}
