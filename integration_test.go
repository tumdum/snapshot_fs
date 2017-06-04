package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

const INPUT = "gutenberg/gutenberg.tar"

func mustReadFile(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return b
}

func findFile(root dir, name string) file {
	for _, f := range root.files() {
		if strings.Contains(f.name(), name) {
			return f
		}
	}
	for _, d := range root.dirs() {
		f := findFile(d, name)
		if f != nil {
			return f
		}
	}
	return nil
}

func TestMd5(t *testing.T) {
	b := mustReadFile(INPUT)
	dir := mustNewDirFromTar(b)
	expected, err := filepath.Glob("gutenberg/expected/*.txt")
	if err != nil {
		panic(err)
	}
	contents := map[string][]byte{}
	for _, p := range expected {
		name := path.Base(p)
		contents[name] = mustReadFile(p)
	}
	for name, content := range contents {
		f := findFile(dir, name)
		if f == nil {
			t.Fatalf("Did not found '%v'", name)
		}
		rc, err := f.readCloser()
		if err != nil {
			t.Fatalf("failed to read file '%v': %v", f.name(), err)
		}
		b, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Fatalf("failed to checksum file '%v': %v", f.name(), err)
		}
		if bytes.Compare(content, b) != 0 {
			t.Fatalf("sss")
		}
		rc.Close()
	}

}
