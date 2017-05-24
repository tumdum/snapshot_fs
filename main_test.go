package main

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

func addFileToZip(w *zip.Writer, path, content string) {
	f, err := w.Create(path)
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(f, strings.NewReader(content))
	if err != nil {
		panic(err)
	}
}

func verifyStatus(s fuse.Status, t *testing.T) {
	if !s.Ok() {
		t.Fatalf("Status not ok: %v", s)
	}
}

func makeZipFile(files map[string]string) []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)

	for path, content := range files {
		addFileToZip(w, path, content)
	}

	w.Flush()
	w.Close()
	return b.Bytes()
}

var (
	flatFile = map[string]string{
		"foo.txt": "foo.txt file content",
		"bar":     "bar file content",
		"empty":   "empty",
	}
	multiLevel = map[string]string{
		"a/b":     "c",
		"b":       "d",
		"e":       "f",
		"g/h/i/j": "k",
		"g/h/i/l": "m",
		"g/h/n":   "o",
	}
)

func MustNewZipFs(b []byte) pathfs.FileSystem {
	r := bytes.NewReader(b)
	fs, err := NewZipFs(r, int64(r.Len()))
	if err != nil {
		panic(err)
	}
	return fs
}

func TestZipFsOpenDirOnEmptyFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(nil))
	entries, status := fs.OpenDir("", &fuse.Context{})
	verifyStatus(status, t)

	if len(entries) != 0 {
		t.Fatalf("Expected 0 entries, got %d: %v", len(entries), entries)
	}
}

func TestZipFsOpenDirOnFlatFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(flatFile))
	entries, status := fs.OpenDir("", &fuse.Context{})
	verifyStatus(status, t)

	if len(entries) != len(flatFile) {
		t.Fatalf("Expected 3 entries, got %d: %v", len(entries), entries)
	}
	for _, entry := range entries {
		if _, ok := flatFile[entry.Name]; !ok {
			t.Fatalf("Found unexpected name '%v'", entry.Name)
		}
	}
}

func TestZipFsOpenDirOnFileInFlatFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(flatFile))
	entries, status := fs.OpenDir("empty", &fuse.Context{})
	if status.Ok() {
		t.Fatalf("Expected error status, found ok")
	}
	if len(entries) != 0 {
		t.Fatalf("Expected no entries, found '%v'", entries)
	}
}

func TestZipFsOpenDirOnMultiLevelFile(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(multiLevel))
	entries, status := fs.OpenDir("", &fuse.Context{})
	verifyStatus(status, t)

	expected := map[string]struct{}{
		"a": struct{}{},
		"b": struct{}{},
		"e": struct{}{},
		"g": struct{}{},
	}
	if len(entries) != len(expected) {
		t.Fatalf("Expected 4 entries, got %d: %v", len(entries), entries)
	}
	for _, entry := range entries {
		if _, ok := expected[entry.Name]; !ok {
			t.Fatalf("Found unexpected name '%v'", entry.Name)
		}
	}
}

func TestZipFsOpenDirOnMultiLevelFileSubdir(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(multiLevel))
	entries, status := fs.OpenDir("g/h", &fuse.Context{})
	verifyStatus(status, t)

	expected := map[string]struct{}{
		"i": struct{}{},
		"n": struct{}{},
	}

	for _, entry := range entries {
		if _, ok := expected[entry.Name]; !ok {
			t.Fatalf("Found unexpected name '%v'", entry.Name)
		}
	}
}

func TestZipFsOpenDirModeMultiLevel(t *testing.T) {
	fs := MustNewZipFs(makeZipFile(multiLevel))
	entries, _ := fs.OpenDir("", &fuse.Context{})
	for _, entry := range entries {
		_, isFile := multiLevel[entry.Name]
		if isFile && (entry.Mode&fuse.S_IFREG == 0) {
			t.Fatalf("File '%v' is not a file", entry.Name)
		} else if !isFile && (entry.Mode&fuse.S_IFDIR == 0) {
			t.Fatalf("Dir '%v' is not a dir", entry.Name)
		}
	}
	entries, _ = fs.OpenDir("g/h", &fuse.Context{})
	for _, entry := range entries {
		_, isFile := multiLevel[entry.Name]
		if isFile && (entry.Mode&fuse.S_IFREG == 0) {
			t.Fatalf("File '%v' is not a file", entry.Name)
		} else if !isFile && (entry.Mode&fuse.S_IFDIR == 0) {
			t.Fatalf("Dir '%v' is not a dir", entry.Name)
		}
	}
}
