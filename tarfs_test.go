package main

import (
	"archive/tar"
	"bytes"
	"testing"
)

func makeTarFile(m map[string][]byte) []byte {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)

	for path, content := range m {
		header := tar.Header{
			Name: path,
			Mode: 0755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(&header); err != nil {
			panic(err)
		}
		if _, err := tw.Write(content); err != nil {
			panic(err)
		}
	}

	if err := tw.Flush(); err != nil {
		panic(err)
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func MustNewTarFs(b []byte) dir {
	r := bytes.NewReader(b)
	d, err := NewTarFs(r)
	if err != nil {
		panic(err)
	}
	return d
}

func TestTarFsOnEmpty(t *testing.T) {
	fs := MustNewTarFs(makeTarFile(nil))
	if dirs := fs.dirs(); len(dirs) != 0 {
		t.Fatalf("expected no dirs, found: '%v'", dirs)
	}
	if files := fs.files(); len(files) != 0 {
		t.Fatalf("expected no files, found: '%v'", files)
	}
	if dir := fs.findDir("test"); dir != nil {
		t.Fatalf("expected no dir named 'test', found one: %v", dir)
	}
	if file := fs.findFile("test"); file != nil {
		t.Fatalf("expected no file named 'test', found one: %v", file)
	}
}

func TestXX(t *testing.T) {
	fs := MustNewTarFs(makeTarFile(multiLevel))
	expected := map[string]struct{}{
		"a": {},
		"g": {},
	}
	if len(fs.dirs()) != len(expected) {
		t.Fatalf("Expected %d dirs, got %d", len(expected), len(fs.dirs()))
	}
	for _, d := range fs.dirs() {
		if _, ok := expected[d.name()]; !ok {
			t.Fatalf("Unexpected dir '%v'", d.name())
		}
	}
	expected = map[string]struct{}{
		"b": {},
		"e": {},
	}
	if len(fs.files()) != len(expected) {
		t.Fatalf("Expected %d files, got %d", len(expected), len(fs.files()))
	}
	for _, f := range fs.files() {
		if _, ok := expected[f.name()]; !ok {
			t.Fatalf("Unexpected file '%v'", f.name())
		}
	}

	d := recursiveFindDir(fs, "g/h/i")
	expected = map[string]struct{}{
		"j": {},
		"l": {},
	}
	if len(expected) != len(d.files()) {
		t.Fatalf("Expected %d files, got %d", len(expected), len(fs.files()))
	}
	for _, f := range d.files() {
		if _, ok := expected[f.name()]; !ok {
			t.Fatalf("Unexpected file '%v'", f.name())
		}
	}
}