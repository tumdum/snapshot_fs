package main

import (
	"bytes"
	"testing"
)

func mustDirFromZip(m map[string][]byte) dir {
	b := makeZipFileBytes(m)
	r := bytes.NewReader(b)
	d, err := newDirFromZip(r, int64(len(b)))
	if err != nil {
		panic(err)
	}
	return d
}

func TestDirApi(t *testing.T) {
	root := mustDirFromZip(multiLevelWithZip)
	dirs := root.dirs()
	expectedDirs := map[string]struct{}{
		"a": {},
	}
	if len(expectedDirs) != len(dirs) {
		t.Fatalf("Expected '%v', got '%v'", expectedDirs, dirs)
	}
	d := recursiveFindDir(root, "a/d")
	if d == nil {
		t.Fatalf("Failed to get 'a/d' dir")
	}
	f := recursiveFindFile(root, "a/d/g/h/i/j")
	if f == nil {
		t.Fatalf("Failed to get 'a/d/g/h/i/j' file")
	}
	b, err := allBytes(f)
	if err != nil {
		t.Fatalf("Failed to open '%v': %v", f.name(), err)
	}
	if string(b) != "k" {
		t.Fatalf("Incorrect content, expected \"k\", got '%v'", string(b))
	}
}
