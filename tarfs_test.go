package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"github.com/dsnet/compress/bzip2"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/ulikunitz/xz"
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

		isDir := string(content) == "dir"
		buf := content
		if isDir {
			header.Typeflag = tar.TypeDir
			header.Size = 0
		} else {
			var b bytes.Buffer
			if strings.HasSuffix(path, ".gz") {
				w := gzip.NewWriter(&b)
				if _, err := w.Write(content); err != nil {
					panic(err)
				}
				w.Close()
				buf = b.Bytes()
			} else if strings.HasSuffix(path, ".xz") {
				w, err := xz.NewWriter(&b)
				if err != nil {
					panic(err)
				}
				if _, err := w.Write(content); err != nil {
					panic(err)
				}
				w.Close()
				buf = b.Bytes()
			} else if strings.HasSuffix(path, ".bz2") {
				var b bytes.Buffer
				w, err := bzip2.NewWriter(&b, &bzip2.WriterConfig{Level: 3})
				if err != nil {
					panic(err)
				}
				if _, err := w.Write(content); err != nil {
					panic(err)
				}
				w.Close()
				buf = b.Bytes()
			}
			header.Size = int64(len(buf))
		}

		if err := tw.WriteHeader(&header); err != nil {
			panic(err)
		}

		if !isDir {
			if _, err := tw.Write(buf); err != nil {
				panic(err)
			}
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

func MustNewDirFromTar(b []byte) dir {
	r := bytes.NewReader(b)
	d, err := newDirFromTar(r)
	if err != nil {
		panic(err)
	}
	return d
}

func MustNewTarFs(b []byte) pathfs.FileSystem {
	r := bytes.NewReader(b)
	fs, err := NewTarFs(r)
	if err != nil {
		panic(err)
	}
	return fs
}

func TestTarFsOnEmpty(t *testing.T) {
	fs := MustNewDirFromTar(makeTarFile(nil))
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

func TestTarFsFilesAndDirs(t *testing.T) {
	fs := MustNewDirFromTar(makeTarFile(multiLevel))
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

func TestTarFsAllBytes(t *testing.T) {
	dir := MustNewDirFromTar(makeTarFile(multiLevel))
	for path, expected := range multiLevel {
		f := recursiveFindFile(dir, path)
		if f == nil {
			t.Fatalf("Did not find expected file '%v'", path)
		}
		content, err := allBytes(f)
		if err != nil {
			t.Fatalf("Failed to read '%v': %v", path, err)
		}
		if bytes.Compare(expected, content) != 0 {
			t.Fatalf("expected content '%v', got '%v'", expected, content)
		}
	}
}

func TestTarFsSize(t *testing.T) {
	dir := MustNewDirFromTar(makeTarFile(multiLevel))
	for path, expected := range multiLevel {
		f := recursiveFindFile(dir, path)
		if f == nil {
			t.Fatalf("Did not find expected file '%v'", path)
		}
		if got, err := f.size(); err != nil {
			t.Fatalf("Failed to get size: %v", err)
		} else if len(expected) != int(got) {
			t.Fatalf("For '%v' expected size %d, got %d", path, len(expected), got)
		}
	}
}

func TestTarDirs(t *testing.T) {
	dir := MustNewDirFromTar(makeTarFile(multiLevelWithDirs))
	expected := map[string]struct{}{
		"a": {},
		"d": {},
	}
	dirs := dir.dirs()
	if len(dirs) != len(expected) {
		t.Fatalf("Expected %d dirs, got %d: %v vs %v", len(expected), len(dirs), expected, dirs)
	}
	for _, d := range dirs {
		if _, ok := expected[d.name()]; !ok {
			t.Fatalf("Unexpected dir '%v'", d.name())
		}
	}
	files := dir.files()
	if len(files) > 0 {
		t.Fatalf("Unexpected files: '%v'", files)
	}

	d := recursiveFindDir(dir, "d")
	if len(d.dirs()) != 0 {
		t.Fatalf("Expected empty dir, got '%v'", d.dirs())
	}
	if len(d.files()) != 0 {
		t.Fatalf("Expected empty files, got '%v'", d.files())
	}
}
