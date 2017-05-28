package main

import (
	"bytes"
	"testing"
)

func mustDirFromZip(m map[string][]byte) dir {
	b := makeZipFileBytes(m)
	r := bytes.NewReader(b)
	d, err := NewDirFromZip(r, int64(len(b)))
	if err != nil {
		panic(err)
	}
	return d
}

func TestXXX(t *testing.T) {
	root := mustDirFromZip(multiLevelWithZip)
	dirs := root.Dirs()
	expectedDirs := map[string]struct{}{
		"a": {},
	}
	if len(expectedDirs) != len(dirs) {
		t.Fatalf("Expected '%v', got '%v'", expectedDirs, dirs)
	}
	d := recursiveFindDir(root, "a/d.zip")
	if d == nil {
		t.Fatalf("Failed to get 'a/d.zip' dir")
	}
	f := recursiveFindFile(root, "a/d.zip/g/h/i/j")
	if f == nil {
		t.Fatalf("Failed to get 'a/d.zip/g/h/i/j' file")
	}
	b, err := f.Bytes()
	if err != nil {
		t.Fatalf("Failed to open '%v': %v", f.Name(), err)
	}
	if string(b) != "k" {
		t.Fatalf("Incorrect content, expected \"k\", got '%v'", string(b))
	}
}
