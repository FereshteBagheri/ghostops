package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	err := WriteFile(path, func(w io.Writer) error {
		_, err := io.WriteString(w, "hello")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello" {
		t.Errorf("file = %q", got)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestFailedWriteKeepsOldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	os.WriteFile(path, []byte("old"), 0o644)

	boom := errors.New("boom")
	err := WriteFile(path, func(w io.Writer) error {
		io.WriteString(w, "half of the new")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Errorf("file = %q, want the old content", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}

func TestMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no", "such", "dir", "out.json")
	if err := WriteFile(path, func(io.Writer) error { return nil }); err == nil {
		t.Error("want an error for a missing directory")
	}
}

func TestPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "out"), 0o755)
	if err := WriteFile(filepath.Join(dir, "out"), func(io.Writer) error { return nil }); err == nil {
		t.Error("want an error when path is a directory")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temporary file left behind: %v", entries)
	}
}
