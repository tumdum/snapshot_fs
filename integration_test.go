package main

import (
	"crypto/md5"
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

func mustHashFile(path string) [md5.Size]byte {
	return md5.Sum(mustReadFile(path))
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
	checksums := map[string][md5.Size]byte{}
	for _, p := range expected {
		name := path.Base(p)
		checksums[name] = mustHashFile(p)
	}
	for name, chsum := range checksums {
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
		ch := md5.Sum(b)
		if ch != chsum {
		}
		rc.Close()
	}

}
