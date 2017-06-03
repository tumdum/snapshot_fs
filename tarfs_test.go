package main

import (
	"archive/tar"
	"bytes"
	"strings"
	"testing"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

var (
	multiLevelWithTar = map[string][]byte{
		"a/d.tar": makeTarFile(multiLevel),
		"e":       []byte("f"),
	}
	multiLevelWithDirsTagged = map[string][]byte{
		"a":     []byte("dir"),
		"a/b":   []byte("c"),
		"d":     []byte("dir"),
		"d/e/f": []byte("dir"),
	}
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
			if strings.HasSuffix(path, ".gz") {
				buf = mustPackGzip(content)
			} else if strings.HasSuffix(path, ".xz") {
				buf = mustPackXz(content)
			} else if strings.HasSuffix(path, ".bz2") {
				buf = mustPackBzip(content)
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

func mustNewDirFromTar(b []byte) dir {
	r := bytes.NewReader(b)
	d, err := newDirFromTar(r)
	if err != nil {
		panic(err)
	}
	return d
}

func MustNewTarFs(b []byte) pathfs.FileSystem {
	r := bytes.NewReader(b)
	fs, err := newTarFs(r)
	if err != nil {
		panic(err)
	}
	return fs
}

func TestTarFsOnEmpty(t *testing.T) {
	fs := mustNewDirFromTar(makeTarFile(nil))
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
	fs := mustNewDirFromTar(makeTarFile(multiLevel))
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

func TestTarFsFilesAndTaggedDirs(t *testing.T) {
	fs := mustNewDirFromTar(makeTarFile(multiLevelWithDirsTagged))
	e := recursiveFindDir(fs, "d/e")
	dirs := e.dirs()
	expected := map[string]struct{}{"f": {}}
	if len(expected) != len(dirs) {
		t.Fatalf("Expected %d dirs, got %d: %v vs %v", len(expected), len(dirs), expected, dirs)
	}
	for _, dir := range dirs {
		if _, ok := expected[dir.name()]; !ok {
			t.Fatalf("Unexpected dir '%v'", dir.name())
		}
	}
}

func TestTarFsAllBytes(t *testing.T) {
	dir := mustNewDirFromTar(makeTarFile(multiLevel))
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
	dir := mustNewDirFromTar(makeTarFile(multiLevel))
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
	dir := mustNewDirFromTar(makeTarFile(multiLevelWithDirs))
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

func TestTarFsGetAttrOfTar(t *testing.T) {
	fs := MustNewTarFs(makeTarFile(multiLevelWithTar))
	attr, status := fs.GetAttr("a/d", &fuse.Context{})
	verifyStatus("a/d", status, t)
	if attr.Mode&fuse.S_IFDIR == 0 {
		t.Fatalf("'a/d' should be dir, but is not")
	}
	_, status = fs.GetAttr("a/d/b", &fuse.Context{})
	verifyStatus("a/d/b", status, t)
}

func TestTarFsGetAttrOfZip(t *testing.T) {
	fs := MustNewTarFs(makeTarFile(multiLevelWithZip))
	attr, status := fs.GetAttr("a/d", &fuse.Context{})
	verifyStatus("a/d", status, t)
	if attr.Mode&fuse.S_IFDIR == 0 {
		t.Fatalf("'a/d' should be dir, but is not")
	}
	_, status = fs.GetAttr("a/d/b", &fuse.Context{})
	verifyStatus("a/d/b", status, t)
}
