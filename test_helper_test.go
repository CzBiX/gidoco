package main

import (
	"io"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v6"
)

// touchFile creates a file at the given path inside the filesystem.
func touchFile(t *testing.T, fs billy.Filesystem, path string) {
	t.Helper()
	f, err := fs.Create(path)
	if err != nil {
		t.Fatalf("failed to create file %q: %v", path, err)
	}
	f.Close()
}

// mkdirAll creates a directory (and parents) inside the filesystem.
func mkdirAll(t *testing.T, fs billy.Filesystem, path string) {
	t.Helper()
	if err := fs.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("failed to mkdir %q: %v", path, err)
	}
}

// writeFile creates a file with the given content in the filesystem, creating
// parent directories as needed.
func writeFile(t *testing.T, fs billy.Filesystem, path string, content []byte) {
	t.Helper()
	if dir := parentDir(path); dir != "" {
		if err := fs.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %q: %v", dir, err)
		}
	}

	f, err := fs.Create(path)
	if err != nil {
		t.Fatalf("Create %q: %v", path, err)
	}

	if _, err := f.Write(content); err != nil {
		t.Fatalf("Write %q: %v", path, err)
	}
	f.Close()
}

// parentDir returns the directory component of a slash-separated path, or ""
// if there is no directory component.
func parentDir(p string) string {
	before, _, found := strings.Cut(p, "/")
	if !found {
		return ""
	}
	return before
}

func readFile(t *testing.T, fs billy.Filesystem, path string) []byte {
	t.Helper()
	f, err := fs.Open(path)
	if err != nil {
		t.Fatalf("Open %q: %v", path, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll %q: %v", path, err)
	}
	return data
}
