// Package atomicfile writes files so that readers see either the old file or
// the complete new one, never a half-written file.
package atomicfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// WriteFile calls write with a temporary file in the same directory as path,
// then renames it to path. If anything fails, path is left untouched.
func WriteFile(path string, write func(w io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	if err := write(tmp); err != nil {
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
		return &os.PathError{Op: "write", Path: path, Err: errors.New("is a directory")}
	}
	return os.Rename(tmp.Name(), path)
}
